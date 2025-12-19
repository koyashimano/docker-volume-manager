package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the global configuration
type Config struct {
	Defaults Defaults          `yaml:"defaults"`
	Paths    Paths             `yaml:"paths"`
	Projects map[string]Project `yaml:"projects,omitempty"`
}

// Defaults contains default settings
type Defaults struct {
	CompressFormat    string `yaml:"compress_format"`
	KeepGenerations   int    `yaml:"keep_generations"`
	StopBeforeBackup  bool   `yaml:"stop_before_backup"`
}

// Paths contains path settings
type Paths struct {
	Backups  string `yaml:"backups"`
	Archives string `yaml:"archives"`
}

// Project contains project-specific settings
type Project struct {
	KeepGenerations int `yaml:"keep_generations,omitempty"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		Defaults: Defaults{
			CompressFormat:   "tar.gz",
			KeepGenerations:  5,
			StopBeforeBackup: false,
		},
		Paths: Paths{
			Backups:  filepath.Join(home, ".dvm", "backups"),
			Archives: filepath.Join(home, ".dvm", "archives"),
		},
		Projects: make(map[string]Project),
	}
}

// Load loads configuration from a file
func Load(path string) (*Config, error) {
	// Expand ~ to home directory
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(home, path[1:])
	}

	// If file doesn't exist, return default config
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Expand ~ in paths
	cfg.Paths.Backups = expandPath(cfg.Paths.Backups)
	cfg.Paths.Archives = expandPath(cfg.Paths.Archives)

	return cfg, nil
}

// Save saves configuration to a file
func (c *Config) Save(path string) error {
	path = expandPath(path)

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// GetConfigPath returns the default config path
func GetConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dvm", "config.yaml")
}

// expandPath expands a leading "~" to the current user's home directory.
//
// If the home directory cannot be determined (e.g., user home directory unavailable),
// it logs a warning to stderr and returns the original path unchanged. Callers must
// be aware that a "~" prefix may remain unexpanded in this fallback case, which could
// cause file operations to fail with misleading errors later.
//
// Returns:
//   - The expanded path if successful
//   - The original path unchanged if expansion fails
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			// Log the error to stderr for debugging and fall back to the original path
			fmt.Fprintf(os.Stderr, "Warning: failed to expand path %q: %v\n", path, err)
			return path
		}
		if path == "~" {
			return home
		}
		return filepath.Join(home, path[1:])
	}
	return path
}

// EnsureDirectories ensures all necessary directories exist
func (c *Config) EnsureDirectories() error {
	dirs := []string{
		c.Paths.Backups,
		c.Paths.Archives,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}
