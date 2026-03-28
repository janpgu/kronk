package cmd

import (
	"fmt"
	"os"

	"github.com/janpgu/kronk/internal/db"
	"github.com/janpgu/kronk/internal/ui"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show all jobs and their current status",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Graceful degradation — if the DB doesn't exist yet, say so kindly.
	if _, err := os.Stat(cfg.DBPath); os.IsNotExist(err) {
		fmt.Println(ui.MutedStyle.Render("No jobs yet. Add one with: kronk add <command> --name <name> --schedule <schedule>"))
		return nil
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer database.Close()

	jobs, err := db.GetAllJobs(database)
	if err != nil {
		return err
	}

	if len(jobs) == 0 {
		fmt.Println(ui.MutedStyle.Render("No jobs yet. Add one with: kronk add <command> --name <name> --schedule <schedule>"))
		return nil
	}

	headers := []string{"Name", "Schedule", "Cron", "Status", "Retries", "Next Run"}
	widths := []int{16, 20, 14, 10, 7, 22}

	rows := make([][]string, len(jobs))
	for i, j := range jobs {
		nextRun := "—"
		if j.NextRunAt != nil {
			nextRun = j.NextRunAt.Format("Mon 2 Jan 15:04")
		}

		rows[i] = []string{
			j.Name,
			j.ScheduleRaw,
			j.ScheduleCron,
			string(j.Status),
			fmt.Sprintf("%d", j.MaxRetries),
			nextRun,
		}
	}

	fmt.Println(ui.RenderTable(headers, rows, widths))
	return nil
}
