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
	Size   bool
}

// VolumeListItem represents a volume in the list
type VolumeListItem struct {
	Service    string
	VolumeName string
	LastUsed   time.Time
	InUse      bool
	Size       int64 // -1 means unavailable
}

// List lists volumes
func (c *Context) List(opts ListOptions) error {
	volumes, err := c.Docker.ListVolumes()
	if err != nil {
		return err
	}

	var volumeSizes map[string]int64
	if opts.Size {
		volumeSizes, err = c.Docker.GetVolumeSizes()
		if err != nil {
			return fmt.Errorf("failed to get volume sizes: %w", err)
		}
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
			Size:       -1,
		}

		if volumeSizes != nil {
			if size, ok := volumeSizes[vol.Name]; ok {
				item.Size = size
			}
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
		return c.outputJSON(items, opts.Size)
	case "csv":
		return c.outputCSV(items, opts.Size)
	default:
		return c.outputTable(items, opts.Size)
	}
}

func (c *Context) outputTable(items []VolumeListItem, showSize bool) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	header := "SERVICE\tVOLUME\tLAST_USED\tSTATUS"
	if showSize {
		header += "\tSIZE"
	}
	fmt.Fprintln(w, header)

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

		line := fmt.Sprintf("%s\t%s\t%s\t%s",
			service,
			item.VolumeName,
			lastUsed,
			status,
		)
		if showSize {
			size := "N/A"
			if item.Size >= 0 {
				size = FormatSize(item.Size)
			}
			line += fmt.Sprintf("\t%s", size)
		}
		fmt.Fprintln(w, line)
	}

	return nil
}

func (c *Context) outputJSON(items []VolumeListItem, showSize bool) error {
	// Create a slice of map[string]interface{} for JSON output
	output := make([]map[string]interface{}, len(items))
	for i, item := range items {
		status := "unused"
		if item.InUse {
			status = "in-use"
		}

		entry := map[string]interface{}{
			"service":   item.Service,
			"volume":    item.VolumeName,
			"last_used": FormatTimestamp(item.LastUsed),
			"status":    status,
		}
		if showSize {
			if item.Size >= 0 {
				entry["size"] = item.Size
			} else {
				entry["size"] = nil
			}
		}
		output[i] = entry
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func (c *Context) outputCSV(items []VolumeListItem, showSize bool) error {
	w := csv.NewWriter(os.Stdout)
	defer w.Flush()

	// Write header
	header := []string{"service", "volume", "last_used", "status"}
	if showSize {
		header = append(header, "size")
	}
	if err := w.Write(header); err != nil {
		return err
	}

	// Write records
	for _, item := range items {
		status := "unused"
		if item.InUse {
			status = "in-use"
		}

		record := []string{
			item.Service,
			item.VolumeName,
			FormatTimestamp(item.LastUsed),
			status,
		}
		if showSize {
			if item.Size >= 0 {
				record = append(record, FormatSize(item.Size))
			} else {
				record = append(record, "N/A")
			}
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}

	return nil
}
