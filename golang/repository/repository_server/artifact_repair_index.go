package main

// artifact_repair_index.go — SCAR-3: owner RepairIndex operation.
//
// Contract: repository.cas_present_index_unknown_is_owner_repairable
//   When an artifact blob exists in the repository-owned local POSIX CAS
//   (artifacts/<key>.bin) with a matching manifest sidecar
//   (artifacts/<key>.manifest.json) whose size + sha256 the blob satisfies, a
//   repository index row that is UNKNOWN / missing / skeleton / non-PUBLISHED is
//   a REPAIRABLE divergence — reconstituted ONLY through the repository owner,
//   from EXISTING repository-owned evidence, never minting identity, never
//   touching desired state, never rolling packages forward.
//
// This file provides:
//   - probeLocalCASArtifact          — identity-safe CAS evidence probe (shared with RepairArtifact)
//   - backfillPublishIndexFromCAS     — repository-owned index reconstitution (shared with RepairArtifact)
//   - repairIndexFromCAS             — the bulk owner operation (dry-run default; commit to apply)
//   - classifyRepairIndexState        — pure classifier (unit-testable for the UNKNOWN sentinel)
//
// ── POST-PROTO-REGEN WIRING (documentation only; not compiled here) ──────────
// After adding RepairIndex to proto/repository.proto and running generateCode.sh,
// wire the RPC + CLI + MCP tool as follows (identity/behaviour already proven by
// repairIndexFromCAS + repair_index_test.go):
//
//   // server RPC handler (e.g. in artifact_verify_rpc.go, next to RepairArtifact):
//   func (srv *server) RepairIndex(ctx context.Context, req *repopb.RepairIndexRequest) (*repopb.RepairIndexResponse, error) {
//       // The safe default is NO-MUTATION: model the request as `bool commit` so the
//       // zero value is dry-run, and set DryRun = !req.GetCommit().
//       rep, err := srv.repairIndexFromCAS(ctx, RepairIndexOptions{
//           DryRun:          !req.GetCommit(),
//           PublisherFilter: req.GetPublisherFilter(),
//           NameFilter:      req.GetNameFilter(),
//       })
//       if err != nil { return nil, status.Error(codes.Internal, err.Error()) }
//       return rep.toProto(), nil // trivial field copy; identity fields echoed from evidence
//   }
//
//   // CLI (golang/globularcli/repo_verify_cmds.go), under repoCmd:
//   //   globular repository repair-index [--commit] [--publisher P] [--package N]
//   //   default = dry-run (report only); --commit applies. Prints per-item action +
//   //   the Scanned/Repairable/Repaired/Refused/SkippedOk/SkippedPolicy summary.
//
//   // MCP tool: repository_repair_index (dry-run default), mirroring repository_repair_artifact.
//
// Note on the repository_artifact_lifecycle_stuck repair plan's "confirm a
// desired build still references the build id" precondition: that guards operator
// resurrection of intentionally-removed builds. RepairIndex does NOT read the
// controller's desired state (the four-layer rule: repository owns repository
// repair); the resurrection concern is handled structurally instead — a
// deleted/GC'd artifact has no CAS blob, so probeLocalCASArtifact fails and the
// item is refused, never repaired.

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// probeLocalCASArtifact reads the CAS manifest sidecar + blob for an artifact key
// and validates them against each other: the .manifest.json must parse, the .bin
// must exist, and the blob size + sha256 must match the manifest's declared
// values. Returns (manifest, rawManifestJSON, true) ONLY when the local CAS
// evidence is self-consistent — the identity-safe basis for index repair.
// Identity always comes from the on-disk manifest; nothing is minted.
//
// Extracted from RepairArtifact's Project-D probe so RepairArtifact and
// RepairIndex share one identity-safe implementation.
func (srv *server) probeLocalCASArtifact(ctx context.Context, key string) (*repopb.ArtifactManifest, []byte, bool) {
	if srv.localStorage == nil {
		return nil, nil, false
	}
	mData, mErr := srv.localStorage.ReadFile(ctx, manifestStorageKey(key))
	if mErr != nil || len(mData) == 0 {
		return nil, nil, false
	}
	manifest, _, parseErr := unmarshalManifestWithState(mData)
	if parseErr != nil || manifest == nil {
		return nil, nil, false
	}
	binKey := binaryStorageKey(key)
	fi, statErr := srv.localStorage.Stat(ctx, binKey)
	if statErr != nil {
		return nil, nil, false
	}
	if declared := manifest.GetSizeBytes(); declared > 0 && fi.Size() != declared {
		return nil, nil, false
	}
	if expected := manifest.GetChecksum(); expected != "" {
		actual, readErr := checksumLocalFile(srv.localStorage.LocalPath(binKey))
		if readErr != nil || !digestEqual(actual, expected) {
			return nil, nil, false
		}
	}
	return manifest, mData, true
}

