package main

// package_config_pre.go — Phase F-final pre-install config policy gate.
//
// Runs BEFORE InstallPackage mutates any files. Loads the artifact's manifest,
// scans its declared configs[], and:
//
//   1. Captures pre-install file checksums so post-install receipts have
//      accurate before/after.
//   2. Detects FAIL_ON_LOCAL_MODIFICATION conflicts and aborts the apply
//      with a CONFLICT receipt — the package install never starts.
//
// Returns:
//   - (snapshot, nil)  when the policy permits the apply.
//   - (snapshot, err)  when a hard CONFLICT was found AND a CONFLICT receipt
//                      was already emitted; caller fails the apply with this
//                      error and never touches the binary.

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
	repositorypb "github.com/globulario/services/golang/repository/repositorypb"
)

// configSnapshot is the pre-install record of one declared config file's
// state on disk. The post-install receipt path consumes this to compute
// checksum_after / classify the action accurately.
type configSnapshot struct {
	Resolved          *repositorypb.PackageConfigFile // with defaults filled
	ChecksumBefore    string                          // hex sha256 of file pre-install (empty if absent or SECRET)
	ExistedPreInstall bool
	ManifestChecksum  string // checksum_at_install from the manifest (when shipped)
}

// applyConfigPolicyPreInstall runs the pre-install config gate. The returned
// map is keyed by absolute path so the post-install hook can match each
// receipt back to its pre-install state. The returned `policyObservable`
// bool distinguishes three previously-indistinguishable cases:
//
//   - (snap, true,  nil)  policy was consulted and permits the apply
//   - (nil,  true,  nil)  no repo configured / pre-Phase-D artifact / no
//                         configs declared — policy IS observable and
//                         legitimately empty
//   - (nil,  false, nil)  repository unreachable OR manifest fetch failed —
//                         policy is NOT observable; the caller must decide
//                         whether to fail-open (current behaviour) or
//                         fail-closed (recommended when the user package
//                         declared FAIL_ON_LOCAL_MODIFICATION configs in
//                         the previous install).
//   - (snap, _,     err)  FAIL_ON_LOCAL_MODIFICATION hit; CONFLICT receipt
//                         already recorded; apply MUST abort.
//
// Calling code should treat any non-nil error as a hard abort: a CONFLICT
// receipt has already been recorded and the apply must NOT proceed to
// InstallPackage.
//
// Why policyObservable matters: previously the three "skip the gate"
// outcomes returned the identical (nil, nil) shape — "no repo", "manifest
// missing because artifact is pre-Phase-D", AND "repo dial failed" all
// collapsed into the same fail-open response. A transient repository
// outage silently bypassed the FAIL_ON_LOCAL_MODIFICATION safety net for
// every concurrent install. The new bool lets the caller distinguish "I
// know the policy is empty" from "I couldn't observe the policy" and log
// the gap (forbidden.error_absorbed_into_empty_map).
func (srv *NodeAgentServer) applyConfigPolicyPreInstall(
	ctx context.Context,
	repoAddr string,
	publisherID string,
	pkg *node_agentpb.InstalledPackage,
	workflowRunID string,
) (map[string]*configSnapshot, bool, error) {
	if repoAddr == "" {
		return nil, true, nil // legacy / no-repo path: no config policy to enforce
	}

	// Read the artifact's manifest. If we can't reach the repository, the
	// apply continues without per-config receipts (preserving today's
	// fail-open behaviour), but policyObservable=false so the caller can
	// log the observability gap. Promoting this to fail-closed is the
	// next step once the observability data confirms how often this fires.
	conn, _, err := actions.DialRepository(ctx, repoAddr)
	if err != nil {
		log.Printf("config-policy: dial repository failed (policy unobservable; apply proceeds): %v", err)
		return nil, false, nil
	}
	defer conn.Close()
	repo := repositorypb.NewPackageRepositoryClient(conn)
	manifest, err := fetchManifestForReceipts(withAgentAuth(ctx), repo, publisherID, pkg)
	if err != nil {
		log.Printf("config-policy: fetch manifest failed for %s/%s@%s (policy unobservable; apply proceeds): %v", publisherID, pkg.GetName(), pkg.GetVersion(), err)
		return nil, false, nil
	}
	if manifest == nil {
		// Pre-Phase-D artifacts have no configs. Policy IS observable
		// (we consulted the repo and got a definitive "no manifest").
		return nil, true, nil
	}
	configs := manifest.GetConfigs()
	if len(configs) == 0 {
		return nil, true, nil
	}

	// Build the snapshot map and check for hard CONFLICTs.
	snapshots := make(map[string]*configSnapshot, len(configs))
	now := time.Now().Unix()
	for _, c := range configs {
		resolved := resolveConfigEntry(c)
		path := resolved.GetPath()
		if path == "" {
			continue
		}

		snap := &configSnapshot{
			Resolved:         resolved,
			ManifestChecksum: strings.TrimSpace(c.GetChecksumAtInstall()),
		}

		// Don't read SECRET files — only record that the path exists.
		if resolved.GetConfigKind() == repositorypb.ConfigKind_CONFIG_SECRET {
			snapshots[path] = snap
			continue
		}

		// Capture pre-install checksum if the file exists.
		preSum := fileSHA256(path)
		if preSum != "" {
			snap.ExistedPreInstall = true
			snap.ChecksumBefore = preSum
		}

		// FAIL_ON_LOCAL_MODIFICATION gate: if the manifest claims a checksum
		// at install time and the file on disk differs, refuse the apply.
		if resolved.GetMergeStrategy() == repositorypb.MergeStrategy_MERGE_FAIL_ON_LOCAL_MODIFICATION &&
			snap.ExistedPreInstall && snap.ManifestChecksum != "" &&
			!strings.EqualFold(canonHex(preSum), canonHex(snap.ManifestChecksum)) {

			// Emit a CONFLICT receipt and abort. The CLI surfaces it via
			// `globular pkg config conflicts`.
			receipt := &repositorypb.PackageConfigReceipt{
				NodeId:         srv.nodeID,
				PublisherId:    publisherID,
				Name:           pkg.GetName(),
				Platform:       pkg.GetPlatform(),
				BuildNumber:    pkg.GetBuildNumber(),
				Path:           path,
				ConfigKind:     resolved.GetConfigKind(),
				MergeStrategy:  resolved.GetMergeStrategy(),
				ChecksumBefore: preSum,
				ChecksumAfter:  preSum, // unchanged — we did not write
				Action:         repositorypb.ConfigReceiptAction_CONFIG_RECEIPT_CONFLICT,
				WorkflowRunId:  workflowRunID,
				TimestampUnix:  now,
				Sensitive:      resolved.GetSensitive(),
				Reason:         "MERGE_FAIL_ON_LOCAL_MODIFICATION: local file differs from package's checksum_at_install",
			}
			if _, recErr := repo.RecordConfigReceipt(withAgentAuth(ctx),
				&repositorypb.RecordConfigReceiptRequest{Receipt: receipt}); recErr != nil {
				log.Printf("config-policy: emit CONFLICT receipt failed: %v", recErr)
			}
			snapshots[path] = snap
			return snapshots, true, fmt.Errorf("config conflict: %s has local modifications and merge_strategy=FAIL_ON_LOCAL_MODIFICATION (run `globular pkg config conflicts %s/%s` to inspect)",
				path, publisherID, pkg.GetName())
		}

		snapshots[path] = snap
	}
	log.Printf("config-policy: %d config(s) classified pre-install for %s/%s@%s",
		len(snapshots), publisherID, pkg.GetName(), pkg.GetVersion())
	return snapshots, true, nil
}

