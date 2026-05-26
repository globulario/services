// Package local implements a DNS provider that talks to the local
// globular-dns gRPC service.  This is the right choice when the
// Globular DNS service is authoritative for the zone (e.g. GoDaddy NS
// records point to the Globular server).
package local

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/dns/dns_client"
	"github.com/globulario/services/golang/dnsprovider"
	"github.com/globulario/services/golang/security"
)

func init() {
	dnsprovider.Register("local", NewLocalProvider)
}

// LocalProvider manages DNS records via the globular-dns gRPC service.
type LocalProvider struct {
	zone    string
	address string // gRPC address of the DNS service (resolved/repaired from etcd)
	ttl     int    // default TTL preserved for RepairedConfig

	addressRepaired bool   // address was missing/invalid and substituted from etcd
	originalAddress string // verbatim credentials.address as provided (kept only for SelfRepairer payload)

	ensureMu     sync.Mutex
	ensuredZones map[string]bool // zones successfully registered
}

// NewLocalProvider creates a LocalProvider.
// Config.Credentials may contain:
//
//	"address" — gRPC endpoint of the dns.DnsService.
//
// Accepted forms (in decreasing order of explicitness):
//
//	"host:port"   — e.g. "globule-nuc:10006" or "10.0.0.8:10006"
//	"host"        — port resolved from etcd service discovery
//	""            — both host and port resolved from etcd
//	"host:bogus"  — invalid/out-of-range port is dropped and refilled
//	                from etcd (typo guard: "10.0.0.8:100006" → "10.0.0.8:10006")
//
// When the address has to be repaired, the provider exposes the corrected
// Config via SelfRepairer.RepairedConfig() so the caller can persist the
// fix and avoid repeating the repair on every reload.
func NewLocalProvider(cfg dnsprovider.Config) (dnsprovider.Provider, error) {
	rawAddr := cfg.Credentials["address"]
	addr, repaired, err := normalizeDNSAddress(rawAddr)
	if err != nil {
		return nil, fmt.Errorf("dns provider: %w", err)
	}
	return &LocalProvider{
		zone:            cfg.Zone,
		address:         addr,
		ttl:             cfg.DefaultTTL,
		addressRepaired: repaired,
		originalAddress: rawAddr,
		ensuredZones:    make(map[string]bool),
	}, nil
}

// normalizeDNSAddress returns a sanitized "host:port" for the dns.DnsService
// gRPC endpoint, along with a flag indicating whether the input had to be
// repaired. It accepts inputs in the forms described on NewLocalProvider and
// pulls the missing/invalid pieces from etcd via service discovery.
func normalizeDNSAddress(raw string) (string, bool, error) {
	raw = strings.TrimSpace(raw)

	// Resolve the canonical endpoint from etcd up front so we have a port
	// (and possibly a host) to substitute in if the input is broken.
	canonical := config.ResolveDNSGrpcEndpoint("")
	canonicalHost, canonicalPort := "", ""
	if canonical != "" {
		if h, p, splitErr := net.SplitHostPort(canonical); splitErr == nil {
			canonicalHost, canonicalPort = h, p
		}
	}

	// Empty input — fully fall back to etcd.
	if raw == "" {
		if canonical == "" {
			return "", false, fmt.Errorf("no address configured and service discovery failed")
		}
		return canonical, true, nil
	}

	host, port, splitErr := net.SplitHostPort(raw)
	if splitErr != nil {
		// No port specified — treat the entire string as the host and
		// borrow the port from etcd.
		host = raw
		port = ""
	}

	portOK := false
	if port != "" {
		if n, err := strconv.Atoi(port); err == nil && n > 0 && n <= 65535 {
			portOK = true
		}
	}

	if portOK {
		return net.JoinHostPort(host, port), false, nil
	}

	// Port is missing or out of range — repair from etcd.
	if canonicalPort == "" {
		return "", false, fmt.Errorf("invalid port in address %q and service discovery has no dns endpoint to recover from", raw)
	}
	if host == "" {
		host = canonicalHost
	}
	if host == "" {
		return "", false, fmt.Errorf("invalid address %q and no host available from service discovery", raw)
	}
	return net.JoinHostPort(host, canonicalPort), true, nil
}

func (p *LocalProvider) Name() string { return "local" }

// dial creates a short-lived DNS client connection with cluster authentication.
func (p *LocalProvider) dial() (*dns_client.Dns_Client, error) {
	client, err := dns_client.NewDnsService_Client(p.address, "dns.DnsService")
	if err != nil {
		return nil, err
	}

	// Set the cluster domain so the interceptor accepts the request.
	// We use the local service token for authentication.
	if domain, err := config.GetDomain(); err == nil && domain != "" {
		client.SetDomain(domain)
	}

	if mac, err := config.GetMacAddress(); err == nil && mac != "" {
		if token, err := security.GetLocalToken(mac); err == nil && token != "" {
			client.SetTokenCtx(token)
		}
	}

	return client, nil
}

