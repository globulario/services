// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.runtime_proof
// @awareness file_role=independent_runtime_evidence_for_installed_services
// @awareness implements=globular.platform:intent.runtime.identity.requires_proof
// @awareness implements=globular.platform:intent.health.requires_fresh_evidence
// @awareness implements=globular.platform:intent.node_agent.is_executor_not_cluster_brain
// @awareness risk=critical
package main

// runtime_proof.go — Phase 2 of the Diagnostic Honesty Refactor.
//
// Implements GetServiceRuntimeProof, an RPC that returns independent runtime
// evidence (running PID, /proc/<pid>/exe sha256, systemd effective unit, on-
// disk binary sha256) for one or every installed service on this node.
//
// The reply distinguishes CLAIMS (desired/installed identity from etcd) from
// PROOFS (process identity from /proc + systemd). Consumers — cluster_doctor,
// the verifier, controller convergence — reconcile the two and emit:
//   service.running_binary_hash_mismatch   (critical)
//   service.running_version_mismatch       (critical)
//   service.old_pid_after_upgrade          (critical)
//   service.runtime_identity_unproven      (degraded)
//
// The Prime Directive: systemd-active + installed-state are CLAIMS. Proof is
// the running process, the bytes on disk, and the effective unit.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/globulario/services/golang/identity"
	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/supervisor"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// runtimeProofDeps is the small injection surface that lets tests drive the
// pure collection function without touching real systemd / /proc / disk.
type runtimeProofDeps struct {
	ShowProperties func(ctx context.Context, unit string, props ...string) (map[string]string, error)
	HashFile       func(path string) (string, error)
	ReadProcExe    func(pid int) (string, error)
	// ProcStartTime returns the wall-clock start time of a running PID,
	// preferring sub-second precision over systemd's ExecMainStartTimestamp
	// (which truncates to whole seconds). Returns (zero, error) when the
	// PID is gone or unreadable; callers fall back to the systemd value.
	ProcStartTime func(pid int) (time.Time, error)
	Now           func() time.Time
}

func defaultRuntimeProofDeps() runtimeProofDeps {
	return runtimeProofDeps{
		ShowProperties: supervisor.ShowProperties,
		HashFile:       cachedSha256,
		ReadProcExe: func(pid int) (string, error) {
			return os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
		},
		ProcStartTime: procStartTimeFromStat,
		Now:           time.Now,
	}
}

// procStartTimeFromStat reads the ctime of /proc/<pid> as a wall-clock
// timestamp with nanosecond precision. The /proc/<pid> directory is created
// by the kernel when the process is forked, so its ctime is the process's
// true start time — much finer than systemd's whole-second
// ExecMainStartTimestamp. Returns (zero, error) when the PID has exited.
func procStartTimeFromStat(pid int) (time.Time, error) {
	fi, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	if err != nil {
		return time.Time{}, err
	}
	return fi.ModTime(), nil
}

// systemd properties to fetch. Keep this list narrow: every property adds
// parse + branch cost in a hot path (called once per installed service on
// every doctor / verifier sweep).
var runtimeProofSystemdProperties = []string{
	"ActiveState",
	"SubState",
	"Type",
	"ExecStart",
	"FragmentPath",
	"MainPID",
	"ExecMainStartTimestamp",
}

