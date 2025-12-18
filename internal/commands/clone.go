package commands

import (
	"fmt"
)

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

	// Resolve source volume name
	sourceVolume, err := c.ResolveVolumeName(opts.Service)
	if err != nil {
		return err
	}

	// Construct target volume name
	targetVolume := opts.NewName
	if c.ProjectName != "" {
		// Add project prefix if it doesn't have one
		if targetVolume[:len(c.ProjectName)] != c.ProjectName+"_" {
			targetVolume = c.ProjectName + "_" + targetVolume
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
		fmt.Printf("Warning: failed to update metadata: %v\n", err)
	}

	if !c.Quiet {
		fmt.Printf("✓ Clone complete: %s\n", targetVolume)
	}

	return nil
}
