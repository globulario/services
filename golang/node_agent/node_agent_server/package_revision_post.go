package main

// package_revision_post.go — Phase F node-agent post-install hook.
//
// After ApplyPackageRelease successfully installs+restarts+verifies a package,
// this file:
//
//   1) calls Repository.RecordInstalledRevision so the repository's
//      installed-history table gets a new row (action=install/upgrade/rollback).
//      The rollback workflow + ListRollbackCandidates RPC consume this table.
//
//   2) reads the artifact manifest (Configs[]) and emits one
//      Repository.RecordConfigReceipt per declared config file. Each receipt
//      classifies what the node-agent did to that file:
//        - PRESERVED: file existed pre-install with the same checksum
//        - REPLACED:  file existed pre-install with a different checksum
//        - GENERATED: ConfigKind=GENERATED — node-agent regenerated it
//        - SKIPPED_SECRET: ConfigKind=SECRET — node-agent never reads/writes
//        - FAILED:    pre-install stat failed (best-effort fallback)
//
// Both calls are fire-and-forget. The apply response is unchanged on failure.
//
// Why this file exists separately from apply_package_release.go:
//   - keeps the apply handler focused on install + restart + verify
//   - avoids growing apply_package_release.go past its current ~415 lines
//   - future-proofs the contract: when the node-agent gets full per-file
//     receipt emission (snapshots / merges), the helpers live here

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
	repositorypb "github.com/globulario/services/golang/repository/repositorypb"
	"google.golang.org/grpc/metadata"
)

// recordRevisionAndReceipts is the post-success hook.
// All errors are logged + swallowed — the apply response must not change.
//
//
// snapshot is the per-config state captured BEFORE InstallPackage mutated
// the filesystem (see applyConfigPolicyPreInstall). When non-nil it lets us
// emit accurate PRESERVED/REPLACED receipts; when nil (legacy artifact, or
// repository unreachable at apply start), classifyConfigOutcome's stat-only
// fallback is used.
func (srv *NodeAgentServer) recordRevisionAndReceipts(
	ctx context.Context,
	repoAddr string,
	req *node_agentpb.ApplyPackageReleaseRequest,
	installed *node_agentpb.InstalledPackage,
	previous *node_agentpb.InstalledPackage,
	snapshot map[string]*configSnapshot,
) {
	if repoAddr == "" || installed == nil {
		return
	}

	// Decide the action label.
	action := "install"
	switch {
	case req.GetRollbackMode():
		action = "rollback"
	case previous != nil && previous.GetVersion() != "":
		action = "upgrade"
	}

	publisherID := strings.TrimSpace(req.GetPublisher())
	if publisherID == "" {
		publisherID = defaultPublisherID
	}

	// Dial repository via the exported helper that other node-agent code
	// uses (same TLS material, same outgoing-context setup).
	conn, _, err := actions.DialRepository(ctx, repoAddr)
	if err != nil {
		log.Printf("post-apply: dial repository failed: %v", err)
		return
	}
	defer conn.Close()
	authCtx := withAgentAuth(ctx)
	repo := repositorypb.NewPackageRepositoryClient(conn)

	// 1) Installed revision row — best-effort.
	srv.recordInstalledRevision(authCtx, repo, action, publisherID, installed, previous, req)

	// 2) Config receipts — read manifest configs[], emit one receipt each.
	srv.emitConfigReceipts(authCtx, repo, publisherID, installed, req, snapshot)
}

func (srv *NodeAgentServer) recordInstalledRevision(
	ctx context.Context,
	repo repositorypb.PackageRepositoryClient,
	action, publisherID string,
	installed, previous *node_agentpb.InstalledPackage,
	req *node_agentpb.ApplyPackageReleaseRequest,
) {
	previousRevID := ""
	if previous != nil {
		previousRevID = previous.GetOperationId() // closest stable id we have on disk
	}
	rev := &repositorypb.InstalledPackageRevision{
		PublisherId:              publisherID,
		Name:                     installed.GetName(),
		Kind:                     mapKindStringToProto(installed.GetKind()),
		Version:                  installed.GetVersion(),
		BuildId:                  installed.GetBuildId(),
		BuildNumber:              installed.GetBuildNumber(),
		Platform:                 installed.GetPlatform(),
		Checksum:                 installed.GetChecksum(),
		InstalledAtUnix:          time.Now().Unix(),
		InstalledByWorkflowRunId: req.GetWorkflowRunId(),
		NodeId:                   srv.nodeID,
		PreviousRevisionId:       previousRevID,
		ServiceStatusBefore:      previousStatus(previous),
		ServiceStatusAfter:       "running",
		Action:                   action,
	}
	if _, err := repo.RecordInstalledRevision(ctx, &repositorypb.RecordInstalledRevisionRequest{
		Revision: rev,
	}); err != nil {
		log.Printf("post-apply: RecordInstalledRevision %s failed: %v", action, err)
		return
	}
	log.Printf("post-apply: recorded installed revision action=%s package=%s/%s@%s build=%d",
		action, publisherID, installed.GetName(), installed.GetVersion(), installed.GetBuildNumber())
}

