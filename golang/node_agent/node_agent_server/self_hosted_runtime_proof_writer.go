// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.self_hosted_proof
// @awareness file_role=post_restart_runtime_proof_writer_for_control_plane_binaries
// @awareness implements=globular.platform:intent.runtime.identity.requires_proof
// @awareness implements=globular.platform:intent.installed_state.owned_by_node_agent
// @awareness implements=globular.platform:intent.health.requires_fresh_evidence
// @awareness risk=critical
package main

// self_hosted_runtime_proof_writer.go — Project B implementation.
//
// Implements Pattern 1: Post-Restart Runtime Proof Writer for self-hosted
// control-plane components. The writer may refresh installed_state ONLY
// when the full runtime proof chain succeeds:
//
//   desired identity (ServiceRelease)
//     → repository manifest (resolved_entrypoint_checksum)
//       → running process MainPID (systemd)
//         → /proc/<MainPID>/exe symlink
//           → on-disk binary sha256
//             → hash match against manifest checksum
//               → installed_state refresh
//
// Out of scope (forbidden by handoff):
//   - manual etcd put / desired-state mutation
//   - removing the buildId guard from heartbeat.go
//   - promoting status based on version string alone
//   - clearing metadata.error without runtime proof
//   - applying to all services (allowlist is narrow)
//   - using package tarball sha256 as binary identity
//
// Reference: loads/self_install_record_refresh_impact.md.
// Reference: loads/self_install_record_refresh_result.md (post-implementation).

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/installed_state"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/supervisor"
)

// selfHostedServiceNames is the narrow allowlist of self-hosted control-plane
// components whose installed_state may be refreshed by the post-restart
// runtime proof writer.
//
// These components share three properties that justify the special path:
//
//  1. They are part of the control plane — node-agent installs other
//     services; cluster-controller dispatches reconciliation;
//     cluster-doctor evaluates findings on a periodic cycle; repository
//     is the manifest authority other services depend on.
//  2. They restart themselves during their own apply, which interrupts
//     the workflow ConvergenceResultV1 receipt path.
//  3. Their runtime identity is verifiable entirely from local evidence:
//     systemd MainPID + /proc/<pid>/exe + on-disk sha256 + manifest
//     entrypoint_checksum.
//
// Ordinary services MUST NOT be added here. They must continue through the
// workflow path. Adding a name here requires an alignment verdict update.
//
// repository was added after Project D's recovery bridge surfaced the same
// staleness symptom (on-disk bytes matched manifest entrypoint_checksum but
// installed_state stayed at the previous version because the workflow
// install path was dep-gated on event hash_drift). Repository meets all
// three criteria.
var selfHostedServiceNames = map[string]bool{
	"node-agent":         true,
	"cluster-controller": true,
	"cluster-doctor":     true,
	"repository":         true,
}

// Explicit proof failure reasons. These replace generic "failed" so the
// dashboard, doctor, and ai-watcher can react to the specific gap. Adding a
// new reason here must also add it to docs/awareness/failure_modes.yaml.
const (
	proofReasonOK                        = ""
	proofReasonNotInAllowlist            = "not_in_self_hosted_allowlist"
	proofReasonManifestMissing           = "manifest_missing"
	proofReasonEntrypointChecksumMissing = "entrypoint_checksum_missing"
	proofReasonProcessNotRunning         = "process_not_running"
	proofReasonProcExeUnreadable         = "proc_exe_unreadable"
	proofReasonBinaryPathUnexpected      = "binary_path_unexpected"
	proofReasonBinaryHashMismatch        = "binary_hash_mismatch"
	proofReasonDesiredIdentityMissing    = "desired_identity_missing"
	proofReasonRepositoryLookupFailed    = "repository_lookup_failed"
	proofReasonOnDiskHashFailed          = "on_disk_hash_failed"
)