// backfillPublishIndexFromCAS re-establishes PUBLISHED index authority for an
// artifact whose local CAS blob+manifest are valid but whose index row is a
// skeleton / non-PUBLISHED / missing. It uses ONLY repository-owned write
// primitives and identity from the passed (already CAS-verified) manifest — it
// NEVER mints identity, NEVER fetches from upstream, NEVER touches desired state.
// Returns the state read back after the backfill.
//
// Extracted verbatim from RepairArtifact's Project-D backfill so both paths share
// one implementation. syncManifestToScylla / UpdatePublishState no-op when Scylla
// is nil; transitionArtifactState is best-effort (an illegal edge from a stuck
// intermediate state is tolerated — publish_state column authority is sufficient).
func (srv *server) backfillPublishIndexFromCAS(ctx context.Context, key string, manifest *repopb.ArtifactManifest, mData []byte) string {
	// Step 1: write the full row (manifest_json + PUBLISHED publish_state) via the
	// canonical repository-owned path (never a raw INSERT).
	srv.syncManifestToScylla(ctx, key, manifest, repopb.PublishState_PUBLISHED, mData)

	// Step 2: ensure the publish_state column is PUBLISHED. Idempotent.
	if srv.scylla != nil {
		if updErr := srv.scylla.UpdatePublishState(ctx, key, repopb.PublishState_PUBLISHED.String()); updErr != nil {
			slog.Warn("repair: UpdatePublishState failed during backfill", "artifact_key", key, "err", updErr)
		}
	}

	// Step 3: best-effort artifact_state transition. The state machine may reject
	// (e.g. DOWNLOADING → PUBLISHED is not a legal edge); tolerated for backfill —
	// publish_state column is the authoritative lifecycle source.
	ref := manifest.GetRef()
	stateFields := ArtifactStateFields{
		BlobKey:     binaryStorageKey(key),
		Checksum:    manifest.GetChecksum(),
		SizeBytes:   manifest.GetSizeBytes(),
		BuildID:     manifest.GetBuildId(),
		BuildNumber: manifest.GetBuildNumber(),
		PublisherID: ref.GetPublisherId(),
		Name:        ref.GetName(),
		Version:     ref.GetVersion(),
		Platform:    ref.GetPlatform(),
	}
	if transErr := srv.transitionArtifactState(ctx, key, PipelinePublished, "project_d_backfill", "", stateFields); transErr != nil {
		slog.Info("repair: artifact_state transition not legal from this state — publish_state column authority is sufficient",
			"artifact_key", key, "err", transErr)
	}
	return string(srv.readArtifactState(ctx, key))
}

// ── RepairIndex bulk owner operation ─────────────────────────────────────────

// RepairIndexOptions parameterises the bulk index repair. The zero value is a
// dry-run over all artifacts (DryRun is false-by-default in Go, so the RPC layer
// must map an omitted commit flag to DryRun=true — see the wiring note above).
type RepairIndexOptions struct {
	DryRun          bool   // when true (safe default at the RPC/CLI layer), report only — mutate nothing
	PublisherFilter string // optional: restrict to one publisher id
	NameFilter      string // optional: restrict to one package name
}

// RepairIndexItem is the per-artifact outcome. Identity fields are copied from
// the on-disk manifest evidence — never synthesised.
type RepairIndexItem struct {
	ArtifactKey string
	Name        string
	Version     string
	Platform    string
	BuildId     string
	StateBefore string
	StateAfter  string
	Action      string // would_repair_publish_index | repair_publish_index | refused | skipped_ok | skipped_policy
	Detail      string
}

// RepairIndexReport aggregates a RepairIndex run.
type RepairIndexReport struct {
	Scanned       int
	Repairable    int
	Repaired      int
	Refused       int
	SkippedOk     int
	SkippedPolicy int
	DryRun        bool
	Items         []RepairIndexItem
}

type repairIndexCategory int

const (
	repairCatPublishedOk repairIndexCategory = iota // already PUBLISHED — nothing to do
	repairCatPolicySkip                             // REVOKED / QUARANTINED — never auto-elevate
	repairCatCandidate                              // UNKNOWN / missing / skeleton / non-PUBLISHED — repair if CAS evidence present
)

