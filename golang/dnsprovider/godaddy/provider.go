package godaddy

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
	defaultBaseURL = "https://api.godaddy.com"
	apiVersion     = "v1"
)

func init() {
	dnsprovider.Register("godaddy", NewGoDaddyProvider)
}

// GoDaddyProvider implements DNS record management via GoDaddy API.
type GoDaddyProvider struct {
	apiKey    string
	apiSecret string
	zone      string
	baseURL   string
	client    *http.Client
	ttl       int
}

// NewGoDaddyProvider creates a new GoDaddy DNS provider.
func NewGoDaddyProvider(cfg dnsprovider.Config) (dnsprovider.Provider, error) {
	apiKey, ok := cfg.Credentials["api_key"]
	if !ok || apiKey == "" {
		return nil, fmt.Errorf("godaddy: api_key is required in credentials")
	}

	apiSecret, ok := cfg.Credentials["api_secret"]
	if !ok || apiSecret == "" {
		return nil, fmt.Errorf("godaddy: api_secret is required in credentials")
	}

	baseURL := defaultBaseURL
	if customURL, ok := cfg.Credentials["base_url"]; ok && customURL != "" {
		baseURL = strings.TrimSuffix(customURL, "/")
	}

	timeout := 30 * time.Second
	if cfg.Timeout > 0 {
		timeout = cfg.Timeout
	}

	return &GoDaddyProvider{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		zone:      cfg.Zone,
		baseURL:   baseURL,
		client: &http.Client{
			Timeout: timeout,
		},
		ttl: cfg.DefaultTTL,
	}, nil
}

func (p *GoDaddyProvider) Name() string {
	return "godaddy"
}

// GoDaddy API record structure
type godaddyRecord struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Data     string `json:"data"`
	TTL      int    `json:"ttl,omitempty"`
	Priority int    `json:"priority,omitempty"`
}

func (p *GoDaddyProvider) UpsertA(ctx context.Context, zone string, name string, ip string, ttl int) error {
	if err := p.validateZone(zone); err != nil {
		return err
	}

	record := godaddyRecord{
		Type: "A",
		Name: name,
		Data: ip,
		TTL:  p.resolveTTL(ttl),
	}

	return p.upsertRecord(ctx, name, "A", []godaddyRecord{record})
}

func (p *GoDaddyProvider) UpsertAAAA(ctx context.Context, zone string, name string, ip string, ttl int) error {
	if err := p.validateZone(zone); err != nil {
		return err
	}

	record := godaddyRecord{
		Type: "AAAA",
		Name: name,
		Data: ip,
		TTL:  p.resolveTTL(ttl),
	}

	return p.upsertRecord(ctx, name, "AAAA", []godaddyRecord{record})
}

func (p *GoDaddyProvider) UpsertCNAME(ctx context.Context, zone string, name string, target string, ttl int) error {
	if err := p.validateZone(zone); err != nil {
		return err
	}

	// GoDaddy expects CNAME target without trailing dot
	target = strings.TrimSuffix(target, ".")

	record := godaddyRecord{
		Type: "CNAME",
		Name: name,
		Data: target,
		TTL:  p.resolveTTL(ttl),
	}

	return p.upsertRecord(ctx, name, "CNAME", []godaddyRecord{record})
}

func (p *GoDaddyProvider) UpsertTXT(ctx context.Context, zone string, name string, values []string, ttl int) error {
	if err := p.validateZone(zone); err != nil {
		return err
	}

	// GoDaddy supports multiple TXT records with the same name
	records := make([]godaddyRecord, len(values))
	for i, val := range values {
		records[i] = godaddyRecord{
			Type: "TXT",
			Name: name,
			Data: val,
			TTL:  p.resolveTTL(ttl),
		}
	}

	return p.upsertRecord(ctx, name, "TXT", records)
}

func (p *GoDaddyProvider) DeleteTXT(ctx context.Context, zone string, name string, values []string) error {
	if err := p.validateZone(zone); err != nil {
		return err
	}

	if len(values) == 0 {
		// Delete all TXT records for this name
		return p.deleteRecord(ctx, name, "TXT")
	}

	// Get current records
	current, err := p.getRecords(ctx, name, "TXT")
	if err != nil {
		return err
	}

	// Filter out the values to delete
	valuesToDelete := make(map[string]bool)
	for _, v := range values {
		valuesToDelete[v] = true
	}

	remaining := make([]godaddyRecord, 0)
	for _, rec := range current {
		if !valuesToDelete[rec.Data] {
			remaining = append(remaining, rec)
		}
	}

	// If no records remain, delete the record set
	if len(remaining) == 0 {
		return p.deleteRecord(ctx, name, "TXT")
	}

	// Otherwise, update with remaining records
	return p.upsertRecord(ctx, name, "TXT", remaining)
}

