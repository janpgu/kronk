package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the kronk version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("kronk %s\n", rootCmd.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
