package manual

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/globulario/services/golang/dnsprovider"
)

func init() {
	dnsprovider.Register("manual", NewManualProvider)
}

// ManualProvider prints DNS operations for manual execution.
// This is useful for enterprises with strict change-control processes
// where automated DNS updates are not permitted.
//
// Operations are printed to stdout (or a configured writer) in a
// human-readable format with exact commands to execute.
type ManualProvider struct {
	zone   string
	output io.Writer
}

// NewManualProvider creates a new ManualProvider.
func NewManualProvider(cfg dnsprovider.Config) (dnsprovider.Provider, error) {
	return &ManualProvider{
		zone:   cfg.Zone,
		output: os.Stdout,
	}, nil
}

// SetOutput sets the output writer (useful for testing).
func (p *ManualProvider) SetOutput(w io.Writer) {
	p.output = w
}

func (p *ManualProvider) Name() string {
	return "manual"
}

func (p *ManualProvider) UpsertA(ctx context.Context, zone string, name string, ip string, ttl int) error {
	if err := p.validateZone(zone); err != nil {
		return err
	}

	fqdn := p.fqdn(name, zone)
	fmt.Fprintf(p.output, "\n┌─────────────────────────────────────────────────────────────┐\n")
	fmt.Fprintf(p.output, "│ MANUAL DNS OPERATION REQUIRED                               │\n")
	fmt.Fprintf(p.output, "├─────────────────────────────────────────────────────────────┤\n")
	fmt.Fprintf(p.output, "│ Action:  CREATE or UPDATE A record                          │\n")
	fmt.Fprintf(p.output, "│ Zone:    %-51s │\n", zone)
	fmt.Fprintf(p.output, "│ Name:    %-51s │\n", fqdn)
	fmt.Fprintf(p.output, "│ Type:    A                                                  │\n")
	fmt.Fprintf(p.output, "│ Value:   %-51s │\n", ip)
	fmt.Fprintf(p.output, "│ TTL:     %-51d │\n", ttl)
	fmt.Fprintf(p.output, "└─────────────────────────────────────────────────────────────┘\n")
	fmt.Fprintf(p.output, "\nExample commands:\n")
	fmt.Fprintf(p.output, "  # Using your DNS provider's CLI or web interface:\n")
	fmt.Fprintf(p.output, "  # 1. Navigate to zone: %s\n", zone)
	fmt.Fprintf(p.output, "  # 2. Create/update A record:\n")
	fmt.Fprintf(p.output, "  #    Name: %s\n", name)
	fmt.Fprintf(p.output, "  #    Type: A\n")
	fmt.Fprintf(p.output, "  #    Value: %s\n", ip)
	fmt.Fprintf(p.output, "  #    TTL: %d seconds\n", ttl)
	fmt.Fprintf(p.output, "\n")

	return nil
}

func (p *ManualProvider) UpsertAAAA(ctx context.Context, zone string, name string, ip string, ttl int) error {
	if err := p.validateZone(zone); err != nil {
		return err
	}

	fqdn := p.fqdn(name, zone)
	fmt.Fprintf(p.output, "\n┌─────────────────────────────────────────────────────────────┐\n")
	fmt.Fprintf(p.output, "│ MANUAL DNS OPERATION REQUIRED                               │\n")
	fmt.Fprintf(p.output, "├─────────────────────────────────────────────────────────────┤\n")
	fmt.Fprintf(p.output, "│ Action:  CREATE or UPDATE AAAA record                       │\n")
	fmt.Fprintf(p.output, "│ Zone:    %-51s │\n", zone)
	fmt.Fprintf(p.output, "│ Name:    %-51s │\n", fqdn)
	fmt.Fprintf(p.output, "│ Type:    AAAA                                               │\n")
	fmt.Fprintf(p.output, "│ Value:   %-51s │\n", ip)
	fmt.Fprintf(p.output, "│ TTL:     %-51d │\n", ttl)
	fmt.Fprintf(p.output, "└─────────────────────────────────────────────────────────────┘\n")
	fmt.Fprintf(p.output, "\nExample commands:\n")
	fmt.Fprintf(p.output, "  # Using your DNS provider's CLI or web interface:\n")
	fmt.Fprintf(p.output, "  # 1. Navigate to zone: %s\n", zone)
	fmt.Fprintf(p.output, "  # 2. Create/update AAAA record:\n")
	fmt.Fprintf(p.output, "  #    Name: %s\n", name)
	fmt.Fprintf(p.output, "  #    Type: AAAA\n")
	fmt.Fprintf(p.output, "  #    Value: %s\n", ip)
	fmt.Fprintf(p.output, "  #    TTL: %d seconds\n", ttl)
	fmt.Fprintf(p.output, "\n")

	return nil
}