// collectServiceRuntimeProof builds one ServiceRuntimeProof from an installed-
// state record + injected collectors. Collection errors are appended to
// proof.Errors[] (degraded, partial proof returned) rather than aborting the
// reply — the consumer can still surface what IS verifiable.
//
// This function does no I/O directly; everything goes through deps.
func collectServiceRuntimeProof(
	ctx context.Context,
	nodeID string,
	pkg *node_agentpb.InstalledPackage,
	deps runtimeProofDeps,
) *node_agentpb.ServiceRuntimeProof {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	name := strings.TrimSpace(pkg.GetName())
	kind := strings.ToUpper(strings.TrimSpace(pkg.GetKind()))
	installedPath := installedBinaryPath(name, kind)

	p := &node_agentpb.ServiceRuntimeProof{
		ServiceName:     canonicalServiceName(name),
		ServiceId:       name,
		NodeId:          nodeID,
		ExpectedBuildId: strings.TrimSpace(pkg.GetBuildId()),
		ExpectedVersion: strings.TrimSpace(pkg.GetVersion()),
		InstalledPath:   installedPath,
		InstalledSha256: normalizeHash(pkg.GetMetadata()["entrypoint_checksum"]),
		CheckedAt:       timestamppb.New(deps.Now()),
	}
	// Fall back to the package Checksum field when metadata is empty (legacy
	// records). normalizeHash strips "sha256:" prefix + lowercases.
	if p.InstalledSha256 == "" {
		p.InstalledSha256 = normalizeHash(pkg.GetChecksum())
	}

	// ── On-disk binary hash ────────────────────────────────────────────
	// The installed_state Checksum is a CLAIM (recorded by apply_package).
	// We re-hash the bytes now: a drift between installed_state.Checksum
	// and the current on-disk hash means the binary was replaced out-of-
	// band after apply (root-cause of the "old binary still on disk"
	// failure mode in the brief).
	//
	// installedBinaryPath() encodes the "<name>_server" convention for
	// SERVICE-kind packages. A few services (mcp, gateway, etc.) ship a
	// binary that does NOT follow that convention — the systemd unit
	// ExecStart is the only authority for the actual deployed path. We
	// record whether the convention path was hashable; if not, the
	// systemd-ExecStart fallback below will compute the real hash.
	installedHashed := false
	if hash, err := deps.HashFile(installedPath); err == nil {
		// Override the claim with the freshly-computed proof. If they
		// differ, the consumer raises service.running_binary_hash_mismatch.
		// Storing both would bloat the proto for no consumer benefit —
		// the claim already lives in installed_state.
		p.InstalledSha256 = hash
		installedHashed = true
	}

	// ── Systemd effective unit + MainPID ───────────────────────────────
	// COMMAND packages intentionally have no systemd unit and no long-running
	// process. Return the binary/install proof we already have without
	// synthesizing an error; otherwise the verifier's generic "errors only"
	// path upgrades an expected CLI package into runtime_identity_unproven.
	if kind == "COMMAND" || name == "" {
		return p
	}
	// Derive the systemd unit name from the identity registry when
	// available; fall back to the "globular-<name>.service" convention
	// otherwise. The registry is the authority for services that ship with
	// upstream's naming (keepalived.service, scylla-server.service, etc.).
	// Without this lookup, ShowProperties("globular-keepalived.service")
	// always misses, no ExecStart is recovered, the binary-hash fallback
	// never runs, and the verdict permanently degrades to
	// runtime_identity_unproven.
	unit := "globular-" + strings.ReplaceAll(name, "_", "-") + ".service"
	if key, ok := identity.NormalizeServiceKey(name); ok {
		if id, ok := identity.IdentityByKey(key); ok && id.UnitName != "" {
			unit = id.UnitName
		}
	}

	props, err := deps.ShowProperties(ctx, unit, runtimeProofSystemdProperties...)
	if err != nil {
		p.Errors = append(p.Errors, fmt.Sprintf("systemctl show %s: %v", unit, err))
		return p
	}

	p.SystemdActiveState = props["ActiveState"]
	p.SystemdSubState = props["SubState"]
	p.SystemdUnitPath = props["FragmentPath"]
	p.EffectiveExecStart = props["ExecStart"]
	p.EffectiveType = props["Type"]

	// Fallback: if the convention-based installed path didn't exist (e.g.
	// services like mcp that don't follow the "<name>_server" convention),
	// pull the real binary path from the unit's ExecStart and hash that.
	// systemd's ExecStart property is the authoritative location of the
	// binary it loads — using it eliminates false-positive
	// package.installed_binary_hash_mismatch findings caused by stale
	// tarball-checksum fallback in p.InstalledSha256.
	if !installedHashed {
		if binPath := firstExecStartPath(p.EffectiveExecStart); binPath != "" && binPath != installedPath {
			if hash, err := deps.HashFile(binPath); err == nil {
				p.InstalledPath = binPath
				p.InstalledSha256 = hash
				installedHashed = true
			}
		}
	}
	if !installedHashed {
		p.Errors = append(p.Errors,
			fmt.Sprintf("hash installed binary %s: file not found and ExecStart fallback unavailable", installedPath))
	}

	// Hash the unit file at FragmentPath. This proves the effective unit
	// systemd is using matches whatever the deploy step rendered (Phase 5
	// then compares this against the rendered template).
	if p.SystemdUnitPath != "" {
		if h, herr := deps.HashFile(p.SystemdUnitPath); herr == nil {
			p.SystemdUnitSha256 = h
		} else {
			p.Errors = append(p.Errors,
				fmt.Sprintf("hash unit file %s: %v", p.SystemdUnitPath, herr))
		}
	}

	if mainPID := strings.TrimSpace(props["MainPID"]); mainPID != "" && mainPID != "0" {
		if pid, err := strconv.Atoi(mainPID); err == nil {
			p.RunningPid = int32(pid)
			// /proc/<pid>/exe deref + sha256.
			if exe, rerr := deps.ReadProcExe(pid); rerr == nil {
				p.RunningExePath = exe
				if h, herr := deps.HashFile(exe); herr == nil {
					p.RunningExeSha256 = h
				} else {
					p.Errors = append(p.Errors,
						fmt.Sprintf("hash running exe %s: %v", exe, herr))
				}
			} else {
				p.Errors = append(p.Errors,
					fmt.Sprintf("read /proc/%d/exe: %v", pid, rerr))
			}
		}
	}

	// Wrapper packages (notably scylla-manager and scylla-manager-agent)
	// intentionally launch a small Globular-managed shell wrapper from
	// /usr/lib/globular/bin that execs the real upstream binary in /usr/bin
	// or /opt. The verifier's binary-identity contract is about the real
	// long-lived executable, not the trampoline script. If we leave
	// InstalledPath/InstalledSha256 pointing at the wrapper, doctor emits a
	// false running_binary_hash_mismatch on every healthy node because
	// /proc/<pid>/exe resolves to the upstream ELF. When the installed path is
	// a Globular wrapper but the running exe is an upstream path, pivot the
	// installed proof to the running binary so the verifier can apply its
	// built-in wraps_upstream_binary semantics.
	if p.GetRunningExePath() != "" &&
		verifierInstalledPathIsGlobularWrapper(p.GetInstalledPath()) &&
		runtimeProofInstalledPathIsUpstream(p.GetRunningExePath()) {
		if h, err := deps.HashFile(p.GetRunningExePath()); err == nil {
			p.InstalledPath = p.GetRunningExePath()
			p.InstalledSha256 = h
		}
	}

	// Process start time — prefer the /proc/<pid> mtime (nanosecond
	// precision) over systemd's ExecMainStartTimestamp (whole seconds).
	// The verifier compares this against the controller's millisecond-
	// precision ApplyTime; using systemd's text form rounds process_start
	// down to .000 and can fire a false-positive service.old_pid_after_upgrade
	// when ApplyTime lands a few hundred ms later in the same second
	// (e.g. .078 from the apply RPC return path).
	procStarted := false
	if p.RunningPid > 0 && deps.ProcStartTime != nil {
		if t, err := deps.ProcStartTime(int(p.RunningPid)); err == nil && !t.IsZero() {
			p.ProcessStartTime = timestamppb.New(t)
			procStarted = true
		}
	}
	// Fallback to the systemd text timestamp on units that have never
	// run, or when /proc/<pid> can't be stat'd (PID raced away).
	if !procStarted {
		if ts := strings.TrimSpace(props["ExecMainStartTimestamp"]); ts != "" && ts != "n/a" {
			if t, perr := parseSystemdTimestamp(ts); perr == nil {
				p.ProcessStartTime = timestamppb.New(t)
			} else {
				p.Errors = append(p.Errors,
					fmt.Sprintf("parse ExecMainStartTimestamp %q: %v", ts, perr))
			}
		}
	}

	// Runtime version probing (live process /version endpoint) is not yet
	// implemented for any service — leave RuntimeVersion empty. The verifier
	// already treats an empty proof.RuntimeVersion as "version match
	// implicit" in computeProofStatus (versionOK), so the absence of a probe
	// is not by itself drift.
	//
	// We deliberately do NOT append a synthetic "probe not implemented"
	// error to p.Errors. That marker used to live here as a degradation
	// pin, but it created a perverse rule: every service had at least one
	// proof error, so the verifier's "errors-but-no-other-findings →
	// runtime_identity_unproven" branch (verifier.go:429) demoted every
	// post-install service whose only true state was "verified". The bug
	// was hidden behind a different finding (bootstrap_ordering_skew)
	// when systemd's seconds-precision timestamp made process_start fall
	// inside the same second as ApplyTime; once /proc/<pid> nanoseconds
	// removed that race, the marker began demoting verdicts that should
	// have been runtime_verified. Once a real /version probe exists per
	// service, that code can append its own targeted error here.
	p.RuntimeVersion = ""
	p.RuntimeBuildId = ""

	return p
}

