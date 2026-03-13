package main

// provenance.go — immutable provenance records for artifact uploads.
//
// Every upload creates a provenance record stored alongside the manifest:
//   artifacts/{key}.provenance.json
//
// Provenance is write-once and never modified.

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc/peer"
)

func provenanceStorageKey(key string) string {
	return artifactsDir + "/" + key + ".provenance.json"
}

// provenanceDigestKey returns the storage key for the provenance SHA-256 digest sidecar file.
func provenanceDigestKey(key string) string {
	return artifactsDir + "/" + key + ".provenance.sha256"
}

// computeProvenanceDigest returns the hex SHA-256 of the provenance JSON.
func computeProvenanceDigest(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

// buildProvenanceRecord creates a ProvenanceRecord from the current request context.
func buildProvenanceRecord(ctx context.Context, manifest *repopb.ArtifactManifest) *repopb.ProvenanceRecord {
	prov := &repopb.ProvenanceRecord{
		TimestampUnix: time.Now().Unix(),
	}

	// Extract identity from AuthContext.
	authCtx := security.FromContext(ctx)
	if authCtx != nil {
		prov.Subject = authCtx.Subject
		prov.PrincipalType = authCtx.PrincipalType
		prov.AuthMethod = authCtx.AuthMethod
		prov.ClusterId = authCtx.ClusterID
	}

	// Extract source IP from peer info.
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		prov.SourceIp = p.Addr.String()
	}

	// Extract build metadata from manifest.
	if manifest != nil {
		prov.BuildCommit = manifest.GetBuildCommit()
		prov.BuildSource = manifest.GetBuildSource()
	}

	// Day-0 bootstrap marker: sa publishing from loopback is the installer's
	// post-install catalog population step. Tag it explicitly so Day-0
	// publishes are queryable without timestamp heuristics.
	// This is an audit label only — it does NOT affect trust enforcement.
	if prov.Subject == "sa" && isLoopbackAddr(prov.SourceIp) {
		prov.BuildSource = "day0-bootstrap"
	}

	// Fallback cluster ID from config.
	if prov.ClusterId == "" {
		if domain, err := config.GetDomain(); err == nil {
			prov.ClusterId = domain
		}
	}

	return prov
}

// isLoopbackAddr returns true if the address string is a loopback address.
func isLoopbackAddr(addr string) bool {
	return strings.HasPrefix(addr, "127.") ||
		strings.HasPrefix(addr, "[::1]") ||
		addr == "::1"
}

// writeProvenance persists a provenance record to storage and writes a SHA-256
// digest sidecar file for integrity verification. Returns the hex digest.
func (srv *server) writeProvenance(ctx context.Context, key string, prov *repopb.ProvenanceRecord) (string, error) {
	data, err := json.MarshalIndent(prov, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal provenance: %w", err)
	}

	storageKey := provenanceStorageKey(key)
	if err := srv.Storage().WriteFile(ctx, storageKey, data, 0o644); err != nil {
		return "", fmt.Errorf("write provenance %q: %w", storageKey, err)
	}

	// Compute and persist the provenance digest sidecar.
	digest := computeProvenanceDigest(data)
	digestStorageKey := provenanceDigestKey(key)
	if err := srv.Storage().WriteFile(ctx, digestStorageKey, []byte(digest), 0o644); err != nil {
		slog.Warn("provenance digest write failed (non-fatal)", "key", key, "err", err)
	}

	slog.Debug("provenance written", "key", key, "subject", prov.Subject, "digest", digest)
	return digest, nil
}

// readProvenance reads a provenance record from storage.
// Returns nil (not error) for legacy artifacts without provenance.
func (srv *server) readProvenance(ctx context.Context, key string) *repopb.ProvenanceRecord {
	data, err := srv.Storage().ReadFile(ctx, provenanceStorageKey(key))
	if err != nil {
		return nil // legacy artifact, no provenance
	}

	prov := &repopb.ProvenanceRecord{}
	if err := json.Unmarshal(data, prov); err != nil {
		slog.Warn("corrupt provenance file", "key", key, "err", err)
		return nil
	}
	return prov
}

// readProvenanceDigest reads the SHA-256 digest sidecar for a provenance record.
// Returns empty string if the digest file does not exist (legacy artifacts).
func (srv *server) readProvenanceDigest(ctx context.Context, key string) string {
	data, err := srv.Storage().ReadFile(ctx, provenanceDigestKey(key))
	if err != nil {
		return ""
	}
	return string(data)
}
