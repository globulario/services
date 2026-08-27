// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.etcd_initial_cluster_reconcile
// @awareness file_role=node_local_etcd_initial_cluster_reconciler_against_the_live_ring
//
//globular:enforces objectstore.minio.config_render_source_must_be_etcd
package main

import (
	"context"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/globulario/services/golang/config"
)

const etcdRenderedConfigPath = "/var/lib/globular/config/etcd.yaml"

var (
	etcdInitialClusterLineRe = regexp.MustCompile(`(?m)^([ \t]*initial-cluster:[ \t]*).*$`)
	etcdClusterStateLineRe   = regexp.MustCompile(`(?m)^([ \t]*initial-cluster-state:[ \t]*).*$`)
)

// reconcileEtcdInitialCluster keeps /var/lib/globular/config/etcd.yaml's
// initial-cluster equal to the LIVE etcd ring, and promotes
// initial-cluster-state to "existing" once this node is one member among many.
//
// WHY THIS EXISTS. initial-cluster is derived state — a pure function of ring
// membership — but nothing rewrites it after the file is first laid down. The
// founding node keeps the Day-0 single-node seed forever, and every later node
// freezes whatever membership existed at its own join moment. Observed on a
// fresh 1.2.344 cluster: node-1 carried initial-cluster with only itself while
// the live ring had five members, and node-3 carried three of the five.
//
// WHY IT NEVER RESTARTS ETCD. Both fields are read only when a member boots
// with an empty data-dir; a running member ignores them entirely and tracks
// membership through raft. Rewriting the file corrects what a FUTURE start
// would do, which is precisely where the danger is: the founding node's
// self-only list combined with initial-cluster-state "new" means a wiped
// restart would silently BOOTSTRAP A SECOND CLUSTER instead of rejoining the
// existing one. Promoting the state to "existing" turns that silent split-brain
// into an honest refusal to start until the member is re-added.
//
// Authority is the live ring, never the node registry — a registry can list a
// node that raft has not accepted. Members that have been added but have not
// yet started carry an empty Name; including one would render a malformed
// initial-cluster entry, so they are skipped and the whole update is deferred
// until every peer is nameable.
func (srv *NodeAgentServer) reconcileEtcdInitialCluster(ctx context.Context) {
	raw, err := os.ReadFile(etcdRenderedConfigPath)
	if err != nil {
		return // etcd is not configured on this node — nothing to reconcile.
	}

	cli, err := config.GetEtcdClient()
	if err != nil {
		return // Authority unreachable — leave the file alone rather than guess.
	}
	resp, err := cli.MemberList(ctx)
	if err != nil || resp == nil {
		return
	}

	entries := make([]string, 0, len(resp.Members))
	for _, m := range resp.Members {
		name := strings.TrimSpace(m.GetName())
		peers := m.GetPeerURLs()
		if name == "" || len(peers) == 0 {
			// An added-but-unstarted member. Defer the whole update rather than
			// emit a partial ring that a future bootstrap would trust.
			return
		}
		entries = append(entries, name+"="+strings.TrimSpace(peers[0]))
	}
	// A single-member ring is the legitimate Day-0 shape; leave it, including
	// its "new" state, so a genuine first bootstrap still works.
	if len(entries) < 2 {
		return
	}
	sort.Strings(entries)
	desired := strings.Join(entries, ",")

	out, changedCluster := replaceCapturedValue(raw, etcdInitialClusterLineRe, desired, false)
	out, changedState := replaceCapturedValue(out, etcdClusterStateLineRe, "existing", true)
	if !changedCluster && !changedState {
		return
	}

	if err := writeFileAtomic(etcdRenderedConfigPath, out, 0o644); err != nil {
		log.Printf("etcd-initial-cluster: failed to update %s: %v", etcdRenderedConfigPath, err)
		return
	}
	log.Printf("etcd-initial-cluster: converged %s from the live ring (%d members, state=existing, no restart — read only at bootstrap)",
		etcdRenderedConfigPath, len(entries))
}

// replaceCapturedValue rewrites the value after a matched key prefix, quoting it
// when quote is set. Returns the new bytes and whether anything actually changed,
// so callers can stay silent and write nothing on a no-op pass.
func replaceCapturedValue(raw []byte, re *regexp.Regexp, value string, quote bool) ([]byte, bool) {
	loc := re.FindSubmatchIndex(raw)
	if loc == nil {
		return raw, false
	}
	prefixEnd := loc[3]
	current := strings.Trim(strings.TrimSpace(string(raw[prefixEnd:loc[1]])), `'"`)
	if current == value {
		return raw, false
	}
	rendered := value
	if quote {
		rendered = `"` + value + `"`
	}
	out := make([]byte, 0, len(raw)+len(rendered))
	out = append(out, raw[:prefixEnd]...)
	out = append(out, rendered...)
	out = append(out, raw[loc[1]:]...)
	return out, true
}
