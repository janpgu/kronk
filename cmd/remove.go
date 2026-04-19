package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/janpgu/kronk/internal/db"
	"github.com/janpgu/kronk/internal/ui"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a job and all its run history",
	Args:  cobra.ExactArgs(1),
	RunE:  runRemove,
}

func init() {
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer database.Close()

	// Confirm the job exists before prompting.
	if _, err := db.GetJob(database, name); err != nil {
		return err
	}

	// Prompt for confirmation.
	fmt.Printf("Remove job %s and all its run history? [y/N] ", ui.BoldStyle.Render(name))

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("could not read response: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response != "y" && response != "yes" {
		fmt.Println("Aborted.")
		return nil
	}

	if err := db.DeleteJob(database, name); err != nil {
		return err
	}

	ui.PrintSuccess(fmt.Sprintf("Job %q removed.", name))
	return nil
}

// runRemoveConfirmed deletes a job and its run history without prompting.
// Used by tests to bypass the interactive confirmation.
func runRemoveConfirmed(name string) error {
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer database.Close()

	if err := db.DeleteJob(database, name); err != nil {
		return err
	}

	ui.PrintSuccess(fmt.Sprintf("Job %q removed.", name))
	return nil
}
