package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/globulario/services/golang/dnsprovider"
)

const (
	defaultBaseURL = "https://api.cloudflare.com/client/v4"
)

func init() {
	dnsprovider.Register("cloudflare", NewCloudflareProvider)
}

// CloudflareProvider implements DNS record management via Cloudflare API.
type CloudflareProvider struct {
	apiToken string
	zone     string
	zoneID   string
	baseURL  string
	client   *http.Client
	ttl      int
}

// NewCloudflareProvider creates a new Cloudflare DNS provider.
func NewCloudflareProvider(cfg dnsprovider.Config) (dnsprovider.Provider, error) {
	apiToken, ok := cfg.Credentials["api_token"]
	if !ok || apiToken == "" {
		return nil, fmt.Errorf("cloudflare: api_token is required in credentials")
	}

	baseURL := defaultBaseURL
	if customURL, ok := cfg.Credentials["base_url"]; ok && customURL != "" {
		baseURL = strings.TrimSuffix(customURL, "/")
	}

	timeout := 30 * time.Second
	if cfg.Timeout > 0 {
		timeout = cfg.Timeout
	}

	p := &CloudflareProvider{
		apiToken: apiToken,
		zone:     cfg.Zone,
		baseURL:  baseURL,
		client: &http.Client{
			Timeout: timeout,
		},
		ttl: cfg.DefaultTTL,
	}

	// Get zone ID
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	zoneID, err := p.getZoneID(ctx)
	if err != nil {
		return nil, fmt.Errorf("cloudflare: failed to get zone ID: %w", err)
	}
	p.zoneID = zoneID

	return p, nil
}

func (p *CloudflareProvider) Name() string {
	return "cloudflare"
}

// Cloudflare API structures
type cloudflareResponse struct {
	Success  bool                   `json:"success"`
	Errors   []cloudflareError      `json:"errors"`
	Messages []cloudflareMessage    `json:"messages"`
	Result   json.RawMessage        `json:"result"`
}

