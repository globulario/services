package dnsprovider

import (
	"context"
	"time"
)

// Provider is the interface for DNS record management.
// It provides the minimal primitives required for public DNS + ACME DNS-01.
//
// Design principles:
// - zone is the root zone (e.g. "globular.cloud")
// - name is relative record name (e.g. "globule-ryzen" or "_acme-challenge.globule-ryzen")
// - TXT records support multiple values (ACME sometimes uses multiple tokens)
type Provider interface {
	// Name returns the provider type identifier (e.g., "godaddy", "route53", "manual")
	Name() string

	// UpsertA creates or updates an A record.
	// zone: root zone (e.g., "globular.cloud")
	// name: relative name (e.g., "globule-ryzen" for globule-ryzen.globular.cloud)
	// ip: IPv4 address
	// ttl: time-to-live in seconds
	UpsertA(ctx context.Context, zone string, name string, ip string, ttl int) error

	// UpsertAAAA creates or updates an AAAA record.
	// zone: root zone (e.g., "globular.cloud")
	// name: relative name (e.g., "globule-ryzen" for globule-ryzen.globular.cloud)
	// ip: IPv6 address
	// ttl: time-to-live in seconds
	UpsertAAAA(ctx context.Context, zone string, name string, ip string, ttl int) error

	// UpsertCNAME creates or updates a CNAME record.
	// zone: root zone (e.g., "globular.cloud")
	// name: relative name (e.g., "www" for www.globular.cloud)
	// target: target FQDN (must end with dot, e.g., "globule-ryzen.globular.cloud.")
	// ttl: time-to-live in seconds
	UpsertCNAME(ctx context.Context, zone string, name string, target string, ttl int) error

	// UpsertTXT creates or updates a TXT record.
	// Supports multiple values for the same name (required for ACME multi-token challenges).
	// zone: root zone (e.g., "globular.cloud")
	// name: relative name (e.g., "_acme-challenge.globule-ryzen")
	// values: TXT record values (can have multiple)
	// ttl: time-to-live in seconds
	UpsertTXT(ctx context.Context, zone string, name string, values []string, ttl int) error

	// DeleteTXT removes specific TXT record values.
	// zone: root zone
	// name: relative name
	// values: TXT record values to delete (if empty, deletes all TXT records for this name)
	DeleteTXT(ctx context.Context, zone string, name string, values []string) error

	// GetRecords queries current DNS records.
	// zone: root zone
	// name: relative name (empty string returns all records in zone)
	// rtype: record type ("A", "AAAA", "CNAME", "TXT", or "" for all types)
	// Returns slice of records matching the query.
	GetRecords(ctx context.Context, zone string, name string, rtype string) ([]Record, error)
}

// Record represents a DNS record returned by GetRecords.
type Record struct {
	Zone   string    `json:"zone"`   // Root zone
	Name   string    `json:"name"`   // Relative name
	Type   string    `json:"type"`   // Record type (A, AAAA, CNAME, TXT)
	Value  string    `json:"value"`  // Record value
	TTL    int       `json:"ttl"`    // Time-to-live in seconds
	Expiry time.Time `json:"expiry"` // When this record expires (best effort)
}

// Config describes provider configuration.
// This is used by the registry to instantiate providers.
type Config struct {
	// Type is the provider identifier (e.g., "godaddy", "route53", "cloudflare", "manual")
	Type string `json:"type"`

	// Zone is the DNS zone this provider manages (e.g., "globular.cloud")
	Zone string `json:"zone"`

	// Credentials holds provider-specific authentication credentials.
	// Keys depend on provider type:
	//   - godaddy: "api_key", "api_secret"
	//   - route53: uses AWS SDK credential chain (no explicit creds here)
	//   - cloudflare: "api_token" or "api_key" + "api_email"
	//   - manual: none (human performs DNS operations)
	Credentials map[string]string `json:"credentials,omitempty"`

	// DefaultTTL is the default time-to-live for DNS records (in seconds)
	// If 0, provider will use its own default (typically 300-600)
	DefaultTTL int `json:"default_ttl,omitempty"`

	// Timeout for DNS operations (optional, provider-specific)
	Timeout time.Duration `json:"timeout,omitempty"`
}

// ProviderError wraps provider-specific errors with context.
type ProviderError struct {
	Provider string // Provider name
	Op       string // Operation (e.g., "UpsertA", "DeleteTXT")
	Zone     string // Zone
	Name     string // Record name
	Err      error  // Underlying error
}

func (e *ProviderError) Error() string {
	return e.Provider + "." + e.Op + "(" + e.Zone + "/" + e.Name + "): " + e.Err.Error()
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}
