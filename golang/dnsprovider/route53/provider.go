package route53

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/globulario/services/golang/dnsprovider"
)

func init() {
	dnsprovider.Register("route53", NewRoute53Provider)
}

// Route53Provider implements DNS record management via AWS Route53.
// It uses the AWS SDK credential chain:
// 1. Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_SESSION_TOKEN)
// 2. Shared credentials file (~/.aws/credentials)
// 3. IAM role (for EC2 instances)
// 4. ECS task role (for containers)
type Route53Provider struct {
	client   *route53.Route53
	zone     string
	zoneID   string
	ttl      int64
	region   string
	hostedZone *route53.HostedZone
}

// NewRoute53Provider creates a new Route53 DNS provider.
// Credentials are loaded via AWS SDK credential chain (no explicit credentials needed).
func NewRoute53Provider(cfg dnsprovider.Config) (dnsprovider.Provider, error) {
	region := "us-east-1" // Route53 is global, but SDK needs a region
	if r, ok := cfg.Credentials["region"]; ok && r != "" {
		region = r
	}

	// Create AWS session (uses standard AWS credential chain)
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	client := route53.New(sess)

	p := &Route53Provider{
		client: client,
		zone:   cfg.Zone,
		ttl:    int64(cfg.DefaultTTL),
		region: region,
	}

	// Find hosted zone ID
	if err := p.findHostedZone(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to find hosted zone: %w", err)
	}

	return p, nil
}

func (p *Route53Provider) Name() string {
	return "route53"
}

func (p *Route53Provider) UpsertA(ctx context.Context, zone string, name string, ip string, ttl int) error {
	if err := p.validateZone(zone); err != nil {
		return err
	}

	return p.upsertRecord(ctx, name, "A", []string{ip}, p.resolveTTL(ttl))
}

func (p *Route53Provider) UpsertAAAA(ctx context.Context, zone string, name string, ip string, ttl int) error {
	if err := p.validateZone(zone); err != nil {
		return err
	}

	return p.upsertRecord(ctx, name, "AAAA", []string{ip}, p.resolveTTL(ttl))
}

func (p *Route53Provider) UpsertCNAME(ctx context.Context, zone string, name string, target string, ttl int) error {
	if err := p.validateZone(zone); err != nil {
		return err
	}

	// Ensure target ends with dot
	if !strings.HasSuffix(target, ".") {
		target = target + "."
	}

	return p.upsertRecord(ctx, name, "CNAME", []string{target}, p.resolveTTL(ttl))
}

func (p *Route53Provider) UpsertTXT(ctx context.Context, zone string, name string, values []string, ttl int) error {
	if err := p.validateZone(zone); err != nil {
		return err
	}

	// Route53 requires TXT values to be quoted
	quotedValues := make([]string, len(values))
	for i, val := range values {
		quotedValues[i] = fmt.Sprintf(`"%s"`, val)
	}

	return p.upsertRecord(ctx, name, "TXT", quotedValues, p.resolveTTL(ttl))
}

func (p *Route53Provider) DeleteTXT(ctx context.Context, zone string, name string, values []string) error {
	if err := p.validateZone(zone); err != nil {
		return err
	}

	fqdn := p.fqdn(name)

	// If no specific values, delete all TXT records
	if len(values) == 0 {
		return p.deleteRecord(ctx, fqdn, "TXT")
	}

	// Get current records
	current, err := p.getRecordSet(ctx, fqdn, "TXT")
	if err != nil {
		return err
	}
	if current == nil {
		return nil // Already deleted
	}

	// Filter out values to delete
	valuesToDelete := make(map[string]bool)
	for _, v := range values {
		quotedVal := fmt.Sprintf(`"%s"`, v)
		valuesToDelete[quotedVal] = true
	}

	remaining := make([]*route53.ResourceRecord, 0)
	for _, rec := range current.ResourceRecords {
		if rec.Value != nil && !valuesToDelete[*rec.Value] {
			remaining = append(remaining, rec)
		}
	}

	// If no records remain, delete the record set
	if len(remaining) == 0 {
		return p.deleteRecord(ctx, fqdn, "TXT")
	}

	// Otherwise, update with remaining records
	current.ResourceRecords = remaining
	return p.changeRecordSet(ctx, "UPSERT", current)
}

func (p *Route53Provider) GetRecords(ctx context.Context, zone string, name string, rtype string) ([]dnsprovider.Record, error) {
	if err := p.validateZone(zone); err != nil {
		return nil, err
	}

	// List all record sets
	input := &route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(p.zoneID),
	}

	if name != "" {
		fqdn := p.fqdn(name)
		input.StartRecordName = aws.String(fqdn)
	}

	if rtype != "" {
		input.StartRecordType = aws.String(rtype)
	}

	resp, err := p.client.ListResourceRecordSetsWithContext(ctx, input)
	if err != nil {
		return nil, p.wrapError("GetRecords", zone, name, err)
	}

	// Convert to our Record format
	var records []dnsprovider.Record
	for _, rs := range resp.ResourceRecordSets {
		// Filter by name if specified
		if name != "" {
			expectedFQDN := p.fqdn(name)
			if rs.Name == nil || *rs.Name != expectedFQDN {
				continue
			}
		}

		// Filter by type if specified
		if rtype != "" && (rs.Type == nil || *rs.Type != rtype) {
			continue
		}

		// Extract relative name
		relativeName := p.extractRelativeName(aws.StringValue(rs.Name))

		// Convert resource records
		for _, rr := range rs.ResourceRecords {
			if rr.Value != nil {
				value := *rr.Value
				// Remove quotes from TXT records
				if rs.Type != nil && *rs.Type == "TXT" {
					value = strings.Trim(value, `"`)
				}

				records = append(records, dnsprovider.Record{
					Zone:  zone,
					Name:  relativeName,
					Type:  aws.StringValue(rs.Type),
					Value: value,
					TTL:   int(aws.Int64Value(rs.TTL)),
				})
			}
		}
	}

	return records, nil
}