// ensureManagedDomain adds the zone to the DNS service's managed domain list
// if it is not already present. This is required before any record operations.
//
// Errors are NOT cached — each call retries if the previous attempt failed.
// This handles cases where the DNS service restarts and loses its in-memory
// zone list: the next reconciliation cycle will re-register the zone.
func (p *LocalProvider) ensureManagedDomain() error {
	p.ensureMu.Lock()
	defer p.ensureMu.Unlock()

	if p.ensuredZones[p.zone] {
		return nil // already confirmed for this zone
	}

	client, err := p.dial()
	if err != nil {
		return fmt.Errorf("local dns: connect for domain check: %w", err)
	}
	defer client.Close()

	// Get current managed domains
	domains, err := client.GetDomains()
	if err != nil {
		return fmt.Errorf("local dns: GetDomains: %w", err)
	}

	// Check if zone is already managed
	for _, d := range domains {
		if d == p.zone {
			p.ensuredZones[p.zone] = true
			return nil
		}
	}

	// Add our zone to the list
	domains = append(domains, p.zone)
	if err := client.SetDomains("", domains); err != nil {
		return fmt.Errorf("local dns: SetDomains: %w", err)
	}

	p.ensuredZones[p.zone] = true
	return nil
}

// fqdn builds the full domain name the DNS service expects.
// The DNS gRPC API uses full domain names with trailing dot
// (e.g. "_acme-challenge.globular.cloud.").
func (p *LocalProvider) fqdn(name string) string {
	var d string
	if name == "" || name == "@" {
		d = p.zone
	} else if strings.HasSuffix(name, "."+p.zone) {
		d = name
	} else {
		d = name + "." + p.zone
	}
	if !strings.HasSuffix(d, ".") {
		d += "."
	}
	return d
}

func (p *LocalProvider) UpsertA(ctx context.Context, zone, name, ip string, ttl int) error {
	if err := p.checkZone(zone); err != nil {
		return err
	}
	if err := p.ensureManagedDomain(); err != nil {
		return err
	}
	client, err := p.dial()
	if err != nil {
		return fmt.Errorf("local dns: connect: %w", err)
	}
	defer client.Close()
	_, err = client.SetA("", p.fqdn(name), ip, uint32(ttl))
	return err
}

func (p *LocalProvider) UpsertAAAA(ctx context.Context, zone, name, ip string, ttl int) error {
	if err := p.checkZone(zone); err != nil {
		return err
	}
	if err := p.ensureManagedDomain(); err != nil {
		return err
	}
	client, err := p.dial()
	if err != nil {
		return fmt.Errorf("local dns: connect: %w", err)
	}
	defer client.Close()
	_, err = client.SetAAAA("", p.fqdn(name), ip, uint32(ttl))
	return err
}

func (p *LocalProvider) UpsertCNAME(ctx context.Context, zone, name, target string, ttl int) error {
	return fmt.Errorf("local dns: CNAME records not yet supported")
}

func (p *LocalProvider) UpsertTXT(ctx context.Context, zone, name string, values []string, ttl int) error {
	if err := p.checkZone(zone); err != nil {
		return err
	}
	if err := p.ensureManagedDomain(); err != nil {
		return err
	}
	client, err := p.dial()
	if err != nil {
		return fmt.Errorf("local dns: connect: %w", err)
	}
	defer client.Close()

	domain := p.fqdn(name)
	for _, val := range values {
		if _, err := client.SetTXT("", domain, val, uint32(ttl)); err != nil {
			return fmt.Errorf("local dns: SetTXT %s: %w", domain, err)
		}
	}
	return nil
}

func (p *LocalProvider) DeleteTXT(ctx context.Context, zone, name string, values []string) error {
	if err := p.checkZone(zone); err != nil {
		return err
	}
	client, err := p.dial()
	if err != nil {
		return fmt.Errorf("local dns: connect: %w", err)
	}
	defer client.Close()

	domain := p.fqdn(name)
	if len(values) == 0 {
		return client.RemoveTXT("", domain, "")
	}
	for _, val := range values {
		if err := client.RemoveTXT("", domain, val); err != nil {
			return fmt.Errorf("local dns: RemoveTXT %s: %w", domain, err)
		}
	}
	return nil
}

