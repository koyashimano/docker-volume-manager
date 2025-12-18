package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/docker/docker/api/types/volume"
	"github.com/koyashimano/docker-volume-manager/internal/database"
)

// InspectOptions contains options for inspect command
type InspectOptions struct {
	Files   bool
	Top     int
	Format  string
	Service string
}

// Inspect shows detailed information about a volume
func (c *Context) Inspect(opts InspectOptions) error {
	if opts.Service == "" {
		return fmt.Errorf("service name is required")
	}

	// Resolve volume name
	volumeName, err := c.ResolveVolumeName(opts.Service)
	if err != nil {
		return err
	}

	// Get volume info
	vol, err := c.Docker.GetVolume(volumeName)
	if err != nil {
		return err
	}

	// Get metadata
	meta, _ := c.DB.GetVolumeMetadata(volumeName)

	// Get in-use status
	inUse, _ := c.Docker.IsVolumeInUse(volumeName)
	containers, _ := c.Docker.GetContainersUsingVolume(volumeName)

	// Format output
	switch opts.Format {
	case "json":
		return c.inspectJSON(vol, meta, inUse, containers)
	case "yaml":
		return c.inspectYAML(vol, meta, inUse, containers)
	default:
		return c.inspectTable(vol, meta, inUse, containers)
	}
}

func (c *Context) inspectTable(vol interface{}, meta interface{}, inUse bool, containers []string) error {
	volData, _ := vol.(*volume.Volume)
	metaData, _ := meta.(*database.VolumeMetadata)

	fmt.Printf("Volume: %s\n", volData.Name)
	fmt.Printf("Driver: %s\n", volData.Driver)
	fmt.Printf("Mountpoint: %s\n", volData.Mountpoint)
	fmt.Printf("Created: %s\n", volData.CreatedAt)
	fmt.Printf("Status: %s\n", map[bool]string{true: "in-use", false: "unused"}[inUse])

	if len(containers) > 0 {
		fmt.Printf("Used by: %v\n", containers)
	}

	if metaData != nil {
		if !metaData.LastAccessed.IsZero() {
			fmt.Printf("Last accessed: %s\n", FormatTimestamp(metaData.LastAccessed))
		}
		if !metaData.LastBackup.IsZero() {
			fmt.Printf("Last backup: %s\n", FormatTimestamp(metaData.LastBackup))
		}
		fmt.Printf("Backup count: %d\n", metaData.BackupCount)
	}

	return nil
}

func (c *Context) inspectJSON(vol interface{}, meta interface{}, inUse bool, containers []string) error {
	volData, _ := vol.(*volume.Volume)
	metaData, _ := meta.(*database.VolumeMetadata)

	data := map[string]interface{}{
		"name":       volData.Name,
		"driver":     volData.Driver,
		"mountpoint": volData.Mountpoint,
		"created":    volData.CreatedAt,
		"in_use":     inUse,
		"containers": containers,
	}

	if metaData != nil {
		data["last_accessed"] = metaData.LastAccessed
		data["last_backup"] = metaData.LastBackup
		data["backup_count"] = metaData.BackupCount
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func (c *Context) inspectYAML(vol interface{}, meta interface{}, inUse bool, containers []string) error {
	// Simple YAML output (not using yaml library to avoid import)
	volData, _ := vol.(*volume.Volume)
	metaData, _ := meta.(*database.VolumeMetadata)

	fmt.Printf("name: %s\n", volData.Name)
	fmt.Printf("driver: %s\n", volData.Driver)
	fmt.Printf("mountpoint: %s\n", volData.Mountpoint)
	fmt.Printf("created: %s\n", volData.CreatedAt)
	fmt.Printf("in_use: %v\n", inUse)

	if len(containers) > 0 {
		fmt.Println("containers:")
		for _, c := range containers {
			fmt.Printf("  - %s\n", c)
		}
	}

	if metaData != nil {
		if !metaData.LastAccessed.IsZero() {
			fmt.Printf("last_accessed: %s\n", FormatTimestamp(metaData.LastAccessed))
		}
		if !metaData.LastBackup.IsZero() {
			fmt.Printf("last_backup: %s\n", FormatTimestamp(metaData.LastBackup))
		}
		fmt.Printf("backup_count: %d\n", metaData.BackupCount)
	}

	return nil
}
