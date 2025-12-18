package commands

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FormatSize formats a size in bytes to human-readable format
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatTimestamp formats a timestamp
func FormatTimestamp(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04:05")
}

// GenerateBackupFilename generates a backup filename
func GenerateBackupFilename(serviceName, format string) string {
	timestamp := time.Now().Format("2006-01-02_150405")
	extension := ".tar.gz"

	if format == "tar.zst" {
		extension = ".tar.zst"
	} else if format == "tar" {
		extension = ".tar"
	}

	return fmt.Sprintf("%s_%s%s", serviceName, timestamp, extension)
}

// GetFileSize returns the size of a file
func GetFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// CalculateChecksum calculates SHA256 checksum of a file
func CalculateChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// Confirm asks user for confirmation
func Confirm(prompt string) bool {
	fmt.Printf("%s [y/N]: ", prompt)
	var response string
	fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

// FindBackupFile finds the latest backup file for a service
func FindBackupFile(backupDir, serviceName string) (string, error) {
	pattern := filepath.Join(backupDir, fmt.Sprintf("%s_*.tar.gz", serviceName))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}

	if len(matches) == 0 {
		// Try tar.zst
		pattern = filepath.Join(backupDir, fmt.Sprintf("%s_*.tar.zst", serviceName))
		matches, err = filepath.Glob(pattern)
		if err != nil {
			return "", err
		}
	}

	if len(matches) == 0 {
		return "", ErrBackupNotFound
	}

	// Return the most recent file
	var latest string
	var latestTime time.Time

	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}

		if latest == "" || info.ModTime().After(latestTime) {
			latest = match
			latestTime = info.ModTime()
		}
	}

	if latest == "" {
		return "", ErrBackupNotFound
	}

	return latest, nil
}

// ListBackupFiles lists all backup files for a service
func ListBackupFiles(backupDir, serviceName string) ([]string, error) {
	patterns := []string{
		filepath.Join(backupDir, fmt.Sprintf("%s_*.tar.gz", serviceName)),
		filepath.Join(backupDir, fmt.Sprintf("%s_*.tar.zst", serviceName)),
	}

	var all []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		all = append(all, matches...)
	}

	return all, nil
}

// EnsureDirectory ensures a directory exists
func EnsureDirectory(path string) error {
	return os.MkdirAll(path, 0755)
}

// CopyFile copies a file from src to dst
func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// MoveFile moves a file from src to dst
func MoveFile(src, dst string) error {
	// Try rename first (faster if on same filesystem)
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// Fall back to copy + delete
	if err := CopyFile(src, dst); err != nil {
		return err
	}

	return os.Remove(src)
}
