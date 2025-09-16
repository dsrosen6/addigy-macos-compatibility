package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/dsrosen6/addigy-macos-compatibility/internal/addigy"
	"github.com/dsrosen6/addigy-macos-compatibility/internal/sofa"
)

func main() {
	if err := runReport(); err != nil {
		fmt.Printf("An error occured: %v\n", err)
		os.Exit(1)
	}
}

func runReport() error {
	ctx := context.Background()
	httpClient := http.DefaultClient
	a := addigy.NewAddigyClient(httpClient, os.Getenv("ADDIGY_API_KEY"))

	sd, err := sofa.GetSofaData(ctx, httpClient)
	if err != nil {
		return fmt.Errorf("fetching SOFA data: %w", err)
	}

	if sd == nil {
		return fmt.Errorf("received no data from SOFA")
	}

	params := map[string]any{
		"per_page": 2000,
	}

	devices, err := getAndProcessAllDevices(ctx, a, sd, params)
	if err != nil {
		return fmt.Errorf("fetching all devices: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting user home directory")
	}

	fp := filepath.Join(home, "Downloads/device_compatibility.csv")
	f, err := os.Create(fp)
	if err != nil {
		return fmt.Errorf("creating csv file: %w", err)
	}
	defer f.Close()

	if err := devicesToCSV(devices, f); err != nil {
		return fmt.Errorf("writing data to csv: %w", err)
	}

	return nil
}
