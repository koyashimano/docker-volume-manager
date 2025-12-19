package compose

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ComposeFile represents a Docker Compose file
type ComposeFile struct {
	Name     string                 `yaml:"name,omitempty"`
	Services map[string]Service     `yaml:"services"`
	Volumes  map[string]interface{} `yaml:"volumes,omitempty"`
	path     string
}

// Service represents a service in compose file
type Service struct {
	Image   string        `yaml:"image,omitempty"`
	Volumes []interface{} `yaml:"volumes,omitempty"`
}

// VolumeMapping represents a parsed volume mapping
type VolumeMapping struct {
	VolumeName string
	MountPath  string
	Service    string
}

func normalizeProjectName(name string) string {
	normalized := strings.ToLower(name)

	var b strings.Builder
	for _, r := range normalized {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		}
	}

	return strings.TrimLeft(b.String(), "-_.")
}

// FindComposeFile searches for a compose file in the given directory
func FindComposeFile(dir string) (string, error) {
	if dir == "" {
		dir = "."
	}

	candidates := []string{
		"compose.yaml",
		"compose.yml",
		"docker-compose.yaml",
		"docker-compose.yml",
	}

	for _, name := range candidates {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("compose file not found in %s", dir)
}

// LoadComposeFile loads a Docker Compose file
func LoadComposeFile(path string) (*ComposeFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cf ComposeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, err
	}

	cf.path = path
	return &cf, nil
}

// GetProjectName determines the project name based on priority
func (cf *ComposeFile) GetProjectName(override string) string {
	// 1. Command line override
	if override != "" {
		return normalizeProjectName(override)
	}

	// 2. name field in compose file
	if cf.Name != "" {
		return normalizeProjectName(cf.Name)
	}

	// 3. COMPOSE_PROJECT_NAME env var
	if env := os.Getenv("COMPOSE_PROJECT_NAME"); env != "" {
		return normalizeProjectName(env)
	}

	// 4. Directory name
	dir := filepath.Dir(cf.path)

	// If the compose file is in the current directory (dir is "."),
	// use the actual current working directory name
	if dir == "." {
		if cwd, err := os.Getwd(); err == nil {
			return normalizeProjectName(filepath.Base(cwd))
		}
	}

	return normalizeProjectName(filepath.Base(dir))
}

// GetVolumeMapping returns volume mapping for a service
func (cf *ComposeFile) GetVolumeMapping(serviceName string) ([]VolumeMapping, error) {
	service, ok := cf.Services[serviceName]
	if !ok {
		return nil, fmt.Errorf("service %s not found", serviceName)
	}

	var mappings []VolumeMapping
	for _, volSpec := range service.Volumes {
		switch v := volSpec.(type) {
		case string:
			// Short-form syntax: "volume_name:/path" or "./host/path:/container/path"
			parts := strings.Split(v, ":")
			if len(parts) < 2 {
				continue
			}

			source := parts[0]
			target := parts[1]

			// Only named volumes (not bind mounts)
			if !strings.HasPrefix(source, "/") && !strings.HasPrefix(source, ".") && !strings.HasPrefix(source, "~") {
				mappings = append(mappings, VolumeMapping{
					VolumeName: source,
					MountPath:  target,
					Service:    serviceName,
				})
			}

		case map[string]interface{}:
			// Long-form syntax: {type: volume, source: name, target: /path, ...}
			srcVal, okSrc := v["source"]
			tgtVal, okTgt := v["target"]
			if !okSrc || !okTgt {
				continue
			}

			source, okSourceStr := srcVal.(string)
			target, okTargetStr := tgtVal.(string)
			if !okSourceStr || !okTargetStr || source == "" || target == "" {
				continue
			}

			// Check if type is explicitly set to something other than "volume"
			if typeVal, okType := v["type"]; okType {
				if typeStr, okTypeStr := typeVal.(string); okTypeStr && typeStr != "volume" {
					continue
				}
			}

			// Only named volumes (not bind mounts)
			if !strings.HasPrefix(source, "/") && !strings.HasPrefix(source, ".") && !strings.HasPrefix(source, "~") {
				mappings = append(mappings, VolumeMapping{
					VolumeName: source,
					MountPath:  target,
					Service:    serviceName,
				})
			}
		}
	}

	return mappings, nil
}

// GetAllVolumeMappings returns all volume mappings in the compose file
func (cf *ComposeFile) GetAllVolumeMappings() []VolumeMapping {
	var mappings []VolumeMapping
	for serviceName := range cf.Services {
		if m, err := cf.GetVolumeMapping(serviceName); err == nil {
			mappings = append(mappings, m...)
		}
	}
	return mappings
}

// GetFullVolumeName returns the full Docker volume name
func (cf *ComposeFile) GetFullVolumeName(serviceName, projectName string) (string, error) {
	mappings, err := cf.GetVolumeMapping(serviceName)
	if err != nil {
		return "", err
	}

	if len(mappings) == 0 {
		return "", fmt.Errorf("no named volumes found for service %s", serviceName)
	}

	// If multiple volumes, return the first one (common case: one volume per service)
	return fmt.Sprintf("%s_%s", projectName, mappings[0].VolumeName), nil
}

// GetAllFullVolumeNames returns all full volume names for the project
func (cf *ComposeFile) GetAllFullVolumeNames(projectName string) []string {
	mappings := cf.GetAllVolumeMappings()
	var names []string
	seen := make(map[string]bool)

	for _, m := range mappings {
		fullName := fmt.Sprintf("%s_%s", projectName, m.VolumeName)
		if !seen[fullName] {
			names = append(names, fullName)
			seen[fullName] = true
		}
	}

	return names
}

// GetServiceByVolumeName finds the service using a volume
func (cf *ComposeFile) GetServiceByVolumeName(volumeName, projectName string) (string, error) {
	// Strip project prefix if present
	shortName := volumeName
	prefix := projectName + "_"
	if strings.HasPrefix(volumeName, prefix) {
		shortName = strings.TrimPrefix(volumeName, prefix)
	}

	for serviceName := range cf.Services {
		mappings, err := cf.GetVolumeMapping(serviceName)
		if err != nil {
			continue
		}

		for _, m := range mappings {
			if m.VolumeName == shortName {
				return serviceName, nil
			}
		}
	}

	return "", fmt.Errorf("no service found using volume %s", volumeName)
}
