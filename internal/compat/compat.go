package compat

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"

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
	IncludedOSVersions  []int
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
	LatestCompatibleOS int    `json:"latest_compatible"`
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

	if err := c.runMaxOSReport(ctx); err != nil {
		return fmt.Errorf("running full report: %w", err)
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

func (c *Client) runMaxOSReport(ctx context.Context) error {
	fmt.Println("Fetching and processing info from Addigy. This may take a few minutes...")
	devices, err := c.getAndProcessDevices(ctx)
	if err != nil {
		return fmt.Errorf("getting and processind devices: %w", err)
	}

	if len(c.opts.IncludedOSVersions) > 0 {
		fmt.Println("Filtering by OS version(s):", c.opts.IncludedOSVersions)
		devices = filterDevicesByMaxOS(devices, c.opts.IncludedOSVersions)
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

func (c *Client) getAndProcessDevices(ctx context.Context) ([]Device, error) {
	params := make(map[string]any)
	if len(c.opts.IncludedPolicyNames) > 0 {
		// get policies by name
		fmt.Println("Filtering by policies:", strings.Join(c.opts.IncludedPolicyNames, ", "))
		policyIDs, err := c.addigyClient.GetPolicyIDsByName(ctx, c.opts.IncludedPolicyNames)
		if err != nil {
			return nil, fmt.Errorf("getting policies: %w", err)
		}

		if len(policyIDs) == 0 {
			return nil, errors.New("did not find any policies")
		}

		f := addigy.DeviceSearchFilter{
			AuditField: "policy_ids",
			Operation:  "contains",
			Type:       "list",
			Value:      policyIDs,
		}

		params["query"] = addigy.DeviceSearchPayload{
			Filters: []addigy.DeviceSearchFilter{f},
		}
	}

	slog.Debug("device search params", "params", params)
	var allDevices []Device
	addigyDevices, err := c.addigyClient.SearchDevices(ctx, 100, params)
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
			slog.Debug("getting policy by id", "policy_id", policyID, "error", err)
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

	policies, err := c.addigyClient.SearchPolicies(ctx, params)
	if err != nil {
		return addigy.Policy{}, fmt.Errorf("searching addigy policies: %w", err)
	}

	if len(policies) != 1 {
		return addigy.Policy{}, fmt.Errorf("received unexpected policy total - got %d, expected 1", len(policies))
	}

	allPolicies[policyID] = policies[0]
	slog.Debug("added policy to cache", "policy_id", policyID, "cache_size", len(allPolicies))
	return allPolicies[policyID], nil
}

func filterDevicesByMaxOS(devices []Device, osVersions []int) []Device {
	var filtered []Device
	for _, d := range devices {
		if slices.Contains(osVersions, d.LatestCompatibleOS) {
			filtered = append(filtered, d)
			slog.Debug("adding device to filtered list", "device_name", d.Name, "max_os", d.LatestCompatibleOS)
		} else {
			slog.Debug("device does not match max os filter", "device_name", d.Name, "max_os", d.LatestCompatibleOS)
		}
	}

	return filtered
}

func devicesToCSV(devices []Device, w io.Writer) error {
	writer := csv.NewWriter(w)

	if err := writer.Write([]string{"Agent ID", "Name", "Hardware Model", "Policy Name", "Latest Compatible OS"}); err != nil {
		return fmt.Errorf("writing headers: %w", err)
	}

	for _, d := range devices {
		osVers := strconv.Itoa(d.LatestCompatibleOS)
		if osVers == "0" {
			osVers = "Unsupported"
		}

		record := []string{d.AgentID, d.Name, d.HardwareModel, d.PolicyName, osVers}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("writing record: %w", err)
		}
	}
	writer.Flush()
	return writer.Error()
}
