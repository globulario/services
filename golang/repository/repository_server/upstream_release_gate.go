// @awareness namespace=globular.platform
// @awareness component=platform_repository.upstream_release_gate
// @awareness file_role=ingestion_release_authority_gate
// @awareness implements=globular.platform:intent.package.release_authority_is_rbac_native
// @awareness risk=high
package main

// upstream_release_gate.go — in-cluster release-authority gate on the
// upstream-sync ingestion path (P2).
//
// The repository pulls release-index.json from a forge (GitHub) and each entry
// carries a CI-stamped `channel`. Trusting that field makes CI the release
// authority: a build is STABLE because CI *said so* in a JSON file. This gate
// re-derives channel authority from RBAC so CI's claim is untrusted input.
//
// Two-step trust (docs/design/package-lifecycle.md §3.4, mirrored from the
// AllocateUpload gate) — the steps must never collapse:
//
//	FEDERATION    (who is speaking): the upstream forge identity matches a
//	              registered trusted publisher for the namespace. Grants nothing.
//	AUTHORIZATION (what they may do): the federated subject holds release.allocate
//	              on the namespace (RBAC only). This alone permits STABLE.
//
// Invariants enforced (docs/awareness/package_identity_invariants.yaml):
//   - package.release_allocation_requires_rbac_permission
//   - package.forge_binding_is_not_authorization   ← the reason both steps exist
//   - package.ci_is_not_release_authority
//   - package.stable_channel_requires_release_permission
//
// Rollout safety: the gate is INERT for a namespace with no registered trusted
// publishers (unmanaged → channel unchanged). It activates per-namespace only
// once a release authority is declared, so it never retroactively downgrades a
// pre-existing sync. The action is a non-destructive STABLE→DEV downgrade
// (imported and inspectable, simply not convergeable), reversible by granting
// the authority and re-syncing.

import (
	"context"
	"log/slog"
	"strings"

	rbacpb "github.com/globulario/services/golang/rbac/rbacpb"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// upstreamForgeSubject derives the forge identity (the RBAC subject candidate)
// for an upstream source: configured GitHub owner, else repo, else source name.
// This is the identity FEDERATION resolves; it carries no authority by itself.
func upstreamForgeSubject(src *repopb.UpstreamSource) string {
	if s := strings.TrimSpace(src.GetOwner()); s != "" {
		return s
	}
	if s := strings.TrimSpace(src.GetRepo()); s != "" {
		return s
	}
	return strings.TrimSpace(src.GetName())
}

// evaluateUpstreamRelease gathers the three facts the gate needs for an imported
// entry WITHOUT an AuthContext (the sync path has none):
//
//	managed    — the namespace has registered trusted publishers, so a release
//	             authority has been declared and the gate is active. No trusted
//	             publishers ⇒ unmanaged ⇒ inert (safe rollout).
//	federated  — the forge identity matches a trusted publisher for the namespace
//	             (FEDERATION). Grants nothing alone.
//	authorized — the federated subject holds release.allocate on the namespace
//	             (AUTHORIZATION, RBAC only). This is what permits STABLE.
//
// Error handling balances availability and security: a trusted-publisher store
// error fails toward `unmanaged` (a transient hiccup must not downgrade every
// namespace), while the RBAC authorization step fails closed (consistent with
// the AllocateUpload gate — an unprovable permission is no permission).
func (srv *server) evaluateUpstreamRelease(ctx context.Context, n *normalizedEntry, src *repopb.UpstreamSource) (managed, federated, authorized bool) {
	tps, err := srv.listTrustedPublishers(ctx, n.Publisher)
	if err != nil {
		slog.Warn("upstream-release-gate: trusted-publisher lookup failed; treating namespace as unmanaged",
			"publisher", n.Publisher, "err", err)
		return false, false, false
	}
	if len(tps) == 0 {
		return false, false, false // unmanaged: no declared release authority
	}
	managed = true

	subject := upstreamForgeSubject(src)
	if subject == "" {
		return managed, false, false
	}
	federated = srv.matchesTrustedPublisherBySubject(ctx, n.Publisher, n.Name, subject)
	if !federated {
		return managed, false, false
	}

	// FEDERATION proven; AUTHORIZATION is a separate RBAC step
	// (package.forge_binding_is_not_authorization).
	ok, aerr := srv.subjectHoldsReleaseAuthority(subject, rbacpb.SubjectType_APPLICATION, n.Publisher)
	if aerr != nil {
		slog.Warn("upstream-release-gate: release authority check failed; denying STABLE (fail-closed)",
			"publisher", n.Publisher, "subject", subject, "err", aerr)
		return managed, federated, false
	}
	return managed, federated, ok
}

// upstreamReleaseDecision is the pure channel decision for an upstream import.
// Only STABLE (the convergeable channel — the controller resolves desired state
// from STABLE only) requires release authority; every other channel passes
// through untouched. For a managed namespace, STABLE survives only when the
// upstream publisher is BOTH federated AND authorized; otherwise it is
// downgraded to DEV. Returns the final channel string and whether a downgrade
// occurred. Kept pure so the policy is unit-tested without RBAC or a store.
func upstreamReleaseDecision(claimed string, managed, federated, authorized bool) (string, bool) {
	if channelFromString(claimed) != repopb.ArtifactChannel_STABLE {
		return claimed, false
	}
	if !managed {
		return claimed, false
	}
	if federated && authorized {
		return claimed, false
	}
	return "dev", true
}