// emitConfigReceipts reads the manifest's declared configs[] and emits one
// receipt per entry. Pre-install file state is captured via the helper
// `previousFileSHA256` so we can classify PRESERVED vs REPLACED. The hook
// runs AFTER install, so we read the post-install file state to compute
// checksum_after — checksum_before is what we sampled at the start of this
// helper (best-effort; legacy installs without manifest configs simply emit
// no receipts).
func (srv *NodeAgentServer) emitConfigReceipts(
	ctx context.Context,
	repo repositorypb.PackageRepositoryClient,
	publisherID string,
	installed *node_agentpb.InstalledPackage,
	req *node_agentpb.ApplyPackageReleaseRequest,
	snapshot map[string]*configSnapshot,
) {
	manifest, err := fetchManifestForReceipts(ctx, repo, publisherID, installed)
	if err != nil || manifest == nil {
		// Legacy artifact without configs — nothing to emit. This is the
		// expected state for most pre-Phase-D packages.
		return
	}
	configs := manifest.GetConfigs()
	if len(configs) == 0 {
		return
	}
	now := time.Now().Unix()
	for _, c := range configs {
		// Resolve defaults (kind → merge_strategy, sensitive flag).
		resolved := resolveConfigEntry(c)
		// Prefer snapshot-aware classification when the pre-install gate ran.
		var snap *configSnapshot
		if snapshot != nil {
			snap = snapshot[resolved.GetPath()]
		}
		action, before, after := classifyOutcomeWithSnapshot(resolved, snap)
		receipt := &repositorypb.PackageConfigReceipt{
			NodeId:         srv.nodeID,
			PublisherId:    publisherID,
			Name:           installed.GetName(),
			Platform:       installed.GetPlatform(),
			BuildNumber:    installed.GetBuildNumber(),
			Path:           resolved.GetPath(),
			ConfigKind:     resolved.GetConfigKind(),
			MergeStrategy:  resolved.GetMergeStrategy(),
			ChecksumBefore: before,
			ChecksumAfter:  after,
			Action:         action,
			WorkflowRunId:  req.GetWorkflowRunId(),
			TimestampUnix:  now,
			Sensitive:      resolved.GetSensitive(),
		}
		if _, err := repo.RecordConfigReceipt(ctx, &repositorypb.RecordConfigReceiptRequest{
			Receipt: receipt,
		}); err != nil {
			log.Printf("post-apply: RecordConfigReceipt %s failed: %v", resolved.GetPath(), err)
			continue
		}
	}
	log.Printf("post-apply: emitted %d config receipt(s) for %s/%s@%s",
		len(configs), publisherID, installed.GetName(), installed.GetVersion())
}

// fetchManifestForReceipts reads the manifest the node-agent just installed.
// Builds the same ArtifactRef the install path used.
func fetchManifestForReceipts(
	ctx context.Context,
	repo repositorypb.PackageRepositoryClient,
	publisherID string,
	installed *node_agentpb.InstalledPackage,
) (*repositorypb.ArtifactManifest, error) {
	ref := &repositorypb.ArtifactRef{
		PublisherId: publisherID,
		Name:        installed.GetName(),
		Version:     installed.GetVersion(),
		Platform:    installed.GetPlatform(),
		Kind:        mapKindStringToProto(installed.GetKind()),
	}
	resp, err := repo.GetArtifactManifest(ctx, &repositorypb.GetArtifactManifestRequest{
		Ref:         ref,
		BuildNumber: installed.GetBuildNumber(),
	})
	if err != nil {
		return nil, err
	}
	return resp.GetManifest(), nil
}

