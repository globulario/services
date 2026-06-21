// Package boundarycheck is the shared core behind every PR-16 release-boundary
// front door (the MCP release_verify_boundary tool and the
// `globular release verify-boundary` CLI command).
//
// It owns exactly one of each: one Evidence shape, one collection
// orchestration (Collect), one evidence→Inputs mapping (MapInputs), and one
// call into the pure evaluator (release_boundary.Evaluate). Front doors supply
// only transport — a set of Fetchers closures that perform the owner-RPC calls
// with their own client, auth, and timeout. This keeps "one mapping, one
// evaluator, multiple front doors": no front door forks the proof logic.
//
// All evidence comes from typed owner RPCs (meta.storage_is_not_semantic_
// authority): desired↦controller, published-artifact↦repository,
// installed+runtime↦node-agent. Collection errors are recorded with their real
// message (meta.connection_errors_must_not_be_absorbed) and absent evidence
// degrades to INDETERMINATE in the evaluator (meta.fallback_must_degrade_
// semantics) — never a silent proof.
package boundarycheck

import (
	"context"
	"strconv"
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/release_boundary"
	repositorypb "github.com/globulario/services/golang/repository/repositorypb"
)

// Evidence holds the raw owner-RPC results for one (service, node) plus any
// real errors hit while collecting them. Mapping to release_boundary.Inputs is
// a pure function of this struct (MapInputs).
type Evidence struct {
	Desired   *cluster_controllerpb.DesiredService
	Manifest  *repositorypb.ArtifactManifest
	Verify    *repositorypb.VerifyArtifactResponse
	VerifyErr error
	Installed *node_agentpb.InstalledPackage
	Runtime   *node_agentpb.ServiceRuntimeProof

	CollectionErrors map[string]string
}

func (e *Evidence) addErr(source, msg string) {
	if e.CollectionErrors == nil {
		e.CollectionErrors = map[string]string{}
	}
	e.CollectionErrors[source] = msg
}

// Fetchers is the transport a front door supplies. Each closure performs one
// owner RPC with the caller's own client/auth/timeout and returns raw protos.
// A nil closure is treated as an unavailable source (→ INDETERMINATE).
//
// Resolve calls the repository's deterministic resolver (ResolveArtifact). When
// the request carries a build_id it resolves by that identity and the returned
// manifest carries the authoritative publisher-qualified ref — so no front door
// ever guesses the publisher or scans storage.
type Fetchers struct {
	Desired   func(ctx context.Context) ([]*cluster_controllerpb.DesiredService, error)
	Resolve   func(ctx context.Context, req *repositorypb.ResolveArtifactRequest) (*repositorypb.ArtifactManifest, error)
	Verify    func(ctx context.Context, ref *repositorypb.ArtifactRef, buildID string) (*repositorypb.VerifyArtifactResponse, error)
	Installed func(ctx context.Context, nodeID, kind, name string) (*node_agentpb.InstalledPackage, error)
	Runtime   func(ctx context.Context, nodeID, serviceName string) ([]*node_agentpb.ServiceRuntimeProof, error)
}

// Options carries front-door resolution hints.
type Options struct {
	// Publisher is an explicit override for legacy records that lack a build_id
	// pin and whose service_id is a bare name. It is owner-supplied operator
	// input, never inferred from storage keys.
	Publisher string
}

