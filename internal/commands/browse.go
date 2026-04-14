package commands

import "fmt"

// BrowseOptions contains options for browse command
type BrowseOptions struct {
	All       bool
	Long      bool
	Recursive bool
	Path      string
	Service   string
}

// Browse lists the contents of a volume
func (c *Context) Browse(opts BrowseOptions) error {
	if opts.Service == "" {
		return fmt.Errorf("service name is required")
	}

	// Resolve volume name
	volumeName, err := c.ResolveVolumeName(opts.Service)
	if err != nil {
		return err
	}

	// Build ls flags
	var lsArgs []string
	if opts.Long || opts.All {
		flags := "-"
		if opts.Long {
			flags += "l"
		}
		if opts.All {
			flags += "a"
		}
		lsArgs = append(lsArgs, flags)
	}
	if opts.Recursive {
		lsArgs = append(lsArgs, "-R")
	}

	// Run ls in volume
	output, err := c.Docker.ListVolumeContents(volumeName, opts.Path, lsArgs)
	if err != nil {
		return err
	}

	// Update last-accessed metadata so clean --stale works correctly
	if err := c.DB.UpdateLastAccessed(volumeName); err != nil {
		fmt.Printf("Warning: failed to update metadata: %v\n", err)
	}

	if output != "" {
		fmt.Println(output)
	}

	return nil
}
