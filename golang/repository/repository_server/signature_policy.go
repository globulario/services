package main

// signature_policy.go — Phase F Part 3 signature-installability policy.
//
// One central decision: given an artifact and the repository's signature
// policy, can it be PUBLISHED / resolved / downloaded?
//
// Policy is read from etcd at /globular/repository/security/policy. Defaults
// in defaultSignaturePolicy are conservative for production — core packages
// require trusted signatures, third-party do not, dev mode is OFF.
//
// The repository never makes signature decisions in scattered call sites.
// Every installability gate (sync publish, resolver, DownloadArtifact,
// rollback eligibility, repository findings) calls signaturePolicyDecision.

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/encoding/protojson"
)

const signaturePolicyEtcdKey = "/globular/repository/security/policy"

// defaultSignaturePolicy returns the migration-safe defaults applied when
// /globular/repository/security/policy is absent in etcd.
//
// IMPORTANT: this default is intentionally PERMISSIVE so that upgrading an
// existing cluster does NOT instantly block unsigned core artifacts. The
// stricter rule (require_signatures_for_core=true) is the production target,
// but it must be opt-in: operators register trusted publisher keys, sign
// their core artifacts, then write the strict policy to etcd:
//
//   etcdctl put /globular/repository/security/policy '{
//     "require_signatures_for_core":      true,
//     "trusted_core_publishers":          ["core@globular.io"],
//     "quarantine_on_invalid_signature":  true
//   }'
//
// REVOKED keys are ALWAYS disqualifying regardless of policy — the
// permissive default does NOT loosen that.
func defaultSignaturePolicy() *repopb.SignaturePolicy {
	return &repopb.SignaturePolicy{
		RequireSignaturesForCore:      false,
		RequireSignaturesForAll:       false,
		AllowUnsignedLocalDevelopment: true,
		TrustedCorePublishers:         []string{"core@globular.io"},
		QuarantineOnInvalidSignature:  true,
	}
}

// signaturePolicyCache memoizes the policy read from etcd. Refreshed via
// LoadSignaturePolicy() with a small TTL — we don't need fresh reads on
// every download. Backed by an in-process atomic snapshot (not a singleton)
// so tests can swap their own policy via SetSignaturePolicyForTest.
type signaturePolicyCache struct {
	mu       sync.Mutex
	policy   *repopb.SignaturePolicy
	loadedAt time.Time
	ttl      time.Duration
	override *repopb.SignaturePolicy // tests / admin RPC
}

func newSignaturePolicyCache() *signaturePolicyCache {
	return &signaturePolicyCache{ttl: 30 * time.Second}
}

// SetPolicyForTest replaces the in-memory policy unconditionally. Production
// callers don't use this — they go through LoadSignaturePolicy.
func (c *signaturePolicyCache) SetPolicyForTest(p *repopb.SignaturePolicy) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.override = p
}

// CurrentPolicy returns the current policy, refreshing from etcd if the
// cache TTL expired. Falls back to defaults on any read error.
func (c *signaturePolicyCache) CurrentPolicy(ctx context.Context) *repopb.SignaturePolicy {
	c.mu.Lock()
	if c.override != nil {
		o := c.override
		c.mu.Unlock()
		return o
	}
	if c.policy != nil && time.Since(c.loadedAt) < c.ttl {
		p := c.policy
		c.mu.Unlock()
		return p
	}
	c.mu.Unlock()

	// Read fresh.
	p := loadSignaturePolicyFromEtcd(ctx)
	c.mu.Lock()
	c.policy = p
	c.loadedAt = time.Now()
	c.mu.Unlock()
	return p
}

// loadSignaturePolicyFromEtcd reads the policy message; returns defaults on
// any error / missing key.
func loadSignaturePolicyFromEtcd(ctx context.Context) *repopb.SignaturePolicy {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return defaultSignaturePolicy()
	}
	tctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	resp, err := cli.Get(tctx, signaturePolicyEtcdKey, clientv3.WithLimit(1))
	if err != nil || len(resp.Kvs) == 0 {
		return defaultSignaturePolicy()
	}
	var p repopb.SignaturePolicy
	if err := protojson.Unmarshal(resp.Kvs[0].Value, &p); err != nil {
		return defaultSignaturePolicy()
	}
	// Backfill defaults for unset fields. Empty TrustedCorePublishers means
	// "no core publishers" — operators must opt in. We treat zero as default
	// because that's the more common operator intent.
	if len(p.TrustedCorePublishers) == 0 {
		p.TrustedCorePublishers = []string{"core@globular.io"}
	}
	return &p
}

