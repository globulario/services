package main

import (
	"context"
	"errors"
	"testing"

	"github.com/globulario/services/golang/config"
)

func TestPublishCAMetadataLocked_PublishesPlaceholderWhenCAUnavailable(t *testing.T) {
	srv := &server{
		state: &controllerState{
			CAGeneration: 1,
		},
	}

	origFP := caSPKIFingerprintFn
	origSaveMeta := saveCAMetadataFn
	origSaveCert := saveCACertificateIfEmptyFn
	t.Cleanup(func() {
		caSPKIFingerprintFn = origFP
		saveCAMetadataFn = origSaveMeta
		saveCACertificateIfEmptyFn = origSaveCert
	})

	caSPKIFingerprintFn = func(string) (string, error) {
		return "", errors.New("ca not readable")
	}

	var got config.CAMetadata
	saveCAMetadataFn = func(_ context.Context, meta config.CAMetadata) error {
		got = meta
		return nil
	}
	saveCACertificateIfEmptyFn = func(_ context.Context, _ []byte) error {
		t.Fatal("SaveCACertificateIfEmpty must not be called for placeholder publish path")
		return nil
	}

	srv.publishCAMetadataLocked()

	if got.Fingerprint != "pending-day0-bootstrap" {
		t.Fatalf("placeholder fingerprint=%q, want pending-day0-bootstrap", got.Fingerprint)
	}
	if got.Active {
		t.Fatal("placeholder Active must be false")
	}
}
