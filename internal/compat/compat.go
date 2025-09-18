package compat

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/dsrosen6/addigy-macos-compatibility/internal/addigy"
	"github.com/dsrosen6/addigy-macos-compatibility/internal/sofa"
)

var (
	sofaData    *sofa.Data
	allPolicies = make(map[string]addigy.Policy)
)

type Options struct {
	Debug               bool
	AddigyAPIKey        string
	FilePath            string
	IncludedOSVersions  []string
	IncludedPolicyNames []string
}

type Client struct {
	httpClient   *http.Client
	addigyClient *addigy.Client
	opts         Options
}

type Device struct {
	AgentID            string `json:"agentid"`
	Name               string `json:"name"`
	HardwareModel      string `json:"hardware_model"`
	PolicyName         string `json:"policy_name"`
	LatestCompatibleOS string `json:"latest_compatible"`
}

func Run(opts Options) error {
	var (
		err error
	)

	ctx := context.Background()

	c := newClient(opts)
	if c.opts.Debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	fmt.Println("Fetching SOFA data...")
	sofaData, err = sofa.GetSofaData(ctx, c.httpClient)
	if err != nil {
		return fmt.Errorf("fetching SOFA data: %w", err)
	}

	if sofaData == nil {
		return fmt.Errorf("received no data from SOFA")
	}

	if len(c.opts.IncludedPolicyNames) > 0 || len(c.opts.IncludedOSVersions) > 0 {
		if err := c.runReportWithOptions(ctx); err != nil {
			return fmt.Errorf("running report with options: %w", err)
		}
	} else {
		if err := c.runFullReport(ctx); err != nil {
			return fmt.Errorf("running full report: %w", err)
		}
	}

	return nil
}

func newClient(opts Options) *Client {
	h := http.DefaultClient
	return &Client{
		httpClient:   h,
		addigyClient: addigy.NewAddigyClient(h, opts.AddigyAPIKey),
		opts:         opts,
	}
}

func (c *Client) runFullReport(ctx context.Context) error {
	fmt.Println("Fetching and processing info from Addigy. This may take a few minutes...")
	devices, err := c.getAndProcessDevices(ctx, nil)
	if err != nil {
		return fmt.Errorf("getting and processing all devices: %w", err)
	}

	fmt.Println("Creating CSV file...")
	f, err := os.Create(c.opts.FilePath)
	if err != nil {
		return fmt.Errorf("creating csv file: %w", err)
	}
	defer f.Close()

	if err := devicesToCSV(devices, f); err != nil {
		return fmt.Errorf("writing data to CSV: %w", err)
	}
	fmt.Println("Done! CSV file available at:", c.opts.FilePath)

	return nil
}

func (c *Client) runReportWithOptions(ctx context.Context) error {
	params := make(map[string]any)
	if len(c.opts.IncludedPolicyNames) > 0 {
		// get policies by name
		policyIDs, err := c.addigyClient.GetPolicyIDsByName(ctx, c.opts.IncludedPolicyNames)
		if err != nil {
			return fmt.Errorf("getting policies: %w", err)
		}
		//TODO: add policy ids param
	}

	devices, err := c.getAndProcessDevices(ctx, params)
	if err != nil {
		return fmt.Errorf("getting and processing devices: %w", err)
	}
	
}

func devicesToCSV(devices []Device, w io.Writer) error {
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

func (c *Client) getAndProcessDevices(ctx context.Context, deviceParams map[string]any) ([]Device, error) {
	var allDevices []Device
	addigyDevices, err := c.addigyClient.SearchDevices(ctx, 100, deviceParams)
	if err != nil {
		return nil, fmt.Errorf("getting Device data from Addigy: %w", err)
	}

	for _, d := range addigyDevices {
		dev := c.processDeviceData(ctx, &d)
		allDevices = append(allDevices, dev)
	}

	return allDevices, nil
}

func (c *Client) processDeviceData(ctx context.Context, ad *addigy.Device) Device {
	d := &Device{
		AgentID: ad.AgentID,
	}

	policyID, ok := ad.Facts["policy_id"].Value.(string)
	if !ok {
		d.PolicyName = "N/A"
	} else {
		p, err := c.getPolicyByID(ctx, policyID)
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

	d.LatestCompatibleOS = sofa.GetLatestCompatibleOS(sofaData, d.HardwareModel)
	return *d
}

func (c *Client) getPolicyByID(ctx context.Context, policyID string) (addigy.Policy, error) {
	if policy, ok := allPolicies[policyID]; ok {
		slog.Debug("cache hit for policy", "policy_id", policy.ID, "policy_name", policy.Name)
		return policy, nil
	}
	slog.Debug("cache miss for policy", "policy_id", policyID)

	params := map[string]any{
		"policies": []string{policyID},
	}

	fmt.Println("searching for policy:", policyID)
	policies, err := c.addigyClient.SearchPolicies(ctx, params)
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
