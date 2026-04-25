package config

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// EtcdKeyCAMetadata is the canonical etcd key for CA fingerprint + validity metadata.
// Published by the cluster controller on startup, after CA creation, after CA rotation,
// and after controller state restore. Node agents poll this to detect CA drift without
// querying the gateway — a mismatch means service certs must be regenerated.
//
// +globular:schema:key="/globular/pki/ca"
// +globular:schema:writer="globular-cluster-controller"
// +globular:schema:readers="globular-node-agent,globular-cluster-doctor"
// +globular:schema:description="Active CA SPKI fingerprint, issuer, validity window, generation."
// +globular:schema:invariants="Single-writer (controller); generation monotonic; never patched externally."
const EtcdKeyCAMetadata = "/globular/pki/ca"

// EtcdKeyCACertificate is the etcd key holding the public CA certificate PEM.
// Published alongside EtcdKeyCAMetadata. Node agents fetch this on rejoin to
// bootstrap trust before the first service cert is issued.
//
// +globular:schema:key="/globular/pki/ca.crt"
// +globular:schema:writer="globular-cluster-controller"
// +globular:schema:readers="globular-node-agent"
// +globular:schema:description="Public CA certificate PEM for bootstrapping node trust on rejoin."
const EtcdKeyCACertificate = "/globular/pki/ca.crt"

// CAMetadata is the CA descriptor published by the cluster controller to etcd.
// Node agents compare their local CA SPKI fingerprint to Fingerprint to detect
// CA rotation. A mismatch means the local CA (and all certs it signed) are stale.
//
// JSON schema (canonical — matches etcd value exactly):
//
//	{
//	  "generation": 1,
//	  "fingerprint": "<sha256 of SubjectPublicKeyInfo>",
//	  "issuer": "...",
//	  "not_before": "2024-01-01T00:00:00Z",
//	  "not_after":  "2026-01-01T00:00:00Z",
//	  "active": true
//	}
type CAMetadata struct {
	// Generation is incremented each time the CA is rotated. Monotonic.
	Generation int `json:"generation"`

	// Fingerprint is SHA-256 hex of the CA certificate's SubjectPublicKeyInfo.
	// Stable across renewals (same key pair); changes only on full CA rotation.
	Fingerprint string `json:"fingerprint"`

	// Issuer is the CA certificate's Subject.CommonName.
	Issuer string `json:"issuer"`

	// NotBefore and NotAfter are the CA validity window in RFC3339 format.
	NotBefore string `json:"not_before"`
	NotAfter  string `json:"not_after"`

	// Active is true when this CA is the currently trusted signing authority.
	// Set to false during a rotation grace period to support dual-CA trust.
	Active bool `json:"active"`
}

// NotAfterTime parses NotAfter as a time.Time. Returns zero value on error.
func (m *CAMetadata) NotAfterTime() time.Time {
	if m == nil || m.NotAfter == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, m.NotAfter)
	return t
}

// NotBeforeTime parses NotBefore as a time.Time. Returns zero value on error.
func (m *CAMetadata) NotBeforeTime() time.Time {
	if m == nil || m.NotBefore == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, m.NotBefore)
	return t
}

// SaveCAMetadata writes the CA metadata to etcd. Called by the cluster controller
// on startup and whenever the CA cert changes.
func SaveCAMetadata(ctx context.Context, meta CAMetadata) error {
	if meta.Fingerprint == "" {
		return fmt.Errorf("pki ca metadata: fingerprint required")
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("pki ca metadata: marshal: %w", err)
	}

	cli, err := GetEtcdClient()
	if err != nil {
		return fmt.Errorf("pki ca metadata: etcd unavailable: %w", err)
	}
	if _, err := cli.Put(ctx, EtcdKeyCAMetadata, string(data)); err != nil {
		return fmt.Errorf("pki ca metadata: etcd put: %w", err)
	}
	return nil
}

// LoadCAMetadata reads the CA metadata from etcd.
// Returns nil, nil if the key has not been written yet (pre-bootstrap).
func LoadCAMetadata(ctx context.Context) (*CAMetadata, error) {
	cli, err := GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("pki ca metadata: etcd unavailable: %w", err)
	}
	resp, err := cli.Get(ctx, EtcdKeyCAMetadata)
	if err != nil {
		return nil, fmt.Errorf("pki ca metadata: etcd get: %w", err)
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	var meta CAMetadata
	if err := json.Unmarshal(resp.Kvs[0].Value, &meta); err != nil {
		return nil, fmt.Errorf("pki ca metadata: parse: %w", err)
	}
	return &meta, nil
}

// SaveCACertificateIfEmpty writes the CA certificate PEM to etcd only when the
// key is absent. This preserves an existing CA cert during hot restarts while
// ensuring node agents always have a path to fetch the cluster CA on rejoin.
func SaveCACertificateIfEmpty(ctx context.Context, pemBytes []byte) error {
	if len(pemBytes) == 0 {
		return fmt.Errorf("pki ca cert: empty PEM")
	}
	cli, err := GetEtcdClient()
	if err != nil {
		return fmt.Errorf("pki ca cert: etcd unavailable: %w", err)
	}
	// Only write if absent.
	resp, err := cli.Get(ctx, EtcdKeyCACertificate)
	if err != nil {
		return fmt.Errorf("pki ca cert: etcd get: %w", err)
	}
	if len(resp.Kvs) > 0 {
		return nil // already present — leave it alone
	}
	if _, err := cli.Put(ctx, EtcdKeyCACertificate, string(pemBytes)); err != nil {
		return fmt.Errorf("pki ca cert: etcd put: %w", err)
	}
	return nil
}

// LoadCACertificate reads the public CA certificate PEM from etcd.
// Returns nil, nil if the key has not been written yet.
func LoadCACertificate(ctx context.Context) ([]byte, error) {
	cli, err := GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("pki ca cert: etcd unavailable: %w", err)
	}
	resp, err := cli.Get(ctx, EtcdKeyCACertificate)
	if err != nil {
		return nil, fmt.Errorf("pki ca cert: etcd get: %w", err)
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	return resp.Kvs[0].Value, nil
}
