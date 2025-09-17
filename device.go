package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"

	"github.com/dsrosen6/addigy-macos-compatibility/internal/addigy"
	"github.com/dsrosen6/addigy-macos-compatibility/internal/sofa"
)

type device struct {
	AgentID            string `json:"agentid"`
	Name               string `json:"name"`
	HardwareModel      string `json:"hardware_model"`
	PolicyName         string `json:"policy_name"`
	LatestCompatibleOS string `json:"latest_compatible"`
}

func devicesToCSV(devices []device, w io.Writer) error {
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

func getAndProcessAllDevices(ctx context.Context, a *addigy.Client, sd *sofa.DataResp, deviceParams map[string]any) ([]device, error) {
	var (
		allDevices  []device
		allPolicies = make(map[string]addigy.Policy)
	)

	addigyDevices, err := a.SearchDevices(ctx, 200, deviceParams)
	if err != nil {
		return nil, fmt.Errorf("getting device data from Addigy: %w", err)
	}

	for _, d := range addigyDevices {
		dev := processDeviceData(ctx, a, &d, sd, allPolicies)
		allDevices = append(allDevices, dev)
	}

	return allDevices, nil
}

func processDeviceData(ctx context.Context, a *addigy.Client, ad *addigy.Device, sd *sofa.DataResp, allPolicies map[string]addigy.Policy) device {
	d := &device{
		AgentID: ad.AgentID,
	}

	policyID, ok := ad.Facts["policy_id"].Value.(string)
	if !ok {
		d.PolicyName = "N/A"
	} else {
		p, err := a.GetPolicyByID(ctx, policyID, allPolicies)
		if err != nil {
			d.PolicyName = "ERROR"
		}

		d.PolicyName = p.Name
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
