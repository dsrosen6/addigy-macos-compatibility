package addigy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

const (
	addigyURL = "https://api.addigy.com/api/v2"
)

type Policy struct {
	ID             string  `json:"ID"`
	ParentPolicyID *string `json:"parent"`
	Name           string  `json:"name"`
}

type Device struct {
	AgentID string                `json:"agentid"`
	Facts   map[string]DeviceFact `json:"facts"`
}
type DeviceFact struct {
	Value    any    `json:"value"`
	Type     string `json:"type"`
	ErrorMsg string `json:"error_msg"`
}

type Client struct {
	httpClient *http.Client
	apiKey     string
}

type devicesSearchResp struct {
	Devices  []Device `json:"items"`
	Metadata metadata `json:"metadata"`
}

type metadata struct {
	Page        int `json:"page"`
	PerPage     int `json:"per_page"`
	PageCount   int `json:"page_count"`
	ResultCount int `json:"result_count"`
	Total       int `json:"total"`
}

func NewAddigyClient(httpClient *http.Client, apiKey string) *Client {
	return &Client{
		httpClient: httpClient,
		apiKey:     apiKey,
	}
}

func (a *Client) SearchDevices(ctx context.Context, perPage int, baseParams map[string]any) ([]Device, error) {
	url := fmt.Sprintf("%s/%s", addigyURL, "devices")
	var devices []Device
	page := 1

	for {
		params := make(map[string]any)
		for k, v := range baseParams {
			params[k] = v
		}
		params["page"] = page
		params["per_page"] = perPage

		b, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshaling params: %w", err)
		}
		payload := bytes.NewBuffer(b)

		resp := &devicesSearchResp{}
		if err := a.doAPIRequest(ctx, http.MethodPost, url, payload, resp); err != nil {
			return nil, fmt.Errorf("running api request: %w", err)
		}
		devices = append(devices, resp.Devices...)
		slog.Debug("got batch of devices from addigy", "new_devices", len(resp.Devices), "total_devices", len(devices))

		if resp.Metadata.Page >= resp.Metadata.PageCount {
			break
		}

		page++
	}

	return devices, nil
}

func (a *Client) SearchPolicies(ctx context.Context, params map[string]any) ([]Policy, error) {
	url := fmt.Sprintf("%s/%s", addigyURL, "oa/policies/query")

	b, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshaling params: %w", err)
	}
	payload := bytes.NewBuffer(b)

	var policies []Policy
	if err := a.doAPIRequest(ctx, http.MethodPost, url, payload, &policies); err != nil {
		return nil, fmt.Errorf("running api request: %w", err)
	}

	return policies, nil
}

func (a *Client) doAPIRequest(ctx context.Context, method, url string, payload io.Reader, target any) error {
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