// classifyRepairIndexState maps a durable index state to a repair category. Pure
// function (no store access) so the UNKNOWN-sentinel classification is unit
// testable without Scylla. UNKNOWN is deliberately a repair candidate (it is a
// divergence, not a terminal state); REVOKED/QUARANTINED are policy states that
// repair must never elevate.
func classifyRepairIndexState(state ArtifactPipelineState) repairIndexCategory {
	switch state {
	case PipelinePublished:
		return repairCatPublishedOk
	case PipelineRevoked, PipelineQuarantined:
		return repairCatPolicySkip
	default:
		return repairCatCandidate
	}
}

// repairIndexFromCAS is the owner bulk operation. It enumerates existing artifact
// build records from the repository-owned local CAS (each artifacts/<key>.manifest.json
// sidecar is one build record — this also surfaces rows the broken index no longer
// resolves), and for each divergent row reconstitutes PUBLISHED index authority
// from the checksum-verified local blob + sidecar. It NEVER mints identity, NEVER
// fetches from upstream, NEVER reads or writes desired state, and NEVER elevates
// REVOKED/QUARANTINED. DryRun reports the plan without mutating anything.
func (srv *server) repairIndexFromCAS(ctx context.Context, opts RepairIndexOptions) (*RepairIndexReport, error) {
	report := &RepairIndexReport{DryRun: opts.DryRun}
	if srv.localStorage == nil {
		return report, nil
	}

	const manifestSuffix = ".manifest.json"
	root := srv.localStorage.LocalPath(artifactsDir)
	entries, globErr := filepath.Glob(filepath.Join(root, "*"+manifestSuffix))
	if globErr != nil {
		return report, nil // malformed pattern is impossible here; treat as empty
	}

	for _, path := range entries {
		base := filepath.Base(path)
		key := strings.TrimSuffix(base, manifestSuffix)
		if key == "" || key == base {
			continue // not a manifest sidecar
		}

		// Identity comes from the sidecar manifest — never invented.
		mData, mErr := srv.localStorage.ReadFile(ctx, manifestStorageKey(key))
		if mErr != nil || len(mData) == 0 {
			continue
		}
		manifest, _, parseErr := unmarshalManifestWithState(mData)
		if parseErr != nil || manifest == nil {
			continue
		}
		ref := manifest.GetRef()
		if opts.PublisherFilter != "" && ref.GetPublisherId() != opts.PublisherFilter {
			continue
		}
		if opts.NameFilter != "" && ref.GetName() != opts.NameFilter {
			continue
		}

		report.Scanned++
		state := srv.readArtifactState(ctx, key)
		item := RepairIndexItem{
			ArtifactKey: key,
			Name:        ref.GetName(),
			Version:     ref.GetVersion(),
			Platform:    ref.GetPlatform(),
			BuildId:     manifest.GetBuildId(),
			StateBefore: string(state),
			StateAfter:  string(state),
		}

		switch classifyRepairIndexState(state) {
		case repairCatPublishedOk:
			item.Action = "skipped_ok"
			item.Detail = "already published"
			report.SkippedOk++

		case repairCatPolicySkip:
			item.Action = "skipped_policy"
			item.Detail = "terminal/policy state (" + string(state) + ") — not auto-elevated"
			report.SkippedPolicy++

		default: // repairCatCandidate
			// Repair ONLY from present, checksum-verified CAS evidence. If the
			// evidence is missing or contradictory, REFUSE — never fetch upstream,
			// never mint identity, never roll desired forward.
			probeManifest, probeData, ok := srv.probeLocalCASArtifact(ctx, key)
			if !ok {
				item.Action = "refused"
				item.Detail = "CAS blob/sidecar evidence missing or contradictory (size/checksum) — not repairable from local evidence"
				report.Refused++
				break
			}
			// Identity re-asserted strictly from the probed manifest.
			item.BuildId = probeManifest.GetBuildId()
			report.Repairable++
			if opts.DryRun {
				item.Action = "would_repair_publish_index"
				item.Detail = "CAS blob+manifest verified; would reconstitute PUBLISHED index row from local evidence"
			} else {
				item.StateAfter = srv.backfillPublishIndexFromCAS(ctx, key, probeManifest, probeData)
				item.Action = "repair_publish_index"
				item.Detail = "PUBLISHED index row reconstituted from local CAS evidence"
				report.Repaired++
			}
		}

		report.Items = append(report.Items, item)
	}
	return report, nil
}
