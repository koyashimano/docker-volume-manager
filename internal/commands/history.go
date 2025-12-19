package commands

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/koyashimano/docker-volume-manager/internal/database"
)

// HistoryOptions contains options for history command
type HistoryOptions struct {
	Limit   int
	All     bool
	Service string
}

// History shows backup history
func (c *Context) History(opts HistoryOptions) error {
	limit := opts.Limit
	if limit == 0 {
		limit = 10
	}

	var records []*database.BackupRecord
	var err error

	if opts.Service != "" {
		// Get history for specific service
		volumeName, err := c.ResolveVolumeName(opts.Service)
		if err != nil {
			// Try as volume name directly
			volumeName = opts.Service
		}

		records, err = c.DB.GetBackupRecords(volumeName, limit)
		if err != nil {
			return err
		}
	} else if opts.All {
		// Get all history
		records, err = c.DB.GetAllBackupRecords(limit)
		if err != nil {
			return err
		}
	} else {
		// Get history for current project
		allRecords, err := c.DB.GetAllBackupRecords(0)
		if err != nil {
			return err
		}

		// Filter by project
		for _, rec := range allRecords {
			if rec.ProjectName == c.ProjectName {
				records = append(records, rec)
				if len(records) >= limit {
					break
				}
			}
		}
	}

	if len(records) == 0 {
		fmt.Println("No backup history found")
		return nil
	}

	// Display as table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "SERVICE\tTIMESTAMP\tSIZE\tTAG\tPATH")

	for _, rec := range records {
		serviceName := rec.ServiceName
		if serviceName == "" {
			serviceName = rec.VolumeName
		}

		tag := rec.Tag
		if tag == "" {
			tag = "-"
		}

		// Shorten path for display
		displayPath := rec.FilePath
		if len(displayPath) > 50 {
			displayPath = "..." + displayPath[len(displayPath)-47:]
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			serviceName,
			FormatTimestamp(rec.CreatedAt),
			FormatSize(rec.Size),
			tag,
			displayPath,
		)
	}

	return nil
}