func (p *GoDaddyProvider) GetRecords(ctx context.Context, zone string, name string, rtype string) ([]dnsprovider.Record, error) {
	if err := p.validateZone(zone); err != nil {
		return nil, err
	}

	// If both name and type specified, get specific records
	if name != "" && rtype != "" {
		records, err := p.getRecords(ctx, name, rtype)
		if err != nil {
			return nil, err
		}
		return p.convertRecords(records), nil
	}

	// Get all records for the domain (or filtered by name/type)
	var url string
	if name != "" && rtype != "" {
		url = fmt.Sprintf("%s/%s/domains/%s/records/%s/%s", p.baseURL, apiVersion, zone, rtype, name)
	} else if rtype != "" {
		url = fmt.Sprintf("%s/%s/domains/%s/records/%s", p.baseURL, apiVersion, zone, rtype)
	} else if name != "" {
		// GoDaddy API doesn't support filtering by name only, get all and filter
		url = fmt.Sprintf("%s/%s/domains/%s/records", p.baseURL, apiVersion, zone)
	} else {
		url = fmt.Sprintf("%s/%s/domains/%s/records", p.baseURL, apiVersion, zone)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, p.wrapError("GetRecords", zone, name, err)
	}

	p.setAuthHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, p.wrapError("GetRecords", zone, name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return []dnsprovider.Record{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, p.wrapError("GetRecords", zone, name, fmt.Errorf("API error: %d %s", resp.StatusCode, string(body)))
	}

	var records []godaddyRecord
	if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
		return nil, p.wrapError("GetRecords", zone, name, err)
	}

	return p.convertRecords(records), nil
}

// Helper methods

func (p *GoDaddyProvider) upsertRecord(ctx context.Context, name, rtype string, records []godaddyRecord) error {
	// GoDaddy API endpoint: PUT /v1/domains/{domain}/records/{type}/{name}
	url := fmt.Sprintf("%s/%s/domains/%s/records/%s/%s",
		p.baseURL, apiVersion, p.zone, rtype, name)

	body, err := json.Marshal(records)
	if err != nil {
		return p.wrapError("upsertRecord", p.zone, name, err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
	if err != nil {
		return p.wrapError("upsertRecord", p.zone, name, err)
	}

	req.Header.Set("Content-Type", "application/json")
	p.setAuthHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return p.wrapError("upsertRecord", p.zone, name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return p.wrapError("upsertRecord", p.zone, name, fmt.Errorf("API error: %d %s", resp.StatusCode, string(body)))
	}

	return nil
}

func (p *GoDaddyProvider) deleteRecord(ctx context.Context, name, rtype string) error {
	// GoDaddy API endpoint: DELETE /v1/domains/{domain}/records/{type}/{name}
	url := fmt.Sprintf("%s/%s/domains/%s/records/%s/%s",
		p.baseURL, apiVersion, p.zone, rtype, name)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return p.wrapError("deleteRecord", p.zone, name, err)
	}

	p.setAuthHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return p.wrapError("deleteRecord", p.zone, name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return p.wrapError("deleteRecord", p.zone, name, fmt.Errorf("API error: %d %s", resp.StatusCode, string(body)))
	}

	return nil
}

func (p *GoDaddyProvider) getRecords(ctx context.Context, name, rtype string) ([]godaddyRecord, error) {
	url := fmt.Sprintf("%s/%s/domains/%s/records/%s/%s",
		p.baseURL, apiVersion, p.zone, rtype, name)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, p.wrapError("getRecords", p.zone, name, err)
	}

	p.setAuthHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, p.wrapError("getRecords", p.zone, name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return []godaddyRecord{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, p.wrapError("getRecords", p.zone, name, fmt.Errorf("API error: %d %s", resp.StatusCode, string(body)))
	}

	var records []godaddyRecord
	if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
		return nil, p.wrapError("getRecords", p.zone, name, err)
	}

	return records, nil
}

func (p *GoDaddyProvider) setAuthHeaders(req *http.Request) {
	req.Header.Set("Authorization", fmt.Sprintf("sso-key %s:%s", p.apiKey, p.apiSecret))
}

func (p *GoDaddyProvider) validateZone(zone string) error {
	if zone != p.zone {
		return &dnsprovider.ProviderError{
			Provider: "godaddy",
			Op:       "validateZone",
			Zone:     zone,
			Err:      fmt.Errorf("zone mismatch: expected %q, got %q", p.zone, zone),
		}
	}
	return nil
}

func (p *GoDaddyProvider) resolveTTL(ttl int) int {
	if ttl > 0 {
		return ttl
	}
	if p.ttl > 0 {
		return p.ttl
	}
	return 600 // GoDaddy default
}

func (p *GoDaddyProvider) wrapError(op, zone, name string, err error) error {
	return &dnsprovider.ProviderError{
		Provider: "godaddy",
		Op:       op,
		Zone:     zone,
		Name:     name,
		Err:      err,
	}
}

func (p *GoDaddyProvider) convertRecords(godaddyRecs []godaddyRecord) []dnsprovider.Record {
	records := make([]dnsprovider.Record, len(godaddyRecs))
	for i, rec := range godaddyRecs {
		records[i] = dnsprovider.Record{
			Zone:  p.zone,
			Name:  rec.Name,
			Type:  rec.Type,
			Value: rec.Data,
			TTL:   rec.TTL,
			// Expiry calculation would require knowing when record was created
			// Leave as zero time for now
		}
	}
	return records
}
