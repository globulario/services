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
	"strconv"
	"strings"
	"time"

	"github.com/globulario/services/golang/installed_state"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/supervisor"
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
	Now            func() time.Time
}

func defaultRuntimeProofDeps() runtimeProofDeps {
	return runtimeProofDeps{
		ShowProperties: supervisor.ShowProperties,
		HashFile:       cachedSha256,
		ReadProcExe: func(pid int) (string, error) {
			return os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
		},
		Now: time.Now,
	}
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
		ServiceName:      canonicalServiceName(name),
		ServiceId:        name,
		NodeId:           nodeID,
		ExpectedBuildId:  strings.TrimSpace(pkg.GetBuildId()),
		ExpectedVersion:  strings.TrimSpace(pkg.GetVersion()),
		InstalledPath:    installedPath,
		InstalledSha256:  normalizeHash(pkg.GetMetadata()["entrypoint_checksum"]),
		CheckedAt:        timestamppb.New(deps.Now()),
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
	if hash, err := deps.HashFile(installedPath); err == nil {
		// Override the claim with the freshly-computed proof. If they
		// differ, the consumer raises service.running_binary_hash_mismatch.
		// Storing both would bloat the proto for no consumer benefit —
		// the claim already lives in installed_state.
		p.InstalledSha256 = hash
	} else {
		p.Errors = append(p.Errors,
			fmt.Sprintf("hash installed binary %s: %v", installedPath, err))
	}

	// ── Systemd effective unit + MainPID ───────────────────────────────
	// COMMAND packages have no service; mark unproven and return what we
	// have (on-disk hash above).
	if kind == "COMMAND" || name == "" {
		p.Errors = append(p.Errors, "no systemd unit for package (kind=COMMAND)")
		return p
	}
	unit := "globular-" + strings.ReplaceAll(name, "_", "-") + ".service"

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

	// Process start time — parse the systemd timestamp. Format example:
	// "Mon 2026-05-20 16:44:00 UTC". On units that have never run, the
	// property is empty or "n/a" — silently skip.
	if ts := strings.TrimSpace(props["ExecMainStartTimestamp"]); ts != "" && ts != "n/a" {
		if t, perr := parseSystemdTimestamp(ts); perr == nil {
			p.ProcessStartTime = timestamppb.New(t)
		} else {
			p.Errors = append(p.Errors,
				fmt.Sprintf("parse ExecMainStartTimestamp %q: %v", ts, perr))
		}
	}

	// Runtime version probing (live process /version endpoint) is not yet
	// implemented for every service. Mark unproven so the consumer raises
	// service.runtime_identity_unproven at degraded severity rather than
	// silently treating "no runtime version known" as match.
	p.RuntimeVersion = ""
	p.RuntimeBuildId = ""
	p.Errors = append(p.Errors,
		"runtime_version probe not implemented (service.runtime_identity_unproven)")

	return p
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
	for _, kind := range []string{"SERVICE", "INFRASTRUCTURE", "APPLICATION"} {
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
