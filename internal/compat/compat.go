package compat

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/dsrosen6/addigy-macos-compatibility/internal/addigy"
	"github.com/dsrosen6/addigy-macos-compatibility/internal/sofa"
)

type Device struct {
	AgentID            string `json:"agentid"`
	Name               string `json:"name"`
	HardwareModel      string `json:"hardware_model"`
	PolicyName         string `json:"policy_name"`
	LatestCompatibleOS string `json:"latest_compatible"`
}

func RunReport() error {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	ctx := context.Background()
	httpClient := http.DefaultClient
	a := addigy.NewAddigyClient(httpClient, os.Getenv("ADDIGY_API_KEY"))

	fmt.Println("Fetching SOFA data...")
	sd, err := sofa.GetSofaData(ctx, httpClient)
	if err != nil {
		return fmt.Errorf("fetching SOFA data: %w", err)
	}

	if sd == nil {
		return fmt.Errorf("received no data from SOFA")
	}

	fmt.Println("Fetching and processing info from Addigy. This may take a few minutes...")
	devices, err := GetAndProcessAllDevices(ctx, a, sd, nil)
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

	fmt.Println("Creating CSV file...")
	if err := DevicesToCSV(devices, f); err != nil {
		return fmt.Errorf("writing data to csv: %w", err)
	}
	fmt.Printf("Done! CSV file available at %s\n", fp)

	return nil
}

func DevicesToCSV(devices []Device, w io.Writer) error {
	writer := csv.NewWriter(w)

	if err := writer.Write([]string{"Agent ID", "Name", "Hardware Model", "Policy Name", "Latest Compatible OS"}); err != nil {
		return fmt.Errorf("writing headers: %w", err)
	}

	for _, d := range devices {
		record := []string{d.AgentID, d.Name, d.HardwareModel, d.PolicyName, d.LatestCompatibleOS}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("writing record: %w", err)
		}
	}
	writer.Flush()
	return writer.Error()
}

func GetAndProcessAllDevices(ctx context.Context, a *addigy.Client, sd *sofa.DataResp, deviceParams map[string]any) ([]Device, error) {
	var (
		allDevices  []Device
		allPolicies = make(map[string]addigy.Policy)
	)

	addigyDevices, err := a.SearchDevices(ctx, 100, deviceParams)
	if err != nil {
		return nil, fmt.Errorf("getting Device data from Addigy: %w", err)
	}

	for _, d := range addigyDevices {
		dev := processDeviceData(ctx, a, &d, sd, allPolicies)
		allDevices = append(allDevices, dev)
	}

	return allDevices, nil
}

func processDeviceData(ctx context.Context, a *addigy.Client, ad *addigy.Device, sd *sofa.DataResp, allPolicies map[string]addigy.Policy) Device {
	d := &Device{
		AgentID: ad.AgentID,
	}

	policyID, ok := ad.Facts["policy_id"].Value.(string)
	if !ok {
		d.PolicyName = "N/A"
	} else {
		p, err := GetPolicyByID(ctx, a, policyID, allPolicies)
		if err != nil {
			slog.Error("getting policy by id", "policy_id", policyID, "error", err)
			d.PolicyName = "ERROR"
		} else {
			d.PolicyName = p.Name
		}
	}

	if d.Name, ok = ad.Facts["device_name"].Value.(string); !ok {
		d.Name = "N/A"
	}

	if d.HardwareModel, ok = ad.Facts["product_name"].Value.(string); !ok {
		d.HardwareModel = "N/A"
	}

	d.LatestCompatibleOS = sofa.GetLatestCompatibleOS(sd, d.HardwareModel)
	return *d
}

func GetPolicyByID(ctx context.Context, a *addigy.Client, policyID string, allPolicies map[string]addigy.Policy) (addigy.Policy, error) {
	if policy, ok := allPolicies[policyID]; ok {
		slog.Debug("cache hit for policy", "policy_id", policy.ID, "policy_name", policy.Name)
		return policy, nil
	}
	slog.Debug("cache miss for policy", "policy_id", policyID)

	params := map[string]any{
		"policies": []string{policyID},
	}

	fmt.Println("searching for policy:", policyID)
	policies, err := a.SearchPolicies(ctx, params)
	if err != nil {
		return addigy.Policy{}, fmt.Errorf("searching addigy policies: %w", err)
	}
	fmt.Println("policy search result:", policies)

	if len(policies) != 1 {
		return addigy.Policy{}, fmt.Errorf("received unexpected policy total - got %d, expected 1", len(policies))
	}

	allPolicies[policyID] = policies[0]
	slog.Debug("added policy to cache", "policy_id", policyID, "cache_size", len(allPolicies))
	return allPolicies[policyID], nil
}