// Collect performs the owner-RPC fan-out via the supplied Fetchers. Each RPC
// failure records the real error and leaves that evidence absent; the pure
// evaluator converts absence into INDETERMINATE.
func Collect(ctx context.Context, f Fetchers, serviceID, nodeID string, opts Options) *Evidence {
	ev := &Evidence{}
	pkgName := serviceShortName(serviceID)

	// 1. Desired state — find the desired service by ServiceId.
	if f.Desired == nil {
		ev.addErr("desired", "desired-state fetcher unavailable")
	} else if services, err := f.Desired(ctx); err != nil {
		ev.addErr("desired", "GetDesiredState: "+err.Error())
	} else {
		for _, svc := range services {
			if svc.GetServiceId() == serviceID {
				ev.Desired = svc
				break
			}
		}
		if ev.Desired == nil {
			ev.addErr("desired", "service "+serviceID+" not found in desired state")
		}
	}

	// 2 + 3. Repository manifest + verify. Resolve the manifest by the desired
	// build_id (the sole convergence identity) via the repository's own
	// deterministic resolver; the resolved manifest carries the authoritative
	// publisher-qualified ref. No publisher guessing, no storage scan.
	if ev.Desired != nil {
		publisher := opts.Publisher
		if publisher == "" {
			publisher = servicePublisher(serviceID)
		}
		buildID := ev.Desired.GetBuildId()

		switch {
		case f.Resolve == nil:
			ev.addErr("manifest", "resolve fetcher unavailable")
		case buildID == "" && publisher == "":
			// Bare service_id, no build_id pin, no override → cannot form an
			// owner-authoritative reference. INDETERMINATE, never a guess.
			ev.addErr("manifest", "cannot resolve artifact: desired build_id not pinned and no publisher (pass --publisher)")
		default:
			req := &repositorypb.ResolveArtifactRequest{
				PublisherId: publisher,
				Name:        pkgName,
				Kind:        repositorypb.ArtifactKind_SERVICE,
				Platform:    ev.Desired.GetPlatform(),
				Version:     ev.Desired.GetVersion(),
				BuildId:     buildID,
			}
			if m, err := f.Resolve(ctx, req); err != nil {
				ev.addErr("manifest", "ResolveArtifact: "+err.Error())
			} else {
				ev.Manifest = m
			}
		}

		// Verify against the authoritative ref — from the resolved manifest when
		// available, else the publisher-qualified ref we asked for.
		verifyRef := resolvedRef(ev.Manifest, publisher, pkgName, ev.Desired.GetVersion(), ev.Desired.GetPlatform())
		switch {
		case f.Verify == nil:
			ev.addErr("verify", "verify fetcher unavailable")
		case verifyRef == nil:
			ev.addErr("verify", "no publisher-qualified ref to verify (manifest unresolved)")
		default:
			if v, err := f.Verify(ctx, verifyRef, buildID); err != nil {
				ev.VerifyErr = err
				ev.addErr("verify", "VerifyArtifact: "+err.Error())
			} else {
				ev.Verify = v
			}
		}
	}

	// 4. Installed package (node-agent).
	if f.Installed == nil {
		ev.addErr("installed", "installed-package fetcher unavailable")
	} else if pkg, err := f.Installed(ctx, nodeID, "SERVICE", pkgName); err != nil {
		ev.addErr("installed", "GetInstalledPackage: "+err.Error())
	} else if pkg == nil {
		ev.addErr("installed", "package "+pkgName+" not installed on "+nodeID)
	} else {
		ev.Installed = pkg
	}

	// 5. Runtime proof (node-agent).
	if f.Runtime == nil {
		ev.addErr("runtime", "runtime-proof fetcher unavailable")
	} else if proofs, err := f.Runtime(ctx, nodeID, pkgName); err != nil {
		ev.addErr("runtime", "GetServiceRuntimeProof: "+err.Error())
	} else if proof := pickRuntimeProof(proofs, serviceID, pkgName); proof == nil {
		ev.addErr("runtime", "no runtime proof for "+pkgName+" on "+nodeID)
	} else {
		ev.Runtime = proof
	}

	return ev
}

