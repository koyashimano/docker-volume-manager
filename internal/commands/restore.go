package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

	// Build candidate names for lookup (service, full volume, short volume)
	seen := make(map[string]struct{})
	var searchNames []string
	addName := func(name string) {
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		searchNames = append(searchNames, name)
	}

	addName(serviceName)
	addName(svcName)
	addName(volumeName)

	if c.ProjectName != "" {
		prefix := c.ProjectName + "_"
		shortName := strings.TrimPrefix(volumeName, prefix)
		addName(shortName)
	}

	// Get backup directory
	backupDir := filepath.Join(c.Config.Paths.Backups, c.ProjectName)

	// List backups if requested
	if opts.List {
		return c.listBackups(backupDir, searchNames...)
	}

	// Select backup
	var backupFile string

	if opts.Select {
		backupFile, err = c.selectBackup(backupDir, searchNames...)
		if err != nil {
			return err
		}
	} else {
		// Use latest backup
		backupFile, err = FindBackupFile(backupDir, searchNames...)
		if err != nil {
			target := serviceName
			if target == "" && len(searchNames) > 0 {
				target = searchNames[0]
			}
			if target == "" {
				target = volumeName
			}
			return fmt.Errorf("no backup found for %s: %w", target, err)
		}
	}

	return c.restoreFromFile(backupFile, volumeName, opts)
}

func (c *Context) restoreFromFile(backupFile, volumeName string, opts RestoreOptions) error {
	// If volume name not specified, try to infer from backup filename
	if volumeName == "" {
		// Parse the filename to extract service name
		// Expected format: servicename_YYYYMMDD_HHMMSS.tar.gz
		// To handle service names with underscores, we look for a timestamp pattern
		baseName := filepath.Base(backupFile)

		// Remove extension(s)
		baseName = strings.TrimSuffix(baseName, filepath.Ext(baseName))
		if strings.HasSuffix(baseName, ".tar") {
			baseName = strings.TrimSuffix(baseName, ".tar")
		}

		// Try to find the last underscore followed by a timestamp (YYYYMMDD_HHMMSS format)
		parts := strings.Split(baseName, "_")
		if len(parts) < 3 {
			return fmt.Errorf("backup filename %q does not match expected format (service_YYYYMMDD_HHMMSS.tar.gz). Please specify volume name explicitly with --target", filepath.Base(backupFile))
		}

		// Join all parts except the last two (which should be date and time)
		// This assumes format: service_name_20060102_150405
		serviceName := strings.Join(parts[:len(parts)-2], "_")
		if serviceName == "" {
			return fmt.Errorf("could not extract service name from backup filename %q. Please specify volume name explicitly with --target", filepath.Base(backupFile))
		}

		var err error
		volumeName, err = c.ResolveVolumeName(serviceName)
		if err != nil {
			volumeName = c.ProjectName + "_" + serviceName
		}
	}

	if volumeName == "" {
		return fmt.Errorf("cannot determine volume name from backup file. Please specify volume name explicitly with --target")
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

func (c *Context) listBackups(backupDir string, names ...string) error {
	files, err := ListBackupFiles(backupDir, names...)
	if err != nil {
		return err
	}

	displayName := "volume"
	if len(names) > 0 && names[0] != "" {
		displayName = names[0]
	}

	if len(files) == 0 {
		fmt.Printf("No backups found for %s\n", displayName)
		return nil
	}

	fmt.Printf("Available backups for %s:\n", displayName)
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

func (c *Context) selectBackup(backupDir string, names ...string) (string, error) {
	files, err := ListBackupFiles(backupDir, names...)
	if err != nil {
		return "", err
	}

	displayName := "volume"
	if len(names) > 0 && names[0] != "" {
		displayName = names[0]
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no backups found for %s", displayName)
	}

	fmt.Printf("Available backups for %s:\n", displayName)
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
	if _, err := fmt.Scanln(&choice); err != nil {
		return "", fmt.Errorf("failed to read selection: %w", err)
	}

	idx, err := strconv.Atoi(choice)
	if err != nil || idx < 1 || idx > len(files) {
		return "", fmt.Errorf("invalid selection")
	}

	return files[idx-1], nil
}
