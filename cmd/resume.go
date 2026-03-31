package cmd

import (
	"fmt"

	"github.com/janpgu/kronk/internal/db"
	"github.com/janpgu/kronk/internal/job"
	"github.com/janpgu/kronk/internal/ui"
	"github.com/spf13/cobra"
)

var resumeCmd = &cobra.Command{
	Use:   "resume <name>",
	Short: "Resume a paused job",
	Args:  cobra.ExactArgs(1),
	RunE:  runResume,
}

func init() {
	rootCmd.AddCommand(resumeCmd)
}

func runResume(cmd *cobra.Command, args []string) error {
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

	if j.Status == job.StatusActive {
		fmt.Printf("Job %s is already active.\n", ui.BoldStyle.Render(name))
		return nil
	}

	j.Status = job.StatusActive
	if err := db.UpdateJob(database, j); err != nil {
		return err
	}

	ui.PrintSuccess(fmt.Sprintf("Job %q resumed.", name))
	return nil
}