// selfHostedRuntimeProof is the evidence bundle assembled by
// buildSelfHostedRuntimeProof. All fields are populated only when the full
// proof chain succeeds; a partial bundle is never returned.
type selfHostedRuntimeProof struct {
	ServiceName        string
	DesiredVersion     string
	DesiredBuildID     string
	DesiredBuildNumber int64
	ManifestChecksum   string // resolved_entrypoint_checksum (binary sha256)
	BinaryPath         string // expected path from installedBinaryPath()
	ProcExePath        string // resolved /proc/<pid>/exe symlink
	OnDiskSHA256       string // computed sha256 of ProcExePath
	RunningPID         int
	PIDStartUnix       int64 // unix seconds when the running PID started (anchor for InstalledUnix/UpdatedUnix)
	ProofSource        string
	ProofTimeUnixMs    int64
}

// selfHostedProofDeps is the injection surface for tests.
type selfHostedProofDeps struct {
	// FindRunningPID returns the systemd MainPID of the canonical service.
	// Returns 0 if not running. The default reads MainPID from
	// systemctl show globular-<name>.service.
	FindRunningPID func(ctx context.Context, canonicalName string) int
	// ReadProcExe returns the absolute path /proc/<pid>/exe resolves to.
	ReadProcExe func(pid int) (string, error)
	// ReadPIDStartUnix returns the unix-seconds time the running PID started.
	// The default reads the mtime of /proc/<pid>, which the Linux kernel sets
	// at task creation — second-precision and avoids parsing clock-tick fields
	// out of /proc/<pid>/stat.
	ReadPIDStartUnix func(pid int) (int64, error)
	// HashFile returns the sha256 of the file (lowercased, no "sha256:" prefix).
	HashFile func(path string) (string, error)
	// FetchServiceRelease reads the latest ServiceRelease record for this
	// canonical service from etcd. Returns nil if not found.
	FetchServiceRelease func(ctx context.Context, canonicalName string) (*cluster_controllerpb.ServiceRelease, error)
	Now func() time.Time
}

func defaultSelfHostedProofDeps() selfHostedProofDeps {
	return selfHostedProofDeps{
		FindRunningPID:      defaultFindRunningPID,
		ReadProcExe:         func(pid int) (string, error) { return os.Readlink(fmt.Sprintf("/proc/%d/exe", pid)) },
		ReadPIDStartUnix:    defaultReadPIDStartUnix,
		HashFile:            cachedSha256,
		FetchServiceRelease: defaultFetchServiceRelease,
		Now:                 time.Now,
	}
}

// defaultReadPIDStartUnix returns the unix-seconds time the running PID
// started, using the modification time of /proc/<pid>. The kernel sets the
// procfs entry's mtime at task creation. This anchor is what the verifier
// compares against ApplyTime when deciding whether the binary on disk
// matches the running process — using time.Now() here would cause every
// heartbeat refresh to look like an upgrade-without-restart.
func defaultReadPIDStartUnix(pid int) (int64, error) {
	st, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	if err != nil {
		return 0, err
	}
	return st.ModTime().Unix(), nil
}

// defaultFindRunningPID asks systemd for the MainPID of globular-<name>.service.
// Returns 0 if the unit is inactive or MainPID is unparseable.
func defaultFindRunningPID(ctx context.Context, canonicalName string) int {
	unit := "globular-" + canonicalName + ".service"
	props, err := supervisor.ShowProperties(ctx, unit, "MainPID")
	if err != nil || props == nil {
		return 0
	}
	mp := strings.TrimSpace(props["MainPID"])
	if mp == "" || mp == "0" {
		return 0
	}
	pid, err := strconv.Atoi(mp)
	if err != nil || pid <= 0 {
		return 0
	}
	return pid
}

