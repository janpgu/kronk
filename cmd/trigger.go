package cmd

import (
	"fmt"

	"github.com/janpgu/kronk/internal/db"
	"github.com/janpgu/kronk/internal/ui"
	"github.com/janpgu/kronk/internal/worker"
	"github.com/spf13/cobra"
)

var triggerCmd = &cobra.Command{
	Use:   "trigger <name>",
	Short: "Run a job immediately, ignoring its schedule",
	Args:  cobra.ExactArgs(1),
	RunE:  runTrigger,
}

func init() {
	rootCmd.AddCommand(triggerCmd)
}

func runTrigger(cmd *cobra.Command, args []string) error {
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

	fmt.Printf("Triggering %s...\n", ui.BoldStyle.Render(name))

	if err := worker.Execute(database, j, true); err != nil {
		return err
	}

	return nil
}
