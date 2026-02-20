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

func TestExpandEnvVars(t *testing.T) {
	t.Run("simpleVar", func(t *testing.T) {
		t.Setenv("TEST_VAR", "hello")
		if got := expandEnvVars("$TEST_VAR"); got != "hello" {
			t.Fatalf("expected hello, got %s", got)
		}
	})

	t.Run("bracedVar", func(t *testing.T) {
		t.Setenv("TEST_VAR", "hello")
		if got := expandEnvVars("${TEST_VAR}"); got != "hello" {
			t.Fatalf("expected hello, got %s", got)
		}
	})

	t.Run("bracedVarUnset", func(t *testing.T) {
		t.Setenv("TEST_VAR", "")
		os.Unsetenv("TEST_VAR")
		if got := expandEnvVars("${TEST_UNSET_VAR_12345}"); got != "" {
			t.Fatalf("expected empty, got %s", got)
		}
	})

	t.Run("colonDashDefault_unset", func(t *testing.T) {
		os.Unsetenv("TEST_UNSET_VAR_12345")
		if got := expandEnvVars("${TEST_UNSET_VAR_12345:-fallback}"); got != "fallback" {
			t.Fatalf("expected fallback, got %s", got)
		}
	})

	t.Run("colonDashDefault_empty", func(t *testing.T) {
		t.Setenv("TEST_VAR", "")
		if got := expandEnvVars("${TEST_VAR:-fallback}"); got != "fallback" {
			t.Fatalf("expected fallback, got %s", got)
		}
	})

	t.Run("colonDashDefault_set", func(t *testing.T) {
		t.Setenv("TEST_VAR", "value")
		if got := expandEnvVars("${TEST_VAR:-fallback}"); got != "value" {
			t.Fatalf("expected value, got %s", got)
		}
	})

	t.Run("dashDefault_unset", func(t *testing.T) {
		os.Unsetenv("TEST_UNSET_VAR_12345")
		if got := expandEnvVars("${TEST_UNSET_VAR_12345-fallback}"); got != "fallback" {
			t.Fatalf("expected fallback, got %s", got)
		}
	})

	t.Run("dashDefault_empty", func(t *testing.T) {
		t.Setenv("TEST_VAR", "")
		if got := expandEnvVars("${TEST_VAR-fallback}"); got != "" {
			t.Fatalf("expected empty (set but empty), got %s", got)
		}
	})

	t.Run("dashDefault_set", func(t *testing.T) {
		t.Setenv("TEST_VAR", "value")
		if got := expandEnvVars("${TEST_VAR-fallback}"); got != "value" {
			t.Fatalf("expected value, got %s", got)
		}
	})

	t.Run("noSubstitution", func(t *testing.T) {
		if got := expandEnvVars("plain-text"); got != "plain-text" {
			t.Fatalf("expected plain-text, got %s", got)
		}
	})

	t.Run("mixed", func(t *testing.T) {
		t.Setenv("APP_NAME", "myapp")
		if got := expandEnvVars("prefix-${APP_NAME}-suffix"); got != "prefix-myapp-suffix" {
			t.Fatalf("expected prefix-myapp-suffix, got %s", got)
		}
	})
}

func TestLoadComposeFileExpandsEnvVars(t *testing.T) {
	tmp := t.TempDir()
	composePath := filepath.Join(tmp, "compose.yaml")

	content := `name: ${TEST_COMPOSE_NAME:-my-project}
services:
  web:
    image: nginx
`

	if err := os.WriteFile(composePath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write compose file: %v", err)
	}

	t.Run("usesDefault", func(t *testing.T) {
		os.Unsetenv("TEST_COMPOSE_NAME")
		cf, err := LoadComposeFile(composePath)
		if err != nil {
			t.Fatalf("failed to load compose file: %v", err)
		}
		if got := cf.GetProjectName(""); got != "my-project" {
			t.Fatalf("expected my-project, got %s", got)
		}
	})

	t.Run("usesEnvValue", func(t *testing.T) {
		t.Setenv("TEST_COMPOSE_NAME", "custom-name")
		cf, err := LoadComposeFile(composePath)
		if err != nil {
			t.Fatalf("failed to load compose file: %v", err)
		}
		if got := cf.GetProjectName(""); got != "custom-name" {
			t.Fatalf("expected custom-name, got %s", got)
		}
	})
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