// MapInputs is the PURE mapping from collected owner-RPC evidence to
// release_boundary.Inputs. Deterministic, no I/O — unit-testable with proto
// literals and no live cluster.
//
// Critical: InstallCommittedUnix is fed ONLY from metadata["installed_at"] (the
// per-build install-commit time). proto InstalledPackage.InstalledUnix (the
// preserved first-install time) is deliberately NOT used — see
// docs/design/pr16-release-boundary-evidence-mapping.md.
func MapInputs(serviceID, nodeID string, ev *Evidence) release_boundary.Inputs {
	in := release_boundary.Inputs{
		ServiceName: serviceID,
		NodeName:    nodeID,
		PackageKind: "SERVICE",
	}
	if ev == nil {
		return in
	}

	if ev.Desired != nil {
		in.DesiredBuildID = ev.Desired.GetBuildId()
	}

	if ev.Manifest != nil {
		in.Manifest = &release_boundary.ManifestEvidence{
			BuildID:            ev.Manifest.GetBuildId(),
			PublishState:       publishStateString(ev.Manifest.GetPublishState()),
			EntrypointChecksum: ev.Manifest.GetEntrypointChecksum(),
			ProvenanceGitSHA:   ev.Manifest.GetProvenance().GetBuildCommit(),
		}
	}

	// Repository (A0): only an explicit verification response is evidence.
	// An RPC error → absent evidence → A0 INDETERMINATE (NOT verified=false).
	if ev.Verify != nil {
		in.Repository = &release_boundary.RepositoryEvidence{
			Present:  true,
			Verified: ev.Verify.GetStatus() == repositorypb.ArtifactVerifyStatus_ARTIFACT_VERIFY_OK,
			Reason:   ev.Verify.GetReason(),
		}
	}

	if ev.Installed != nil {
		in.Installed = &release_boundary.InstalledEvidence{
			BuildID:              ev.Installed.GetBuildId(),
			EntrypointChecksum:   ev.Installed.GetMetadata()["entrypoint_checksum"],
			InstallCommittedUnix: parseInstalledAtUnix(ev.Installed.GetMetadata()),
		}
	}

	if ev.Runtime != nil {
		in.Runtime = &release_boundary.RuntimeEvidence{
			Running:          ev.Runtime.GetSystemdActiveState() == "active",
			PID:              int(ev.Runtime.GetRunningPid()),
			RunningExeSHA256: ev.Runtime.GetRunningExeSha256(),
			ProcessStartUnix: timestampUnix(ev.Runtime),
		}
		// Wrapper / unhashable detection from the owner-reported installed_path
		// (mirrors verifier.installedPathIsUpstream — managed binaries live
		// under /usr/lib/globular/bin/). Only mark unhashable when the path is
		// present and clearly upstream; an empty path stays hashable so other
		// assertions drive the verdict.
		if p := ev.Runtime.GetInstalledPath(); p != "" && isUpstreamInstalledPath(p) {
			in.Unhashable = true
		}
	}

	return in
}

// Run is the convenience front-door entry: collect, map, evaluate. It returns
// both the report and the raw evidence (for surfacing collection errors).
func Run(ctx context.Context, f Fetchers, serviceID, nodeID string, opts Options) (release_boundary.Report, *Evidence) {
	ev := Collect(ctx, f, serviceID, nodeID, opts)
	report := release_boundary.Evaluate(MapInputs(serviceID, nodeID, ev))
	return report, ev
}

// resolvedRef returns the publisher-qualified ArtifactRef to verify against:
// the resolved manifest's ref when present (authoritative), otherwise a ref
// built from an explicit publisher. Returns nil when neither is available — the
// caller must not verify without an owner-authoritative reference.
func resolvedRef(m *repositorypb.ArtifactManifest, publisher, name, version, platform string) *repositorypb.ArtifactRef {
	if m != nil && m.GetRef() != nil {
		return m.GetRef()
	}
	if publisher == "" {
		return nil
	}
	return &repositorypb.ArtifactRef{
		PublisherId: publisher,
		Name:        name,
		Version:     version,
		Platform:    platform,
		Kind:        repositorypb.ArtifactKind_SERVICE,
	}
}

// parseInstalledAtUnix reads the per-build install-commit timestamp from the
// install receipt metadata. Absent / empty / malformed / non-positive → 0,
// which the evaluator treats as A4 INDETERMINATE. There is deliberately NO
// fallback to proto InstalledPackage.InstalledUnix (first-install time).
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

// publishStateString maps the PublishState enum to the exact string the
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
// repeated response, matching on service_id first then short name, falling back
// to the sole proof when exactly one is returned.
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