func (p *ManualProvider) UpsertCNAME(ctx context.Context, zone string, name string, target string, ttl int) error {
	if err := p.validateZone(zone); err != nil {
		return err
	}

	fqdn := p.fqdn(name, zone)
	// Ensure target ends with dot
	if !strings.HasSuffix(target, ".") {
		target = target + "."
	}

	fmt.Fprintf(p.output, "\n┌─────────────────────────────────────────────────────────────┐\n")
	fmt.Fprintf(p.output, "│ MANUAL DNS OPERATION REQUIRED                               │\n")
	fmt.Fprintf(p.output, "├─────────────────────────────────────────────────────────────┤\n")
	fmt.Fprintf(p.output, "│ Action:  CREATE or UPDATE CNAME record                      │\n")
	fmt.Fprintf(p.output, "│ Zone:    %-51s │\n", zone)
	fmt.Fprintf(p.output, "│ Name:    %-51s │\n", fqdn)
	fmt.Fprintf(p.output, "│ Type:    CNAME                                              │\n")
	fmt.Fprintf(p.output, "│ Target:  %-51s │\n", target)
	fmt.Fprintf(p.output, "│ TTL:     %-51d │\n", ttl)
	fmt.Fprintf(p.output, "└─────────────────────────────────────────────────────────────┘\n")
	fmt.Fprintf(p.output, "\nExample commands:\n")
	fmt.Fprintf(p.output, "  # Using your DNS provider's CLI or web interface:\n")
	fmt.Fprintf(p.output, "  # 1. Navigate to zone: %s\n", zone)
	fmt.Fprintf(p.output, "  # 2. Create/update CNAME record:\n")
	fmt.Fprintf(p.output, "  #    Name: %s\n", name)
	fmt.Fprintf(p.output, "  #    Type: CNAME\n")
	fmt.Fprintf(p.output, "  #    Target: %s\n", target)
	fmt.Fprintf(p.output, "  #    TTL: %d seconds\n", ttl)
	fmt.Fprintf(p.output, "\n")

	return nil
}

func (p *ManualProvider) UpsertTXT(ctx context.Context, zone string, name string, values []string, ttl int) error {
	if err := p.validateZone(zone); err != nil {
		return err
	}

	fqdn := p.fqdn(name, zone)
	isACMEChallenge := strings.HasPrefix(name, "_acme-challenge.")

	fmt.Fprintf(p.output, "\n┌─────────────────────────────────────────────────────────────┐\n")
	fmt.Fprintf(p.output, "│ MANUAL DNS OPERATION REQUIRED                               │\n")
	fmt.Fprintf(p.output, "├─────────────────────────────────────────────────────────────┤\n")
	fmt.Fprintf(p.output, "│ Action:  CREATE or UPDATE TXT record                        │\n")
	if isACMEChallenge {
		fmt.Fprintf(p.output, "│ Purpose: ACME DNS-01 Challenge (Let's Encrypt)              │\n")
	}
	fmt.Fprintf(p.output, "│ Zone:    %-51s │\n", zone)
	fmt.Fprintf(p.output, "│ Name:    %-51s │\n", fqdn)
	fmt.Fprintf(p.output, "│ Type:    TXT                                                │\n")
	fmt.Fprintf(p.output, "│ TTL:     %-51d │\n", ttl)
	fmt.Fprintf(p.output, "└─────────────────────────────────────────────────────────────┘\n")

	fmt.Fprintf(p.output, "\nValues:\n")
	for i, val := range values {
		// Truncate long values (ACME tokens) for readability
		display := val
		if len(val) > 50 {
			display = val[:47] + "..."
		}
		fmt.Fprintf(p.output, "  %d. \"%s\"\n", i+1, display)
	}

	fmt.Fprintf(p.output, "\nExample commands:\n")
	fmt.Fprintf(p.output, "  # Using your DNS provider's CLI or web interface:\n")
	fmt.Fprintf(p.output, "  # 1. Navigate to zone: %s\n", zone)
	fmt.Fprintf(p.output, "  # 2. Create/update TXT record:\n")
	fmt.Fprintf(p.output, "  #    Name: %s\n", name)
	fmt.Fprintf(p.output, "  #    Type: TXT\n")
	fmt.Fprintf(p.output, "  #    Values: (see above, one or more strings)\n")
	fmt.Fprintf(p.output, "  #    TTL: %d seconds\n", ttl)

	if isACMEChallenge {
		fmt.Fprintf(p.output, "\n⚠️  IMPORTANT: This is an ACME DNS-01 challenge.\n")
		fmt.Fprintf(p.output, "    You must create this TXT record BEFORE continuing.\n")
		fmt.Fprintf(p.output, "    The ACME server will query %s to verify ownership.\n", fqdn)
		fmt.Fprintf(p.output, "    After verification, you can delete this record.\n")
	}
	fmt.Fprintf(p.output, "\n")

	return nil
}

