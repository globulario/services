// Package installed_state provides read/write/query operations for the
// canonical installed-package registry stored in etcd.
//
// etcd key schema:
//
//	/globular/nodes/{node_id}/packages/{kind}/{name}
//
// Values are protojson-encoded node_agent.InstalledPackage records.
//
// This package is used by:
//   - Cluster Controller: authoritative writer via CommitInstalledPackage
//   - Node Agent: reads state and emits convergence evidence (no direct authoritative writes)
//   - Gateway: reads records for admin UI queries
// @awareness namespace=globular.platform
// @awareness component=platform_installed_state
// @awareness file_role=installed_state_authority
// @awareness enforces=globular.platform:invariant.state.installed_not_catalog
// @awareness risk=critical
package installed_state

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/globulario/services/golang/binhash"
	"github.com/globulario/services/golang/config"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	// keyPrefix is the etcd key prefix for installed packages.
	keyPrefix = "/globular/nodes/"

	// defaultTimeout for etcd operations.
	defaultTimeout = 5 * time.Second
)

// packageKey returns the etcd key for an installed package.
// Format: /globular/nodes/{node_id}/packages/{kind}/{name}
func packageKey(nodeID, kind, name string) string {
	return keyPrefix + nodeID + "/packages/" + strings.ToUpper(kind) + "/" + name
}

// nodePackagesPrefix returns the etcd prefix for all packages on a node.
func nodePackagesPrefix(nodeID string) string {
	return keyPrefix + nodeID + "/packages/"
}

// nodeKindPrefix returns the etcd prefix for packages of a given kind on a node.
func nodeKindPrefix(nodeID, kind string) string {
	return keyPrefix + nodeID + "/packages/" + strings.ToUpper(kind) + "/"
}

// CommitInstalledPackage is the controller-side counterpart to WriteInstalledPackage.
// It writes (or overwrites) the authoritative installed-package record using
// StateCommitWrite (30 s timeout, 6 retries, jittered backoff). Call this from
// the convergence committer after reading a ConvergenceResultV1; never call it
// from the node-agent.
func CommitInstalledPackage(ctx context.Context, pkg *node_agentpb.InstalledPackage) error {
	if pkg.GetNodeId() == "" {
		return fmt.Errorf("installed_state: node_id is required")
	}
	if pkg.GetName() == "" {
		return fmt.Errorf("installed_state: name is required")
	}
	if pkg.GetKind() == "" {
		return fmt.Errorf("installed_state: kind is required")
	}
	if pkg.UpdatedUnix == 0 {
		// WARNING: wall-clock anchoring is the INC-2026-0016 bug class. Callers
		// should set UpdatedUnix from /proc/<pid> mtime, not time.Now().
		log.Printf("installed_state: WARNING CommitInstalledPackage defaulting UpdatedUnix to time.Now() for %s/%s/%s — caller should set from /proc/<pid> mtime", pkg.GetNodeId(), pkg.GetKind(), pkg.GetName())
		pkg.UpdatedUnix = time.Now().Unix()
	}
	if pkg.InstalledUnix == 0 {
		pkg.InstalledUnix = pkg.UpdatedUnix
	}
	if pkg.Status == "" {
		pkg.Status = "installed"
	}
	data, err := protojson.Marshal(pkg)
	if err != nil {
		return fmt.Errorf("installed_state: marshal: %w", err)
	}
	key := packageKey(pkg.GetNodeId(), pkg.GetKind(), pkg.GetName())
	if err := config.PutRuntimeWithClass(ctx, key, data, config.StateCommitWrite); err != nil {
		return err
	}
	cleanupStaleKindsByDiskTruth(ctx, pkg, "commit")
	return nil
}

// WriteInstalledPackage writes an installed-package record from the node-agent
// heartbeat path. It uses NormalRuntimeWrite (4 s timeout, 2 retries) which
// is appropriate for best-effort heartbeat reporting.
//
// Callers MUST read the existing record first and skip this call when the
// existing record has build_id set — that indicates a controller-committed
// record written via CommitInstalledPackage, which must not be overwritten.
// Phase 1 of syncInstalledStateToEtcd enforces this contract.
func WriteInstalledPackage(ctx context.Context, pkg *node_agentpb.InstalledPackage) error {
	if pkg.GetNodeId() == "" {
		return fmt.Errorf("installed_state: node_id is required")
	}
	if pkg.GetName() == "" {
		return fmt.Errorf("installed_state: name is required")
	}
	if pkg.GetKind() == "" {
		return fmt.Errorf("installed_state: kind is required")
	}
	if pkg.UpdatedUnix == 0 {
		// WARNING: wall-clock anchoring is the INC-2026-0016 bug class. Callers
		// should set UpdatedUnix from /proc/<pid> mtime, not time.Now().
		log.Printf("installed_state: WARNING WriteInstalledPackage defaulting UpdatedUnix to time.Now() for %s/%s/%s — caller should set from /proc/<pid> mtime", pkg.GetNodeId(), pkg.GetKind(), pkg.GetName())
		pkg.UpdatedUnix = time.Now().Unix()
	}
	if pkg.InstalledUnix == 0 {
		pkg.InstalledUnix = pkg.UpdatedUnix
	}
	if pkg.Status == "" {
		pkg.Status = "installed"
	}
	data, err := protojson.Marshal(pkg)
	if err != nil {
		return fmt.Errorf("installed_state: marshal: %w", err)
	}
	key := packageKey(pkg.GetNodeId(), pkg.GetKind(), pkg.GetName())
	if err := config.PutRuntimeWithClass(ctx, key, data, config.NormalRuntimeWrite); err != nil {
		return err
	}
	cleanupStaleKindsByDiskTruth(ctx, pkg, "write")
	return nil
}