// defaultFetchServiceRelease reads
// /globular/resources/ServiceRelease/<publisher>/<service> from etcd and
// unmarshals the JSON value into the canonical proto-Go struct.
//
// Publisher is hardcoded to "core@globular.io" because all three allowlist
// services share that publisher today. If we later support multiple
// publishers per canonical name, the allowlist becomes a map[name]publisher.
func defaultFetchServiceRelease(ctx context.Context, canonicalName string) (*cluster_controllerpb.ServiceRelease, error) {
	cli, err := config.GetEtcdClient()
	if err != nil || cli == nil {
		return nil, fmt.Errorf("etcd client unavailable: %w", err)
	}
	const publisher = "core@globular.io"
	key := fmt.Sprintf("/globular/resources/ServiceRelease/%s/%s", publisher, canonicalName)
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := cli.Get(tctx, key)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	var rel cluster_controllerpb.ServiceRelease
	if err := json.Unmarshal(resp.Kvs[0].Value, &rel); err != nil {
		return nil, fmt.Errorf("unmarshal ServiceRelease %s: %w", canonicalName, err)
	}
	return &rel, nil
}

// buildSelfHostedRuntimeProof walks the proof chain for one canonical name.
// Returns (proof, "") only when every step succeeds. Returns (nil, reason)
// with an explicit reason from the proofReason* constants on any failure.
//
// The order of checks is fixed: identity first (cheapest), then process,
// then on-disk hash (most expensive). This minimizes work for components
// that aren't ready yet.
func buildSelfHostedRuntimeProof(ctx context.Context, canonicalName string, deps selfHostedProofDeps) (*selfHostedRuntimeProof, string) {
	if !selfHostedServiceNames[canonicalName] {
		return nil, proofReasonNotInAllowlist
	}

	// Step 1: desired identity from repository ServiceRelease.
	rel, err := deps.FetchServiceRelease(ctx, canonicalName)
	if err != nil {
		return nil, proofReasonRepositoryLookupFailed
	}
	if rel == nil || rel.Status == nil {
		return nil, proofReasonManifestMissing
	}
	desiredVersion := strings.TrimSpace(rel.Status.ResolvedVersion)
	desiredBuildID := strings.TrimSpace(rel.Status.ResolvedBuildID)
	manifestChecksum := strings.ToLower(strings.TrimSpace(rel.Status.ResolvedEntrypointChecksum))
	if desiredVersion == "" {
		return nil, proofReasonDesiredIdentityMissing
	}
	if manifestChecksum == "" {
		return nil, proofReasonEntrypointChecksumMissing
	}

	// Step 2: locate the running process via systemd MainPID.
	pid := deps.FindRunningPID(ctx, canonicalName)
	if pid <= 0 {
		return nil, proofReasonProcessNotRunning
	}

	// Step 3: resolve /proc/<pid>/exe.
	procExe, err := deps.ReadProcExe(pid)
	if err != nil || strings.TrimSpace(procExe) == "" {
		return nil, proofReasonProcExeUnreadable
	}

	// Step 4: validate binary path follows the convention. The
	// installedBinaryPath helper encodes the "<name>_server" rule for
	// SERVICE-kind packages. For self-hosted components the convention
	// is authoritative — they ship with predictable binary names.
	expectedPath := installedBinaryPath(canonicalName, "SERVICE")
	if procExe != expectedPath {
		return nil, proofReasonBinaryPathUnexpected
	}

	// Step 5: compute sha256 of the resolved binary.
	onDiskHash, err := deps.HashFile(procExe)
	if err != nil {
		return nil, proofReasonOnDiskHashFailed
	}
	onDiskHash = strings.ToLower(strings.TrimSpace(onDiskHash))

	// Step 6: hash must equal manifest entrypoint_checksum.
	if onDiskHash != manifestChecksum {
		return nil, proofReasonBinaryHashMismatch
	}

	// Read the PID's start time — the anchor for installed_state timestamps.
	// A read failure is NOT fatal: the verifier check that depends on this
	// (service.old_pid_after_upgrade) only requires the anchor be ≤ PID start.
	// A zero anchor preserves the legacy "now" behavior for that field, which
	// is the worst case we'd have without this fix anyway.
	var pidStartUnix int64
	if deps.ReadPIDStartUnix != nil {
		if v, err := deps.ReadPIDStartUnix(pid); err == nil {
			pidStartUnix = v
		}
	}

	now := deps.Now()
	return &selfHostedRuntimeProof{
		ServiceName:        canonicalName,
		DesiredVersion:     desiredVersion,
		DesiredBuildID:     desiredBuildID,
		DesiredBuildNumber: rel.Status.ResolvedBuildNumber,
		ManifestChecksum:   manifestChecksum,
		BinaryPath:         expectedPath,
		ProcExePath:        procExe,
		OnDiskSHA256:       onDiskHash,
		RunningPID:         pid,
		PIDStartUnix:       pidStartUnix,
		ProofSource:        "self_hosted_runtime_proof",
		ProofTimeUnixMs:    now.UnixMilli(),
	}, proofReasonOK
}

