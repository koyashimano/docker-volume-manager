package commands

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"
)

// ListOptions contains options for list command
type ListOptions struct {
	All    bool
	Unused bool
	Stale  int
	Size   bool
	Format string
}

// VolumeListItem represents a volume in the list
type VolumeListItem struct {
	Service    string
	VolumeName string
	Size       int64
	LastUsed   time.Time
	InUse      bool
}

// List lists volumes
func (c *Context) List(opts ListOptions) error {
	volumes, err := c.Docker.ListVolumes()
	if err != nil {
		return err
	}

	var items []VolumeListItem

	for _, vol := range volumes {
		// Filter by project if compose is loaded and not --all
		if !opts.All && c.Compose != nil {
			// Check if volume belongs to this project
			if c.ProjectName != "" && vol.Name[:len(c.ProjectName)] != c.ProjectName {
				continue
			}
		}

		inUse, _ := c.Docker.IsVolumeInUse(vol.Name)

		// Filter unused if requested
		if opts.Unused && inUse {
			continue
		}

		// Get metadata
		meta, _ := c.DB.GetVolumeMetadata(vol.Name)

		// Filter by stale if requested
		if opts.Stale > 0 && !meta.LastAccessed.IsZero() {
			if time.Since(meta.LastAccessed).Hours() < float64(opts.Stale*24) {
				continue
			}
		}

		// Get service name if available
		serviceName := c.GetServiceName(vol.Name)

		item := VolumeListItem{
			Service:    serviceName,
			VolumeName: vol.Name,
			InUse:      inUse,
		}

		if meta != nil {
			item.LastUsed = meta.LastAccessed
		}

		items = append(items, item)
	}

	// Sort
	if opts.Size {
		sort.Slice(items, func(i, j int) bool {
			return items[i].Size > items[j].Size
		})
	} else {
		sort.Slice(items, func(i, j int) bool {
			return items[i].VolumeName < items[j].VolumeName
		})
	}

	// Output
	switch opts.Format {
	case "json":
		return c.outputJSON(items)
	case "csv":
		return c.outputCSV(items)
	default:
		return c.outputTable(items)
	}
}

func (c *Context) outputTable(items []VolumeListItem) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "SERVICE\tVOLUME\tLAST_USED\tSTATUS")

	for _, item := range items {
		service := item.Service
		if service == "" {
			service = "-"
		}

		lastUsed := FormatTimestamp(item.LastUsed)
		status := "unused"
		if item.InUse {
			status = "in-use"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			service,
			item.VolumeName,
			lastUsed,
			status,
		)
	}

	return nil
}

func (c *Context) outputJSON(items []VolumeListItem) error {
	fmt.Println("[")
	for i, item := range items {
		status := "unused"
		if item.InUse {
			status = "in-use"
		}

		fmt.Printf("  {\"service\": \"%s\", \"volume\": \"%s\", \"last_used\": \"%s\", \"status\": \"%s\"}",
			item.Service,
			item.VolumeName,
			FormatTimestamp(item.LastUsed),
			status,
		)

		if i < len(items)-1 {
			fmt.Println(",")
		} else {
			fmt.Println()
		}
	}
	fmt.Println("]")
	return nil
}

func (c *Context) outputCSV(items []VolumeListItem) error {
	fmt.Println("service,volume,last_used,status")

	for _, item := range items {
		status := "unused"
		if item.InUse {
			status = "in-use"
		}

		fmt.Printf("%s,%s,%s,%s\n",
			item.Service,
			item.VolumeName,
			FormatTimestamp(item.LastUsed),
			status,
		)
	}

	return nil
}
