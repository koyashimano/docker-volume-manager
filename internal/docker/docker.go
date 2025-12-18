package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

const (
	// DefaultContainerTimeout is the default timeout for container operations
	DefaultContainerTimeout = 10
)

// Client wraps Docker client
type Client struct {
	cli *client.Client
	ctx context.Context
}

// VolumeInfo contains volume information
type VolumeInfo struct {
	Name       string
	Driver     string
	Mountpoint string
	CreatedAt  time.Time
	Size       int64
	InUse      bool
}

// NewClient creates a new Docker client
func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	return &Client{
		cli: cli,
		ctx: context.Background(),
	}, nil
}

// Close closes the Docker client
func (c *Client) Close() error {
	return c.cli.Close()
}

// ListVolumes lists all volumes
func (c *Client) ListVolumes() ([]*volume.Volume, error) {
	vols, err := c.cli.VolumeList(c.ctx, volume.ListOptions{})
	if err != nil {
		return nil, err
	}
	return vols.Volumes, nil
}

// GetVolume gets information about a specific volume
func (c *Client) GetVolume(name string) (*volume.Volume, error) {
	vol, err := c.cli.VolumeInspect(c.ctx, name)
	if err != nil {
		return nil, err
	}
	return &vol, nil
}

// VolumeExists checks if a volume exists
func (c *Client) VolumeExists(name string) bool {
	_, err := c.GetVolume(name)
	return err == nil
}

// IsVolumeInUse checks if a volume is in use by any container
func (c *Client) IsVolumeInUse(volumeName string) (bool, error) {
	containers, err := c.cli.ContainerList(c.ctx, container.ListOptions{
		All: true,
	})
	if err != nil {
		return false, err
	}

	for _, cont := range containers {
		for _, mnt := range cont.Mounts {
			if mnt.Name == volumeName {
				return true, nil
			}
		}
	}

	return false, nil
}

// GetContainersUsingVolume returns containers using the volume
func (c *Client) GetContainersUsingVolume(volumeName string) ([]string, error) {
	containers, err := c.cli.ContainerList(c.ctx, container.ListOptions{
		All: true,
	})
	if err != nil {
		return nil, err
	}

	var result []string
	for _, cont := range containers {
		for _, mnt := range cont.Mounts {
			if mnt.Name == volumeName {
				if len(cont.Names) > 0 {
					result = append(result, cont.Names[0])
				}
				break
			}
		}
	}

	return result, nil
}

// CreateVolume creates a new volume
func (c *Client) CreateVolume(name string) error {
	_, err := c.cli.VolumeCreate(c.ctx, volume.CreateOptions{
		Name: name,
	})
	return err
}

// RemoveVolume removes a volume
func (c *Client) RemoveVolume(name string, force bool) error {
	return c.cli.VolumeRemove(c.ctx, name, force)
}

// BackupVolume backs up a volume to a tar.gz file
func (c *Client) BackupVolume(volumeName, outputPath string, compress bool) error {
	// Determine tar compression option
	tarCompressionOption := ""
	if compress {
		tarCompressionOption = "z"
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	// Run a temporary container to create the backup
	resp, err := c.cli.ContainerCreate(c.ctx, &container.Config{
		Image: "alpine:latest",
		Cmd:   []string{"tar", "c" + tarCompressionOption + "f", "/backup/data.tar.gz", "-C", "/source", "."},
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:     mount.TypeVolume,
				Source:   volumeName,
				Target:   "/source",
				ReadOnly: true,
			},
			{
				Type:   mount.TypeBind,
				Source: outputDir,
				Target: "/backup",
			},
		},
	}, nil, nil, "")
	if err != nil {
		return err
	}

	// Ensure container cleanup
	defer c.cli.ContainerRemove(c.ctx, resp.ID, container.RemoveOptions{Force: true})

	// Start the container
	if err := c.cli.ContainerStart(c.ctx, resp.ID, container.StartOptions{}); err != nil {
		return err
	}

	// Wait for completion
	statusCh, errCh := c.cli.ContainerWait(c.ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			// Get logs for debugging
			logs, err := c.cli.ContainerLogs(c.ctx, resp.ID, container.LogsOptions{
				ShowStdout: true,
				ShowStderr: true,
			})
			if err != nil {
				return fmt.Errorf("backup failed with status %d and could not retrieve logs: %w", status.StatusCode, err)
			}
			defer logs.Close()

			logData, err := io.ReadAll(logs)
			if err != nil {
				return fmt.Errorf("backup failed with status %d and error reading logs: %w", status.StatusCode, err)
			}
			return fmt.Errorf("backup failed with status %d: %s", status.StatusCode, string(logData))
		}
	}

	// Rename the output file
	tempPath := filepath.Join(outputDir, "data.tar.gz")
	if err := os.Rename(tempPath, outputPath); err != nil {
		return err
	}

	return nil
}

