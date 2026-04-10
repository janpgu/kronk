package cmd

import (
	"fmt"
	"time"

	"github.com/janpgu/kronk/internal/db"
	"github.com/janpgu/kronk/internal/ui"
	"github.com/spf13/cobra"
)

var pruneDays int

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Delete run history older than N days",
	Args:  cobra.NoArgs,
	RunE:  runPrune,
}

func init() {
	pruneCmd.Flags().IntVar(&pruneDays, "days", 30, "delete runs older than this many days")
	rootCmd.AddCommand(pruneCmd)
}

func runPrune(cmd *cobra.Command, args []string) error {
	if pruneDays < 1 {
		return fmt.Errorf("--days must be at least 1")
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer database.Close()

	before := time.Now().AddDate(0, 0, -pruneDays)
	n, err := db.PruneRuns(database, before)
	if err != nil {
		return err
	}

	if n == 0 {
		fmt.Printf("No runs older than %d days found.\n", pruneDays)
		return nil
	}

	ui.PrintSuccess(fmt.Sprintf("Deleted %d run(s) older than %d days.", n, pruneDays))
	return nil
}
