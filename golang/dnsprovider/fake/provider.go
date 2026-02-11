package fake

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/dnsprovider"
)

// FakeProvider is an in-memory DNS provider for testing.
// It stores records in a map and simulates DNS operations without
// making real network calls.
type FakeProvider struct {
	zone    string
	records map[string][]dnsprovider.Record
	mu      sync.RWMutex
	ttl     int

	// For testing error scenarios
	failUpsertA    bool
	failUpsertTXT  bool
	failDeleteTXT  bool
	failGetRecords bool
}

// NewFakeProvider creates a new in-memory fake provider for testing.
func NewFakeProvider(cfg dnsprovider.Config) (dnsprovider.Provider, error) {
	return &FakeProvider{
		zone:    cfg.Zone,
		records: make(map[string][]dnsprovider.Record),
		ttl:     cfg.DefaultTTL,
	}, nil
}

func (p *FakeProvider) Name() string {
	return "fake"
}

// SetFailure configures the provider to fail specific operations (for testing).
func (p *FakeProvider) SetFailure(op string, fail bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch op {
	case "UpsertA":
		p.failUpsertA = fail
	case "UpsertTXT":
		p.failUpsertTXT = fail
	case "DeleteTXT":
		p.failDeleteTXT = fail
	case "GetRecords":
		p.failGetRecords = fail
	}
}

func (p *FakeProvider) UpsertA(ctx context.Context, zone string, name string, ip string, ttl int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.failUpsertA {
		return fmt.Errorf("fake: simulated UpsertA failure")
	}

	if err := p.validateZoneLocked(zone); err != nil {
		return err
	}

	key := p.recordKey(name, "A")
	p.records[key] = []dnsprovider.Record{{
		Zone:   zone,
		Name:   name,
		Type:   "A",
		Value:  ip,
		TTL:    p.resolveTTL(ttl),
		Expiry: time.Now().Add(time.Duration(p.resolveTTL(ttl)) * time.Second),
	}}

	return nil
}

func (p *FakeProvider) UpsertAAAA(ctx context.Context, zone string, name string, ip string, ttl int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.validateZoneLocked(zone); err != nil {
		return err
	}

	key := p.recordKey(name, "AAAA")
	p.records[key] = []dnsprovider.Record{{
		Zone:   zone,
		Name:   name,
		Type:   "AAAA",
		Value:  ip,
		TTL:    p.resolveTTL(ttl),
		Expiry: time.Now().Add(time.Duration(p.resolveTTL(ttl)) * time.Second),
	}}

	return nil
}

func (p *FakeProvider) UpsertCNAME(ctx context.Context, zone string, name string, target string, ttl int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.validateZoneLocked(zone); err != nil {
		return err
	}

	key := p.recordKey(name, "CNAME")
	p.records[key] = []dnsprovider.Record{{
		Zone:   zone,
		Name:   name,
		Type:   "CNAME",
		Value:  target,
		TTL:    p.resolveTTL(ttl),
		Expiry: time.Now().Add(time.Duration(p.resolveTTL(ttl)) * time.Second),
	}}

	return nil
}

func (p *FakeProvider) UpsertTXT(ctx context.Context, zone string, name string, values []string, ttl int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.failUpsertTXT {
		return fmt.Errorf("fake: simulated UpsertTXT failure")
	}

	if err := p.validateZoneLocked(zone); err != nil {
		return err
	}

	key := p.recordKey(name, "TXT")
	records := make([]dnsprovider.Record, len(values))
	for i, val := range values {
		records[i] = dnsprovider.Record{
			Zone:   zone,
			Name:   name,
			Type:   "TXT",
			Value:  val,
			TTL:    p.resolveTTL(ttl),
			Expiry: time.Now().Add(time.Duration(p.resolveTTL(ttl)) * time.Second),
		}
	}
	p.records[key] = records

	return nil
}

func (p *FakeProvider) DeleteTXT(ctx context.Context, zone string, name string, values []string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.failDeleteTXT {
		return fmt.Errorf("fake: simulated DeleteTXT failure")
	}

	if err := p.validateZoneLocked(zone); err != nil {
		return err
	}

	key := p.recordKey(name, "TXT")

	if len(values) == 0 {
		// Delete all TXT records
		delete(p.records, key)
		return nil
	}

	// Delete specific values
	current, exists := p.records[key]
	if !exists {
		return nil // Already deleted
	}

	valuesToDelete := make(map[string]bool)
	for _, v := range values {
		valuesToDelete[v] = true
	}

	remaining := make([]dnsprovider.Record, 0)
	for _, rec := range current {
		if !valuesToDelete[rec.Value] {
			remaining = append(remaining, rec)
		}
	}

	if len(remaining) == 0 {
		delete(p.records, key)
	} else {
		p.records[key] = remaining
	}

	return nil
}

func (p *FakeProvider) GetRecords(ctx context.Context, zone string, name string, rtype string) ([]dnsprovider.Record, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.failGetRecords {
		return nil, fmt.Errorf("fake: simulated GetRecords failure")
	}

	if err := p.validateZoneLocked(zone); err != nil {
		return nil, err
	}

	// If both name and type specified, return specific records
	if name != "" && rtype != "" {
		key := p.recordKey(name, rtype)
		if records, exists := p.records[key]; exists {
			return records, nil
		}
		return []dnsprovider.Record{}, nil
	}

	// Return all matching records
	var result []dnsprovider.Record
	for _, records := range p.records {
		for _, rec := range records {
			match := true
			if name != "" && rec.Name != name {
				match = false
			}
			if rtype != "" && rec.Type != rtype {
				match = false
			}
			if match {
				result = append(result, rec)
			}
		}
	}

	return result, nil
}

// Helper methods

func (p *FakeProvider) recordKey(name, rtype string) string {
	return fmt.Sprintf("%s/%s", strings.ToLower(rtype), name)
}

func (p *FakeProvider) validateZoneLocked(zone string) error {
	if zone != p.zone {
		return &dnsprovider.ProviderError{
			Provider: "fake",
			Op:       "validateZone",
			Zone:     zone,
			Err:      fmt.Errorf("zone mismatch: expected %q, got %q", p.zone, zone),
		}
	}
	return nil
}

func (p *FakeProvider) resolveTTL(ttl int) int {
	if ttl > 0 {
		return ttl
	}
	if p.ttl > 0 {
		return p.ttl
	}
	return 600
}

// Testing helpers

// Reset clears all stored records (for test cleanup).
func (p *FakeProvider) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.records = make(map[string][]dnsprovider.Record)
}

// RecordCount returns the number of record sets stored.
func (p *FakeProvider) RecordCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.records)
}

// HasRecord checks if a specific record exists.
func (p *FakeProvider) HasRecord(name, rtype, value string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	key := p.recordKey(name, rtype)
	records, exists := p.records[key]
	if !exists {
		return false
	}

	for _, rec := range records {
		if rec.Value == value {
			return true
		}
	}
	return false
}
