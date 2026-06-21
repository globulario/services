package main

// tools_release.go — PR-16 Phase 2: the release_verify_boundary MCP tool.
//
// It gathers owner-RPC evidence for one service binary on one node, maps that
// evidence into release_boundary.Inputs, calls the pure verdict evaluator, and
// returns the structured report. It is read-only (intent:awareness.
// mcp_bridge_exposes_safe_tools_only) — no repair, no mutation, no storage
// reads. Every truth source is the owning actor's typed RPC
// (meta.storage_is_not_semantic_authority): desired↦controller,
// published-artifact↦repository, installed+runtime↦node-agent.
//
// Connection errors are NOT absorbed (meta.connection_errors_must_not_be_
// absorbed): a failing RPC yields absent evidence (→ INDETERMINATE in the pure
// engine) AND its real error is surfaced in the response "collection_errors"
// map, so a partial outage is visible rather than silently degraded.

import (
	"context"
	"strconv"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/release_boundary"
	repositorypb "github.com/globulario/services/golang/repository/repositorypb"
	"google.golang.org/protobuf/types/known/emptypb"
)

func registerReleaseTools(s *server) {

	// ── release_verify_boundary ─────────────────────────────────────────
	s.register(toolDef{
		Name:        "release_verify_boundary",
		Description: "Verify that a service's desired published artifact is the same artifact installed on a node and currently running, using the PR-16 release-boundary proof evaluator. Read-only: proves the build_id published by the repository equals what is installed and running, and that the process restarted after install. Returns PROVEN / FAILED / INDETERMINATE / NOT_APPLICABLE per assertion (A0..A4).",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"service_id": {Type: "string", Description: "Service identifier, e.g. \"globular/echo\" or \"echo\". Matched against desired-state ServiceId."},
				"node_id":    {Type: "string", Description: "The node to inspect installed + runtime evidence on."},
			},
			Required: []string{"service_id", "node_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		serviceID := getStr(args, "service_id")
		if serviceID == "" {
			serviceID = getStr(args, "service_name")
		}
		nodeID := getStr(args, "node_id")
		if nodeID == "" {
			nodeID = getStr(args, "node_name")
		}

		ev := s.collectReleaseBoundaryEvidence(ctx, serviceID, nodeID)
		inputs := mapReleaseBoundaryInputs(serviceID, nodeID, ev)
		report := release_boundary.Evaluate(inputs)

		out := reportToMap(report)
		if len(ev.collectionErrors) > 0 {
			// Surface real connection/RPC errors alongside the verdict — never
			// absorb them into a generic INDETERMINATE.
			out["collection_errors"] = ev.collectionErrors
		}
		return out, nil
	})
}

// releaseBoundaryEvidence holds the raw owner-RPC results plus any real errors
// encountered collecting them. Mapping to release_boundary.Inputs is a pure
// function of this struct (see mapReleaseBoundaryInputs).
type releaseBoundaryEvidence struct {
	desired   *cluster_controllerpb.DesiredService
	manifest  *repositorypb.ArtifactManifest
	verify    *repositorypb.VerifyArtifactResponse
	verifyErr error
	installed *node_agentpb.InstalledPackage
	runtime   *node_agentpb.ServiceRuntimeProof

	collectionErrors map[string]string
}

func (e *releaseBoundaryEvidence) addErr(source, msg string) {
	if e.collectionErrors == nil {
		e.collectionErrors = map[string]string{}
	}
	e.collectionErrors[source] = msg
}

