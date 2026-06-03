// @awareness namespace=globular.platform
// @awareness component=platform_repository.repair_authorization
// @awareness file_role=explicit_unseal_path_for_proven_mis_sealed_official_artifacts
// @awareness risk=high
package main

// repair_authorization.go — explicit repair/unseal path for sealed official
// artifacts. The default enforceOfficialNamespaceSeal in local_publish_guard.go
// is absolute: once an official-stable (publisher, name, version, platform)
// tuple is published with a given digest, no other byte stream may claim that
// identity. That contract is the right default — silent overwrite of official
// artifacts would erode the integrity model that the rest of the cluster
// trusts.
//
// But there is one legitimate case where the seal must be repairable: when
// the sealed bytes are themselves a phantom — i.e. the published artifact
// carries the wrong identity (e.g. v1.2.131 bytes sealed under v1.2.143).
// Without a repair path the cluster is stuck: the official desired-state
// resolver picks up the phantom checksum, the convergence check declares
// "converged" against the wrong binary, and the real CI artifact can never
// be installed.
//
// This file adds the explicit repair path with FOUR independent gates:
//
//  1. Caller must present `x-repair-unseal-official: true` in gRPC metadata.
//     There is no implicit unseal — every repair is intentional and visible.
//
//  2. Caller must present the EXACT digest of the artifact they intend to
//     replace in `x-repair-prior-digest`. The server compares it to the
//     currently-sealed digest; mismatch is an immediate rejection. This
//     ensures the caller actually investigated the phantom — they cannot
//     have stumbled into the unseal path by accident.
//
//  3. Caller must present a non-empty `x-repair-reason` text. The reason is
//     written verbatim into the audit event so post-hoc review can establish
//     why the repair was authorized.
//
//  4. Caller's auth subject must be the official publisher's authoritative
//     account (`sa` for `core@globular.io` on the current trust model) — or
//     hold the `repository.write` capability granted to the cluster root.
//     RepoWrite is already required for any UploadArtifact, so the additional
//     check here is the subject==publisher predicate.
//
// All four must pass, and the success path emits a structured audit event
// `pkg.repair_unseal` carrying old digest, new digest, build_ids, reason,
// publisher, version, platform, principal, source IP, and timestamp.

import (
	"context"
	"log/slog"
	"strings"

	"google.golang.org/grpc/metadata"
)

// gRPC metadata keys for the repair path. All keys are lowercase per gRPC
// metadata canonicalization; clients must send them as lowercase.
const (
	mdRepairUnseal      = "x-repair-unseal-official"
	mdRepairReason      = "x-repair-reason"
	mdRepairPriorDigest = "x-repair-prior-digest"
)

// RepairAuthorization captures the caller's intent to repair a sealed
// official artifact. A nil value means no repair was requested, which is
// the default and correct path for normal publishes.
type RepairAuthorization struct {
	// Requested is true only when the caller explicitly opts in via metadata.
	// Any seal-bypass attempt with Requested=false MUST be rejected.
	Requested bool

	// Reason is the operator-supplied explanation, written verbatim into
	// the audit event. Required and must be non-empty when Requested=true.
	Reason string

	// PriorDigest is the digest of the currently-sealed artifact the caller
	// believes they are replacing. The server compares this to the actually-
	// sealed digest; mismatch is an immediate rejection (the caller has the
	// wrong picture of repository state). Required when Requested=true.
	PriorDigest string

	// Used is set to true by the FIRST gate that consults repair to allow a
	// rejection bypass. Phase 32: post-success audit emission keys off Used,
	// so repair metadata that was sent but never actually consumed (e.g.
	// because no immutability gate fired) does not generate a misleading
	// pkg.repair_unseal event.
	Used bool

	// PriorBuildID is the build_id of the existing PUBLISHED row whose
	// identity is being repaired. Populated by the gate that authorizes
	// the bypass (read from the release ledger at decision time) so the
	// post-success audit can record full prior-vs-new identity.
	PriorBuildID string
}

// getRepairAuthorization parses repair-intent metadata from the incoming
// gRPC context. Returns nil if no repair was requested. Returns a populated
// struct (possibly with empty Reason / PriorDigest, which the seal check
// will reject) otherwise. This function never returns an error — validation
// happens at the seal check so all rejection codes flow through one site.
func getRepairAuthorization(ctx context.Context) *RepairAuthorization {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil
	}
	unseal := firstMetadataValue(md, mdRepairUnseal)
	if !strings.EqualFold(strings.TrimSpace(unseal), "true") {
		return nil
	}
	return &RepairAuthorization{
		Requested:   true,
		Reason:      strings.TrimSpace(firstMetadataValue(md, mdRepairReason)),
		PriorDigest: strings.TrimSpace(firstMetadataValue(md, mdRepairPriorDigest)),
	}
}

// firstMetadataValue returns the first value for a metadata key, or "".
func firstMetadataValue(md metadata.MD, key string) string {
	if vs := md[key]; len(vs) > 0 {
		return vs[0]
	}
	return ""
}

// logRepairAuthorized writes a slog + structured audit event recording an
// approved repair of a sealed official artifact. The audit event survives
// independently of the artifact record, so even if the new artifact is
// later overwritten, the repair history remains queryable.
//
// Fields are intentionally verbose: post-hoc review of who repaired what,
// when, and why is the whole point of this audit lane.
func (srv *server) logRepairAuthorized(
	ctx context.Context,
	publisher, name, version, platform string,
	priorDigest, newDigest string,
	priorBuildID, newBuildID string,
	repair *RepairAuthorization,
) {
	slog.Warn("official artifact UNSEALED via repair authorization",
		"publisher", publisher,
		"name", name,
		"version", version,
		"platform", platform,
		"prior_digest", priorDigest,
		"new_digest", newDigest,
		"prior_build_id", priorBuildID,
		"new_build_id", newBuildID,
		"reason", repair.Reason,
	)
	srv.publishAuditEvent(ctx, "repair_unseal", map[string]any{
		"publisher":      publisher,
		"name":           name,
		"version":        version,
		"platform":       platform,
		"prior_digest":   priorDigest,
		"new_digest":     newDigest,
		"prior_build_id": priorBuildID,
		"new_build_id":   newBuildID,
		"reason":         repair.Reason,
	})
}
