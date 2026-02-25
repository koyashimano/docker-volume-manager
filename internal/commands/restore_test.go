package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRestoreRejectsSelectWithBackupFile(t *testing.T) {
	tmp := t.TempDir()
	backupFile := filepath.Join(tmp, "db_20250101_120000.tar.gz")
	if err := os.WriteFile(backupFile, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("failed to create backup file: %v", err)
	}

	ctx := &Context{}
	err := ctx.Restore(RestoreOptions{
		Target:     "db",
		BackupFile: backupFile,
		Select:     true,
	})
	if err == nil {
		t.Fatal("expected error when --select is used with explicit backup file")
	}
	if !strings.Contains(err.Error(), "--select cannot be used") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestRestoreRejectsListWithBackupFile(t *testing.T) {
	tmp := t.TempDir()
	backupFile := filepath.Join(tmp, "db_20250101_120000.tar.gz")
	if err := os.WriteFile(backupFile, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("failed to create backup file: %v", err)
	}

	ctx := &Context{}
	err := ctx.Restore(RestoreOptions{
		Target:     "db",
		BackupFile: backupFile,
		List:       true,
	})
	if err == nil {
		t.Fatal("expected error when --list is used with explicit backup file")
	}
	if !strings.Contains(err.Error(), "--list cannot be used") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestRestoreRejectsFilePathAsServiceName(t *testing.T) {
	tmp := t.TempDir()

	// Create a file that will be used as the "service name" (first arg)
	targetFile := filepath.Join(tmp, "not_a_service.tar.gz")
	if err := os.WriteFile(targetFile, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	// Create the backup file (second arg)
	backupFile := filepath.Join(tmp, "db_20250101_120000.tar.gz")
	if err := os.WriteFile(backupFile, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("failed to create backup file: %v", err)
	}

	ctx := &Context{}
	err := ctx.Restore(RestoreOptions{
		Target:     targetFile,
		BackupFile: backupFile,
	})
	if err == nil {
		t.Fatal("expected error when target is a file path")
	}
	if !strings.Contains(err.Error(), "is a file path, not a service name") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestRestoreRejectsNonExistentBackupFile(t *testing.T) {
	ctx := &Context{}
	err := ctx.Restore(RestoreOptions{
		Target:     "db",
		BackupFile: "/nonexistent/path/backup.tar.gz",
	})
	if err == nil {
		t.Fatal("expected error when backup file does not exist")
	}
	if !strings.Contains(err.Error(), "backup file not found") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestRestoreRejectsDirectoryAsBackupFile(t *testing.T) {
	tmp := t.TempDir()
	dirPath := filepath.Join(tmp, "adir")
	if err := os.Mkdir(dirPath, 0o755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	ctx := &Context{}
	err := ctx.Restore(RestoreOptions{
		Target:     "db",
		BackupFile: dirPath,
	})
	if err == nil {
		t.Fatal("expected error when backup path is a directory")
	}
	if !strings.Contains(err.Error(), "backup path is not a regular file") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
