package cmd

import (
	"fmt"
	"time"

	"github.com/janpgu/kronk/internal/db"
	"github.com/janpgu/kronk/internal/job"
	"github.com/janpgu/kronk/internal/scheduler"
	"github.com/janpgu/kronk/internal/ui"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <command>",
	Short: "Add a new scheduled job",
	Args:  cobra.ExactArgs(1),
	RunE:  runAdd,
}

var (
	addName     string
	addSchedule string
	addRetries  int
	addTimeout  int
)

func init() {
	addCmd.Flags().StringVar(&addName, "name", "", "unique name for the job (required)")
	addCmd.Flags().StringVar(&addSchedule, "schedule", "", "when to run, e.g. \"every night\" (required)")
	addCmd.Flags().IntVar(&addRetries, "retries", 0, "number of times to retry on failure")
	addCmd.Flags().IntVar(&addTimeout, "timeout", 0, "kill the job after this many seconds (0 = no timeout)")

	_ = addCmd.MarkFlagRequired("name")
	_ = addCmd.MarkFlagRequired("schedule")

	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	command := args[0]

	// Parse and validate the schedule.
	cronExpr, err := scheduler.Parse(addSchedule)
	if err != nil {
		return err
	}

	// Compute the first run time.
	nextRun, err := scheduler.NextRun(cronExpr, time.Now())
	if err != nil {
		return fmt.Errorf("could not compute next run time: %w", err)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer database.Close()

	j := &job.Job{
		Name:           addName,
		Command:        command,
		ScheduleRaw:    addSchedule,
		ScheduleCron:   cronExpr,
		MaxRetries:     addRetries,
		TimeoutSeconds: addTimeout,
		NextRunAt:      &nextRun,
	}

	if _, err := db.AddJob(database, j); err != nil {
		return err
	}

	// Print confirmation.
	fmt.Printf("Job added:  %s\n", ui.BoldStyle.Render(addName))
	fmt.Printf("Schedule:   %q %s %s\n", addSchedule, ui.MutedStyle.Render(ui.Arrow), ui.MutedStyle.Render(cronExpr))
	fmt.Printf("Next run:   %s\n", nextRun.Format("Mon 2 Jan 2006 at 15:04"))

	return nil
}
