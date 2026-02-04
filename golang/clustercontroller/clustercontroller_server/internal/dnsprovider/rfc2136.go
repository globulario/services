package dnsprovider

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/miekg/dns"
)

// RFC2136Provider implements DNS updates via RFC2136 (Dynamic Updates) (PR8)
type RFC2136Provider struct {
	config    Config
	server    string // DNS server address (host:port)
	tsigKey   string // TSIG key name
	tsigSecret string // TSIG secret (base64)
	zone      string // DNS zone name (must end with .)
}

// NewRFC2136Provider creates a new RFC2136 provider
// Required config keys:
//   - server: DNS server address (e.g., "ns1.example.com:53")
//   - tsig_key: TSIG key name (optional, for authenticated updates)
//   - tsig_secret: TSIG secret in base64 (optional)
func NewRFC2136Provider(cfg Config) (*RFC2136Provider, error) {
	server := cfg.ProviderConfig["server"]
	if server == "" {
		return nil, fmt.Errorf("rfc2136: server is required in provider_config")
	}

	// Ensure zone ends with .
	zone := cfg.Domain
	if zone[len(zone)-1] != '.' {
		zone = zone + "."
	}

	provider := &RFC2136Provider{
		config:     cfg,
		server:     server,
		tsigKey:    cfg.ProviderConfig["tsig_key"],
		tsigSecret: cfg.ProviderConfig["tsig_secret"],
		zone:       zone,
	}

	log.Printf("external dns (rfc2136): initialized for domain %s via server %s", cfg.Domain, server)
	return provider, nil
}

// UpsertA creates or updates an A record
func (p *RFC2136Provider) UpsertA(ctx context.Context, name string, ips []net.IP, ttl int) error {
	if err := ValidateName(name); err != nil {
		return err
	}

	// Filter private IPs if not allowed
	ips = FilterPublicIPs(ips, p.config.AllowPrivateIPs)
	if len(ips) == 0 {
		log.Printf("external dns (rfc2136): skipping A record %s (no public IPs)", name)
		return nil
	}

	// Ensure FQDN
	fqdn := name
	if fqdn[len(fqdn)-1] != '.' {
		fqdn = fqdn + "."
	}

	// Create DNS update message
	msg := new(dns.Msg)
	msg.SetUpdate(p.zone)

	// Remove existing A records for this name
	rr := &dns.A{
		Hdr: dns.RR_Header{
			Name:   fqdn,
			Rrtype: dns.TypeA,
			Class:  dns.ClassANY,
		},
	}
	msg.RemoveRRset([]dns.RR{rr})

	// Add new A records
	for _, ip := range ips {
		if ip.To4() != nil {
			rr := &dns.A{
				Hdr: dns.RR_Header{
					Name:   fqdn,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    uint32(ttl),
				},
				A: ip,
			}
			msg.Insert([]dns.RR{rr})
		}
	}

	return p.sendUpdate(ctx, msg, fmt.Sprintf("A %s -> %v", name, ips))
}

// UpsertAAAA creates or updates an AAAA record
func (p *RFC2136Provider) UpsertAAAA(ctx context.Context, name string, ips []net.IP, ttl int) error {
	if err := ValidateName(name); err != nil {
		return err
	}

	// Filter private IPs if not allowed
	ips = FilterPublicIPs(ips, p.config.AllowPrivateIPs)
	if len(ips) == 0 {
		log.Printf("external dns (rfc2136): skipping AAAA record %s (no public IPs)", name)
		return nil
	}

	// Ensure FQDN
	fqdn := name
	if fqdn[len(fqdn)-1] != '.' {
		fqdn = fqdn + "."
	}

	// Create DNS update message
	msg := new(dns.Msg)
	msg.SetUpdate(p.zone)

	// Remove existing AAAA records for this name
	rr := &dns.AAAA{
		Hdr: dns.RR_Header{
			Name:   fqdn,
			Rrtype: dns.TypeAAAA,
			Class:  dns.ClassANY,
		},
	}
	msg.RemoveRRset([]dns.RR{rr})

	// Add new AAAA records
	for _, ip := range ips {
		if ip.To16() != nil && ip.To4() == nil {
			rr := &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   fqdn,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    uint32(ttl),
				},
				AAAA: ip,
			}
			msg.Insert([]dns.RR{rr})
		}
	}

	return p.sendUpdate(ctx, msg, fmt.Sprintf("AAAA %s -> %v", name, ips))
}

// Delete removes all records for a name
func (p *RFC2136Provider) Delete(ctx context.Context, name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}

	// Ensure FQDN
	fqdn := name
	if fqdn[len(fqdn)-1] != '.' {
		fqdn = fqdn + "."
	}

	// Create DNS update message to delete all records
	msg := new(dns.Msg)
	msg.SetUpdate(p.zone)

	// Delete ANY record type for this name
	rr := &dns.ANY{
		Hdr: dns.RR_Header{
			Name:   fqdn,
			Rrtype: dns.TypeANY,
			Class:  dns.ClassANY,
		},
	}
	msg.RemoveRRset([]dns.RR{rr})

	return p.sendUpdate(ctx, msg, fmt.Sprintf("DELETE %s", name))
}

// sendUpdate sends a DNS update message to the server
func (p *RFC2136Provider) sendUpdate(ctx context.Context, msg *dns.Msg, description string) error {
	// Add TSIG if configured
	if p.tsigKey != "" && p.tsigSecret != "" {
		msg.SetTsig(p.tsigKey, dns.HmacSHA256, 300, time.Now().Unix())
	}

	client := &dns.Client{
		Timeout: 10 * time.Second,
	}

	// Set TSIG secret if configured
	if p.tsigKey != "" && p.tsigSecret != "" {
		client.TsigSecret = map[string]string{
			p.tsigKey: p.tsigSecret,
		}
	}

	// Send update
	resp, _, err := client.ExchangeContext(ctx, msg, p.server)
	if err != nil {
		return fmt.Errorf("rfc2136 update failed (%s): %w", description, err)
	}

	if resp.Rcode != dns.RcodeSuccess {
		return fmt.Errorf("rfc2136 update rejected (%s): %s", description, dns.RcodeToString[resp.Rcode])
	}

	log.Printf("external dns (rfc2136): updated %s", description)
	return nil
}

// Close cleans up provider resources
func (p *RFC2136Provider) Close() error {
	return nil
}
