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
	switch runtime.GOOS {
	case "windows":
		fmt.Println("  The easiest way to set up kronk is with the install script:")
		fmt.Println()
		fmt.Println("  irm https://raw.githubusercontent.com/janpgu/kronk/main/install.ps1 | iex")
		fmt.Println()
		fmt.Println("  Or manually register with Task Scheduler:")
		fmt.Println()
		fmt.Printf("  schtasks /create /tn \"kronk\" /tr \"wscript.exe //B %%USERPROFILE%%\\bin\\kronk-tick.vbs\" /sc MINUTE /mo 1\n")

	case "darwin":
		fmt.Println("  The easiest way to set up kronk is with the install script:")
		fmt.Println()
		fmt.Println("  curl -fsSL https://raw.githubusercontent.com/janpgu/kronk/main/install.sh | sh")
		fmt.Println()
		fmt.Println("  Or manually add to crontab (crontab -e):")
		fmt.Println()
		fmt.Printf("  * * * * * %s tick\n", binary)

	default:
		fmt.Println("  The easiest way to set up kronk is with the install script:")
		fmt.Println()
		fmt.Println("  curl -fsSL https://raw.githubusercontent.com/janpgu/kronk/main/install.sh | sh")
		fmt.Println()
		fmt.Println("  Or manually add to crontab (crontab -e):")
		fmt.Println()
		fmt.Printf("  * * * * * %s tick\n", binary)
	}
}

