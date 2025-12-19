package commands

import (
	"fmt"
	"regexp"
	"strings"
)

// volumeNamePattern defines valid characters for Docker volume names
var volumeNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// validateVolumeName validates that a volume name contains only allowed characters
func validateVolumeName(name string) error {
	if name == "" {
		return fmt.Errorf("volume name cannot be empty")
	}
	if len(name) > 255 {
		return fmt.Errorf("volume name too long (max 255 characters)")
	}
	// Explicitly reject dangerous names
	if name == "." || name == ".." {
		return fmt.Errorf("volume name %q is not allowed", name)
	}
	if !volumeNamePattern.MatchString(name) {
		return fmt.Errorf("volume name %q contains invalid characters (allowed: alphanumeric, underscore, hyphen, period; must start with alphanumeric)", name)
	}
	// Prevent path traversal attempts
	if strings.Contains(name, "..") {
		return fmt.Errorf("volume name %q contains invalid sequence '..'", name)
	}
	return nil
}

// CloneOptions contains options for clone command
type CloneOptions struct {
	Service string
	NewName string
}

// Clone clones a volume
func (c *Context) Clone(opts CloneOptions) error {
	if opts.Service == "" {
		return fmt.Errorf("service name is required")
	}

	if opts.NewName == "" {
		return fmt.Errorf("new name is required")
	}

	// Validate new name contains only allowed characters
	if err := validateVolumeName(opts.NewName); err != nil {
		return err
	}

	// Resolve source volume name
	sourceVolume, err := c.ResolveVolumeName(opts.Service)
	if err != nil {
		return err
	}

	// Construct target volume name
	targetVolume := opts.NewName
	if c.ProjectName != "" {
		// Add project prefix if it doesn't have one
		prefix := c.ProjectName + "_"
		if !strings.HasPrefix(targetVolume, prefix) {
			targetVolume = prefix + targetVolume
		}
	}

	// Check if target already exists
	if c.Docker.VolumeExists(targetVolume) {
		if !Confirm(fmt.Sprintf("Volume %s already exists. Overwrite?", targetVolume)) {
			return fmt.Errorf("clone cancelled")
		}

		// Delete existing volume
		if err := c.Docker.RemoveVolume(targetVolume, true); err != nil {
			return fmt.Errorf("failed to remove existing volume: %w", err)
		}
	}

	if !c.Quiet {
		fmt.Printf("Cloning %s to %s...\n", sourceVolume, targetVolume)
	}

	// Copy volume
	if err := c.Docker.CopyVolume(sourceVolume, targetVolume); err != nil {
		return fmt.Errorf("clone failed: %w", err)
	}

	// Update metadata
	if err := c.DB.UpdateLastAccessed(targetVolume); err != nil {
		return fmt.Errorf("clone completed but failed to update metadata for %s: %w", targetVolume, err)
	}

	if !c.Quiet {
		fmt.Printf("✓ Clone complete: %s\n", targetVolume)
	}

	return nil
}
