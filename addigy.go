package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	addigyURL = "https://api.addigy.com/api/v2"
)

var (
	// map of policy ids to names
	allPolicies = make(map[string]addigyPolicy)
)

type addigyDeviceFact struct {
	Value    any    `json:"value"`
	Type     string `json:"type"`
	ErrorMsg string `json:"error_msg"`
}

type addigyDeviceSearchResp struct {
	Devices  []addigyDevice `json:"items"`
	Metadata struct {
		Page        int `json:"page"`
		PerPage     int `json:"per_page"`
		PageCount   int `json:"page_count"`
		ResultCount int `json:"result_count"`
		Total       int `json:"total"`
	} `json:"metadata"`
}

type addigyPolicy struct {
	PolicyID       string  `json:"policyId"`
	ParentPolicyID *string `json:"parent"`
	Name           string  `json:"name"`
}

type addigyDevice struct {
	AgentID       string                      `json:"agentid"`
	Facts         map[string]addigyDeviceFact `json:"facts"`
	name          string
	hardwareModel string
	policy        addigyPolicy
}

type addigyClient struct {
	httpClient *http.Client
	apiKey     string
}

func newAddigyClient(httpClient *http.Client, apiKey string) *addigyClient {
	return &addigyClient{
		httpClient: httpClient,
		apiKey:     apiKey,
	}
}

func (a *addigyClient) processDeviceData(ctx context.Context, device *addigyDevice) (*addigyDevice, error) {
	policyID, ok := device.Facts["policy_id"].Value.(string)
	if !ok {
		return nil, fmt.Errorf("getting policy id")
	}

	if device.name, ok = device.Facts["device_name"].Value.(string); !ok {
		device.name = "Unknown"
	}

	if device.hardwareModel, ok = device.Facts["hardware_model"].Value.(string); !ok {
		device.hardwareModel = "Unknown"
	}

	var err error
	device.policy, err = a.getPolicyByID(ctx, policyID)
	if err != nil {
		return nil, fmt.Errorf("getting policy info: %w", err)
	}

	return device, nil
}

func (a *addigyClient) getDevices(ctx context.Context, params map[string]any) ([]addigyDevice, error) {
	url := fmt.Sprintf("%s/%s", addigyURL, "devices")

	var payload io.Reader
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshaling params: %w", err)
		}
		payload = bytes.NewBuffer(b)
	}

	devices := &addigyDeviceSearchResp{}
	if err := a.doAPIRequest(ctx, http.MethodPost, url, payload, devices); err != nil {
		return nil, fmt.Errorf("running API request: %w", err)
	}

	return devices.Devices, nil
}

func (a *addigyClient) getPolicyByID(ctx context.Context, policyID string) (addigyPolicy, error) {
	if policy, ok := allPolicies[policyID]; ok {
		return policy, nil
	}

	url := fmt.Sprintf("%s/%s", addigyURL, "oa/policies/query")
	params := map[string]any{
		"policies": []string{policyID},
	}

	b, err := json.Marshal(params)
	if err != nil {
		return addigyPolicy{}, fmt.Errorf("marshaling params: %w", err)
	}
	payload := bytes.NewBuffer(b)

	var policies []addigyPolicy
	if err := a.doAPIRequest(ctx, http.MethodPost, url, payload, &policies); err != nil {
		return addigyPolicy{}, fmt.Errorf("running API request: %w", err)
	}

	if len(policies) != 1 {
		return addigyPolicy{}, fmt.Errorf("received unexpected policy total - got %d, expected 1", len(policies))
	}

	allPolicies[policyID] = policies[0]
	return allPolicies[policyID], nil
}

func (a *addigyClient) doAPIRequest(ctx context.Context, method, url string, payload io.Reader, target any) error {
	req, err := http.NewRequestWithContext(ctx, method, url, payload)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Add("x-api-key", a.apiKey)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: status: %d, body: %s", resp.StatusCode, string(body))
	}
	if target != nil {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("converting response to bytes: %w", err)
		}

		if err := json.Unmarshal(body, target); err != nil {
			return fmt.Errorf("unmarshaling response to json: %w", err)
		}
	}

	return nil
}