// proofCanRefreshInstalledState returns (true, "") when the existing record
// disagrees with the proof in any material field, and (false, reason)
// when the record already reflects the proven identity (idempotent skip).
//
// The "already_canonical" skip is the idempotency guard: the writer can be
// invoked on every heartbeat cycle without producing etcd churn or write
// amplification.
func proofCanRefreshInstalledState(existing *node_agentpb.InstalledPackage, proof *selfHostedRuntimeProof) (bool, string) {
	if proof == nil {
		return false, "no_proof"
	}
	if existing == nil {
		return true, ""
	}
	statusOK := existing.GetStatus() == "installed"
	versionOK := existing.GetVersion() == proof.DesiredVersion
	checksumOK := strings.EqualFold(strings.TrimSpace(existing.GetChecksum()), proof.ManifestChecksum)
	buildIDOK := proof.DesiredBuildID == "" || existing.GetBuildId() == proof.DesiredBuildID
	metaErr := strings.TrimSpace(existing.GetMetadata()["error"])
	proofMetaOK := existing.GetMetadata()["proof_source"] == "self_hosted_runtime_proof"
	// Timestamp anchor check: the verifier flags service.old_pid_after_upgrade
	// when ApplyTime = max(InstalledUnix, UpdatedUnix) is newer than the PID
	// start. A canonical record must not have either timestamp ahead of PID
	// start. When PIDStartUnix is unknown (0), skip the time check — the
	// proof anchor isn't available, so don't force an unnecessary write.
	timesOK := proof.PIDStartUnix == 0 ||
		(existing.GetInstalledUnix() <= proof.PIDStartUnix &&
			existing.GetUpdatedUnix() <= proof.PIDStartUnix)
	if statusOK && versionOK && checksumOK && buildIDOK && metaErr == "" && proofMetaOK && timesOK {
		return false, "already_canonical"
	}
	return true, ""
}

// applySelfHostedInstalledStateRefresh returns the refreshed record. Callers
// are responsible for writing it via installed_state.WriteInstalledPackage.
//
// The function never returns the *same* pointer it was given — it copies the
// existing record before mutating so the caller's snapshot is untouched.
func applySelfHostedInstalledStateRefresh(existing *node_agentpb.InstalledPackage, proof *selfHostedRuntimeProof, nodeID string) *node_agentpb.InstalledPackage {
	// Anchor timestamps to the running PID's start time when available. The
	// verifier compares max(InstalledUnix, UpdatedUnix) against PID start to
	// decide "upgrade without restart"; using time.Now() here would cause
	// every heartbeat refresh to look like a fresh upgrade and flag the
	// (correctly) older PID as stale. Fall back to ProofTimeUnixMs/1000 only
	// when PIDStartUnix is unavailable (legacy behavior — strictly worse,
	// but no regression vs. pre-fix).
	anchor := proof.PIDStartUnix
	if anchor == 0 {
		anchor = proof.ProofTimeUnixMs / 1000
	}

	var pkg *node_agentpb.InstalledPackage
	if existing != nil {
		cp := *existing
		pkg = &cp
		// Deep-copy metadata so we can mutate without aliasing.
		md := make(map[string]string, len(existing.Metadata)+5)
		for k, v := range existing.Metadata {
			md[k] = v
		}
		pkg.Metadata = md
		// Clamp InstalledUnix forward to anchor only if existing is missing
		// or in the future relative to the anchor. Preserving an older
		// install timestamp is correct — the binary may have been installed
		// before the current PID started — but a value > anchor is the bug
		// this fix corrects.
		if pkg.InstalledUnix == 0 || pkg.InstalledUnix > anchor {
			pkg.InstalledUnix = anchor
		}
	} else {
		pkg = &node_agentpb.InstalledPackage{
			NodeId:        nodeID,
			Name:          proof.ServiceName,
			Kind:          "SERVICE",
			InstalledUnix: anchor,
			Metadata:      map[string]string{},
		}
	}
	pkg.Version = proof.DesiredVersion
	pkg.BuildId = proof.DesiredBuildID
	pkg.BuildNumber = proof.DesiredBuildNumber
	pkg.Checksum = proof.ManifestChecksum
	pkg.Status = "installed"
	pkg.UpdatedUnix = anchor
	// Stale failure error has been disproven by the proof; clear it.
	delete(pkg.Metadata, "error")
	pkg.Metadata["proof_source"] = proof.ProofSource
	pkg.Metadata["proof_manifest_checksum"] = proof.ManifestChecksum
	pkg.Metadata["proof_on_disk_sha256"] = proof.OnDiskSHA256
	pkg.Metadata["proof_binary_path"] = proof.BinaryPath
	pkg.Metadata["proof_time_unix_ms"] = fmt.Sprintf("%d", proof.ProofTimeUnixMs)
	pkg.Metadata["entrypoint_checksum"] = proof.ManifestChecksum
	return pkg
}