// resolveConfigEntry mirrors repository_server.ResolveConfigEntry — applies
// per-kind defaults so the receipt always carries a concrete merge_strategy.
func resolveConfigEntry(c *repositorypb.PackageConfigFile) *repositorypb.PackageConfigFile {
	if c == nil {
		return nil
	}
	out := &repositorypb.PackageConfigFile{
		Path:                c.GetPath(),
		ConfigKind:          c.GetConfigKind(),
		OwnerPackage:        c.GetOwnerPackage(),
		MergeStrategy:       c.GetMergeStrategy(),
		PreserveOnUpgrade:   c.GetPreserveOnUpgrade(),
		RestoreOnRollback:   c.GetRestoreOnRollback(),
		Sensitive:           c.GetSensitive(),
	}
	if out.MergeStrategy == repositorypb.MergeStrategy_MERGE_STRATEGY_UNSPECIFIED {
		switch out.ConfigKind {
		case repositorypb.ConfigKind_CONFIG_DEFAULT:
			out.MergeStrategy = repositorypb.MergeStrategy_MERGE_REPLACE
		case repositorypb.ConfigKind_CONFIG_OPERATOR_OVERRIDE:
			out.MergeStrategy = repositorypb.MergeStrategy_MERGE_PRESERVE
		case repositorypb.ConfigKind_CONFIG_GENERATED:
			out.MergeStrategy = repositorypb.MergeStrategy_MERGE_TEMPLATE_RENDER
		case repositorypb.ConfigKind_CONFIG_SECRET:
			out.MergeStrategy = repositorypb.MergeStrategy_MERGE_SECRET_EXTERNAL
		case repositorypb.ConfigKind_CONFIG_RUNTIME_STATE:
			out.MergeStrategy = repositorypb.MergeStrategy_MERGE_APPEND_ONLY
		default:
			out.MergeStrategy = repositorypb.MergeStrategy_MERGE_REPLACE
		}
	}
	if out.ConfigKind == repositorypb.ConfigKind_CONFIG_SECRET {
		out.Sensitive = true
	}
	if out.ConfigKind == repositorypb.ConfigKind_CONFIG_OPERATOR_OVERRIDE && !out.PreserveOnUpgrade {
		out.PreserveOnUpgrade = true
	}
	return out
}

// classifyConfigOutcome inspects the file currently on disk and infers what
// the install just did. SECRET is reported as SKIPPED_SECRET — the node-agent
// must never read sensitive paths even to compute a checksum, so we report
// empty checksums.
func classifyConfigOutcome(c *repositorypb.PackageConfigFile) (
	action repositorypb.ConfigReceiptAction,
	before, after string,
) {
	if c == nil {
		return repositorypb.ConfigReceiptAction_CONFIG_RECEIPT_FAILED, "", ""
	}
	if c.GetConfigKind() == repositorypb.ConfigKind_CONFIG_SECRET {
		return repositorypb.ConfigReceiptAction_CONFIG_RECEIPT_SKIPPED_SECRET, "", ""
	}
	if c.GetConfigKind() == repositorypb.ConfigKind_CONFIG_GENERATED {
		// node-agent re-renders generated configs at install time.
		return repositorypb.ConfigReceiptAction_CONFIG_RECEIPT_GENERATED,
			"", fileSHA256(c.GetPath())
	}

	// DEFAULT / OPERATOR_OVERRIDE / RUNTIME_STATE: compare current disk
	// content to the manifest's checksum_at_install (when set). When the
	// manifest didn't carry one, fall back to "REPLACED" so the receipt is
	// truthful — the node-agent did write something.
	currentSum := fileSHA256(c.GetPath())
	manifestSum := strings.TrimSpace(c.GetChecksumAtInstall())
	if currentSum == "" {
		return repositorypb.ConfigReceiptAction_CONFIG_RECEIPT_FAILED, manifestSum, ""
	}
	if manifestSum != "" && strings.EqualFold(canonHex(currentSum), canonHex(manifestSum)) {
		return repositorypb.ConfigReceiptAction_CONFIG_RECEIPT_PRESERVED, manifestSum, currentSum
	}
	return repositorypb.ConfigReceiptAction_CONFIG_RECEIPT_REPLACED, manifestSum, currentSum
}

// fileSHA256 returns the lowercase hex sha256 of a file. Empty string on any
// error (file missing, permission denied, etc) — the receipt classification
// treats empty as FAILED.
func fileSHA256(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// canonHex strips an optional "sha256:" prefix and lowercases.
func canonHex(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	return strings.TrimPrefix(s, "sha256:")
}

func mapKindStringToProto(kind string) repositorypb.ArtifactKind {
	switch strings.ToUpper(strings.TrimSpace(kind)) {
	case "SERVICE":
		return repositorypb.ArtifactKind_SERVICE
	case "APPLICATION":
		return repositorypb.ArtifactKind_APPLICATION
	case "INFRASTRUCTURE":
		return repositorypb.ArtifactKind_INFRASTRUCTURE
	case "COMMAND":
		return repositorypb.ArtifactKind_COMMAND
	case "AGENT":
		return repositorypb.ArtifactKind_AGENT
	}
	return repositorypb.ArtifactKind_SERVICE
}

func previousStatus(p *node_agentpb.InstalledPackage) string {
	if p == nil {
		return ""
	}
	return p.GetStatus()
}

func withAgentAuth(ctx context.Context) context.Context {
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		return metadata.NewOutgoingContext(ctx, md)
	}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		return metadata.NewOutgoingContext(ctx, md)
	}
	// In tests / unauthenticated paths, return ctx unchanged.
	return ctx
}

// fmtRevID is a tiny helper to keep log lines tight.
func fmtRevID(prev *node_agentpb.InstalledPackage) string {
	if prev == nil {
		return "(none)"
	}
	return fmt.Sprintf("%s@%s build=%d",
		prev.GetName(), prev.GetVersion(), prev.GetBuildNumber())
}

var _ = fmtRevID // silence unused-when-debugging