func verifierInstalledPathIsGlobularWrapper(path string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return false
	}
	return strings.HasPrefix(path, filepath.Clean(globularBinDir)+string(os.PathSeparator))
}

func runtimeProofInstalledPathIsUpstream(path string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return false
	}
	switch {
	case strings.HasPrefix(path, "/usr/bin/"),
		strings.HasPrefix(path, "/usr/sbin/"),
		strings.HasPrefix(path, "/usr/libexec/"),
		strings.HasPrefix(path, "/opt/"):
		return true
	default:
		return false
	}
}

// firstExecStartPath returns the absolute path of the executable referenced by
// the systemctl ExecStart property. systemd emits ExecStart as either a bare
// command line ("/usr/lib/globular/bin/foo --flag") or a structured value
// ("{ path=/usr/lib/globular/bin/foo ; argv[]=... }"). We extract the first
// absolute path token from either form. Returns "" when no path is found.
func firstExecStartPath(execStart string) string {
	s := strings.TrimSpace(execStart)
	if s == "" {
		return ""
	}
	// Structured form: "{ path=/usr/lib/... ; argv[]=... ; ... }".
	if idx := strings.Index(s, "path="); idx >= 0 {
		rest := s[idx+len("path="):]
		// Path runs until first whitespace, ';', or '}'.
		end := len(rest)
		for i, r := range rest {
			if r == ' ' || r == '\t' || r == ';' || r == '}' {
				end = i
				break
			}
		}
		if p := strings.TrimSpace(rest[:end]); strings.HasPrefix(p, "/") {
			return p
		}
	}
	// Bare command line: first whitespace-delimited token starting with '/'.
	for _, tok := range strings.Fields(s) {
		if strings.HasPrefix(tok, "/") {
			return tok
		}
	}
	return ""
}

