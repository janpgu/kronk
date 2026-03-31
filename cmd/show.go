package cmd

import (
	"fmt"

	"github.com/janpgu/kronk/internal/db"
	"github.com/janpgu/kronk/internal/ui"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show all details for a job",
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

func init() {
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
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

	label := ui.MutedStyle.Render

	fmt.Println()
	fmt.Printf("  %s  %s\n", label("Name:    "), ui.BoldStyle.Render(j.Name))
	fmt.Printf("  %s  %s\n", label("Command: "), j.Command)
	fmt.Printf("  %s  %s\n", label("Schedule:"), j.ScheduleRaw)
	fmt.Printf("  %s  %s\n", label("Cron:    "), j.ScheduleCron)
	fmt.Printf("  %s  %s\n", label("Status:  "), ui.StatusStyle(string(j.Status)))
	fmt.Printf("  %s  %d\n", label("Retries: "), j.MaxRetries)

	if j.NextRunAt != nil {
		fmt.Printf("  %s  %s\n", label("Next Run:"), j.NextRunAt.Format("Mon 2 Jan 15:04"))
	} else {
		fmt.Printf("  %s  %s\n", label("Next Run:"), ui.MutedStyle.Render("—"))
	}

	if j.LastRunAt != nil {
		fmt.Printf("  %s  %s\n", label("Last Run:"), j.LastRunAt.Format("Mon 2 Jan 15:04"))
	} else {
		fmt.Printf("  %s  %s\n", label("Last Run:"), ui.MutedStyle.Render("never"))
	}

	fmt.Printf("  %s  %s\n", label("Created: "), j.CreatedAt.Format("Mon 2 Jan 2006 15:04"))
	fmt.Println()

	return nil
}