func (p *ManualProvider) DeleteTXT(ctx context.Context, zone string, name string, values []string) error {
	if err := p.validateZone(zone); err != nil {
		return err
	}

	fqdn := p.fqdn(name, zone)
	isACMEChallenge := strings.HasPrefix(name, "_acme-challenge.")

	fmt.Fprintf(p.output, "\n┌─────────────────────────────────────────────────────────────┐\n")
	fmt.Fprintf(p.output, "│ MANUAL DNS OPERATION REQUIRED                               │\n")
	fmt.Fprintf(p.output, "├─────────────────────────────────────────────────────────────┤\n")
	fmt.Fprintf(p.output, "│ Action:  DELETE TXT record                                  │\n")
	if isACMEChallenge {
		fmt.Fprintf(p.output, "│ Purpose: ACME DNS-01 Challenge Cleanup                      │\n")
	}
	fmt.Fprintf(p.output, "│ Zone:    %-51s │\n", zone)
	fmt.Fprintf(p.output, "│ Name:    %-51s │\n", fqdn)
	fmt.Fprintf(p.output, "│ Type:    TXT                                                │\n")
	fmt.Fprintf(p.output, "└─────────────────────────────────────────────────────────────┘\n")

	if len(values) > 0 {
		fmt.Fprintf(p.output, "\nDelete specific values:\n")
		for i, val := range values {
			display := val
			if len(val) > 50 {
				display = val[:47] + "..."
			}
			fmt.Fprintf(p.output, "  %d. \"%s\"\n", i+1, display)
		}
	} else {
		fmt.Fprintf(p.output, "\nDelete ALL TXT records for this name.\n")
	}

	fmt.Fprintf(p.output, "\nExample commands:\n")
	fmt.Fprintf(p.output, "  # Using your DNS provider's CLI or web interface:\n")
	fmt.Fprintf(p.output, "  # 1. Navigate to zone: %s\n", zone)
	fmt.Fprintf(p.output, "  # 2. Delete TXT record:\n")
	fmt.Fprintf(p.output, "  #    Name: %s\n", name)
	fmt.Fprintf(p.output, "  #    Type: TXT\n")

	if isACMEChallenge {
		fmt.Fprintf(p.output, "\n✓ This ACME challenge record can now be safely deleted.\n")
	}
	fmt.Fprintf(p.output, "\n")

	return nil
}

func (p *ManualProvider) GetRecords(ctx context.Context, zone string, name string, rtype string) ([]dnsprovider.Record, error) {
	if err := p.validateZone(zone); err != nil {
		return nil, err
	}

	fqdn := p.fqdn(name, zone)
	fmt.Fprintf(p.output, "\n┌─────────────────────────────────────────────────────────────┐\n")
	fmt.Fprintf(p.output, "│ MANUAL DNS QUERY                                            │\n")
	fmt.Fprintf(p.output, "├─────────────────────────────────────────────────────────────┤\n")
	fmt.Fprintf(p.output, "│ Action:  QUERY DNS records                                  │\n")
	fmt.Fprintf(p.output, "│ Zone:    %-51s │\n", zone)
	if name != "" {
		fmt.Fprintf(p.output, "│ Name:    %-51s │\n", fqdn)
	} else {
		fmt.Fprintf(p.output, "│ Name:    (all records in zone)                              │\n")
	}
	if rtype != "" {
		fmt.Fprintf(p.output, "│ Type:    %-51s │\n", rtype)
	} else {
		fmt.Fprintf(p.output, "│ Type:    (all types)                                        │\n")
	}
	fmt.Fprintf(p.output, "└─────────────────────────────────────────────────────────────┘\n")
	fmt.Fprintf(p.output, "\nPlease query your DNS provider and enter results manually.\n")
	fmt.Fprintf(p.output, "The ManualProvider cannot query DNS programmatically.\n\n")

	// Return empty slice (manual provider cannot query)
	return []dnsprovider.Record{}, nil
}

func (p *ManualProvider) validateZone(zone string) error {
	if zone != p.zone {
		return &dnsprovider.ProviderError{
			Provider: "manual",
			Op:       "validateZone",
			Zone:     zone,
			Err:      fmt.Errorf("zone mismatch: expected %q, got %q", p.zone, zone),
		}
	}
	return nil
}

func (p *ManualProvider) fqdn(name string, zone string) string {
	if name == "" || name == "@" {
		return zone + "."
	}
	// Remove zone suffix if already present
	if strings.HasSuffix(name, "."+zone) {
		return name + "."
	}
	return name + "." + zone + "."
}
