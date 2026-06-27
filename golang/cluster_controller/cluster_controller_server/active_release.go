package main

// active_release.go — the cluster ACTIVE platform release pointer.
//
// Per docs/design/platform-release-pointer-advance.md (decided 2026-06-27):
// etcd OWNS the active release pointer (/globular/platform/active_release);
// /var/lib/globular/release-index.json becomes a node-local materialized
// projection (a later slice). This file is SLICE 1: the etcd anchor + an
// explicit, convergence-VERIFIED operation to advance it.
//
// The controller writes ONLY the etcd anchor here — no node-local files, no
// os/exec — which keeps it within the controller boundary
// (controller.decides_but_does_not_execute_leaf_work). The convergence gate
// honors controller.platform_upgrade_must_gate_per_node_per_package: the pointer
// means "the cluster is operating against this release", never "we hope it gets
// there".

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/versionutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// activeReleaseAnchorKey is the etcd key holding the authoritative active
// platform release pointer for the cluster.
const activeReleaseAnchorKey = "/globular/platform/active_release"

// activeReleaseAnchor is the JSON document stored at activeReleaseAnchorKey.
type activeReleaseAnchor struct {
	ReleaseTag      string   `json:"release_tag"`
	PlatformRelease string   `json:"platform_release"`
	UpdatedAtUnix   int64    `json:"updated_at_unix"`
	UpdatedBy       string   `json:"updated_by"`
	VerifiedNodes   []string `json:"verified_nodes,omitempty"`
}

// ReadActiveReleaseAnchor returns the current active-release anchor, or
// (nil, nil) when no anchor has been written yet.
func (srv *server) ReadActiveReleaseAnchor(ctx context.Context) (*activeReleaseAnchor, error) {
	if srv.etcdClient == nil {
		return nil, fmt.Errorf("etcd client unavailable")
	}
	resp, err := srv.etcdClient.Get(ctx, activeReleaseAnchorKey)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	var a activeReleaseAnchor
	if err := json.Unmarshal(resp.Kvs[0].Value, &a); err != nil {
		return nil, fmt.Errorf("corrupt active_release anchor: %w", err)
	}
	return &a, nil
}

// WriteActiveReleaseAnchor persists the active-release anchor to etcd.
func (srv *server) WriteActiveReleaseAnchor(ctx context.Context, a *activeReleaseAnchor) error {
	if srv.etcdClient == nil {
		return fmt.Errorf("etcd client unavailable")
	}
	body, err := json.Marshal(a)
	if err != nil {
		return err
	}
	// Classified critical write — the active-release anchor is platform truth;
	// it must go through the governed write seam (config.CriticalWrite), never a
	// raw etcd Put. Honors meta.state_mutations_must_be_durably_committed_*.
	if err := config.PutRuntimeWithClass(ctx, activeReleaseAnchorKey, body, config.CriticalWrite); err != nil {
		return err
	}
	return nil
}

// platformReleaseFromTag derives the platform_release value ("1.2.250") from a
// release tag ("v1.2.250"). The leading v/V is the only normalization — the
// rest of the tag is preserved verbatim so native tags survive.
func platformReleaseFromTag(tag string) string {
	t := strings.TrimSpace(tag)
	t = strings.TrimPrefix(t, "v")
	t = strings.TrimPrefix(t, "V")
	return t
}

// activationLaggard is a (node, package) pair that is installed on the node but
// not at the BOM version — i.e. the node has not converged to the candidate
// release for that package.
type activationLaggard struct {
	NodeID     string
	Package    string
	Installed  string
	BOMVersion string
}

// evaluateActivationReadiness is the pure convergence gate. It returns every
// (node, package) pair that is INSTALLED on a node but does not match the BOM
// version for that package. Packages not installed on a node are skipped
// (operator removals are preserved, mirroring platform-upgrade evaluate);
// packages installed on a node but absent from the BOM are ignored. An empty
// result means the cluster has converged to the BOM. Output is deterministic.
func evaluateActivationReadiness(nodes []NodeView, bom []BOMPackage) []activationLaggard {
	bomVer := make(map[string]string, len(bom))
	for _, p := range bom {
		bomVer[p.Name] = p.Version
	}

	sortedNodes := append([]NodeView(nil), nodes...)
	sort.Slice(sortedNodes, func(i, j int) bool { return sortedNodes[i].NodeID < sortedNodes[j].NodeID })

	var laggards []activationLaggard
	for _, n := range sortedNodes {
		names := make([]string, 0, len(n.InstalledVersions))
		for name := range n.InstalledVersions {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			want, inBOM := bomVer[name]
			if !inBOM {
				continue // not part of the candidate release
			}
			installed := strings.TrimSpace(n.InstalledVersions[name])
			if installed == "" {
				continue // operator removal / not installed — preserved
			}
			// versionutil.Equal is native-version safe (canonicalizes real
			// SemVer, exact-matches native tags like ffmpeg n8.x / minio
			// RELEASE.x), so a non-SemVer package never produces a false laggard.
			if !versionutil.Equal(installed, want) {
				laggards = append(laggards, activationLaggard{
					NodeID:     n.NodeID,
					Package:    name,
					Installed:  installed,
					BOMVersion: want,
				})
			}
		}
	}
	return laggards
}

func summarizeLaggards(l []activationLaggard) string {
	const max = 8
	parts := make([]string, 0, len(l))
	for i, g := range l {
		if i >= max {
			parts = append(parts, fmt.Sprintf("… +%d more", len(l)-max))
			break
		}
		parts = append(parts, fmt.Sprintf("%s/%s installed=%s want=%s", g.NodeID, g.Package, g.Installed, g.BOMVersion))
	}
	return strings.Join(parts, "; ")
}

