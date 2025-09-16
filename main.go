package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
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
	addigyClient := newAddigyClient(httpClient, os.Getenv("ADDIGY_API_KEY"))

	sofaData, err := getSofaData(ctx, httpClient)
	if err != nil {
		return fmt.Errorf("fetching SOFA data: %w", err)
	}

	if sofaData == nil {
		return fmt.Errorf("received no data from SOFA")
	}

	params := map[string]any{
		"per_page": 10,
	}

	devices, err := addigyClient.getDevices(ctx, params)
	if err != nil {
		return fmt.Errorf("fetching Addigy devices: %w", err)
	}

	for _, d := range devices {
		device, err := addigyClient.processDeviceData(ctx, &d)
		if err != nil {
			return fmt.Errorf("processing data for device %s: %w", d.name, err)
		}
		fmt.Printf("%s - Policy Name: %s\n", device.name, device.policy.Name)
	}
	fmt.Printf("Found %d devices\n", len(devices))
	return nil
}
