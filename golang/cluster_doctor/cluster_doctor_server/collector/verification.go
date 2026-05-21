package collector

// verification.go — Phase 9 wire-up of the Diagnostic Honesty Refactor.
//
// The verifier orchestrator in golang/verifier is pure: it takes targets +
// evidence and returns verdicts. This file is the I/O wrapper that runs
// inside the cluster_doctor collector sweep — read desired state from
// etcd, line up per-node ServiceRuntimeProofs already gathered by
// fetchPerNode, call verifier.VerifyTarget, AggregateResult, write each
// per-(node, service) result to /globular/verification/runtime/<node>/<svc>.

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/verifier"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// fetchDesiredServiceTargets reads ServiceRelease and InfrastructureRelease
// records from etcd and folds them into snap.DesiredServiceTargets, keyed
// by canonical service name. Best-effort: a transient etcd error degrades
// the verification step rather than failing the whole sweep.
//
// ServiceRelease keys:           /globular/resources/ServiceRelease/<name>
// InfrastructureRelease keys:    /globular/resources/InfrastructureRelease/<name>
func (c *Collector) fetchDesiredServiceTargets(ctx context.Context, snap *Snapshot) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		snap.addError("etcd", "GetEtcdClient(verification.targets)", err)
		return
	}

	loadCtx, cancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
	defer cancel()

	// ── ServiceRelease ──
	srResp, err := cli.Get(loadCtx, "/globular/resources/ServiceRelease/", clientv3.WithPrefix())
	if err != nil {
		snap.addError("etcd", "Get(ServiceRelease/*)", err)
	} else {
		for _, kv := range srResp.Kvs {
			var rel cluster_controllerpb.ServiceRelease
			if jerr := json.Unmarshal(kv.Value, &rel); jerr != nil {
				continue
			}
			tgt := serviceReleaseToTarget(&rel, snap.Nodes)
			if tgt != nil {
				snap.DesiredServiceTargets[tgt.Service] = tgt
			}
		}
		snap.addSource("etcd.ServiceRelease/*")
	}

	// ── InfrastructureRelease ──
	irResp, err := cli.Get(loadCtx, "/globular/resources/InfrastructureRelease/", clientv3.WithPrefix())
	if err != nil {
		snap.addError("etcd", "Get(InfrastructureRelease/*)", err)
	} else {
		for _, kv := range irResp.Kvs {
			var rel cluster_controllerpb.InfrastructureRelease
			if jerr := json.Unmarshal(kv.Value, &rel); jerr != nil {
				continue
			}
			tgt := infraReleaseToTarget(&rel, snap.Nodes)
			if tgt != nil {
				snap.DesiredServiceTargets[tgt.Service] = tgt
			}
		}
		snap.addSource("etcd.InfrastructureRelease/*")
	}
}

// serviceReleaseToTarget maps a ServiceRelease object into a target.
// Returns nil for releases that have no resolved version (PENDING/WAITING).
func serviceReleaseToTarget(rel *cluster_controllerpb.ServiceRelease, nodes []*cluster_controllerpb.NodeRecord) *DesiredServiceTarget {
	if rel == nil || rel.Spec == nil || rel.Status == nil {
		return nil
	}
	if strings.TrimSpace(rel.Status.ResolvedVersion) == "" {
		return nil // not yet resolved; nothing to verify
	}
	required := requiredNodesFromStatus(rel.Status.Nodes, nodes)
	return &DesiredServiceTarget{
		Service:        strings.TrimSpace(rel.Spec.ServiceName),
		PublisherID:    rel.Spec.PublisherID,
		DesiredVersion: rel.Status.ResolvedVersion,
		DesiredBuildID: rel.Status.ResolvedBuildID,
		DesiredHash:    rel.Status.ResolvedArtifactDigest,
		RuntimeNeeded:  true, // ServiceRelease implies a running systemd unit
		RequiredNodes:  required,
		ApplyTime:      lastTransitionFromStatus(rel.Status.LastTransitionUnixMs),
	}
}

// infraReleaseToTarget maps an InfrastructureRelease object into a target.
// Same shape as serviceReleaseToTarget — the verifier treats both kinds
// identically; the difference is only at apply time.
func infraReleaseToTarget(rel *cluster_controllerpb.InfrastructureRelease, nodes []*cluster_controllerpb.NodeRecord) *DesiredServiceTarget {
	if rel == nil || rel.Spec == nil || rel.Status == nil {
		return nil
	}
	if strings.TrimSpace(rel.Status.ResolvedVersion) == "" {
		return nil
	}
	required := requiredNodesFromStatus(rel.Status.Nodes, nodes)
	return &DesiredServiceTarget{
		Service:        strings.TrimSpace(rel.Spec.Component),
		PublisherID:    rel.Spec.PublisherID,
		DesiredVersion: rel.Status.ResolvedVersion,
		DesiredBuildID: rel.Status.ResolvedBuildID,
		DesiredHash:    rel.Status.ResolvedArtifactDigest,
		// COMMAND-kind components have no systemd unit; the verifier itself
		// derives this distinction from the proof's effective-state fields.
		// We keep RuntimeNeeded true here and let absent-unit data degrade
		// the verdict to runtime_identity_unproven rather than mis-classify.
		RuntimeNeeded: true,
		RequiredNodes: required,
		ApplyTime:     lastTransitionFromStatus(rel.Status.LastTransitionUnixMs),
	}
}

