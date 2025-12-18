package commands

import (
	"fmt"
	"path/filepath"

	"github.com/koyashimano/docker-volume-manager/internal/database"
)

// ArchiveOptions contains options for archive command
type ArchiveOptions struct {
	Output   string
	Verify   bool
	Force    bool
	Services []string
}

// Archive archives and deletes volumes
func (c *Context) Archive(opts ArchiveOptions) error {
	// Determine which volumes to archive
	var volumesToArchive []string

	if len(opts.Services) == 0 {
		// Archive all volumes in project
		if c.Compose == nil {
			return ErrComposeNotFound
		}

		volumesToArchive = c.Compose.GetAllFullVolumeNames(c.ProjectName)
		if len(volumesToArchive) == 0 {
			fmt.Println("No volumes found in project")
			return nil
		}
	} else {
		// Archive specific services
		for _, service := range opts.Services {
			volumeName, err := c.ResolveVolumeName(service)
			if err != nil {
				fmt.Printf("Warning: %s not found, skipping\n", service)
				continue
			}
			volumesToArchive = append(volumesToArchive, volumeName)
		}
	}

	if len(volumesToArchive) == 0 {
		fmt.Println("No volumes to archive")
		return nil
	}

	// Determine output directory
	outputDir := opts.Output
	if outputDir == "" {
		outputDir = filepath.Join(c.Config.Paths.Archives, c.ProjectName)
	}

	if err := EnsureDirectory(outputDir); err != nil {
		return fmt.Errorf("failed to create archive directory: %w", err)
	}

	// Confirm if not forced
	if !opts.Force {
		fmt.Printf("This will archive and DELETE the following volumes:\n")
		for _, vol := range volumesToArchive {
			fmt.Printf("  - %s\n", vol)
		}
		if !Confirm("Continue?") {
			return fmt.Errorf("archive cancelled")
		}
	}

	// Archive each volume
	for _, volumeName := range volumesToArchive {
		if err := c.archiveVolume(volumeName, outputDir, opts); err != nil {
			fmt.Printf("Error archiving %s: %v\n", volumeName, err)
			continue
		}
	}

	return nil
}

func (c *Context) archiveVolume(volumeName, outputDir string, opts ArchiveOptions) error {
	// Check if volume exists
	if !c.Docker.VolumeExists(volumeName) {
		return ErrVolumeNotFound
	}

	// Check if in use
	inUse, _ := c.Docker.IsVolumeInUse(volumeName)
	if inUse && !opts.Force {
		containers, _ := c.Docker.GetContainersUsingVolume(volumeName)
		return fmt.Errorf("volume is in use by: %v (use --force to archive anyway)", containers)
	}

	// Warn if force is being used on an in-use volume
	if inUse && opts.Force && !c.Quiet {
		fmt.Printf("Warning: volume %s is in use, but proceeding due to --force option\n", volumeName)
	}

	// Get service name
	serviceName := c.GetServiceName(volumeName)
	if serviceName == "" {
		serviceName = volumeName
	}

	// Generate filename
	filename := GenerateBackupFilename(serviceName, c.Config.Defaults.CompressFormat)
	archivePath := filepath.Join(outputDir, filename)

	if !c.Quiet {
		fmt.Printf("Archiving %s to %s...\n", volumeName, archivePath)
	}

	// Backup to archive location
	if err := c.Docker.BackupVolume(volumeName, archivePath, true); err != nil {
		return fmt.Errorf("archive backup failed: %w", err)
	}

	// Calculate checksum (reuse if verify was requested)
	var checksum string
	var err error

	// Verify if requested
	if opts.Verify {
		if !c.Quiet {
			fmt.Printf("Verifying archive integrity...\n")
		}

		checksum, err = CalculateChecksum(archivePath)
		if err != nil {
			return fmt.Errorf("checksum calculation failed: %w", err)
		}

		if c.Verbose {
			fmt.Printf("Checksum: %s\n", checksum)
		}
	} else {
		// Calculate checksum only if not already done
		checksum, _ = CalculateChecksum(archivePath)
	}

	// Get file size
	size, _ := GetFileSize(archivePath)

	// Save archive record
	record := &database.BackupRecord{
		VolumeName:  volumeName,
		ServiceName: serviceName,
		ProjectName: c.ProjectName,
		FilePath:    archivePath,
		Size:        size,
		Tag:         "archive",
		Checksum:    checksum,
	}

	if err := c.DB.AddBackupRecord(record); err != nil {
		fmt.Printf("Warning: failed to save archive record: %v\n", err)
	}

	// Delete volume
	if !c.Quiet {
		fmt.Printf("Deleting volume %s...\n", volumeName)
	}

	if err := c.Docker.RemoveVolume(volumeName, false); err != nil {
		return fmt.Errorf("failed to delete volume: %w", err)
	}

	if !c.Quiet {
		fmt.Printf("✓ Archived and deleted: %s (%s)\n", volumeName, FormatSize(size))
	}

	return nil
}
