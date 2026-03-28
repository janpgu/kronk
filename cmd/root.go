// Package cmd implements the kronk CLI commands.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/janpgu/kronk/internal/job"
	"github.com/spf13/cobra"
)

// cfg is the shared config populated by the root command's PersistentPreRunE
// and read by every subcommand. It is package-level within cmd but not global
// to the whole program — subcommands access it via this package.
var cfg = &job.Config{}

// rootCmd is the base command. Running `kronk` with no subcommand prints help.
var rootCmd = &cobra.Command{
	Use:   "kronk",
	Short: "Pull the lever.",
	Long: `kronk — a zero-infrastructure job queue and scheduler.

All state lives in a single SQLite file. No Redis, no brokers, no daemons.
Add a single line to your crontab and kronk handles the rest.`,
}

// Execute is the entry point called by main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// --db flag available on every subcommand.
	rootCmd.PersistentFlags().StringVar(
		&cfg.DBPath, "db", "",
		"path to the kronk database (overrides KRONK_DB env var)",
	)

	// Resolve the DB path before any subcommand runs.
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if cfg.DBPath == "" {
			cfg.DBPath = os.Getenv("KRONK_DB")
		}
		if cfg.DBPath == "" {
			var err error
			cfg.DBPath, err = defaultDBPath()
			if err != nil {
				return fmt.Errorf("could not determine database path: %w", err)
			}
		}
		return nil
	}
}

// defaultDBPath returns the platform-appropriate default database location:
//
//	Unix:    ~/.kronk/kronk.db
//	Windows: %APPDATA%\kronk\kronk.db
func defaultDBPath() (string, error) {
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("%%APPDATA%% is not set")
		}
		return filepath.Join(appData, "kronk", "kronk.db"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not find home directory: %w", err)
	}
	return filepath.Join(home, ".kronk", "kronk.db"), nil
}