// cleanupStaleKindsByDiskTruth uses the actual on-disk binary as the
// arbiter for which installed_state records are real and which are
// ghosts. After a successful write, scan every record for the same
// (nodeID, name) under any kind and delete those whose stored
// `metadata.entrypoint_checksum` does not match the live sha256 of
// the binary on disk.
//
// Why disk-truth and not kind-trust:
//
// Naive "delete every record under a different kind" obeys the writer's
// kind claim blindly. If a less-authoritative writer (e.g. a bare
// heartbeat with no proof) writes the SAME name under a different kind
// it would obediently delete the authoritative record that has full
// cryptographic proof under the older kind — exactly the kind of
// barer-record-kills-better-record failure caught live 2026-06-03.
// Disk truth is the only neutral arbiter that does not require us to
// trust the writer.
//
// Records with no proof (`entrypoint_checksum` empty) are not deleted —
// they may be legacy records that pre-date the proof writer. They are
// also not authoritative enough to delete records that have proof. The
// new write itself is held to the same rule: if WE have no proof, we do
// not initiate cleanup at all.
//
// Errors are logged but never fail the parent write.
func cleanupStaleKindsByDiskTruth(ctx context.Context, pkg *node_agentpb.InstalledPackage, source string) {
	nodeID := pkg.GetNodeId()
	name := pkg.GetName()
	keepKind := strings.ToUpper(pkg.GetKind())
	if nodeID == "" || name == "" || keepKind == "" {
		return
	}
	newProof := strings.TrimSpace(pkg.GetMetadata()["entrypoint_checksum"])
	if newProof == "" {
		return
	}
	entryPath := binhash.ResolveServiceBinaryPath(name, pkg.GetMetadata()["proof_binary_path"])
	if entryPath == "" {
		return
	}
	onDisk := binhash.HashOrEmpty(entryPath)
	if onDisk == "" {
		return
	}
	pkgs, err := ListInstalledPackages(ctx, nodeID, "")
	if err != nil {
		log.Printf("installed_state: cleanup-stale(%s) list failed for %s/%s: %v", source, nodeID, name, err)
		return
	}
	for _, victim := range staleSiblingKinds(pkgs, keepKind, name, onDisk) {
		if delErr := DeleteInstalledPackage(ctx, nodeID, victim, name); delErr != nil {
			log.Printf("installed_state: cleanup-stale(%s) delete %s/%s/%s failed: %v",
				source, nodeID, victim, name, delErr)
			continue
		}
		log.Printf("installed_state: cleanup-stale(%s) deleted ghost %s/%s/%s — its entry_chk did not match disk %s at %s (kept %s/%s)",
			source, nodeID, victim, name, binhash.Short(onDisk), entryPath, keepKind, name)
	}
}

