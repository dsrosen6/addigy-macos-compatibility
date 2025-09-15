package main

import "net/http"

type sofaMacModel struct {
	MarketingName string   `json:"MarketingName"`
	SupportedOS   []string `json:"SupportedOS"`
	OSVersions    []int    `json:"OSVersions"`
}

type sofaFeed struct {
	Models map[string]sofaMacModel `json:"Models"`
}

type addigyClient struct {
	httpClient *http.Client
	apiKey     string
}
