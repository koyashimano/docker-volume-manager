package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// RestoreOptions contains options for restore command
type RestoreOptions struct {
	Select  bool
	List    bool
	Force   bool
	Restart bool
	Target  string // service name or backup file path
}

// Restore restores volumes from backup
func (c *Context) Restore(opts RestoreOptions) error {
	// If no target specified, restore all volumes in project
	if opts.Target == "" {
		return c.restoreAll(opts)
	}

	// Check if target is a file path
	if _, err := os.Stat(opts.Target); err == nil {
		return c.restoreFromFile(opts.Target, "", opts)
	}

	// Otherwise, treat as service name
	return c.restoreService(opts.Target, opts)
}

func (c *Context) restoreAll(opts RestoreOptions) error {
	if c.Compose == nil {
		return ErrComposeNotFound
	}

	volumes := c.Compose.GetAllFullVolumeNames(c.ProjectName)
	if len(volumes) == 0 {
		fmt.Println("No volumes found in project")
		return nil
	}

	for _, volumeName := range volumes {
		serviceName := c.GetServiceName(volumeName)
		if err := c.restoreService(serviceName, opts); err != nil {
			fmt.Printf("Error restoring %s: %v\n", volumeName, err)
			continue
		}
	}

	return nil
}

func (c *Context) restoreService(serviceName string, opts RestoreOptions) error {
	// Resolve volume name
	volumeName, err := c.ResolveVolumeName(serviceName)
	if err != nil {
		volumeName = serviceName // Might be creating a new volume
	}

	// Get service name for backup lookup
	svcName := c.GetServiceName(volumeName)
	if svcName == "" {
		svcName = serviceName
	}

	// Get backup directory
	backupDir := filepath.Join(c.Config.Paths.Backups, c.ProjectName)

	// List backups if requested
	if opts.List {
		return c.listBackups(svcName, backupDir)
	}

	// Select backup
	var backupFile string

	if opts.Select {
		backupFile, err = c.selectBackup(svcName, backupDir)
		if err != nil {
			return err
		}
	} else {
		// Use latest backup
		backupFile, err = FindBackupFile(backupDir, svcName)
		if err != nil {
			return fmt.Errorf("no backup found for %s: %w", svcName, err)
		}
	}

	return c.restoreFromFile(backupFile, volumeName, opts)
}

func (c *Context) restoreFromFile(backupFile, volumeName string, opts RestoreOptions) error {
	// If volume name not specified, try to infer from backup filename
	if volumeName == "" {
		baseName := filepath.Base(backupFile)
		// Extract service name from filename (format: servicename_timestamp.tar.gz)
		parts := []rune(baseName)
		var serviceName string
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i] == '_' {
				serviceName = string(parts[:i])
				break
			}
		}

		if serviceName != "" {
			var err error
			volumeName, err = c.ResolveVolumeName(serviceName)
			if err != nil {
				volumeName = c.ProjectName + "_" + serviceName
			}
		}
	}

	if volumeName == "" {
		return fmt.Errorf("cannot determine volume name")
	}

	// Check if volume exists and is in use
	if c.Docker.VolumeExists(volumeName) {
		inUse, _ := c.Docker.IsVolumeInUse(volumeName)
		if inUse && !opts.Force {
			if !Confirm(fmt.Sprintf("Volume %s is in use. Continue?", volumeName)) {
				return fmt.Errorf("restore cancelled")
			}
		}

		// Confirm overwrite
		if !opts.Force {
			if !Confirm(fmt.Sprintf("This will overwrite %s. Continue?", volumeName)) {
				return fmt.Errorf("restore cancelled")
			}
		}
	}

	if !c.Quiet {
		fmt.Printf("Restoring %s from %s...\n", volumeName, backupFile)
	}

	// Perform restore
	if err := c.Docker.RestoreVolume(volumeName, backupFile); err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}

	// Update metadata
	if err := c.DB.UpdateLastAccessed(volumeName); err != nil {
		fmt.Printf("Warning: failed to update metadata: %v\n", err)
	}

	if !c.Quiet {
		fmt.Printf("✓ Restore complete: %s\n", volumeName)
	}

	// Restart containers if requested
	if opts.Restart {
		if !c.Quiet {
			fmt.Printf("Restarting containers using %s...\n", volumeName)
		}
		if err := c.Docker.RestartContainersUsingVolume(volumeName); err != nil {
			fmt.Printf("Warning: failed to restart containers: %v\n", err)
		}
	}

	return nil
}

func (c *Context) listBackups(serviceName, backupDir string) error {
	files, err := ListBackupFiles(backupDir, serviceName)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		fmt.Printf("No backups found for %s\n", serviceName)
		return nil
	}

	fmt.Printf("Available backups for %s:\n", serviceName)
	for i, file := range files {
		info, _ := os.Stat(file)
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		fmt.Printf("  %d. %s (%s)\n", i+1, filepath.Base(file), FormatSize(size))
	}

	return nil
}

func (c *Context) selectBackup(serviceName, backupDir string) (string, error) {
	files, err := ListBackupFiles(backupDir, serviceName)
	if err != nil {
		return "", err
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no backups found for %s", serviceName)
	}

	fmt.Printf("Available backups for %s:\n", serviceName)
	for i, file := range files {
		info, _ := os.Stat(file)
		size := int64(0)
		mtime := ""
		if info != nil {
			size = info.Size()
			mtime = info.ModTime().Format("2006-01-02 15:04:05")
		}
		fmt.Printf("  %d. %s (%s) - %s\n", i+1, filepath.Base(file), FormatSize(size), mtime)
	}

	fmt.Print("\nSelect backup number: ")
	var choice string
	fmt.Scanln(&choice)

	idx, err := strconv.Atoi(choice)
	if err != nil || idx < 1 || idx > len(files) {
		return "", fmt.Errorf("invalid selection")
	}

	return files[idx-1], nil
}