// Helper methods

func (p *Route53Provider) upsertRecord(ctx context.Context, name string, rtype string, values []string, ttl int64) error {
	fqdn := p.fqdn(name)

	// Build resource records
	resourceRecords := make([]*route53.ResourceRecord, len(values))
	for i, val := range values {
		resourceRecords[i] = &route53.ResourceRecord{
			Value: aws.String(val),
		}
	}

	// Create change request
	change := &route53.Change{
		Action: aws.String("UPSERT"),
		ResourceRecordSet: &route53.ResourceRecordSet{
			Name:            aws.String(fqdn),
			Type:            aws.String(rtype),
			TTL:             aws.Int64(ttl),
			ResourceRecords: resourceRecords,
		},
	}

	return p.changeRecordSet(ctx, "UPSERT", change.ResourceRecordSet)
}

func (p *Route53Provider) deleteRecord(ctx context.Context, fqdn string, rtype string) error {
	// Get current record set
	current, err := p.getRecordSet(ctx, fqdn, rtype)
	if err != nil {
		return err
	}
	if current == nil {
		return nil // Already deleted
	}

	return p.changeRecordSet(ctx, "DELETE", current)
}

func (p *Route53Provider) changeRecordSet(ctx context.Context, action string, rs *route53.ResourceRecordSet) error {
	input := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(p.zoneID),
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action:            aws.String(action),
					ResourceRecordSet: rs,
				},
			},
		},
	}

	_, err := p.client.ChangeResourceRecordSetsWithContext(ctx, input)
	if err != nil {
		return p.wrapError("changeRecordSet", p.zone, aws.StringValue(rs.Name), err)
	}

	return nil
}

func (p *Route53Provider) getRecordSet(ctx context.Context, fqdn string, rtype string) (*route53.ResourceRecordSet, error) {
	input := &route53.ListResourceRecordSetsInput{
		HostedZoneId:    aws.String(p.zoneID),
		StartRecordName: aws.String(fqdn),
		StartRecordType: aws.String(rtype),
		MaxItems:        aws.String("1"),
	}

	resp, err := p.client.ListResourceRecordSetsWithContext(ctx, input)
	if err != nil {
		return nil, err
	}

	// Check if we found the exact record
	for _, rs := range resp.ResourceRecordSets {
		if aws.StringValue(rs.Name) == fqdn && aws.StringValue(rs.Type) == rtype {
			return rs, nil
		}
	}

	return nil, nil // Not found
}

func (p *Route53Provider) findHostedZone(ctx context.Context) error {
	// List all hosted zones
	input := &route53.ListHostedZonesByNameInput{
		DNSName: aws.String(p.zone),
	}

	resp, err := p.client.ListHostedZonesByNameWithContext(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to list hosted zones: %w", err)
	}

	// Find exact match
	zoneName := p.zone
	if !strings.HasSuffix(zoneName, ".") {
		zoneName = zoneName + "."
	}

	for _, hz := range resp.HostedZones {
		if aws.StringValue(hz.Name) == zoneName {
			p.zoneID = aws.StringValue(hz.Id)
			// Strip /hostedzone/ prefix if present
			p.zoneID = strings.TrimPrefix(p.zoneID, "/hostedzone/")
			p.hostedZone = hz
			return nil
		}
	}

	return fmt.Errorf("hosted zone %q not found in Route53", p.zone)
}

func (p *Route53Provider) fqdn(name string) string {
	if name == "" || name == "@" {
		return p.zone + "."
	}
	// Remove zone suffix if already present
	if strings.HasSuffix(name, "."+p.zone) {
		return name + "."
	}
	return name + "." + p.zone + "."
}

func (p *Route53Provider) extractRelativeName(fqdn string) string {
	// Remove trailing dot
	fqdn = strings.TrimSuffix(fqdn, ".")
	zoneName := strings.TrimSuffix(p.zone, ".")

	if fqdn == zoneName {
		return "@"
	}

	return strings.TrimSuffix(fqdn, "."+zoneName)
}

func (p *Route53Provider) validateZone(zone string) error {
	if zone != p.zone {
		return &dnsprovider.ProviderError{
			Provider: "route53",
			Op:       "validateZone",
			Zone:     zone,
			Err:      fmt.Errorf("zone mismatch: expected %q, got %q", p.zone, zone),
		}
	}
	return nil
}

func (p *Route53Provider) resolveTTL(ttl int) int64 {
	if ttl > 0 {
		return int64(ttl)
	}
	if p.ttl > 0 {
		return p.ttl
	}
	return 300 // Route53 default minimum
}

func (p *Route53Provider) wrapError(op, zone, name string, err error) error {
	return &dnsprovider.ProviderError{
		Provider: "route53",
		Op:       op,
		Zone:     zone,
		Name:     name,
		Err:      err,
	}
}
