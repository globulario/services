package main

// publish_workflow.go — Unified publish pipeline.
//
// After UploadArtifact stores the binary and creates the manifest in VERIFIED
// state, completePublish orchestrates the remaining steps:
//
//   1. Register descriptor in Discovery/Resource service
//   2. Promote manifest to PUBLISHED
//   3. Record the full pipeline as a WorkflowRun for observability
//
// If any step fails, the workflow run is marked FAILED with the exact step,
// error code, and failure class — visible in admin UI and diagnosable by AI.
// The artifact stays in VERIFIED state (not PUBLISHED) so it doesn't appear
// in catalog queries until the issue is fixed and publish is retried.

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/workflow"
	workflowpb "github.com/globulario/services/golang/workflow/workflowpb"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// completePublish runs the post-upload publish pipeline:
// validate laws → register descriptor → promote to PUBLISHED, all recorded as a workflow run.
func (srv *server) completePublish(ctx context.Context, manifest *repopb.ArtifactManifest, key string, prov *repopb.ProvenanceRecord) error {
	ref := manifest.GetRef()
	publisherID := ref.GetPublisherId()
	name := ref.GetName()
	version := ref.GetVersion()

	// ── Step 0: Artifact law validation ─────────────────────────────────
	// Collect PUBLISHED catalog for cross-artifact rules (cycle detection, kind lookup).
	// This is best-effort: if the catalog can't be read, validation is skipped rather
	// than blocking the publish — the laws are enforced on a best-effort basis here.
	catalog := srv.loadPublishedCatalog(ctx)
	if violations := NewArtifactLawValidator(manifest, catalog).Validate(); len(violations) > 0 {
		var msgs []string
		for _, v := range violations {
			msgs = append(msgs, v.Error())
			slog.Warn("artifact law violation — blocking promotion to PUBLISHED",
				"rule", v.Rule, "artifact", v.Artifact, "detail", v.Detail)
		}
		return fmt.Errorf("artifact law violations (%d): %s", len(violations), msgs[0])
	}

	// ── Start workflow run ───────────────────────────────────────────────
	runID := srv.workflowRec.StartRun(ctx, &workflow.RunParams{
		ComponentName:    name,
		ComponentKind:    workflow.KindService, // default; overridden below
		ComponentVersion: version,
		ReleaseKind:      "ArtifactPublish",
		ReleaseObjectID:  fmt.Sprintf("%s/%s", publisherID, name),
		TriggerReason:    workflowpb.TriggerReason_TRIGGER_REASON_MANUAL,
		CorrelationID:    fmt.Sprintf("Publish/%s/%s/%s", publisherID, name, version),
		WorkflowName:     "repository.publish",
	})

	// Record artifact reference.
	srv.workflowRec.AddArtifact(ctx, runID, -1,
		workflowpb.ArtifactKind_ARTIFACT_KIND_PACKAGE,
		name, version, key)

	// ── Step 1: Register descriptor in Resource service ──────────────────
	regStep := srv.workflowRec.RecordStep(ctx, runID, &workflow.StepParams{
		StepKey: "register_descriptor",
		Title:   fmt.Sprintf("Register %s/%s in catalog", publisherID, name),
		Actor:   workflow.ActorRepository,
		Phase:   workflow.PhasePublish,
		Status:  workflow.StepRunning,
	})

	start := time.Now()
	regErr := srv.registerDescriptor(ctx, manifest)
	durationMs := int64(time.Since(start).Milliseconds())

	if regErr != nil {
		// Descriptor registration is best-effort — it populates the catalog/search
		// index but is NOT required for the release pipeline to find the artifact.
		// Promote to PUBLISHED regardless so the release resolver can proceed.
		srv.workflowRec.FailStep(ctx, runID, regStep,
			"repository.register_descriptor_failed",
			regErr.Error(),
			"Check Resource/Discovery service availability",
			workflowpb.FailureClass_FAILURE_CLASS_DEPENDENCY, true)
		slog.Warn("publish workflow: descriptor registration failed (continuing to promote)",
			"key", key, "err", regErr)
	} else {
		srv.workflowRec.CompleteStep(ctx, runID, regStep, "descriptor registered", durationMs)
	}

	// ── Step 2: Promote to PUBLISHED ─────────────────────────────────────
	promStep := srv.workflowRec.RecordStep(ctx, runID, &workflow.StepParams{
		StepKey: "promote_published",
		Title:   fmt.Sprintf("Promote %s to PUBLISHED", name),
		Actor:   workflow.ActorRepository,
		Phase:   workflow.PhasePublish,
		Status:  workflow.StepRunning,
	})

	start = time.Now()
	promErr := srv.promoteToPublished(ctx, key, manifest)
	durationMs = int64(time.Since(start).Milliseconds())

	if promErr != nil {
		srv.workflowRec.FailStep(ctx, runID, promStep,
			"repository.promote_failed",
			promErr.Error(),
			"Check storage write permissions",
			workflowpb.FailureClass_FAILURE_CLASS_REPOSITORY, true)
		srv.workflowRec.FinishRun(ctx, runID, workflow.Failed,
			fmt.Sprintf("promote to PUBLISHED failed: %v", promErr),
			promErr.Error(),
			workflowpb.FailureClass_FAILURE_CLASS_REPOSITORY)
		slog.Error("publish promotion failed: artifact stuck in VERIFIED state",
			"key", key,
			"publisher", publisherID,
			"name", name,
			"version", version,
			"step", "promote_published",
			"err", promErr,
		)
		return promErr
	}
	srv.workflowRec.CompleteStep(ctx, runID, promStep, "promoted to PUBLISHED", durationMs)

	// ── Step 3: Append to release ledger ────────────────────────────────
	if err := srv.appendToLedger(ctx,
		publisherID, name, version,
		manifest.GetBuildId(), manifest.GetChecksum(),
		ref.GetPlatform(), manifest.GetSizeBytes()); err != nil {
		slog.Warn("publish workflow: ledger append failed (non-fatal)",
			"key", key, "err", err)
	}

	// ── Finish: success ──────────────────────────────────────────────────
	srv.workflowRec.FinishRun(ctx, runID, workflow.Succeeded,
		fmt.Sprintf("%s/%s@%s published", publisherID, name, version),
		"", workflow.NoFailure)

	manifest.PublishedUnix = time.Now().Unix()

	slog.Info("publish workflow complete",
		"key", key,
		"publisher", publisherID,
		"name", name,
		"version", version,
		"workflow_run", runID,
	)
	return nil
}

