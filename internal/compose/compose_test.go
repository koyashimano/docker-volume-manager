package compose

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetProjectNameNormalizesDirectoryName(t *testing.T) {
	cf := ComposeFile{
		path: filepath.Join("/tmp", "MyProject", "compose.yaml"),
	}

	if got := cf.GetProjectName(""); got != "myproject" {
		t.Fatalf("expected directory-based project name to be normalized, got %s", got)
	}
}

func TestGetProjectNameUsesCurrentWorkingDirectory(t *testing.T) {
	tmp := t.TempDir()
	upperDir := filepath.Join(tmp, "CapsProject")
	if err := os.Mkdir(upperDir, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}

	if err := os.Chdir(upperDir); err != nil {
		t.Fatalf("failed to change working dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	cf := ComposeFile{path: "compose.yaml"}
	if got := cf.GetProjectName(""); got != "capsproject" {
		t.Fatalf("expected cwd-based project name to be normalized, got %s", got)
	}
}

func TestGetProjectNamePriorityAndNormalization(t *testing.T) {
	basePath := filepath.Join("/tmp", "PriorityProject", "compose.yaml")

	t.Run("overrideHasHighestPriority", func(t *testing.T) {
		cf := ComposeFile{
			Name: "fromfile",
			path: basePath,
		}
		t.Setenv("COMPOSE_PROJECT_NAME", "envName")

		if got := cf.GetProjectName("OverrideName"); got != "overridename" {
			t.Fatalf("expected override to be used and normalized, got %s", got)
		}
	})

	t.Run("nameFieldBeatsEnv", func(t *testing.T) {
		cf := ComposeFile{
			Name: "FromFile",
			path: basePath,
		}
		t.Setenv("COMPOSE_PROJECT_NAME", "envName")

		if got := cf.GetProjectName(""); got != "fromfile" {
			t.Fatalf("expected name field to be used and normalized, got %s", got)
		}
	})

	t.Run("envBeatsDirectory", func(t *testing.T) {
		cf := ComposeFile{
			path: basePath,
		}
		t.Setenv("COMPOSE_PROJECT_NAME", "EnvName")

		if got := cf.GetProjectName(""); got != "envname" {
			t.Fatalf("expected env project name to be used and normalized, got %s", got)
		}
	})
}
