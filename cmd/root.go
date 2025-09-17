package cmd

import (
	"os"

	"github.com/dsrosen6/addigy-macos-compatibility/internal/compat"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use: "mac-compat",
	RunE: func(cmd *cobra.Command, args []string) error {
		return compat.RunReport()
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
