package service

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"downloader/internal/models"
	"downloader/internal/utils"
)

func (s *Service) checkURL(rawURL string) error {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("invalid URL format")
	}

	timeout := 5 * time.Second
	client := http.Client{
		Timeout: timeout,
	}

	resp, err := client.Head(rawURL)
	if err != nil {
		return fmt.Errorf("URL not reachable")
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		return fmt.Errorf("empty content type")
	}

	mime := strings.Split(contentType, ";")[0]

	parts := strings.Split(mime, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid file extension")
	}
	ext := parts[1]

	if _, ok := s.cfg.Conditions.AllowedExtensionsMap[ext]; !ok {
		return fmt.Errorf("invalid file extension")
	}

	return nil
}

func (s *Service) taskProcessing(taskID string, urls []string) {
	var wg sync.WaitGroup
	downloadCh := make(chan string, len(urls))
	errorsCh := make(chan string, len(urls))
	var downloadedFiles, errors []string
	var archivePath string

	for _, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			filePath, err := utils.DownloadFile(url)
			if err != nil {
				s.log.Error("failed to download file",
					slog.String("taskID", taskID),
					slog.String("url", url),
					slog.String("error", err.Error()),
				)
				errorsCh <- fmt.Sprintf("failed to download file %s, error: %s", url, err.Error())
				return
			}
			downloadCh <- filePath
		}(url)
	}
	go func() {
		wg.Wait()
		close(downloadCh)
		close(errorsCh)
	}()

	for downloadCh != nil || errorsCh != nil {
		select {
		case filePath, ok := <-downloadCh:
			if !ok {
				downloadCh = nil
				continue
			}
			downloadedFiles = append(downloadedFiles, filePath)
		case err, ok := <-errorsCh:
			if !ok {
				errorsCh = nil
				continue
			}
			errors = append(errors, err)
		}
	}

	if len(downloadedFiles) > 0 {
		var err error
		archivePath, err = utils.CreateZipArchive(taskID, downloadedFiles)

		if err != nil {
			s.log.Error("failed to create zip archive",
				slog.String("taskID", taskID),
				slog.String("error", err.Error()))

			errors = append(errors, err.Error())
		}

		utils.CleanupFiles(downloadedFiles)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.tasks[taskID].Errors = errors

	if archivePath != "" {
		s.tasks[taskID].URLArchive = archivePath
		s.tasks[taskID].Status = models.StatusCompleted
	} else {
		s.tasks[taskID].Status = models.StatusFailed
	}
}