// requiredNodesFromStatus extracts the node-id list from the per-node
// status array on a release. Falls back to all known nodes when the
// release has no per-node entries yet (pre-resolve / synthetic release).
func requiredNodesFromStatus(statusNodes []*cluster_controllerpb.NodeReleaseStatus, allNodes []*cluster_controllerpb.NodeRecord) []string {
	if len(statusNodes) > 0 {
		out := make([]string, 0, len(statusNodes))
		for _, n := range statusNodes {
			if id := strings.TrimSpace(n.NodeID); id != "" {
				out = append(out, id)
			}
		}
		return out
	}
	out := make([]string, 0, len(allNodes))
	for _, n := range allNodes {
		if id := strings.TrimSpace(n.GetNodeId()); id != "" {
			out = append(out, id)
		}
	}
	return out
}

func lastTransitionFromStatus(unixMs int64) time.Time {
	if unixMs <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(unixMs)
}

// runVerification is the Phase 9 orchestrator. For every desired target
// it locates the matching per-node proof, calls verifier.VerifyTarget,
// aggregates the result, and persists the per-(node, service) verdict
// under the brief's canonical etcd prefix.
//
// Errors here are logged + folded into snap.DataErrors but never abort
// the rest of the sweep — the doctor's other invariants stay actionable
// even if the verification step is partial.
func (c *Collector) runVerification(ctx context.Context, snap *Snapshot) {
	if len(snap.DesiredServiceTargets) == 0 {
		// Nothing to verify yet (pre-bootstrap, or all releases still
		// WAITING/PENDING). Don't synthesize a result.
		return
	}

	now := time.Now()
	var verdicts []verifier.Verdict

	for _, dst := range snap.DesiredServiceTargets {
		for _, nodeID := range dst.RequiredNodes {
			tgt := verifier.Target{
				Service:        dst.Service,
				NodeID:         nodeID,
				DesiredVersion: dst.DesiredVersion,
				DesiredBuildID: dst.DesiredBuildID,
				DesiredHash:    dst.DesiredHash,
				RuntimeNeeded:  dst.RuntimeNeeded,
				ApplyTime:      dst.ApplyTime,
			}
			ev := verifier.Evidence{
				Proof: findProofForService(snap.RuntimeProofs[nodeID], dst.Service),
				// RenderedUnit is intentionally empty: the controller does
				// not yet expose its rendered units through an RPC the
				// doctor can scrape. Phase 5b's effective-config drift
				// sub-check is skipped when RenderedUnit is empty; it
				// remains live for callers that DO have the rendered text.
			}
			verdicts = append(verdicts, verifier.VerifyTarget(tgt, ev, now))
		}
	}

	r := verifier.AggregateResult(verdicts, nil, nil, now)
	snap.VerifierResult = &r
	snap.addSource("verifier.AggregateResult")

	c.persistVerificationResults(ctx, snap, &r)
}

// findProofForService scans a per-node proofs slice for the matching
// service. Names are canonicalised on both sides so a publisher prefix
// in the desired service name doesn't drop the match. Returns nil when
// no proof is present.
func findProofForService(proofs []*node_agentpb.ServiceRuntimeProof, service string) *node_agentpb.ServiceRuntimeProof {
	want := canonForLookup(service)
	for _, p := range proofs {
		if p == nil {
			continue
		}
		if canonForLookup(p.GetServiceName()) == want {
			return p
		}
		if canonForLookup(p.GetServiceId()) == want {
			return p
		}
	}
	return nil
}

func canonForLookup(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	return strings.ToLower(s)
}

// persistVerificationResults writes one etcd key per per-target verdict
// at /globular/verification/runtime/<node>/<service>. Best-effort; an
// etcd write failure for one target doesn't block the others. Each
// value is the JSON-encoded Verdict for cross-process consumption
// (cluster_controller, ad-hoc CLI, future verifier daemon).
func (c *Collector) persistVerificationResults(ctx context.Context, snap *Snapshot, r *verifier.Result) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		snap.addError("etcd", "GetEtcdClient(verification.write)", err)
		return
	}
	writeCtx, cancel := context.WithTimeout(ctx, c.cfg.ListTimeout)
	defer cancel()

	for _, v := range r.Verdicts {
		key := verifier.EtcdKeyForVerification(v.Target.NodeID, v.Target.Service)
		payload, err := json.Marshal(v)
		if err != nil {
			log.Printf("verification: marshal verdict for %s/%s: %v",
				v.Target.NodeID, v.Target.Service, err)
			continue
		}
		if _, err := cli.Put(writeCtx, key, string(payload)); err != nil {
			snap.addError("etcd", "Put("+key+")", err)
		}
	}
}