type cloudflareError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type cloudflareMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type cloudflareZone struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type cloudflareDNSRecord struct {
	ID       string `json:"id,omitempty"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Content  string `json:"content"`
	TTL      int    `json:"ttl"`
	Priority int    `json:"priority,omitempty"`
	Proxied  bool   `json:"proxied,omitempty"`
}

// getZoneID retrieves the zone ID for the configured zone
func (p *CloudflareProvider) getZoneID(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/zones?name=%s", p.baseURL, p.zone)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	p.setAuthHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var cfResp cloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return "", err
	}

	if !cfResp.Success {
		return "", fmt.Errorf("API error: %v", cfResp.Errors)
	}

	var zones []cloudflareZone
	if err := json.Unmarshal(cfResp.Result, &zones); err != nil {
		return "", err
	}

	if len(zones) == 0 {
		return "", fmt.Errorf("zone %q not found", p.zone)
	}

	return zones[0].ID, nil
}

func (p *CloudflareProvider) UpsertA(ctx context.Context, zone string, name string, ip string, ttl int) error {
	if err := p.validateZone(zone); err != nil {
		return err
	}

	record := cloudflareDNSRecord{
		Type:    "A",
		Name:    p.constructFQDN(name),
		Content: ip,
		TTL:     p.resolveTTL(ttl),
		Proxied: false, // Don't proxy DNS-only records
	}

	return p.upsertRecord(ctx, name, "A", record)
}

func (p *CloudflareProvider) UpsertAAAA(ctx context.Context, zone string, name string, ip string, ttl int) error {
	if err := p.validateZone(zone); err != nil {
		return err
	}

	record := cloudflareDNSRecord{
		Type:    "AAAA",
		Name:    p.constructFQDN(name),
		Content: ip,
		TTL:     p.resolveTTL(ttl),
		Proxied: false,
	}

	return p.upsertRecord(ctx, name, "AAAA", record)
}

func (p *CloudflareProvider) UpsertCNAME(ctx context.Context, zone string, name string, target string, ttl int) error {
	if err := p.validateZone(zone); err != nil {
		return err
	}

	// Cloudflare expects CNAME target with trailing dot
	if !strings.HasSuffix(target, ".") {
		target = target + "."
	}

	record := cloudflareDNSRecord{
		Type:    "CNAME",
		Name:    p.constructFQDN(name),
		Content: target,
		TTL:     p.resolveTTL(ttl),
		Proxied: false,
	}

	return p.upsertRecord(ctx, name, "CNAME", record)
}

func (p *CloudflareProvider) UpsertTXT(ctx context.Context, zone string, name string, values []string, ttl int) error {
	if err := p.validateZone(zone); err != nil {
		return err
	}

	// Delete existing TXT records first
	if err := p.deleteRecordsByType(ctx, name, "TXT"); err != nil {
		return err
	}

	// Create new TXT records
	for _, val := range values {
		record := cloudflareDNSRecord{
			Type:    "TXT",
			Name:    p.constructFQDN(name),
			Content: val,
			TTL:     p.resolveTTL(ttl),
		}

		if err := p.createRecord(ctx, record); err != nil {
			return err
		}
	}

	return nil
}

func (p *CloudflareProvider) DeleteTXT(ctx context.Context, zone string, name string, values []string) error {
	if err := p.validateZone(zone); err != nil {
		return err
	}

	if len(values) == 0 {
		// Delete all TXT records for this name
		return p.deleteRecordsByType(ctx, name, "TXT")
	}

	// Get current records
	records, err := p.listRecords(ctx, name, "TXT")
	if err != nil {
		return err
	}

	// Delete specific values
	valuesToDelete := make(map[string]bool)
	for _, v := range values {
		valuesToDelete[v] = true
	}

	for _, rec := range records {
		if valuesToDelete[rec.Content] {
			if err := p.deleteRecordByID(ctx, rec.ID); err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *CloudflareProvider) GetRecords(ctx context.Context, zone string, name string, rtype string) ([]dnsprovider.Record, error) {
	if err := p.validateZone(zone); err != nil {
		return nil, err
	}

	records, err := p.listRecords(ctx, name, rtype)
	if err != nil {
		return nil, err
	}

	return p.convertRecords(records), nil
}

// Helper methods

func (p *CloudflareProvider) upsertRecord(ctx context.Context, name, rtype string, record cloudflareDNSRecord) error {
	// Check if record exists
	existing, err := p.listRecords(ctx, name, rtype)
	if err != nil {
		return err
	}

	// Delete existing records of this type
	for _, rec := range existing {
		if err := p.deleteRecordByID(ctx, rec.ID); err != nil {
			return err
		}
	}

	// Create new record
	return p.createRecord(ctx, record)
}

func (p *CloudflareProvider) createRecord(ctx context.Context, record cloudflareDNSRecord) error {
	url := fmt.Sprintf("%s/zones/%s/dns_records", p.baseURL, p.zoneID)

	body, err := json.Marshal(record)
	if err != nil {
		return p.wrapError("createRecord", p.zone, record.Name, err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return p.wrapError("createRecord", p.zone, record.Name, err)
	}

	req.Header.Set("Content-Type", "application/json")
	p.setAuthHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return p.wrapError("createRecord", p.zone, record.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return p.wrapError("createRecord", p.zone, record.Name, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body)))
	}

	var cfResp cloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return p.wrapError("createRecord", p.zone, record.Name, err)
	}

	if !cfResp.Success {
		return p.wrapError("createRecord", p.zone, record.Name, fmt.Errorf("API error: %v", cfResp.Errors))
	}

	return nil
}

func (p *CloudflareProvider) listRecords(ctx context.Context, name, rtype string) ([]cloudflareDNSRecord, error) {
	fqdn := p.constructFQDN(name)
	url := fmt.Sprintf("%s/zones/%s/dns_records?name=%s", p.baseURL, p.zoneID, fqdn)

	if rtype != "" {
		url += "&type=" + rtype
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, p.wrapError("listRecords", p.zone, name, err)
	}

	p.setAuthHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, p.wrapError("listRecords", p.zone, name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, p.wrapError("listRecords", p.zone, name, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body)))
	}

	var cfResp cloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return nil, p.wrapError("listRecords", p.zone, name, err)
	}

	if !cfResp.Success {
		return nil, p.wrapError("listRecords", p.zone, name, fmt.Errorf("API error: %v", cfResp.Errors))
	}

	var records []cloudflareDNSRecord
	if err := json.Unmarshal(cfResp.Result, &records); err != nil {
		return nil, p.wrapError("listRecords", p.zone, name, err)
	}

	return records, nil
}

func (p *CloudflareProvider) deleteRecordByID(ctx context.Context, recordID string) error {
	url := fmt.Sprintf("%s/zones/%s/dns_records/%s", p.baseURL, p.zoneID, recordID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	p.setAuthHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (p *CloudflareProvider) deleteRecordsByType(ctx context.Context, name, rtype string) error {
	records, err := p.listRecords(ctx, name, rtype)
	if err != nil {
		return err
	}

	for _, rec := range records {
		if err := p.deleteRecordByID(ctx, rec.ID); err != nil {
			return err
		}
	}

	return nil
}

func (p *CloudflareProvider) setAuthHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+p.apiToken)
}

func (p *CloudflareProvider) validateZone(zone string) error {
	if zone != p.zone {
		return &dnsprovider.ProviderError{
			Provider: "cloudflare",
			Op:       "validateZone",
			Zone:     zone,
			Err:      fmt.Errorf("zone mismatch: expected %q, got %q", p.zone, zone),
		}
	}
	return nil
}

func (p *CloudflareProvider) resolveTTL(ttl int) int {
	if ttl > 0 {
		return ttl
	}
	if p.ttl > 0 {
		return p.ttl
	}
	return 1 // Cloudflare default (1 = automatic)
}

func (p *CloudflareProvider) constructFQDN(name string) string {
	if name == "@" || name == "" {
		return p.zone
	}
	if strings.HasSuffix(name, "."+p.zone) {
		return name
	}
	if strings.HasSuffix(name, ".") {
		return strings.TrimSuffix(name, ".")
	}
	return name + "." + p.zone
}

func (p *CloudflareProvider) wrapError(op, zone, name string, err error) error {
	return &dnsprovider.ProviderError{
		Provider: "cloudflare",
		Op:       op,
		Zone:     zone,
		Name:     name,
		Err:      err,
	}
}

func (p *CloudflareProvider) convertRecords(cfRecords []cloudflareDNSRecord) []dnsprovider.Record {
	records := make([]dnsprovider.Record, len(cfRecords))
	for i, rec := range cfRecords {
		records[i] = dnsprovider.Record{
			Zone:  p.zone,
			Name:  strings.TrimSuffix(rec.Name, "."+p.zone),
			Type:  rec.Type,
			Value: rec.Content,
			TTL:   rec.TTL,
		}
	}
	return records
}