// RestoreVolume restores a volume from a backup file
func (c *Client) RestoreVolume(volumeName, backupPath string) error {
	// Check if backup file exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", backupPath)
	}

	backupDir := filepath.Dir(backupPath)
	backupFile := filepath.Base(backupPath)

	// Detect compression format from file extension
	tarFlags := "xf"
	if strings.HasSuffix(backupPath, ".tar.gz") || strings.HasSuffix(backupPath, ".tgz") {
		tarFlags = "xzf"
	} else if strings.HasSuffix(backupPath, ".tar.zst") {
		// zstd compression would require zstd tool, use auto-detection
		tarFlags = "xf"
	}

	// Create volume if it doesn't exist
	if !c.VolumeExists(volumeName) {
		if err := c.CreateVolume(volumeName); err != nil {
			return err
		}
	}

	// Run a temporary container to restore the backup
	resp, err := c.cli.ContainerCreate(c.ctx, &container.Config{
		Image: "alpine:latest",
		Cmd:   []string{"tar", tarFlags, "/backup/" + backupFile, "-C", "/target"},
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: volumeName,
				Target: "/target",
			},
			{
				Type:     mount.TypeBind,
				Source:   backupDir,
				Target:   "/backup",
				ReadOnly: true,
			},
		},
	}, nil, nil, "")
	if err != nil {
		return err
	}

	// Ensure container cleanup
	defer c.cli.ContainerRemove(c.ctx, resp.ID, container.RemoveOptions{Force: true})

	// Start the container
	if err := c.cli.ContainerStart(c.ctx, resp.ID, container.StartOptions{}); err != nil {
		return err
	}

	// Wait for completion
	statusCh, errCh := c.cli.ContainerWait(c.ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			logs, err := c.cli.ContainerLogs(c.ctx, resp.ID, container.LogsOptions{
				ShowStdout: true,
				ShowStderr: true,
			})
			if err != nil {
				return fmt.Errorf("restore failed with status %d and could not retrieve logs: %w", status.StatusCode, err)
			}
			defer logs.Close()

			logData, err := io.ReadAll(logs)
			if err != nil {
				return fmt.Errorf("restore failed with status %d and could not read logs: %w", status.StatusCode, err)
			}
			return fmt.Errorf("restore failed with status %d: %s", status.StatusCode, string(logData))
		}
	}

	return nil
}

// CopyVolume copies data from one volume to another
func (c *Client) CopyVolume(sourceVolume, targetVolume string) error {
	// Create target volume if it doesn't exist
	if !c.VolumeExists(targetVolume) {
		if err := c.CreateVolume(targetVolume); err != nil {
			return err
		}
	}

	// Run a temporary container to copy data
	resp, err := c.cli.ContainerCreate(c.ctx, &container.Config{
		Image: "alpine:latest",
		Cmd:   []string{"sh", "-c", "cp -a /source/. /target/"},
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:     mount.TypeVolume,
				Source:   sourceVolume,
				Target:   "/source",
				ReadOnly: true,
			},
			{
				Type:   mount.TypeVolume,
				Source: targetVolume,
				Target: "/target",
			},
		},
	}, nil, nil, "")
	if err != nil {
		return err
	}

	// Ensure container cleanup
	defer c.cli.ContainerRemove(c.ctx, resp.ID, container.RemoveOptions{Force: true})

	if err := c.cli.ContainerStart(c.ctx, resp.ID, container.StartOptions{}); err != nil {
		return err
	}

	statusCh, errCh := c.cli.ContainerWait(c.ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return fmt.Errorf("copy failed with status code %d", status.StatusCode)
		}
	}

	return nil
}

// PullImage ensures the alpine image is available
func (c *Client) PullImage(imageName string) error {
	reader, err := c.cli.ImagePull(c.ctx, imageName, image.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()
	_, err = io.Copy(io.Discard, reader)
	return err
}

// StopContainersUsingVolume stops containers using the volume
func (c *Client) StopContainersUsingVolume(volumeName string) error {
	containers, err := c.GetContainersUsingVolume(volumeName)
	if err != nil {
		return err
	}

	timeout := DefaultContainerTimeout
	for _, containerName := range containers {
		// Remove leading slash from container name
		containerName = strings.TrimPrefix(containerName, "/")
		if err := c.cli.ContainerStop(c.ctx, containerName, container.StopOptions{Timeout: &timeout}); err != nil {
			return err
		}
	}

	return nil
}

// RestartContainersUsingVolume restarts containers using the volume
func (c *Client) RestartContainersUsingVolume(volumeName string) error {
	containers, err := c.GetContainersUsingVolume(volumeName)
	if err != nil {
		return err
	}

	timeout := DefaultContainerTimeout
	for _, containerName := range containers {
		containerName = strings.TrimPrefix(containerName, "/")
		if err := c.cli.ContainerRestart(c.ctx, containerName, container.StopOptions{Timeout: &timeout}); err != nil {
			return err
		}
	}

	return nil
}

// GetUnusedVolumes returns volumes not in use
func (c *Client) GetUnusedVolumes() ([]*volume.Volume, error) {
	vols, err := c.ListVolumes()
	if err != nil {
		return nil, err
	}

	var unused []*volume.Volume
	for _, vol := range vols {
		inUse, err := c.IsVolumeInUse(vol.Name)
		if err != nil {
			continue
		}
		if !inUse {
			unused = append(unused, vol)
		}
	}

	return unused, nil
}

// PruneVolumes removes unused volumes
func (c *Client) PruneVolumes() error {
	_, err := c.cli.VolumesPrune(c.ctx, filters.Args{})
	return err
}
