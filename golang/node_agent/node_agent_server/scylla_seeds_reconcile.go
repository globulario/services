// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.scylla_seeds_reconcile
// @awareness file_role=node_local_scylla_seed_reconciler_against_etcd_published_ring_membership
//
//globular:enforces objectstore.minio.config_render_source_must_be_etcd
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/node_agent/node_agent_server/infra_truth"
)

// scyllaSeedsLineRe matches the single SimpleSeedProvider seeds line inside
// scylla.yaml, capturing everything up to and including "seeds:" so the value
// can be replaced without disturbing indentation or any other key.
//
//	    parameters:
//	      - seeds: '10.0.0.63,10.0.0.8'
var scyllaSeedsLineRe = regexp.MustCompile(`(?m)^([ \t]*-[ \t]*seeds:[ \t]*).*$`)

// reconcileScyllaSeeds keeps /etc/scylla/scylla.yaml's seed list equal to the
// ring membership published in etcd at /globular/cluster/scylla/hosts.
//
// WHY THIS EXISTS. The seed list is DERIVED state — a pure function of ring
// membership — but the only writer of scylla.yaml is the scylladb package
// post-install script, which runs once at install time and never again. The
// founding node therefore keeps a self-only seed list forever, and every later
// node freezes whatever membership existed at its own install moment. The
// controller's renderer table also claims this path, but srv.dispatchPlan is a
// no-op (the ApplyPlan RPC was removed), so that render never lands.
//
// WHY IT NEVER RESTARTS SCYLLA. Seeds are read only at process start; a live
// node tracks ring membership through gossip and Raft. Rewriting the file
// corrects what a FUTURE start would discover, which is exactly the risk —
// a wiped restart with a self-only seed list re-bootstraps an isolated ring.
// Restarting to "apply" a start-time-only field would convert a benign
// correction into an availability event on the data tier. This mirrors
// reconcileMinioSystemdConfig, which renders from etcd and never restarts
// MinIO because restart is workflow-coordinated.
//
// The reconciler is a no-op on nodes without scylla.yaml, and it refuses to act
// when etcd cannot supply the authoritative list rather than guessing a seed
// set from local observation.
func (srv *NodeAgentServer) reconcileScyllaSeeds(ctx context.Context) {
	raw, err := os.ReadFile(infra_truth.ScyllaConfigPath)
	if err != nil {
		return // ScyllaDB is not installed on this node — nothing to reconcile.
	}

	// etcd is the sole authority for ring membership. LoadClusterHostList
	// already rejects loopback entries, so a degraded list never reaches here.
	hosts, err := config.LoadClusterHostList(config.EtcdKeyClusterScyllaHosts)
	if err != nil || len(hosts) == 0 {
		return // Authority unavailable — leave the file alone rather than guess.
	}
	desired := strings.Join(hosts, ",")

	loc := scyllaSeedsLineRe.FindSubmatchIndex(raw)
	if loc == nil {
		log.Printf("scylla-seeds: %s has no seed_provider seeds line — leaving it untouched",
			infra_truth.ScyllaConfigPath)
		return
	}
	prefixEnd := loc[3] // end of capture group 1 ("      - seeds: ")
	current := strings.Trim(strings.TrimSpace(string(raw[prefixEnd:loc[1]])), `'"`)
	if current == desired {
		return
	}

	out := make([]byte, 0, len(raw)+len(desired))
	out = append(out, raw[:prefixEnd]...)
	out = append(out, '\'')
	out = append(out, desired...)
	out = append(out, '\'')
	out = append(out, raw[loc[1]:]...)

	if err := writeFileAtomic(infra_truth.ScyllaConfigPath, out, 0o644); err != nil {
		log.Printf("scylla-seeds: failed to update %s: %v", infra_truth.ScyllaConfigPath, err)
		return
	}
	log.Printf("scylla-seeds: seed list converged in %s: %q -> %q (no restart — seeds are read at process start)",
		infra_truth.ScyllaConfigPath, current, desired)
}

// writeFileAtomic writes data to path via a same-directory temp file and a
// rename, so a crash mid-write can never leave scylla.yaml truncated.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write %s: %w", tmpPath, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmpPath, err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("chmod %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmpPath, path, err)
	}
	committed = true
	return nil
}
