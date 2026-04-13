// Package local implements a DNS provider that talks to the local
// globular-dns gRPC service.  This is the right choice when the
// Globular DNS service is authoritative for the zone (e.g. GoDaddy NS
// records point to the Globular server).
package local

import (
	"context"
	"fmt"
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
	address string // gRPC address of the DNS service (e.g. "localhost:10006")

	ensureOnce sync.Once
	ensureErr  error
}

// NewLocalProvider creates a LocalProvider.
// Config.Credentials may contain:
//
//	"address" — gRPC endpoint (default "localhost:10006")
func NewLocalProvider(cfg dnsprovider.Config) (dnsprovider.Provider, error) {
	addr := cfg.Credentials["address"]
	if addr == "" {
		addr = "localhost:10006"
	}
	return &LocalProvider{zone: cfg.Zone, address: addr}, nil
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
func (p *LocalProvider) ensureManagedDomain() error {
	p.ensureOnce.Do(func() {
		client, err := p.dial()
		if err != nil {
			p.ensureErr = fmt.Errorf("local dns: connect for domain check: %w", err)
			return
		}
		defer client.Close()

		// Get current managed domains
		domains, err := client.GetDomains()
		if err != nil {
			p.ensureErr = fmt.Errorf("local dns: GetDomains: %w", err)
			return
		}

		// Check if zone is already managed
		for _, d := range domains {
			if d == p.zone {
				return // already managed
			}
		}

		// Add our zone to the list
		domains = append(domains, p.zone)
		if err := client.SetDomains("", domains); err != nil {
			p.ensureErr = fmt.Errorf("local dns: SetDomains: %w", err)
			return
		}
	})
	return p.ensureErr
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
		// Reset ensureOnce so the new zone gets registered as a managed domain
		p.ensureOnce = sync.Once{}
		p.ensureErr = nil
	}
	return nil
}