// classifyOutcomeWithSnapshot is the post-install variant of
// classifyConfigOutcome that uses a captured pre-install snapshot to compute
// accurate before/after checksums. Falls back to the snapshot-less path
// when no snapshot was taken (legacy artifacts or unreachable repo).
func classifyOutcomeWithSnapshot(c *repositorypb.PackageConfigFile, snap *configSnapshot) (
	action repositorypb.ConfigReceiptAction,
	before, after string,
) {
	if snap == nil {
		return classifyConfigOutcome(c)
	}
	if snap.Resolved == nil {
		return classifyConfigOutcome(c)
	}
	if snap.Resolved.GetConfigKind() == repositorypb.ConfigKind_CONFIG_SECRET {
		return repositorypb.ConfigReceiptAction_CONFIG_RECEIPT_SKIPPED_SECRET, "", ""
	}

	currentSum := fileSHA256(snap.Resolved.GetPath())
	before = snap.ChecksumBefore
	after = currentSum

	if snap.Resolved.GetConfigKind() == repositorypb.ConfigKind_CONFIG_GENERATED {
		return repositorypb.ConfigReceiptAction_CONFIG_RECEIPT_GENERATED, before, after
	}
	if currentSum == "" {
		return repositorypb.ConfigReceiptAction_CONFIG_RECEIPT_FAILED, before, ""
	}
	if before != "" && strings.EqualFold(canonHex(before), canonHex(currentSum)) {
		// File on disk is unchanged from pre-install. PRESERVED — local
		// content kept (or DEFAULT was already aligned with the new package).
		return repositorypb.ConfigReceiptAction_CONFIG_RECEIPT_PRESERVED, before, after
	}
	// File changed during install — node-agent wrote new content.
	return repositorypb.ConfigReceiptAction_CONFIG_RECEIPT_REPLACED, before, after
}