// staleSiblingKinds is the pure decision half of cleanupStaleKindsByDiskTruth:
// given (a) the full list of records on a node, (b) the kind we just wrote,
// (c) the package name, and (d) the current binary's sha256 on disk,
// return the kinds (other than keepKind) under which a record exists for
// the same name AND its stored `metadata.entrypoint_checksum` is non-empty
// AND it does NOT match the disk sha256.
//
// Records with no proof are NOT returned — they cannot be safely classified
// as stale. Records under the keep kind are NEVER returned — the parent
// write is for that kind. Same-disk records under other kinds are kept
// (legitimate duplicates with matching proof are preserved, leaving the
// human/operator to consolidate).
func staleSiblingKinds(pkgs []*node_agentpb.InstalledPackage, keepKind, name, diskSha256 string) []string {
	if name == "" || keepKind == "" || diskSha256 == "" {
		return nil
	}
	keep := strings.ToUpper(keepKind)
	disk := binhash.Normalize(diskSha256)
	var out []string
	seen := map[string]bool{}
	for _, pkg := range pkgs {
		if pkg.GetName() != name {
			continue
		}
		k := strings.ToUpper(pkg.GetKind())
		if k == "" || k == keep || seen[k] {
			continue
		}
		stored := binhash.Normalize(pkg.GetMetadata()["entrypoint_checksum"])
		if stored == "" {
			continue
		}
		if stored == disk {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out
}

// GetInstalledPackage reads a single installed package record from etcd.
func GetInstalledPackage(ctx context.Context, nodeID, kind, name string) (*node_agentpb.InstalledPackage, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("installed_state: etcd client: %w", err)
	}
	// Do NOT close the shared singleton.

	key := packageKey(nodeID, kind, name)
	tctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	resp, err := cli.Get(tctx, key)
	if err != nil {
		return nil, fmt.Errorf("installed_state: get %q: %w", key, err)
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	return unmarshalPackage(resp.Kvs[0].Value)
}

// ListInstalledPackages returns all installed packages on a node, optionally filtered by kind.
func ListInstalledPackages(ctx context.Context, nodeID, kind string) ([]*node_agentpb.InstalledPackage, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("installed_state: etcd client: %w", err)
	}
	// Do NOT close the shared singleton.

	prefix := nodePackagesPrefix(nodeID)
	if kind != "" {
		prefix = nodeKindPrefix(nodeID, kind)
	}

	tctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	resp, err := cli.Get(tctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("installed_state: list %q: %w", prefix, err)
	}

	pkgs := make([]*node_agentpb.InstalledPackage, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		pkg, err := unmarshalPackage(kv.Value)
		if err != nil {
			log.Printf("installed_state: WARNING ListInstalledPackages corrupt record at key %q: %v", string(kv.Key), err)
			continue
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs, nil
}

// DeleteInstalledPackage removes an installed package record from etcd.
func DeleteInstalledPackage(ctx context.Context, nodeID, kind, name string) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("installed_state: etcd client: %w", err)
	}
	// Do NOT close the shared singleton.

	key := packageKey(nodeID, kind, name)
	tctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	_, err = cli.Delete(tctx, key)
	if err != nil {
		return fmt.Errorf("installed_state: delete %q: %w", key, err)
	}
	return nil
}

// ListAllNodes returns installed packages across all nodes, optionally filtered
// by kind and/or package name. Useful for gateway admin endpoints.
func ListAllNodes(ctx context.Context, kind, name string) ([]*node_agentpb.InstalledPackage, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("installed_state: etcd client: %w", err)
	}
	// Do NOT close the shared singleton.

	prefix := keyPrefix
	tctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	resp, err := cli.Get(tctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("installed_state: list all: %w", err)
	}

	kindUpper := strings.ToUpper(kind)
	nameLower := strings.ToLower(strings.TrimSpace(name))
	pkgs := make([]*node_agentpb.InstalledPackage, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		pkg, err := unmarshalPackage(kv.Value)
		if err != nil {
			log.Printf("installed_state: WARNING ListAllNodes corrupt record at key %q: %v", string(kv.Key), err)
			continue
		}
		if kind != "" && strings.ToUpper(pkg.GetKind()) != kindUpper {
			continue
		}
		if nameLower != "" && strings.ToLower(pkg.GetName()) != nameLower {
			continue
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs, nil
}

// DeleteNodePackages removes all installed-package keys for a given node ID.
// Used to clean up stale entries after a restore where the node ID changed.
func DeleteNodePackages(ctx context.Context, nodeID string) (int64, error) {
	if nodeID == "" {
		return 0, fmt.Errorf("installed_state: node_id is required")
	}
	cli, err := config.GetEtcdClient()
	if err != nil {
		return 0, fmt.Errorf("installed_state: etcd client: %w", err)
	}
	prefix := nodePackagesPrefix(nodeID)
	tctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	resp, err := cli.Delete(tctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return 0, fmt.Errorf("installed_state: delete %q: %w", prefix, err)
	}
	return resp.Deleted, nil
}

// ListNodeIDs returns all distinct node IDs that have installed-package keys.
func ListNodeIDs(ctx context.Context) ([]string, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("installed_state: etcd client: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	resp, err := cli.Get(tctx, keyPrefix, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return nil, fmt.Errorf("installed_state: list keys: %w", err)
	}
	seen := make(map[string]bool)
	for _, kv := range resp.Kvs {
		// Key format: /globular/nodes/{node_id}/packages/...
		key := string(kv.Key)
		rest := strings.TrimPrefix(key, keyPrefix)
		if idx := strings.Index(rest, "/"); idx > 0 {
			seen[rest[:idx]] = true
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	return ids, nil
}

func unmarshalPackage(data []byte) (*node_agentpb.InstalledPackage, error) {
	pkg := &node_agentpb.InstalledPackage{}
	if err := protojson.Unmarshal(data, pkg); err != nil {
		return nil, fmt.Errorf("installed_state: unmarshal: %w", err)
	}
	return pkg, nil
}
