package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	sofaDataURL = "https://sofafeed.macadmins.io/v1/macos_data_feed.json"
)

type sofaMacModel struct {
	MarketingName string   `json:"MarketingName"`
	SupportedOS   []string `json:"SupportedOS"`
	OSVersions    []int    `json:"OSVersions"`
}

type sofaData struct {
	Models map[string]sofaMacModel `json:"Models"`
}

func getSofaData(ctx context.Context, httpClient *http.Client) (*sofaData, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sofaDataURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}

	data := &sofaData{}
	if err := json.Unmarshal(body, data); err != nil {
		return nil, fmt.Errorf("unmarshaling json: %w", err)
	}

	return data, nil
}
