package dnsprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"
)

const cloudflareAPIBase = "https://api.cloudflare.com/client/v4"

// CloudflareProvider implements DNS updates via Cloudflare API.
type CloudflareProvider struct {
	config  Config
	apiToken string
	zoneID  string
	client  *http.Client
}

// NewCloudflareProvider creates a new Cloudflare provider.
// Required config keys in ProviderConfig:
//   - api_token: Cloudflare API token (preferred over api_key+email)
//   - zone_id:   Cloudflare zone ID for the managed zone
func NewCloudflareProvider(cfg Config) (*CloudflareProvider, error) {
	token := cfg.ProviderConfig["api_token"]
	if token == "" {
		return nil, fmt.Errorf("cloudflare: api_token is required in provider_config")
	}

	zoneID := cfg.ProviderConfig["zone_id"]
	if zoneID == "" {
		return nil, fmt.Errorf("cloudflare: zone_id is required in provider_config")
	}

	p := &CloudflareProvider{
		config:   cfg,
		apiToken: token,
		zoneID:   zoneID,
		client:   &http.Client{Timeout: 15 * time.Second},
	}

	log.Printf("external dns (cloudflare): initialized for domain %s (zone_id=%s)", cfg.Domain, zoneID)
	return p, nil
}

// UpsertA creates or replaces A records for the given name.
func (p *CloudflareProvider) UpsertA(ctx context.Context, name string, ips []net.IP, ttl int) error {
	if err := ValidateName(name); err != nil {
		return err
	}

	filtered := FilterPublicIPs(ips, p.config.AllowPrivateIPs)
	if len(filtered) == 0 {
		if len(ips) > 0 {
			// All IPs were private — likely a misconfiguration. Warn loudly so it's visible.
			log.Printf("external dns (cloudflare): WARN - all IPs for %s are private/RFC1918 and AllowPrivateIPs=false; record NOT published. Set allow_private_ips=true if this is intentional.", name)
		}
		return nil
	}
	ips = filtered

	if err := p.deleteByType(ctx, name, "A"); err != nil {
		return fmt.Errorf("cloudflare: delete existing A records for %s: %w", name, err)
	}

	for _, ip := range ips {
		if ip.To4() == nil {
			continue // skip IPv6 from A records
		}
		if err := p.createRecord(ctx, name, "A", ip.String(), ttl); err != nil {
			return fmt.Errorf("cloudflare: create A record %s -> %s: %w", name, ip, err)
		}
		log.Printf("external dns (cloudflare): upserted A %s -> %s (ttl=%d)", name, ip, ttl)
	}
	return nil
}

// UpsertAAAA creates or replaces AAAA records for the given name.
func (p *CloudflareProvider) UpsertAAAA(ctx context.Context, name string, ips []net.IP, ttl int) error {
	if err := ValidateName(name); err != nil {
		return err
	}

	ips = FilterPublicIPs(ips, p.config.AllowPrivateIPs)
	if len(ips) == 0 {
		log.Printf("external dns (cloudflare): skipping AAAA record %s (no public IPs)", name)
		return nil
	}

	if err := p.deleteByType(ctx, name, "AAAA"); err != nil {
		return fmt.Errorf("cloudflare: delete existing AAAA records for %s: %w", name, err)
	}

	for _, ip := range ips {
		if ip.To4() != nil {
			continue // skip IPv4 from AAAA records
		}
		if err := p.createRecord(ctx, name, "AAAA", ip.String(), ttl); err != nil {
			return fmt.Errorf("cloudflare: create AAAA record %s -> %s: %w", name, ip, err)
		}
		log.Printf("external dns (cloudflare): upserted AAAA %s -> %s (ttl=%d)", name, ip, ttl)
	}
	return nil
}

// Delete removes all DNS records (any type) for the given name.
func (p *CloudflareProvider) Delete(ctx context.Context, name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	return p.deleteByType(ctx, name, "")
}

// Close is a no-op for HTTP-based providers.
func (p *CloudflareProvider) Close() error { return nil }

// ---- Cloudflare API helpers ----

type cfRecord struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

type cfListResponse struct {
	Success bool       `json:"success"`
	Errors  []cfError  `json:"errors"`
	Result  []cfRecord `json:"result"`
}

type cfCreateResponse struct {
	Success bool      `json:"success"`
	Errors  []cfError `json:"errors"`
}

type cfError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (p *CloudflareProvider) listRecords(ctx context.Context, name, rtype string) ([]cfRecord, error) {
	q := url.Values{"name": {name}}
	if rtype != "" {
		q.Set("type", rtype)
	}
	endpoint := fmt.Sprintf("%s/zones/%s/dns_records?%s", cloudflareAPIBase, p.zoneID, q.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result cfListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, fmt.Errorf("cloudflare API: %v", result.Errors)
	}
	return result.Result, nil
}

func (p *CloudflareProvider) deleteRecord(ctx context.Context, id string) error {
	url := fmt.Sprintf("%s/zones/%s/dns_records/%s", cloudflareAPIBase, p.zoneID, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiToken)

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
	}
	return nil
}

func (p *CloudflareProvider) deleteByType(ctx context.Context, name, rtype string) error {
	records, err := p.listRecords(ctx, name, rtype)
	if err != nil {
		return err
	}
	for _, rec := range records {
		if err := p.deleteRecord(ctx, rec.ID); err != nil {
			return fmt.Errorf("delete record %s: %w", rec.ID, err)
		}
	}
	return nil
}

func (p *CloudflareProvider) createRecord(ctx context.Context, name, rtype, content string, ttl int) error {
	if ttl <= 0 {
		ttl = 300
	}
	rec := cfRecord{
		Type:    rtype,
		Name:    name,
		Content: content,
		TTL:     ttl,
		Proxied: false,
	}

	body, err := json.Marshal(rec)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/zones/%s/dns_records", cloudflareAPIBase, p.zoneID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result cfCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	if !result.Success {
		return fmt.Errorf("cloudflare API: %v", result.Errors)
	}
	return nil
}