// registerDescriptor creates or updates the PackageDescriptor in the Resource
// service via the existing setPackageBundle helper. This replaces the separate
// Discovery call that the CLI used to make.
func (srv *server) registerDescriptor(ctx context.Context, manifest *repopb.ArtifactManifest) error {
	ref := manifest.GetRef()

	descriptor := &resourcepb.PackageDescriptor{
		Id:          ref.GetName(),
		Name:        ref.GetName(),
		PublisherID: ref.GetPublisherId(),
		Version:     ref.GetVersion(),
		Description: manifest.GetDescription(),
		Keywords:    manifest.GetKeywords(),
		Icon:        manifest.GetIcon(),
	}

	// Map artifact kind to resource type.
	switch ref.GetKind() {
	case repopb.ArtifactKind_SERVICE:
		descriptor.Type = resourcepb.PackageType_SERVICE_TYPE
	case repopb.ArtifactKind_APPLICATION:
		descriptor.Type = resourcepb.PackageType_APPLICATION_TYPE
	default:
		descriptor.Type = resourcepb.PackageType_SERVICE_TYPE
	}

	return srv.setPackageBundle(
		manifest.GetChecksum(),
		ref.GetPlatform(),
		int32(manifest.GetSizeBytes()),
		manifest.GetModifiedUnix(),
		descriptor,
	)
}

// promoteToPublished marks an artifact PUBLISHED.
//
// Write order (Scylla-first):
//  1. Verify binary present in authority MinIO (Stat check).
//  2. Write PUBLISHED state to Scylla ledger — REQUIRED. If this fails, the
//     artifact stays VERIFIED and promotion fails. Scylla is the sole authority
//     for publish state; all discovery paths read from Scylla first.
//  3. Write PUBLISHED manifest_json to MinIO — best-effort for compatibility
//     with degraded/single-node reads. Failure is logged but non-fatal since
//     Scylla already has the authoritative PUBLISHED state.
//  4. Invalidate the in-memory cache.
//
// If step 2 fails the manifest stays in VERIFIED state — invisible to the
// release resolver until the Scylla write succeeds.
func (srv *server) promoteToPublished(ctx context.Context, key string, manifest *repopb.ArtifactManifest) error {
	// Step 1: Verify the binary blob is present in authority MinIO.
	binKey := binaryStorageKey(key)
	fi, statErr := srv.Storage().Stat(ctx, binKey)
	if statErr != nil {
		return fmt.Errorf("promote to PUBLISHED blocked: binary %q not found in authority MinIO — artifact cannot be PUBLISHED without its blob: %w",
			binKey, statErr)
	}
	if declared := manifest.GetSizeBytes(); declared > 0 && fi.Size() != declared {
		return fmt.Errorf("promote to PUBLISHED blocked: binary %q size mismatch — manifest declares %d bytes but authority MinIO reports %d bytes",
			binKey, declared, fi.Size())
	}

	manifest.PublishedUnix = time.Now().Unix()

	// Step 2: Write PUBLISHED state to Scylla — this MUST succeed.
	// Scylla publish_state is the authoritative source. All discovery paths
	// (ListArtifacts, GetArtifactVersions, resolveLatestBuildNumber) read from
	// Scylla first. If this write fails, callers must not treat the artifact as
	// PUBLISHED — it stays VERIFIED and the reconciler will retry later.
	if srv.scylla != nil {
		if err := srv.scylla.UpdatePublishState(ctx, key, repopb.PublishState_PUBLISHED.String()); err != nil {
			return fmt.Errorf("promote to PUBLISHED: ledger update failed — artifact stays in VERIFIED state: %w", err)
		}
	}

	// Step 3: Write PUBLISHED manifest_json to MinIO (compatibility layer).
	// Scylla is already authoritative; this write keeps the MinIO copy consistent
	// for degraded-mode reads and tooling that inspects manifest files directly.
	mjson, err := marshalManifestWithState(manifest, repopb.PublishState_PUBLISHED)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	sKey := manifestStorageKey(key)
	if err := srv.Storage().WriteFile(ctx, sKey, mjson, 0o644); err != nil {
		slog.Warn("promoteToPublished: minio manifest write failed (scylla is authoritative, artifact is PUBLISHED)",
			"key", key, "err", err)
	}

	// Step 4: Invalidate the in-memory cache so the next read sees PUBLISHED.
	if srv.cache != nil {
		srv.cache.invalidateManifest(sKey)
	}
	return nil
}

// artifactKindToWorkflow maps repository ArtifactKind to workflow component kind.
func artifactKindToWorkflow(kind repopb.ArtifactKind) workflowpb.ComponentKind {
	switch kind {
	case repopb.ArtifactKind_SERVICE:
		return workflow.KindService
	case repopb.ArtifactKind_INFRASTRUCTURE, repopb.ArtifactKind_SUBSYSTEM:
		return workflow.KindInfra
	default:
		return workflow.KindService
	}
}
