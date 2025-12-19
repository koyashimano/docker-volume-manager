package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/koyashimano/docker-volume-manager/internal/database"
)

// BackupOptions contains options for backup command
type BackupOptions struct {
	Output      string
	Format      string
	NoCompress  bool
	Tag         string
	Stop        bool
	Services    []string
}

// Backup backs up volumes
func (c *Context) Backup(opts BackupOptions) error {
	// Determine which volumes to backup
	var volumesToBackup []string

	if len(opts.Services) == 0 {
		// Backup all volumes in project
		if c.Compose == nil {
			return ErrComposeNotFound
		}

		volumesToBackup = c.Compose.GetAllFullVolumeNames(c.ProjectName)
		if len(volumesToBackup) == 0 {
			fmt.Println("No volumes found in project")
			return nil
		}
	} else {
		// Backup specific services
		for _, service := range opts.Services {
			volumeName, err := c.ResolveVolumeName(service)
			if err != nil {
				fmt.Printf("Warning: %s not found, skipping\n", service)
				continue
			}
			volumesToBackup = append(volumesToBackup, volumeName)
		}
	}

	if len(volumesToBackup) == 0 {
		fmt.Println("No volumes to backup")
		return nil
	}

	// Determine output directory
	outputDir := opts.Output
	if outputDir == "" {
		outputDir = filepath.Join(c.Config.Paths.Backups, c.ProjectName)
	}

	if err := EnsureDirectory(outputDir); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Backup each volume
	for _, volumeName := range volumesToBackup {
		if err := c.backupVolume(volumeName, outputDir, opts); err != nil {
			fmt.Printf("Error backing up %s: %v\n", volumeName, err)
			continue
		}
	}

	return nil
}

func (c *Context) backupVolume(volumeName, outputDir string, opts BackupOptions) error {
	// Check if volume exists
	if !c.Docker.VolumeExists(volumeName) {
		return ErrVolumeNotFound
	}

	// Get service name for metadata
	serviceName := c.GetServiceName(volumeName)

	// Stop containers if requested
	if opts.Stop {
		if !c.Quiet {
			fmt.Printf("Stopping containers using %s...\n", volumeName)
		}
		if err := c.Docker.StopContainersUsingVolume(volumeName); err != nil {
			return fmt.Errorf("failed to stop containers: %w", err)
		}
	}

	// Generate filename using volume name (not service name)
	// This ensures uniqueness even when multiple services share the same volume
	format := opts.Format
	if format == "" {
		format = c.Config.Defaults.CompressFormat
	}

	filename := GenerateBackupFilename(volumeName, format)
	outputPath := filepath.Join(outputDir, filename)

	if !c.Quiet {
		fmt.Printf("Backing up %s to %s...\n", volumeName, outputPath)
	}

	// Perform backup
	compress := !opts.NoCompress && (format == "tar.gz" || format == "tar.zst")
	if err := c.Docker.BackupVolume(volumeName, outputPath, compress); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	// Get file size
	size, _ := GetFileSize(outputPath)

	// Calculate checksum
	checksum, _ := CalculateChecksum(outputPath)

	// Save backup record
	record := &database.BackupRecord{
		VolumeName:  volumeName,
		ServiceName: serviceName,
		ProjectName: c.ProjectName,
		FilePath:    outputPath,
		Size:        size,
		Tag:         opts.Tag,
		Checksum:    checksum,
	}

	if err := c.DB.AddBackupRecord(record); err != nil {
		return fmt.Errorf("backup completed but failed to save backup record: %w", err)
	}

	// Update metadata
	if err := c.DB.UpdateLastBackup(volumeName); err != nil {
		return fmt.Errorf("backup completed but failed to update metadata for volume %s: %w", volumeName, err)
	}

	if !c.Quiet {
		fmt.Printf("✓ Backup complete: %s (%s)\n", filename, FormatSize(size))
	}

	// Cleanup old backups
	keepGenerations := c.Config.Defaults.KeepGenerations
	if projectCfg, ok := c.Config.Projects[c.ProjectName]; ok && projectCfg.KeepGenerations > 0 {
		keepGenerations = projectCfg.KeepGenerations
	}

	if keepGenerations > 0 {
		if deleted, err := c.DB.CleanupOldBackups(volumeName, keepGenerations); err == nil && len(deleted) > 0 {
			// Delete the actual backup files from filesystem
			for _, record := range deleted {
				if err := os.Remove(record.FilePath); err != nil {
					if c.Verbose {
						fmt.Fprintf(os.Stderr, "Warning: failed to delete backup file %s: %v\n", record.FilePath, err)
					}
				}
			}
			if c.Verbose {
				fmt.Printf("Cleaned up %d old backup(s)\n", len(deleted))
			}
		}
	}

	return nil
}
