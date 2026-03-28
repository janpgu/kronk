package cmd

import (
	"fmt"

	"github.com/janpgu/kronk/internal/db"
	"github.com/janpgu/kronk/internal/ui"
	"github.com/janpgu/kronk/internal/worker"
	"github.com/spf13/cobra"
)

var tickCmd = &cobra.Command{
	Use:   "tick",
	Short: "Check for due jobs and run them (called by crontab every minute)",
	RunE:  runTick,
}

var tickVerbose bool

func init() {
	tickCmd.Flags().BoolVar(&tickVerbose, "verbose", false, "print each job as it runs")
	rootCmd.AddCommand(tickCmd)
}

func runTick(cmd *cobra.Command, args []string) error {
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		return err
	}

	dueJobs, err := db.GetDueJobs(database)
	if err != nil {
		return err
	}

	if tickVerbose {
		fmt.Printf("%s %d job(s) due\n", ui.MutedStyle.Render("tick:"), len(dueJobs))
	}

	var firstErr error
	for _, j := range dueJobs {
		if err := worker.Execute(database, j, tickVerbose); err != nil {
			// Record the error but continue — one bad job should not block others.
			ui.PrintError(fmt.Sprintf("failed to execute %q: %s", j.Name, err))
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}
