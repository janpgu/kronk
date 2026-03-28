package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/janpgu/kronk/internal/ui"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run kronk as a long-running process, ticking every 30 seconds",
	RunE:  runDaemon,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runDaemon(cmd *cobra.Command, args []string) error {
	fmt.Printf("kronk running\n")
	fmt.Printf("DB:        %s\n", ui.MutedStyle.Render(cfg.DBPath))
	fmt.Printf("Interval:  %s\n", ui.MutedStyle.Render("30s"))
	fmt.Printf("Press Ctrl+C to stop.\n\n")

	// Run an immediate tick on startup so the user sees activity right away.
	if err := runTickLogic(true); err != nil {
		ui.PrintError(fmt.Sprintf("tick error: %s", err))
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Listen for Ctrl+C or SIGTERM for a clean shutdown.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			fmt.Printf("[%s] tick\n", time.Now().Format("15:04:05"))
			if err := runTickLogic(false); err != nil {
				ui.PrintError(fmt.Sprintf("tick error: %s", err))
			}
		case <-sigs:
			fmt.Println("\nStopped.")
			return nil
		}
	}
}
