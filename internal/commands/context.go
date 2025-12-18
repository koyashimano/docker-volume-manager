package commands

import (
	"github.com/koyashimano/docker-volume-manager/internal/compose"
	"github.com/koyashimano/docker-volume-manager/internal/config"
	"github.com/koyashimano/docker-volume-manager/internal/database"
	"github.com/koyashimano/docker-volume-manager/internal/docker"
)

// Context holds the application context
type Context struct {
	Config      *config.Config
	Docker      *docker.Client
	DB          *database.DB
	Compose     *compose.ComposeFile
	ProjectName string
	Verbose     bool
	Quiet       bool
}

// NewContext creates a new context
func NewContext(cfg *config.Config, verbose, quiet bool) (*Context, error) {
	dockerClient, err := docker.NewClient()
	if err != nil {
		return nil, err
	}

	configPath := config.GetConfigPath()
	dbPath := configPath[:len(configPath)-len("config.yaml")] + "meta.db"
	db, err := database.NewDB(dbPath)
	if err != nil {
		dockerClient.Close()
		return nil, err
	}

	return &Context{
		Config:  cfg,
		Docker:  dockerClient,
		DB:      db,
		Verbose: verbose,
		Quiet:   quiet,
	}, nil
}

// Close closes all connections
func (c *Context) Close() {
	if c.Docker != nil {
		c.Docker.Close()
	}
	if c.DB != nil {
		c.DB.Close()
	}
}

// LoadCompose loads the compose file
func (c *Context) LoadCompose(composePath, projectOverride string) error {
	var cf *compose.ComposeFile
	var err error

	if composePath != "" {
		cf, err = compose.LoadComposeFile(composePath)
	} else {
		path, err := compose.FindComposeFile(".")
		if err != nil {
			return err
		}
		cf, err = compose.LoadComposeFile(path)
	}

	if err != nil {
		return err
	}

	c.Compose = cf
	c.ProjectName = cf.GetProjectName(projectOverride)
	return nil
}

// ResolveVolumeName resolves a service name to a full volume name
func (c *Context) ResolveVolumeName(serviceOrVolume string) (string, error) {
	// If compose is loaded, try to resolve as service name
	if c.Compose != nil {
		fullName, err := c.Compose.GetFullVolumeName(serviceOrVolume, c.ProjectName)
		if err == nil {
			return fullName, nil
		}
	}

	// Otherwise, assume it's already a full volume name
	if c.Docker.VolumeExists(serviceOrVolume) {
		return serviceOrVolume, nil
	}

	// Try with project prefix
	if c.ProjectName != "" {
		withPrefix := c.ProjectName + "_" + serviceOrVolume
		if c.Docker.VolumeExists(withPrefix) {
			return withPrefix, nil
		}
	}

	return "", ErrVolumeNotFound
}

// GetServiceName tries to get the service name from volume name
func (c *Context) GetServiceName(volumeName string) string {
	if c.Compose == nil {
		return ""
	}

	serviceName, err := c.Compose.GetServiceByVolumeName(volumeName, c.ProjectName)
	if err != nil {
		return ""
	}

	return serviceName
}
