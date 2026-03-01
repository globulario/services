package certs

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeKV is an in-memory KV used for tests.
type fakeKV struct {
	mu      sync.Mutex
	bundles map[string]CertBundle
	gen     map[string]uint64
}

func newFakeKV() *fakeKV {
	return &fakeKV{
		bundles: make(map[string]CertBundle),
		gen:     make(map[string]uint64),
	}
}

func (f *fakeKV) AcquireCertIssuerLock(ctx context.Context, domain, nodeID string, ttl time.Duration) (bool, func(), error) {
	return true, func() {}, nil
}

func (f *fakeKV) PutBundle(ctx context.Context, domain string, bundle CertBundle) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	g := f.gen[domain]
	if bundle.Generation > g {
		g = bundle.Generation
	} else {
		g++
	}
	bundle.Generation = g
	f.gen[domain] = g
	f.bundles[domain] = bundle
	return nil
}

func (f *fakeKV) GetBundle(ctx context.Context, domain string) (CertBundle, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.bundles[domain]
	if !ok {
		return CertBundle{}, errors.New("not found")
	}
	return b, nil
}

func (f *fakeKV) WaitForBundle(ctx context.Context, domain string, timeout time.Duration) (CertBundle, error) {
	deadline := time.Now().Add(timeout)
	for {
		if b, err := f.GetBundle(ctx, domain); err == nil {
			return b, nil
		}
		if time.Now().After(deadline) {
			return CertBundle{}, errors.New("timeout")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (f *fakeKV) GetBundleGeneration(ctx context.Context, domain string) (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if g, ok := f.gen[domain]; ok {
		return g, nil
	}
	return 0, errors.New("not found")
}

func TestCertKV_PutGet_RoundTrip(t *testing.T) {
	kv := newFakeKV()
	ctx := context.Background()
	b := CertBundle{Key: []byte("k1"), Fullchain: []byte("f1"), CA: []byte("c1"), Generation: 1}
	if err := kv.PutBundle(ctx, "example.com", b); err != nil {
		t.Fatalf("PutBundle: %v", err)
	}
	got, err := kv.GetBundle(ctx, "example.com")
	if err != nil {
		t.Fatalf("GetBundle: %v", err)
	}
	if string(got.Key) != "k1" || string(got.Fullchain) != "f1" || string(got.CA) != "c1" {
		t.Fatalf("unexpected bundle data: %+v", got)
	}
	if got.Generation != 1 {
		t.Fatalf("expected generation 1, got %d", got.Generation)
	}
}

func TestCertKV_GenerationMonotonic(t *testing.T) {
	kv := newFakeKV()
	ctx := context.Background()
	if err := kv.PutBundle(ctx, "example.com", CertBundle{Generation: 1}); err != nil {
		t.Fatalf("PutBundle: %v", err)
	}
	if err := kv.PutBundle(ctx, "example.com", CertBundle{}); err != nil {
		t.Fatalf("PutBundle second: %v", err)
	}
	b, err := kv.GetBundle(ctx, "example.com")
	if err != nil {
		t.Fatalf("GetBundle: %v", err)
	}
	if b.Generation != 2 {
		t.Fatalf("expected generation 2, got %d", b.Generation)
	}
}

func TestCertKV_WaitForBundle(t *testing.T) {
	kv := newFakeKV()
	ctx := context.Background()
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = kv.PutBundle(ctx, "example.com", CertBundle{})
	}()
	if _, err := kv.WaitForBundle(ctx, "example.com", time.Second); err != nil {
		t.Fatalf("WaitForBundle: %v", err)
	}
}
