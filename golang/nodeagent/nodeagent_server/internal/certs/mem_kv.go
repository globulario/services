package certs

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type memKV struct {
	mu      sync.Mutex
	cond    *sync.Cond
	bundles map[string]CertBundle
}

func NewMemKV() KV {
	m := &memKV{
		bundles: make(map[string]CertBundle),
	}
	m.cond = sync.NewCond(&m.mu)
	return m
}

func (m *memKV) AcquireCertIssuerLock(ctx context.Context, domain, nodeID string, ttl time.Duration) (bool, func(), error) {
	if m == nil {
		return false, nil, fmt.Errorf("kv unavailable")
	}
	return true, func() {}, nil
}

func (m *memKV) PutBundle(ctx context.Context, domain string, bundle CertBundle) error {
	if m == nil {
		return fmt.Errorf("kv unavailable")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	current := m.bundles[domain]
	nextGen := current.Generation + 1
	if bundle.Generation > nextGen {
		nextGen = bundle.Generation
	}
	bundle.Generation = nextGen
	bundle.UpdatedMS = time.Now().UnixMilli()
	m.bundles[domain] = bundle
	m.cond.Broadcast()
	return nil
}

func (m *memKV) GetBundle(ctx context.Context, domain string) (CertBundle, error) {
	if m == nil {
		return CertBundle{}, fmt.Errorf("kv unavailable")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if b, ok := m.bundles[domain]; ok {
		return b, nil
	}
	return CertBundle{}, fmt.Errorf("bundle not found")
}

func (m *memKV) WaitForBundle(ctx context.Context, domain string, timeout time.Duration) (CertBundle, error) {
	if m == nil {
		return CertBundle{}, fmt.Errorf("kv unavailable")
	}
	deadline := time.Now().Add(timeout)
	m.mu.Lock()
	defer m.mu.Unlock()
	for {
		if b, ok := m.bundles[domain]; ok {
			return b, nil
		}
		if ctx != nil {
			select {
			case <-ctx.Done():
				return CertBundle{}, ctx.Err()
			default:
			}
		}
		if time.Now().After(deadline) {
			return CertBundle{}, fmt.Errorf("bundle not found for %s within timeout", domain)
		}
		remaining := time.Until(deadline)
		timer := time.AfterFunc(remaining, m.cond.Broadcast)
		m.cond.Wait()
		timer.Stop()
	}
}

func (m *memKV) GetBundleGeneration(ctx context.Context, domain string) (uint64, error) {
	bundle, err := m.GetBundle(ctx, domain)
	if err != nil {
		return 0, err
	}
	return bundle.Generation, nil
}
