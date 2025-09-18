package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dsrosen6/addigy-macos-compatibility/internal/compat"
	"github.com/spf13/cobra"
)

var (
	debug              bool
	filePath           string
	policiesToFilter   []string
	osVersionsToFilter []int
	rootCmd            = &cobra.Command{
		Use: "mac-compat",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := processOptions()
			if err != nil {
				return err
			}

			return compat.Run(opts)
		},
	}
)

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("An error occured getting the home directory path:", err)
	}

	defaultFilePath := filepath.Join(home, "Downloads", "os_compat.csv")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "enable debug mode")
	rootCmd.PersistentFlags().StringVarP(&filePath, "filepath", "f", defaultFilePath, "file path for CSV - default: ~/Downloads/os_compat.csv")
	rootCmd.PersistentFlags().StringSliceVarP(&policiesToFilter, "policy", "p", []string{}, "filter by policy, case-sensitive") //TODO: make this more clear on multiple flags
	rootCmd.PersistentFlags().IntSliceVarP(&osVersionsToFilter, "filter-os-versions", "o", []int{}, "list of max os versions to filter by, i.e. 14,15")
}

func processOptions() (compat.Options, error) {
	addigyKey := os.Getenv("ADDIGY_API_KEY")
	if addigyKey == "" {
		return compat.Options{}, errors.New("addigy api key is missing - set an environment variable of ADDIGY_API_KEY")
	}

	return compat.Options{
		Debug:               debug,
		AddigyAPIKey:        addigyKey,
		FilePath:            filePath,
		IncludedOSVersions:  osVersionsToFilter,
		IncludedPolicyNames: policiesToFilter,
	}, nil
}
