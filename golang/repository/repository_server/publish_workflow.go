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
// register descriptor → promote to PUBLISHED, all recorded as a workflow run.
func (srv *server) completePublish(ctx context.Context, manifest *repopb.ArtifactManifest, key string, prov *repopb.ProvenanceRecord) error {
	ref := manifest.GetRef()
	publisherID := ref.GetPublisherId()
	name := ref.GetName()
	version := ref.GetVersion()

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
		srv.workflowRec.FailStep(ctx, runID, regStep,
			"repository.register_descriptor_failed",
			regErr.Error(),
			"Check Resource/Discovery service availability",
			workflowpb.FailureClass_FAILURE_CLASS_DEPENDENCY, true)
		srv.workflowRec.FinishRun(ctx, runID, workflow.Failed,
			fmt.Sprintf("descriptor registration failed: %v", regErr),
			regErr.Error(),
			workflowpb.FailureClass_FAILURE_CLASS_DEPENDENCY)

		slog.Warn("publish workflow: descriptor registration failed (artifact stays VERIFIED)",
			"key", key, "err", regErr)
		return regErr
	}
	srv.workflowRec.CompleteStep(ctx, runID, regStep, "descriptor registered", durationMs)

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
		return promErr
	}
	srv.workflowRec.CompleteStep(ctx, runID, promStep, "promoted to PUBLISHED", durationMs)

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

// promoteToPublished re-writes the manifest with PUBLISHED state.
func (srv *server) promoteToPublished(ctx context.Context, key string, manifest *repopb.ArtifactManifest) error {
	manifest.PublishedUnix = time.Now().Unix()
	mjson, err := marshalManifestWithState(manifest, repopb.PublishState_PUBLISHED)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := srv.Storage().WriteFile(ctx, manifestStorageKey(key), mjson, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
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