// Activation actions returned by decideActivation.
const (
	activationNoop     = "noop"
	activationRefuse   = "refuse"
	activationActivate = "activate"
)

// decideActivation is the pure activation policy — idempotency, no-regression,
// and the convergence gate — with no I/O. `converged` is len(laggards)==0,
// computed by the caller. Returns the action and, for a refusal, a human reason.
//
//   - idempotent: candidate already active → noop
//   - no-regression: candidate platform_release older than current → refuse
//     (unless allowRegression); native/non-orderable tags require an explicit
//     override rather than a guess
//   - convergence: not converged → refuse (unless force)
func decideActivation(cur *activeReleaseAnchor, candidateTag, candidatePlatform string, converged, allowRegression, force bool) (action, reason string) {
	if cur != nil && cur.ReleaseTag == candidateTag {
		return activationNoop, ""
	}
	if cur != nil && !allowRegression {
		cmp, err := versionutil.Compare(candidatePlatform, cur.PlatformRelease)
		switch {
		case err != nil:
			if !versionutil.Equal(candidatePlatform, cur.PlatformRelease) {
				return activationRefuse, fmt.Sprintf(
					"platform_release %q cannot be ordered against the current active %q (non-SemVer); pass --allow-regression to override (audited)",
					candidatePlatform, cur.PlatformRelease)
			}
		case cmp < 0:
			return activationRefuse, fmt.Sprintf(
				"platform_release %s is older than the current active %s; pass --allow-regression to override (audited)",
				candidatePlatform, cur.PlatformRelease)
		}
	}
	if !converged && !force {
		return activationRefuse, "one or more node/package(s) have not converged to the BOM — converge first or pass --force"
	}
	return activationActivate, ""
}

// ActivatePlatformRelease advances the cluster's active platform release pointer
// (etcd) to req.ReleaseTag, after a synchronous convergence gate. Leader-gated;
// idempotent; no-regression guarded. See the proto doc + design doc.
func (srv *server) ActivatePlatformRelease(ctx context.Context, req *cluster_controllerpb.ActivatePlatformReleaseRequest) (*cluster_controllerpb.ActivatePlatformReleaseResponse, error) {
	if !srv.isLeader() {
		resp := &cluster_controllerpb.ActivatePlatformReleaseResponse{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/ActivatePlatformRelease", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}

	tag := strings.TrimSpace(req.GetReleaseTag())
	if tag == "" {
		return nil, status.Error(codes.InvalidArgument, "release_tag is required")
	}
	platformRelease := platformReleaseFromTag(tag)

	cur, err := srv.ReadActiveReleaseAnchor(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "read active_release anchor: %v", err)
	}
	prev := ""
	if cur != nil {
		prev = cur.ReleaseTag
	}

	// Convergence gate: every installed package that is part of the candidate
	// BOM must already run the BOM version on every node.
	bom, _, err := srv.fetchLocalRepositoryBOM(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "fetch local repository BOM: %v", err)
	}
	nodes := srv.snapshotNodesForUpgrade()
	laggards := evaluateActivationReadiness(nodes, bom)
	converged := len(laggards) == 0

	switch action, reason := decideActivation(cur, tag, platformRelease, converged, req.GetAllowRegression(), req.GetForce()); action {
	case activationNoop:
		var verified []string
		if cur != nil {
			verified = cur.VerifiedNodes
		}
		return &cluster_controllerpb.ActivatePlatformReleaseResponse{
			Ok:            true,
			PreviousTag:   prev,
			VerifiedNodes: verified,
			Message:       fmt.Sprintf("platform release %s is already active — no-op", tag),
		}, nil
	case activationRefuse:
		if !converged && !req.GetForce() {
			return nil, status.Errorf(codes.FailedPrecondition,
				"refusing to activate %s: %s: %s", tag, reason, summarizeLaggards(laggards))
		}
		return nil, status.Errorf(codes.FailedPrecondition, "refusing to activate %s: %s", tag, reason)
	}

	verified := make([]string, 0, len(nodes))
	for _, n := range nodes {
		verified = append(verified, n.NodeID)
	}
	sort.Strings(verified)

	anchor := &activeReleaseAnchor{
		ReleaseTag:      tag,
		PlatformRelease: platformRelease,
		UpdatedAtUnix:   time.Now().UTC().Unix(),
		UpdatedBy:       "cluster-controller",
		VerifiedNodes:   verified,
	}
	if err := srv.WriteActiveReleaseAnchor(ctx, anchor); err != nil {
		return nil, status.Errorf(codes.Unavailable, "write active_release anchor: %v", err)
	}

	log.Printf("platform.activate_release: active platform release advanced %q -> %q (verified_nodes=%d force=%v allow_regression=%v laggards=%d)",
		prev, tag, len(verified), req.GetForce(), req.GetAllowRegression(), len(laggards))

	msg := fmt.Sprintf("active platform release set to %s (%d node(s) verified)", tag, len(verified))
	if len(laggards) > 0 {
		msg += fmt.Sprintf("; FORCED past %d non-converged node/package(s)", len(laggards))
	}
	return &cluster_controllerpb.ActivatePlatformReleaseResponse{
		Ok:            true,
		Message:       msg,
		VerifiedNodes: verified,
		PreviousTag:   prev,
	}, nil
}