// SignaturePolicyDecision holds the verdict from signaturePolicyDecision.
type SignaturePolicyDecision struct {
	// Required is true when the policy requires a valid trusted signature
	// for this artifact. When Required is false, the artifact is allowed
	// regardless of signature outcome.
	Required bool

	// Status is the underlying SignatureStatus from verifyArtifactSignature.
	Status repopb.SignatureStatus

	// Allowed is the final yes/no: an artifact is allowed when
	//   !Required, OR
	//   Required && Status == SIGNATURE_OK.
	Allowed bool

	// Reason is a short operator-readable string. Empty when Allowed.
	Reason string
}

// signatureRequiredForRef returns whether the artifact MUST have a valid
// trusted signature under the current policy.
func (srv *server) signatureRequiredForRef(p *repopb.SignaturePolicy, ref *repopb.ArtifactRef, sourceProvider string) bool {
	if p == nil {
		return false
	}
	if p.GetRequireSignaturesForAll() {
		return true
	}
	if p.GetAllowUnsignedLocalDevelopment() && strings.EqualFold(sourceProvider, "LOCAL_DIR") {
		return false
	}
	if p.GetRequireSignaturesForCore() {
		for _, core := range p.GetTrustedCorePublishers() {
			if strings.EqualFold(core, ref.GetPublisherId()) {
				return true
			}
		}
	}
	return false
}

// ensureSignaturePolicy lazily initializes the cache so newTestServer (which
// constructs a *server without going through full main()) Just Works.
func (srv *server) ensureSignaturePolicy() *signaturePolicyCache {
	srv.artifactStateMu.Lock()
	defer srv.artifactStateMu.Unlock()
	if srv.signaturePolicy == nil {
		srv.signaturePolicy = newSignaturePolicyCache()
	}
	return srv.signaturePolicy
}

// signaturePolicyDecision is the central decision used by every installability
// gate. expectedDigest is the artifact's recorded checksum.
func (srv *server) signaturePolicyDecision(ctx context.Context, ref *repopb.ArtifactRef, artifactKey, expectedDigest, sourceProvider string) SignaturePolicyDecision {
	p := srv.ensureSignaturePolicy().CurrentPolicy(ctx)
	required := srv.signatureRequiredForRef(p, ref, sourceProvider)
	st, _, _, reason := srv.verifyArtifactSignature(ctx, artifactKey, expectedDigest, ref.GetPublisherId())
	dec := SignaturePolicyDecision{
		Required: required,
		Status:   st,
	}
	switch st {
	case repopb.SignatureStatus_SIGNATURE_OK:
		dec.Allowed = true
		return dec
	case repopb.SignatureStatus_SIGNATURE_REVOKED_KEY:
		// Revoked key is ALWAYS disqualifying — even when signatures aren't
		// required for the publisher, a revoked key means we explicitly
		// don't trust this signature record.
		dec.Allowed = false
		dec.Reason = "publisher key REVOKED"
		return dec
	case repopb.SignatureStatus_SIGNATURE_INVALID,
		repopb.SignatureStatus_SIGNATURE_DIGEST_MISMATCH:
		// Invalid signatures are always disqualifying when present.
		dec.Allowed = false
		dec.Reason = "signature " + strings.ToLower(strings.TrimPrefix(st.String(), "SIGNATURE_"))
		return dec
	}
	// MISSING / UNTRUSTED_PUBLISHER / EXPIRED_KEY / INCONCLUSIVE:
	// allowed only when not required.
	if !required {
		dec.Allowed = true
		return dec
	}
	dec.Allowed = false
	if reason != "" {
		dec.Reason = reason
	} else {
		dec.Reason = strings.ToLower(strings.TrimPrefix(st.String(), "SIGNATURE_"))
	}
	return dec
}
