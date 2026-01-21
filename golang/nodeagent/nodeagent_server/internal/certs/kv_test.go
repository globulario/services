package certs

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestMemKVPutGetRoundTrip(t *testing.T) {
	kv := NewMemKV()
	bundle := CertBundle{Key: []byte("k"), Fullchain: []byte("f"), CA: []byte("c")}
	if err := kv.PutBundle(context.Background(), "example.com", bundle); err != nil {
		t.Fatalf("PutBundle: %v", err)
	}
	got, err := kv.GetBundle(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("GetBundle: %v", err)
	}
	if !reflect.DeepEqual(got.Key, bundle.Key) || !reflect.DeepEqual(got.Fullchain, bundle.Fullchain) || !reflect.DeepEqual(got.CA, bundle.CA) {
		t.Fatalf("bundle mismatch: got %+v", got)
	}
	if got.Generation == 0 {
		t.Fatalf("expected generation set, got %d", got.Generation)
	}
}

func TestMemKVMonotonicGeneration(t *testing.T) {
	kv := NewMemKV()
	ctx := context.Background()

	_ = kv.PutBundle(ctx, "example.com", CertBundle{Generation: 1})
	first, _ := kv.GetBundle(ctx, "example.com")
	if first.Generation != 1 {
		t.Fatalf("expected generation 1, got %d", first.Generation)
	}

	_ = kv.PutBundle(ctx, "example.com", CertBundle{Generation: 1})
	second, _ := kv.GetBundle(ctx, "example.com")
	if second.Generation <= first.Generation {
		t.Fatalf("expected generation to increase, got %d after %d", second.Generation, first.Generation)
	}

	_ = kv.PutBundle(ctx, "example.com", CertBundle{Generation: 10})
	third, _ := kv.GetBundle(ctx, "example.com")
	if third.Generation != 10 {
		t.Fatalf("expected generation to jump to 10, got %d", third.Generation)
	}
}

func TestMemKVWaitForBundle(t *testing.T) {
	kv := NewMemKV()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	done := make(chan CertBundle, 1)
	go func() {
		b, err := kv.WaitForBundle(ctx, "example.com", 800*time.Millisecond)
		if err != nil {
			t.Errorf("WaitForBundle: %v", err)
			return
		}
		done <- b
	}()

	time.Sleep(100 * time.Millisecond)
	_ = kv.PutBundle(context.Background(), "example.com", CertBundle{Generation: 5})

	select {
	case b := <-done:
		if b.Generation == 0 {
			t.Fatalf("expected generation set, got %d", b.Generation)
		}
	case <-time.After(time.Second):
		t.Fatalf("wait timed out")
	}
}