// collectReleaseBoundaryEvidence performs the owner-RPC fan-out. Each RPC
// failure records the real error and leaves that evidence absent; the pure
// evaluator converts absence into INDETERMINATE.
func (s *server) collectReleaseBoundaryEvidence(ctx context.Context, serviceID, nodeID string) *releaseBoundaryEvidence {
	ev := &releaseBoundaryEvidence{}
	pkgName := serviceShortName(serviceID)

	// 1. Desired state (controller) — find the desired service by ServiceId.
	if conn, err := s.clients.get(ctx, controllerEndpoint()); err != nil {
		ev.addErr("desired", "cluster controller unavailable: "+err.Error())
	} else {
		client := cluster_controllerpb.NewClusterControllerServiceClient(conn)
		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		state, err := client.GetDesiredState(callCtx, &emptypb.Empty{})
		cancel()
		if err != nil {
			ev.addErr("desired", "GetDesiredState: "+err.Error())
		} else {
			for _, svc := range state.GetServices() {
				if svc.GetServiceId() == serviceID {
					ev.desired = svc
					break
				}
			}
			if ev.desired == nil {
				ev.addErr("desired", "service "+serviceID+" not found in desired state")
			}
		}
	}

	// 2 + 3. Repository manifest + verify — keyed by the desired artifact.
	// Both require the desired record to build the ArtifactRef.
	if ev.desired != nil {
		ref := &repositorypb.ArtifactRef{
			PublisherId: servicePublisher(serviceID),
			Name:        pkgName,
			Version:     ev.desired.GetVersion(),
			Platform:    ev.desired.GetPlatform(),
			Kind:        repositorypb.ArtifactKind_SERVICE,
		}
		if conn, err := s.clients.get(ctx, repositoryEndpoint()); err != nil {
			ev.addErr("manifest", "repository unavailable: "+err.Error())
			ev.verifyErr = err
		} else {
			client := repositorypb.NewPackageRepositoryClient(conn)

			callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
			mResp, err := client.GetArtifactManifest(callCtx, &repositorypb.GetArtifactManifestRequest{
				Ref:         ref,
				BuildNumber: ev.desired.GetBuildNumber(),
			})
			cancel()
			if err != nil {
				ev.addErr("manifest", "GetArtifactManifest: "+err.Error())
			} else {
				ev.manifest = mResp.GetManifest()
			}

			vCallCtx, vCancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
			vResp, vErr := client.VerifyArtifact(vCallCtx, &repositorypb.VerifyArtifactRequest{
				Ref:             ref,
				BuildId:         ev.desired.GetBuildId(),
				VerifyDigest:    true,
				IncludeLedger:   true,
				IncludeManifest: true,
				IncludeBlob:     true,
			})
			vCancel()
			if vErr != nil {
				ev.verifyErr = vErr
				ev.addErr("verify", "VerifyArtifact: "+vErr.Error())
			} else {
				ev.verify = vResp
			}
		}
	}

	// 4 + 5. Installed + runtime evidence (node-agent).
	if endpoint, err := s.resolveNodeAgentEndpoint(ctx, nodeID); err != nil {
		ev.addErr("node_agent", "node agent endpoint: "+err.Error())
	} else if conn, err := s.clients.get(ctx, endpoint); err != nil {
		ev.addErr("node_agent", "node agent unavailable: "+err.Error())
	} else {
		client := node_agentpb.NewNodeAgentServiceClient(conn)

		iCtx, iCancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		iResp, iErr := client.GetInstalledPackage(iCtx, &node_agentpb.GetInstalledPackageRequest{
			NodeId: nodeID,
			Kind:   "SERVICE",
			Name:   pkgName,
		})
		iCancel()
		if iErr != nil {
			ev.addErr("installed", "GetInstalledPackage: "+iErr.Error())
		} else {
			ev.installed = iResp.GetPackage()
			if ev.installed == nil {
				ev.addErr("installed", "package "+pkgName+" not installed on "+nodeID)
			}
		}

		rCtx, rCancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		rResp, rErr := client.GetServiceRuntimeProof(rCtx, &node_agentpb.GetServiceRuntimeProofRequest{
			NodeId:      nodeID,
			ServiceName: pkgName,
		})
		rCancel()
		if rErr != nil {
			ev.addErr("runtime", "GetServiceRuntimeProof: "+rErr.Error())
		} else {
			ev.runtime = pickRuntimeProof(rResp.GetProofs(), serviceID, pkgName)
			if ev.runtime == nil {
				ev.addErr("runtime", "no runtime proof for "+pkgName+" on "+nodeID)
			}
		}
	}

	return ev
}

// mapReleaseBoundaryInputs is the PURE mapping from collected owner-RPC
// evidence to release_boundary.Inputs. It is deterministic and has no I/O, so
// it is unit-testable with proto literals and no live cluster.
//
// Critical: InstallCommittedUnix is fed ONLY from metadata["installed_at"] (the
// per-build install-commit time). proto InstalledPackage.InstalledUnix is the
// preserved first-install time and is deliberately NOT used — see
// docs/design/pr16-release-boundary-evidence-mapping.md.
func mapReleaseBoundaryInputs(serviceID, nodeID string, ev *releaseBoundaryEvidence) release_boundary.Inputs {
	in := release_boundary.Inputs{
		ServiceName: serviceID,
		NodeName:    nodeID,
		PackageKind: "SERVICE",
	}
	if ev == nil {
		return in
	}

	if ev.desired != nil {
		in.DesiredBuildID = ev.desired.GetBuildId()
	}

	if ev.manifest != nil {
		in.Manifest = &release_boundary.ManifestEvidence{
			BuildID:            ev.manifest.GetBuildId(),
			PublishState:       publishStateString(ev.manifest.GetPublishState()),
			EntrypointChecksum: ev.manifest.GetEntrypointChecksum(),
			ProvenanceGitSHA:   ev.manifest.GetProvenance().GetBuildCommit(),
		}
	}

	// Repository (A0): only an explicit verification response is evidence.
	// An RPC error → absent evidence → A0 INDETERMINATE (NOT verified=false).
	if ev.verify != nil {
		in.Repository = &release_boundary.RepositoryEvidence{
			Present:  true,
			Verified: ev.verify.GetStatus() == repositorypb.ArtifactVerifyStatus_ARTIFACT_VERIFY_OK,
			Reason:   ev.verify.GetReason(),
		}
	}

	if ev.installed != nil {
		in.Installed = &release_boundary.InstalledEvidence{
			BuildID:              ev.installed.GetBuildId(),
			EntrypointChecksum:   ev.installed.GetMetadata()["entrypoint_checksum"],
			InstallCommittedUnix: parseInstalledAtUnix(ev.installed.GetMetadata()),
		}
	}

	if ev.runtime != nil {
		in.Runtime = &release_boundary.RuntimeEvidence{
			Running:          ev.runtime.GetSystemdActiveState() == "active",
			PID:              int(ev.runtime.GetRunningPid()),
			RunningExeSHA256: ev.runtime.GetRunningExeSha256(),
			ProcessStartUnix: timestampUnix(ev.runtime),
		}
		// Wrapper / unhashable detection from the owner-reported installed_path
		// (mirrors verifier.installedPathIsUpstream — managed binaries live
		// under /usr/lib/globular/bin/). Only mark unhashable when we have a
		// path that is clearly upstream; an empty path stays hashable so other
		// assertions drive the verdict.
		if p := ev.runtime.GetInstalledPath(); p != "" && isUpstreamInstalledPath(p) {
			in.Unhashable = true
		}
	}

	return in
}