// refreshSelfHostedInstalledState is the heartbeat-phase entry point. It
// iterates the self-hosted allowlist, builds a proof for each, and writes
// the refreshed installed_state record only when proof passes AND the
// existing record disagrees with the proof.
//
// Failure reasons are logged at INFO level (not WARN/ERROR) because a missing
// manifest or non-running process is the normal state during early boot and
// during the install gap window. The doctor's
// installed_state_runtime_mismatch finding remains the operator-facing
// signal.
func (srv *NodeAgentServer) refreshSelfHostedInstalledState(ctx context.Context) {
	if srv.nodeID == "" {
		return
	}
	deps := defaultSelfHostedProofDeps()
	for canonicalName := range selfHostedServiceNames {
		proof, reason := buildSelfHostedRuntimeProof(ctx, canonicalName, deps)
		if reason != "" {
			// Single log line per failed proof per cycle; debug-level
			// noise is left to GetServiceRuntimeProof which is the proof
			// inspection RPC.
			log.Printf("nodeagent: self-hosted runtime proof skipped %s: %s", canonicalName, reason)
			continue
		}
		existing, _ := installed_state.GetInstalledPackage(ctx, srv.nodeID, "SERVICE", canonicalName)
		canRefresh, refreshReason := proofCanRefreshInstalledState(existing, proof)
		if !canRefresh {
			_ = refreshReason
			continue
		}
		pkg := applySelfHostedInstalledStateRefresh(existing, proof, srv.nodeID)
		// Non-install writer: preserve install-receipt metadata across
		// the proof refresh. applySelfHostedInstalledStateRefresh already
		// deep-copies existing.Metadata when existing != nil, so this is
		// belt-and-suspenders defence — pkg may now carry refreshed proof_*
		// keys + the entrypoint_checksum, but receipt-namespace keys
		// (unit_file_sha256, binary_sha256, installed_by, etc) must survive
		// any future restructuring of applySelfHostedInstalledStateRefresh.
		// See docs/architecture/retire-systemd-sidecars.md.
		PreserveInstallReceiptMetadata(existing, pkg)
		if err := installed_state.WriteInstalledPackage(ctx, pkg); err != nil {
			log.Printf("nodeagent: self-hosted runtime proof write %s/%s: %v", "SERVICE", canonicalName, err)
			continue
		}
		preview := proof.OnDiskSHA256
		if len(preview) > 16 {
			preview = preview[:16]
		}
		log.Printf("nodeagent: self-hosted runtime proof refreshed %s → version=%s buildId=%s sha256=%s… pid=%d",
			canonicalName, proof.DesiredVersion, proof.DesiredBuildID, preview, proof.RunningPID)
	}
}
