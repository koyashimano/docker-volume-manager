package commands

import (
	"fmt"
	"path/filepath"

	"github.com/docker-volume-manager/dvm/internal/database"
)

// CleanOptions contains options for clean command
type CleanOptions struct {
	Unused  bool
	Stale   int
	DryRun  bool
	Archive bool
	Force   bool
}

// Clean cleans up volumes
func (c *Context) Clean(opts CleanOptions) error {
	var volumesToClean []string

	// Get all volumes
	volumes, err := c.Docker.ListVolumes()
	if err != nil {
		return err
	}

	// Filter volumes to clean
	for _, vol := range volumes {
		shouldClean := false

		if opts.Unused {
			inUse, _ := c.Docker.IsVolumeInUse(vol.Name)
			if !inUse {
				shouldClean = true
			}
		}

		if opts.Stale > 0 {
			meta, _ := c.DB.GetVolumeMetadata(vol.Name)
			if meta != nil && !meta.LastAccessed.IsZero() {
				daysSince := int(meta.LastAccessed.Sub(meta.LastAccessed).Hours() / 24)
				if daysSince >= opts.Stale {
					shouldClean = true
				}
			}
		}

		if shouldClean {
			volumesToClean = append(volumesToClean, vol.Name)
		}
	}

	if len(volumesToClean) == 0 {
		if !c.Quiet {
			fmt.Println("No volumes to clean")
		}
		return nil
	}

	// Show what will be cleaned
	fmt.Printf("Volumes to clean (%d):\n", len(volumesToClean))
	for _, volumeName := range volumesToClean {
		meta, _ := c.DB.GetVolumeMetadata(volumeName)
		lastUsed := "never"
		if meta != nil && !meta.LastAccessed.IsZero() {
			lastUsed = FormatTimestamp(meta.LastAccessed)
		}

		fmt.Printf("  - %s (last used: %s)\n", volumeName, lastUsed)
	}

	if opts.DryRun {
		fmt.Println("\n(Dry run - no changes made)")
		return nil
	}

	// Confirm unless forced
	if !opts.Force {
		if !Confirm("\nProceed with cleanup?") {
			return fmt.Errorf("cleanup cancelled")
		}
	}

	// Archive if requested
	var archiveDir string
	if opts.Archive {
		archiveDir = filepath.Join(c.Config.Paths.Archives, "cleanup")
		if err := EnsureDirectory(archiveDir); err != nil {
			return fmt.Errorf("failed to create archive directory: %w", err)
		}
	}

	// Clean each volume
	for _, volumeName := range volumesToClean {
		if err := c.cleanVolume(volumeName, archiveDir); err != nil {
			fmt.Printf("Error cleaning %s: %v\n", volumeName, err)
			continue
		}
	}

	if !c.Quiet {
		fmt.Printf("\n✓ Cleaned %d volume(s)\n", len(volumesToClean))
	}

	return nil
}

func (c *Context) cleanVolume(volumeName, archiveDir string) error {
	// Archive if directory is provided
	if archiveDir != "" {
		if !c.Quiet {
			fmt.Printf("Archiving %s...\n", volumeName)
		}

		serviceName := c.GetServiceName(volumeName)
		if serviceName == "" {
			serviceName = volumeName
		}

		filename := GenerateBackupFilename(serviceName, c.Config.Defaults.CompressFormat)
		archivePath := filepath.Join(archiveDir, filename)

		if err := c.Docker.BackupVolume(volumeName, archivePath, true); err != nil {
			return fmt.Errorf("archive failed: %w", err)
		}

		// Save archive record
		size, _ := GetFileSize(archivePath)
		checksum, _ := CalculateChecksum(archivePath)
		record := &database.BackupRecord{
			VolumeName:  volumeName,
			ServiceName: serviceName,
			ProjectName: c.ProjectName,
			FilePath:    archivePath,
			Size:        size,
			Tag:         "cleanup-archive",
			Checksum:    checksum,
		}
		c.DB.AddBackupRecord(record)
	}

	// Delete volume
	if !c.Quiet {
		fmt.Printf("Deleting %s...\n", volumeName)
	}

	if err := c.Docker.RemoveVolume(volumeName, false); err != nil {
		return fmt.Errorf("failed to delete: %w", err)
	}

	return nil
}
