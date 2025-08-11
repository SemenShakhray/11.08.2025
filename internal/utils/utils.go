package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxSize = 50 << 20

func DownloadFile(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download file: status code %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	mime := strings.Split(contentType, ";")[0]
	ext := strings.Split(mime, "/")[1]

	fileName := generateFileName(ext)
	filePath := filepath.Join(os.TempDir(), fileName)

	out, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %v", err)
	}
	defer out.Close()

	limitedReader := io.LimitReader(resp.Body, maxSize)

	_, err = io.Copy(out, limitedReader)
	if err != nil {
		return "", fmt.Errorf("failed to save file: %v", err)
	}

	return filePath, nil
}

func CreateZipArchive(taskID string, files []string) (string, error) {
	archivePath := filepath.Join(os.TempDir(), taskID+".zip")
	archive, err := os.Create(archivePath)
	if err != nil {
		return "", err
	}
	defer archive.Close()

	zipWriter := zip.NewWriter(archive)
	defer zipWriter.Close()

	for _, file := range files {
		err := addFileToZip(zipWriter, file)
		if err != nil {
			return "", err
		}
	}

	archiveURL := fmt.Sprintf("http://localhost:8080/archives/%s.zip", taskID)

	return archiveURL, nil
}

func addFileToZip(zipWriter *zip.Writer, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}

	header.Name = filepath.Base(filePath)
	header.Method = zip.Deflate

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, file)
	return err
}

func CleanupFiles(files []string) {
	for _, file := range files {
		os.Remove(file)
	}
}

func generateFileName(ext string) string {
	return fmt.Sprintf("file-%d.%s", time.Now().UnixNano(), ext)
}
