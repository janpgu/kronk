package cmd

import (
	"fmt"
	"os"

	"github.com/janpgu/kronk/internal/db"
	"github.com/janpgu/kronk/internal/ui"
	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show recent job run history",
	RunE:  runHistory,
}

var (
	historyJob   string
	historyLimit int
)

func init() {
	historyCmd.Flags().StringVar(&historyJob, "job", "", "filter by job name")
	historyCmd.Flags().IntVar(&historyLimit, "limit", 20, "maximum number of runs to show")
	rootCmd.AddCommand(historyCmd)
}

func runHistory(cmd *cobra.Command, args []string) error {
	// Graceful degradation — if the DB doesn't exist yet, say so kindly.
	if _, err := os.Stat(cfg.DBPath); os.IsNotExist(err) {
		fmt.Println(ui.MutedStyle.Render("No run history yet. Jobs run via 'kronk tick' are recorded here."))
		return nil
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer database.Close()

	// Fetch runs — optionally filtered by job name.
	var runs []*db.RunWithName
	if historyJob != "" {
		j, err := db.GetJob(database, historyJob)
		if err != nil {
			return err
		}
		rawRuns, err := db.GetRunsForJob(database, j.ID, historyLimit)
		if err != nil {
			return err
		}
		runs = db.AttachJobName(rawRuns, j.Name)
	} else {
		runs, err = db.GetAllRunsWithNames(database, historyLimit)
		if err != nil {
			return err
		}
	}

	if len(runs) == 0 {
		fmt.Println(ui.MutedStyle.Render("No run history yet."))
		return nil
	}

	headers := []string{"Job", "Started", "Duration", "Exit", "Output"}
	widths := []int{18, 20, 10, 6, 38}

	rows := make([][]string, len(runs))
	for i, r := range runs {
		started := r.StartedAt.Format("2 Jan 15:04:05")

		duration := "—"
		if r.FinishedAt != nil {
			duration = r.FinishedAt.Sub(r.StartedAt).Round(1e6).String()
		}

		exitCode := "—"
		if r.ExitCode != nil {
			exitCode = fmt.Sprintf("%d", *r.ExitCode)
		}

		// Show stdout if present, fall back to stderr.
		output := r.Stdout
		if output == "" {
			output = r.Stderr
		}

		rows[i] = []string{
			r.JobName,
			started,
			duration,
			exitCode,
			ui.Truncate(output, 40),
		}
	}

	fmt.Println(ui.RenderTable(headers, rows, widths))
	return nil
}