// parseSystemdTimestamp parses the format emitted by systemctl show
// ExecMainStartTimestamp. Systemd uses RFC1123-like English locale day-of-
// week + zone abbreviation, e.g. "Mon 2026-05-20 16:44:00 UTC".
func parseSystemdTimestamp(s string) (time.Time, error) {
	// Try the canonical form first; fall back to RFC1123Z just in case.
	for _, layout := range []string{
		"Mon 2006-01-02 15:04:05 MST",
		"Mon 2006-01-02 15:04:05 -0700",
		time.RFC1123,
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized systemd timestamp")
}

// GetServiceRuntimeProof is the RPC handler. Lists installed packages on
// this node and returns one proof per package (optionally filtered to a
// single service).
func (srv *NodeAgentServer) GetServiceRuntimeProof(
	ctx context.Context,
	req *node_agentpb.GetServiceRuntimeProofRequest,
) (*node_agentpb.GetServiceRuntimeProofResponse, error) {
	if rn := strings.TrimSpace(req.GetNodeId()); rn != "" && rn != srv.nodeID {
		return nil, status.Errorf(codes.InvalidArgument,
			"node_id mismatch: request asserts %q, this node is %q", rn, srv.nodeID)
	}

	wantSvc := canonicalServiceName(strings.TrimSpace(req.GetServiceName()))

	var pkgs []*node_agentpb.InstalledPackage
	for _, kind := range authoritativeInstalledPackageKinds {
		ps, err := installed_state.ListInstalledPackages(ctx, srv.nodeID, kind)
		if err != nil {
			// Soft-fail: a single kind's listing failure should not blank
			// the whole reply. The proof for missing kinds is just absent.
			continue
		}
		for _, p := range ps {
			if wantSvc != "" && canonicalServiceName(p.GetName()) != wantSvc {
				continue
			}
			pkgs = append(pkgs, p)
		}
	}

	deps := defaultRuntimeProofDeps()
	proofs := make([]*node_agentpb.ServiceRuntimeProof, 0, len(pkgs))
	for _, p := range pkgs {
		proofs = append(proofs, collectServiceRuntimeProof(ctx, srv.nodeID, p, deps))
	}
	return &node_agentpb.GetServiceRuntimeProofResponse{Proofs: proofs}, nil
}
