package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/koyashimano/docker-volume-manager/internal/database"
)

// SwapOptions contains options for swap command
type SwapOptions struct {
	Empty    bool
	NoBackup bool
	Restart  bool
	Service  string
	Source   string // backup file path or empty
}

// Swap swaps a volume with another
func (c *Context) Swap(opts SwapOptions) error {
	if opts.Service == "" {
		return fmt.Errorf("service name is required")
	}

	// Resolve volume name
	volumeName, err := c.ResolveVolumeName(opts.Service)
	if err != nil {
		return err
	}

	// Get service name
	serviceName := c.GetServiceName(volumeName)
	if serviceName == "" {
		serviceName = opts.Service
	}

	// Check if volume exists
	if !c.Docker.VolumeExists(volumeName) {
		return ErrVolumeNotFound
	}

	// Backup current volume unless --no-backup
	var backupPath string
	if !opts.NoBackup {
		backupDir := filepath.Join(c.Config.Paths.Backups, c.ProjectName)
		if err := EnsureDirectory(backupDir); err != nil {
			return fmt.Errorf("failed to create backup directory: %w", err)
		}

		filename := GenerateBackupFilename(serviceName+"_swap_backup", c.Config.Defaults.CompressFormat)
		backupPath = filepath.Join(backupDir, filename)

		if !c.Quiet {
			fmt.Printf("Backing up current volume to %s...\n", backupPath)
		}

		if err := c.Docker.BackupVolume(volumeName, backupPath, true); err != nil {
			return fmt.Errorf("backup failed: %w", err)
		}

		// Save backup record
		size, _ := GetFileSize(backupPath)
		checksum, _ := CalculateChecksum(backupPath)
		record := &database.BackupRecord{
			VolumeName:  volumeName,
			ServiceName: serviceName,
			ProjectName: c.ProjectName,
			FilePath:    backupPath,
			Size:        size,
			Tag:         "swap-backup",
			Checksum:    checksum,
		}
		if err := c.DB.AddBackupRecord(record); err != nil && !c.Quiet {
			fmt.Fprintf(os.Stderr, "Warning: failed to save swap backup record: %v\n", err)
		}
	}

	// Stop containers using the volume
	containers, _ := c.Docker.GetContainersUsingVolume(volumeName)
	containersStopped := false
	if len(containers) > 0 {
		if !c.Quiet {
			fmt.Printf("Stopping containers: %v\n", containers)
		}
		if err := c.Docker.StopContainersUsingVolume(volumeName); err != nil {
			return fmt.Errorf("failed to stop containers: %w", err)
		}
		containersStopped = true
	}

	// Helper function to restart containers on error
	restartOnError := func(err error) error {
		if containersStopped && len(containers) > 0 {
			if !c.Quiet {
				fmt.Fprintf(os.Stderr, "Error occurred, restarting containers...\n")
			}
			if restartErr := c.Docker.RestartContainersUsingVolume(volumeName); restartErr != nil {
				return fmt.Errorf("%w (also failed to restart containers: %v)", err, restartErr)
			}
		}
		return err
	}

	// Delete current volume
	if !c.Quiet {
		fmt.Printf("Removing current volume...\n")
	}

	if err := c.Docker.RemoveVolume(volumeName, true); err != nil {
		return restartOnError(fmt.Errorf("failed to remove volume: %w", err))
	}

	// Create new volume
	if !c.Quiet {
		fmt.Printf("Creating new volume...\n")
	}

	if err := c.Docker.CreateVolume(volumeName); err != nil {
		return restartOnError(fmt.Errorf("failed to create volume: %w", err))
	}

	// Restore from source if provided
	if opts.Source != "" && !opts.Empty {
		if !c.Quiet {
			fmt.Printf("Restoring from %s...\n", opts.Source)
		}

		if err := c.Docker.RestoreVolume(volumeName, opts.Source); err != nil {
			return restartOnError(fmt.Errorf("restore failed: %w", err))
		}
	}

	// Restart containers if requested
	if opts.Restart && len(containers) > 0 {
		if !c.Quiet {
			fmt.Printf("Restarting containers...\n")
		}

		for _, containerName := range containers {
			containerName = strings.TrimPrefix(containerName, "/")
			if c.Verbose {
				fmt.Printf("Starting %s\n", containerName)
			}
		}

		if err := c.Docker.RestartContainersUsingVolume(volumeName); err != nil {
			fmt.Printf("Warning: failed to restart some containers: %v\n", err)
		}
	}

	if !c.Quiet {
		if opts.Empty {
			fmt.Printf("✓ Swapped to empty volume: %s\n", volumeName)
		} else if opts.Source != "" {
			fmt.Printf("✓ Swapped to volume from: %s\n", opts.Source)
		}

		if !opts.NoBackup {
			fmt.Printf("Previous data backed up to: %s\n", backupPath)
		}
	}

	return nil
}