func (p *LocalProvider) GetRecords(ctx context.Context, zone, name, rtype string) ([]dnsprovider.Record, error) {
	if err := p.checkZone(zone); err != nil {
		return nil, err
	}
	client, err := p.dial()
	if err != nil {
		return nil, fmt.Errorf("local dns: connect: %w", err)
	}
	defer client.Close()

	domain := p.fqdn(name)
	var records []dnsprovider.Record

	if rtype == "" || rtype == "TXT" {
		vals, err := client.GetTXT(domain)
		if err == nil {
			for _, v := range vals {
				records = append(records, dnsprovider.Record{
					Zone:  zone,
					Name:  name,
					Type:  "TXT",
					Value: v,
				})
			}
		}
	}

	if rtype == "" || rtype == "A" {
		vals, err := client.GetA(domain)
		if err == nil {
			for _, v := range vals {
				records = append(records, dnsprovider.Record{
					Zone:  zone,
					Name:  name,
					Type:  "A",
					Value: v,
				})
			}
		}
	}

	return records, nil
}

// checkZone validates the zone parameter. For the local DNS provider, any zone
// is accepted because the globular-dns service manages multiple zones. When the
// caller passes a zone different from the provider's default, we transparently
// update p.zone so that fqdn() and ensureManagedDomain() use the caller's zone.
func (p *LocalProvider) checkZone(zone string) error {
	if zone != "" && zone != p.zone {
		p.zone = zone
		// New zone — will be registered on the next ensureManagedDomain call.
	}
	return nil
}

// Preflight verifies the configured DNS gRPC endpoint is reachable and that
// the full write-read-delete cycle works against the zone. This is meant to
// run once, before any ACME challenge or A-record publish, so failures show
// up as a clear "the DNS provider is broken" instead of a confusing ACME
// rate-limit storm caused by silent SetTXT failures.
//
// The probe writes a TXT record under a random "_globular-probe-*"
// subdomain, reads it back to confirm the value, deletes it, and reads
// again to confirm the deletion. Any step failing aborts the preflight.
func (p *LocalProvider) Preflight(ctx context.Context, zone string) error {
	if err := p.checkZone(zone); err != nil {
		return err
	}
	if err := p.ensureManagedDomain(); err != nil {
		return fmt.Errorf("local dns preflight: ensure managed domain: %w", err)
	}

	nonceBytes := make([]byte, 8)
	if _, err := rand.Read(nonceBytes); err != nil {
		return fmt.Errorf("local dns preflight: nonce: %w", err)
	}
	nonce := hex.EncodeToString(nonceBytes)
	probeName := "_globular-probe-" + nonce
	probeValue := "globular-preflight-" + nonce
	domain := p.fqdn(probeName)

	client, err := p.dial()
	if err != nil {
		return fmt.Errorf("local dns preflight: dial %s: %w", p.address, err)
	}
	defer client.Close()

	ttl := uint32(60)
	if _, err := client.SetTXT("", domain, probeValue, ttl); err != nil {
		return fmt.Errorf("local dns preflight: SetTXT %s: %w", domain, err)
	}

	// Always best-effort clean up, even on later failures, so a probe never
	// leaves stray records behind.
	cleanup := func() {
		_ = client.RemoveTXT("", domain, probeValue)
	}

	vals, err := client.GetTXT(domain)
	if err != nil {
		cleanup()
		return fmt.Errorf("local dns preflight: GetTXT %s: %w", domain, err)
	}
	found := false
	for _, v := range vals {
		if v == probeValue {
			found = true
			break
		}
	}
	if !found {
		cleanup()
		return fmt.Errorf("local dns preflight: TXT %s did not contain probe value after Set", domain)
	}

	if err := client.RemoveTXT("", domain, probeValue); err != nil {
		return fmt.Errorf("local dns preflight: RemoveTXT %s: %w", domain, err)
	}

	vals, _ = client.GetTXT(domain)
	for _, v := range vals {
		if v == probeValue {
			return fmt.Errorf("local dns preflight: TXT %s still contains probe value after Remove", domain)
		}
	}

	return nil
}

// RepairedConfig returns the corrected Config when NewLocalProvider had to
// substitute a missing or out-of-range port from etcd service discovery.
// Callers should persist the returned Config so the same repair does not
// have to happen on every reload.
func (p *LocalProvider) RepairedConfig() (dnsprovider.Config, bool) {
	if !p.addressRepaired {
		return dnsprovider.Config{}, false
	}
	cfg := dnsprovider.Config{
		Type:        "local",
		Zone:        p.zone,
		Credentials: map[string]string{"address": p.address},
		DefaultTTL:  p.ttl,
	}
	return cfg, true
}