// parseInstalledAtUnix reads the per-build install-commit timestamp from the
// install receipt metadata. Absent / empty / malformed → 0, which the pure
// evaluator treats as A4 INDETERMINATE. There is deliberately NO fallback to
// proto InstalledPackage.InstalledUnix (first-install time).
func parseInstalledAtUnix(metadata map[string]string) int64 {
	if metadata == nil {
		return 0
	}
	raw := strings.TrimSpace(metadata["installed_at"])
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v <= 0 {
		return 0
	}
	return v
}

// publishStateString maps the PublishState enum to the exact string the pure
// evaluator compares against, rather than relying on enum String() formatting.
func publishStateString(ps repositorypb.PublishState) string {
	if ps == repositorypb.PublishState_PUBLISHED {
		return "PUBLISHED"
	}
	return ps.String()
}

// timestampUnix returns the process start time in unix seconds, or 0 if absent.
func timestampUnix(p *node_agentpb.ServiceRuntimeProof) int64 {
	ts := p.GetProcessStartTime()
	if ts == nil {
		return 0
	}
	return ts.AsTime().Unix()
}

// pickRuntimeProof selects the proof matching the requested service from a
// repeated response, matching on service_id first then short name.
func pickRuntimeProof(proofs []*node_agentpb.ServiceRuntimeProof, serviceID, pkgName string) *node_agentpb.ServiceRuntimeProof {
	for _, p := range proofs {
		if p.GetServiceId() == serviceID || p.GetServiceName() == pkgName {
			return p
		}
	}
	if len(proofs) == 1 {
		return proofs[0]
	}
	return nil
}

// isUpstreamInstalledPath mirrors verifier.installedPathIsUpstream: a binary
// managed by Globular lives under one of the managed bin dirs; anything else is
// an upstream wrapper (un-hashable). Kept as a small local mirror because the
// verifier helper is unexported.
func isUpstreamInstalledPath(path string) bool {
	const (
		managed1 = "/usr/lib/globular/bin/"
		managed2 = "/usr/local/lib/globular/bin/"
	)
	return !strings.HasPrefix(path, managed1) && !strings.HasPrefix(path, managed2)
}

// servicePublisher / serviceShortName split a "publisher/name" service id.
func servicePublisher(serviceID string) string {
	if i := strings.LastIndex(serviceID, "/"); i >= 0 {
		return serviceID[:i]
	}
	return ""
}

func serviceShortName(serviceID string) string {
	if i := strings.LastIndex(serviceID, "/"); i >= 0 {
		return serviceID[i+1:]
	}
	return serviceID
}

// reportToMap serializes a release_boundary.Report for the MCP envelope.
func reportToMap(r release_boundary.Report) map[string]interface{} {
	assertions := make([]map[string]interface{}, 0, len(r.Assertions))
	for _, a := range r.Assertions {
		assertions = append(assertions, map[string]interface{}{
			"id":       string(a.ID),
			"name":     a.Name,
			"verdict":  string(a.Verdict),
			"reason":   a.Reason,
			"evidence": a.Evidence,
		})
	}
	return map[string]interface{}{
		"service":    r.ServiceName,
		"node":       r.NodeName,
		"build_id":   r.BuildID,
		"checksum":   r.Checksum,
		"verdict":    string(r.Verdict),
		"assertions": assertions,
	}
}
