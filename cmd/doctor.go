package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/janpgu/kronk/internal/ui"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check kronk's configuration and print setup instructions",
	RunE:  runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	binary, err := os.Executable()
	if err != nil {
		binary = "(unknown)"
	}

	fmt.Println(ui.BoldStyle.Render("kronk doctor"))
	fmt.Println()

	// Config.
	fmt.Println(ui.HeaderStyle.Render("Configuration"))
	fmt.Printf("  DB path:     %s\n", cfg.DBPath)
	fmt.Printf("  Binary:      %s\n", binary)
	fmt.Printf("  OS:          %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println()

	// Checks.
	fmt.Println(ui.HeaderStyle.Render("Checks"))
	checkDBDir(cfg.DBPath)
	checkDBFile(cfg.DBPath)
	fmt.Println()

	// Setup instructions.
	fmt.Println(ui.HeaderStyle.Render("Setup"))
	printSetupInstructions(binary)

	return nil
}

// checkDBDir reports whether the database directory exists.
func checkDBDir(dbPath string) {
	dir := filepath.Dir(dbPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fmt.Printf("  %s DB directory does not exist: %s\n", ui.WarnStyle.Render(ui.WarnMark), dir)
		fmt.Printf("      It will be created automatically on first use.\n")
	} else {
		fmt.Printf("  %s DB directory exists: %s\n", ui.SuccessStyle.Render(ui.CheckMark), dir)
	}
}

// checkDBFile reports whether the database file exists and is readable.
func checkDBFile(dbPath string) {
	f, err := os.Open(dbPath)
	if os.IsNotExist(err) {
		fmt.Printf("  %s DB file does not exist (will be created on first use)\n", ui.WarnStyle.Render(ui.WarnMark))
		return
	}
	if err != nil {
		fmt.Printf("  %s DB file exists but cannot be opened: %s\n", ui.ErrorStyle.Render(ui.CrossMark), err)
		return
	}
	f.Close()
	fmt.Printf("  %s DB file exists and is readable: %s\n", ui.SuccessStyle.Render(ui.CheckMark), dbPath)
}

// printSetupInstructions prints the correct scheduler setup for the current OS.
func printSetupInstructions(binary string) {
	fmt.Println("  Run the following command to register kronk with your system scheduler:")
	fmt.Println()
	fmt.Printf("  %s setup\n", binary)
	fmt.Println()

	switch runtime.GOOS {
	case "windows":
		fmt.Println("  This registers a Task Scheduler task that runs kronk every minute,")
		fmt.Println("  including on battery.")
	default:
		fmt.Printf("  This adds the following crontab entry:\n")
		fmt.Printf("  * * * * * %s tick\n", binary)
	}
}

