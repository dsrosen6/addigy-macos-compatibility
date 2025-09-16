package main

import (
	"context"
	"fmt"
)

type device struct {
	AgentID            string `json:"agentid"`
	Name               string `json:"name"`
	HardwareModel      string `json:"hardware_model"`
	PolicyName         string `json:"policy_name"`
	LatestCompatibleOS string `json:"latest_compatible"`
}

func getAllDevices(ctx context.Context, a *addigyClient, sd *sofaData, deviceParams map[string]any) ([]device, error) {
	var (
		allDevices []device
		policies   = make(map[string]addigyPolicy)
	)

	addigyDevices, err := a.getDevices(ctx, deviceParams)
	if err != nil {
		return nil, fmt.Errorf("getting device data from Addigy: %w", err)
	}

	for _, d := range addigyDevices {

	}
}
