package commands

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
)

// ListOptions contains options for list command
type ListOptions struct {
	All    bool
	Unused bool
	Stale  int
	Format string
}

// VolumeListItem represents a volume in the list
type VolumeListItem struct {
	Service    string
	VolumeName string
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
		if !opts.All && c.Compose != nil && c.ProjectName != "" {
			// Check if volume belongs to this project
			// Volume should start with "projectname_"
			prefix := c.ProjectName + "_"
			if !strings.HasPrefix(vol.Name, prefix) {
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

	// Sort by volume name
	sort.Slice(items, func(i, j int) bool {
		return items[i].VolumeName < items[j].VolumeName
	})

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
	// Create a slice of map[string]string for JSON output
	output := make([]map[string]string, len(items))
	for i, item := range items {
		status := "unused"
		if item.InUse {
			status = "in-use"
		}

		output[i] = map[string]string{
			"service":   item.Service,
			"volume":    item.VolumeName,
			"last_used": FormatTimestamp(item.LastUsed),
			"status":    status,
		}
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func (c *Context) outputCSV(items []VolumeListItem) error {
	w := csv.NewWriter(os.Stdout)
	defer w.Flush()

	// Write header
	if err := w.Write([]string{"service", "volume", "last_used", "status"}); err != nil {
		return err
	}

	// Write records
	for _, item := range items {
		status := "unused"
		if item.InUse {
			status = "in-use"
		}

		if err := w.Write([]string{
			item.Service,
			item.VolumeName,
			FormatTimestamp(item.LastUsed),
			status,
		}); err != nil {
			return err
		}
	}

	return nil
}
