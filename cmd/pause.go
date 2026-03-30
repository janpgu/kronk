package cmd

import (
	"fmt"

	"github.com/janpgu/kronk/internal/db"
	"github.com/janpgu/kronk/internal/job"
	"github.com/janpgu/kronk/internal/ui"
	"github.com/spf13/cobra"
)

var pauseCmd = &cobra.Command{
	Use:   "pause <name>",
	Short: "Pause a job without removing it",
	Args:  cobra.ExactArgs(1),
	RunE:  runPause,
}

func init() {
	rootCmd.AddCommand(pauseCmd)
}

func runPause(cmd *cobra.Command, args []string) error {
	name := args[0]

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer database.Close()

	j, err := db.GetJob(database, name)
	if err != nil {
		return err
	}

	if j.Status == job.StatusPaused {
		fmt.Printf("Job %s is already paused.\n", ui.BoldStyle.Render(name))
		return nil
	}

	j.Status = job.StatusPaused
	if err := db.UpdateJob(database, j); err != nil {
		return err
	}

	ui.PrintSuccess(fmt.Sprintf("Job %q paused.", name))
	return nil
}
