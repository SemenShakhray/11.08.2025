package main

import (
    "bufio"
    "context"
    "errors"
    "flag"
    "fmt"
    "math"
    "os"
    "strconv"
    "strings"
    "sync"
    "time"
)

type numericRange struct {
    start int
    end   int
}

type rangeSlice []numericRange

func (r *rangeSlice) String() string {
    parts := make([]string, 0, len(*r))
    for _, nr := range *r {
        parts = append(parts, fmt.Sprintf("%d:%d", nr.start, nr.end))
    }
    return strings.Join(parts, ",")
}

func (r *rangeSlice) Set(value string) error {
    // expected format: start:end
    pieces := strings.Split(value, ":")
    if len(pieces) != 2 {
        return fmt.Errorf("invalid range %q, expected format start:end", value)
    }
    start, err := strconv.Atoi(strings.TrimSpace(pieces[0]))
    if err != nil {
        return fmt.Errorf("invalid start in range %q: %w", value, err)
    }
    end, err := strconv.Atoi(strings.TrimSpace(pieces[1]))
    if err != nil {
        return fmt.Errorf("invalid end in range %q: %w", value, err)
    }
    if end < start {
        return fmt.Errorf("invalid range %q: end < start", value)
    }
    *r = append(*r, numericRange{start: start, end: end})
    return nil
}

func parseFlags() (outputFile string, timeoutSeconds int, ranges rangeSlice, err error) {
    var (
        fileFlag    string
        timeoutFlag int
        rangesFlag  rangeSlice
    )

    flag.StringVar(&fileFlag, "file", "", "output file to write found prime numbers")
    flag.IntVar(&timeoutFlag, "timeout", 0, "timeout in seconds before program should terminate (required)")
    flag.Var(&rangesFlag, "range", "numeric range in the form start:end; can be specified multiple times")
    flag.Usage = func() {
        fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s --file <filename> --timeout <seconds> --range a:b [--range c:d ...]\n", os.Args[0])
        fmt.Fprintln(flag.CommandLine.Output(), "\nExample:")
        fmt.Fprintln(flag.CommandLine.Output(), "  find_primes --file testfile.txt --timeout 10 --range 1:10 --range 200000:3000000 --range 400:500")
        fmt.Fprintln(flag.CommandLine.Output(), "\nNotes:")
        fmt.Fprintln(flag.CommandLine.Output(), "  - Each --range is processed in its own goroutine")
        fmt.Fprintln(flag.CommandLine.Output(), "  - Writing is handled by a dedicated goroutine")
        fmt.Fprintln(flag.CommandLine.Output(), "  - Goroutines communicate via channels; context controls timeout")
    }
    flag.Parse()

    if fileFlag == "" {
        return "", 0, nil, errors.New("--file is required")
    }
    if timeoutFlag <= 0 {
        return "", 0, nil, errors.New("--timeout must be a positive integer")
    }
    if len(rangesFlag) == 0 {
        return "", 0, nil, errors.New("at least one --range must be provided")
    }
    return fileFlag, timeoutFlag, rangesFlag, nil
}

func isPrime(n int) bool {
    if n < 2 {
        return false
    }
    if n == 2 {
        return true
    }
    if n%2 == 0 {
        return false
    }
    limit := int(math.Sqrt(float64(n)))
    for d := 3; d <= limit; d += 2 {
        if n%d == 0 {
            return false
        }
    }
    return true
}

func workerFindPrimes(ctx context.Context, nr numericRange, primes chan<- int, wg *sync.WaitGroup) {
    defer wg.Done()
    for n := nr.start; n <= nr.end; n++ {
        if ctx.Err() != nil {
            return
        }
        if isPrime(n) {
            select {
            case primes <- n:
            case <-ctx.Done():
                return
            }
        }
    }
}

func writer(ctx context.Context, outputPath string, primes <-chan int) error {
    f, err := os.Create(outputPath)
    if err != nil {
        return err
    }
    defer func() {
        _ = f.Close()
    }()
    buf := bufio.NewWriterSize(f, 1<<20) // 1 MiB buffer for faster writes
    defer func() {
        _ = buf.Flush()
    }()

    for {
        select {
        case n, ok := <-primes:
            if !ok {
                return nil
            }
            if _, err := buf.WriteString(strconv.Itoa(n)); err != nil {
                return err
            }
            if err := buf.WriteByte('\n'); err != nil {
                return err
            }
        case <-ctx.Done():
            // Respect timeout/cancellation promptly
            return ctx.Err()
        }
    }
}

func main() {
    outputFile, timeoutSeconds, ranges, err := parseFlags()
    if err != nil {
        fmt.Fprintln(os.Stderr, "Error:", err)
        flag.Usage()
        os.Exit(2)
    }

    ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
    defer cancel()

    primes := make(chan int, 1024)

    var wg sync.WaitGroup
    wg.Add(len(ranges))
    for _, nr := range ranges {
        r := nr
        go workerFindPrimes(ctx, r, primes, &wg)
    }

    // Close the primes channel after all workers complete
    go func() {
        wg.Wait()
        close(primes)
    }()

    writerErrCh := make(chan error, 1)
    go func() {
        writerErrCh <- writer(ctx, outputFile, primes)
    }()

    // Wait for either timeout/cancel or writer completion
    select {
    case err := <-writerErrCh:
        if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
            fmt.Fprintln(os.Stderr, "Write error:", err)
            os.Exit(1)
        }
        // Normal completion or early exit due to cancellation
    case <-ctx.Done():
        // Inform about timeout, then wait for writer to finish promptly
        fmt.Fprintln(os.Stderr, "Timeout reached, stopping...")
        <-writerErrCh
    }
}

