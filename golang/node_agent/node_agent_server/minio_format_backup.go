package main

// MinIO per-drive format.json backup + restore via etcd.
//
// MinIO writes a small format.json (~200 bytes) under .minio.sys/ on each
// data drive. Without it MinIO refuses to start ("unformatted drive found"),
// and in a distributed pool a single missing format.json on every node leaves
// the cluster unable to reach write quorum — even if the bucket data
// (xl.meta files) is intact on disk. The only documented recovery path is
// `mc admin heal`, which itself needs a quorum of formatted drives to
// bootstrap. Wipes from the approved destructive topology transition path
// (clearMinioSysForModeChange) are intentional; wipes from anything else
// (operator error, filesystem corruption, container ephemeral storage,
// volume re-mount with wrong path) are catastrophic.
//
// This file plugs that gap by snapshotting the per-drive format.json bytes
// into etcd, keyed by (node_id, topology_fingerprint, drive_path). On each
// reconcile cycle the node-agent:
//
//   1. Before starting / verifying MinIO, if format.json is missing on disk
//      AND etcd has a snapshot whose topology_fingerprint matches the
//      current desired state, restore the file. This is idempotent — never
//      overwrites an existing on-disk format.
//   2. After MinIO is confirmed healthy with a valid on-disk format, refresh
//      the etcd snapshot if the bytes have drifted.
//
// The snapshot is invalidated explicitly by clearMinioSysForModeChange when
// a destructive transition wipes .minio.sys — that wipe MUST also wipe the
// matching etcd snapshot so the next reconcile cannot restore the stale
// format and prevent the new topology from initialising.
//
// Per-drive identity stays local: each drive's format.json contains its own
// UUID and pool/set index. The snapshot is a per-drive object too — we
// never expect format.json to be the same across nodes. What's shared is
// the topology_fingerprint, which guards against restoring a snapshot from
// an incompatible topology (e.g. xl-single from before the standalone →
// distributed transition).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
)

const (
	// minioFormatBackupPrefix is the etcd key prefix under which per-drive
	// format.json snapshots are stored. The full key shape is:
	//   /globular/objectstore/format_backup/<node_id>/<drive_basename>
	// e.g. /globular/objectstore/format_backup/eb9a2dac-…/data
	minioFormatBackupPrefix = "/globular/objectstore/format_backup/"

	// minioFormatBackupEtcdTimeout bounds every etcd round-trip. The reconcile
	// loop must not stall on a slow etcd; backup/restore are best-effort.
	minioFormatBackupEtcdTimeout = 3 * time.Second
)

// minioFormatBackup is the JSON envelope persisted in etcd. The format.json
// bytes are stored verbatim; topology metadata lets a restore verify that
// the snapshot is still valid for the current desired state.
type minioFormatBackup struct {
	NodeID              string `json:"node_id"`
	DrivePath           string `json:"drive_path"`           // absolute path, e.g. /mnt/data/data
	TopologyGeneration  int64  `json:"topology_generation"`  // ObjectStoreDesiredState.Generation
	TopologyFingerprint string `json:"topology_fingerprint"` // sha256 of generation|mode|sorted_nodes|drives_per_node|volumes_hash
	FormatJSONBytes     []byte `json:"format_json_bytes"`    // exact bytes of .minio.sys/format.json
	FormatJSONSHA256    string `json:"format_json_sha256"`   // sha256(FormatJSONBytes) for drift detection
	BackedUpAtUnix      int64  `json:"backed_up_at_unix"`
}

// minioDataDirsForState returns the absolute MinIO data directories for this
// node under the given desired state. Mirrors the path math used by
// clearMinioSysForModeChange so the backup key set always covers exactly
// the drives whose format.json a wipe would destroy.
func minioDataDirsForState(state *config.ObjectStoreDesiredState, nodeIP string) []string {
	basePath := "/var/lib/globular/minio"
	if state.NodePaths != nil {
		if p, ok := state.NodePaths[nodeIP]; ok && p != "" {
			basePath = strings.TrimRight(p, "/")
		}
	}
	if state.DrivesPerNode < 2 {
		return []string{filepath.Join(basePath, "data")}
	}
	out := make([]string, 0, state.DrivesPerNode)
	for d := 1; d <= state.DrivesPerNode; d++ {
		out = append(out, filepath.Join(basePath, fmt.Sprintf("data%d", d)))
	}
	return out
}

// minioFormatBackupKey returns the etcd key for a given drive directory.
// drivePath is the data dir (e.g. /mnt/data/data); the key uses the
// basename ("data", "data1", "data2", …) so multi-drive layouts stay
// unambiguous on a single node.
func minioFormatBackupKey(nodeID, drivePath string) string {
	return minioFormatBackupPrefix + nodeID + "/" + filepath.Base(drivePath)
}

// topologyFingerprintForState reuses the same primitive the rest of the
// objectstore reconciler uses to detect topology changes. It exists in
// config but is recomputed here to avoid a circular dependency through
// the rendered_state path which carries extra runtime fields.
func topologyFingerprintForState(state *config.ObjectStoreDesiredState) string {
	if state == nil {
		return ""
	}
	nodes := make([]string, len(state.Nodes))
	copy(nodes, state.Nodes)
	// Sort to match the controller's canonical ordering.
	for i := 1; i < len(nodes); i++ {
		for j := i; j > 0 && nodes[j-1] > nodes[j]; j-- {
			nodes[j-1], nodes[j] = nodes[j], nodes[j-1]
		}
	}
	h := sha256.New()
	fmt.Fprintf(h, "%d|%s|%s|%d|%s",
		state.Generation,
		state.Mode,
		strings.Join(nodes, ","),
		state.DrivesPerNode,
		state.VolumesHash,
	)
	return hex.EncodeToString(h.Sum(nil))
}

// restoreMinioFormatFromEtcd is called BEFORE MinIO is allowed to start
// rendering its files. For every data dir whose format.json is missing on
// disk, if etcd has a snapshot whose topology_fingerprint matches the
// current desired state, write it back to disk.
//
// This is the recovery path that closes the gap. Without it, any cause of
// format.json loss (wipe, container restart with ephemeral fs, accidental
// rm) leaves the entire distributed pool dead until an operator manually
// reformats — losing every object.
//
// Skips silently when:
//   - State is nil, generation is 0, or the topology fingerprint is empty
//     (pre-bootstrap, nothing to compare against).
//   - format.json already exists on disk (never overwrite — MinIO's own
//     bytes are authoritative when present).
//   - etcd has no snapshot for this drive, OR the snapshot is for a
//     different topology (operator transition is in progress, do not
//     fight it).
func (srv *NodeAgentServer) restoreMinioFormatFromEtcd(ctx context.Context, state *config.ObjectStoreDesiredState, nodeIP string) {
	if state == nil || state.Generation == 0 {
		return
	}
	wantFingerprint := topologyFingerprintForState(state)
	if wantFingerprint == "" {
		return
	}

	for _, dir := range minioDataDirsForState(state, nodeIP) {
		formatPath := filepath.Join(dir, ".minio.sys", "format.json")
		if _, err := os.Stat(formatPath); err == nil {
			continue // file exists on disk — leave MinIO's bytes alone
		} else if !os.IsNotExist(err) {
			log.Printf("minio-format-backup: stat %s: %v — skipping restore for this drive", formatPath, err)
			continue
		}

		// Drive dir itself must exist before we drop a file into it.
		// Without this, restore on a node whose disk hasn't been mounted
		// would silently create files under /mnt/data/.
		if _, err := os.Stat(dir); err != nil {
			log.Printf("minio-format-backup: drive dir %s missing (%v) — skipping restore", dir, err)
			continue
		}

		backup, err := srv.loadMinioFormatBackup(ctx, srv.nodeID, dir)
		if err != nil || backup == nil {
			continue
		}
		if backup.TopologyFingerprint != wantFingerprint {
			log.Printf("minio-format-backup: snapshot for %s belongs to topology %s, current is %s — skipping restore (transition in progress)",
				dir, backup.TopologyFingerprint[:8], wantFingerprint[:8])
			continue
		}

		minioSys := filepath.Join(dir, ".minio.sys")
		if err := os.MkdirAll(minioSys, 0o755); err != nil {
			log.Printf("minio-format-backup: mkdir %s: %v — cannot restore format.json", minioSys, err)
			continue
		}
		// Chown to globular:globular so MinIO can read after restore.
		_ = os.Chown(minioSys, minioUID(), minioGID())

		tmp := formatPath + ".restore.tmp"
		if err := os.WriteFile(tmp, backup.FormatJSONBytes, 0o644); err != nil {
			log.Printf("minio-format-backup: write tmp %s: %v", tmp, err)
			continue
		}
		_ = os.Chown(tmp, minioUID(), minioGID())
		if err := os.Rename(tmp, formatPath); err != nil {
			_ = os.Remove(tmp)
			log.Printf("minio-format-backup: rename %s -> %s: %v", tmp, formatPath, err)
			continue
		}
		log.Printf("minio-format-backup: RESTORED %s from etcd snapshot (topology=%s, %d bytes) — MinIO can now bootstrap without operator reformat",
			formatPath, backup.TopologyFingerprint[:8], len(backup.FormatJSONBytes))
	}
}

// backupMinioFormatToEtcd is called AFTER MinIO is verified healthy on this
// node. It reads every drive's on-disk format.json and persists it to etcd
// when the bytes have changed. The healthy precondition prevents us from
// capturing a transient/partial format during a transition.
//
// Skip path: if state is pre-bootstrap or a topology fingerprint cannot be
// computed, do nothing (nothing meaningful to back up). Errors are logged
// but never fatal — the reconcile loop must not block on etcd hiccups.
func (srv *NodeAgentServer) backupMinioFormatToEtcd(ctx context.Context, state *config.ObjectStoreDesiredState, nodeIP string) {
	if state == nil || state.Generation == 0 {
		return
	}
	fingerprint := topologyFingerprintForState(state)
	if fingerprint == "" {
		return
	}

	now := time.Now().Unix()
	for _, dir := range minioDataDirsForState(state, nodeIP) {
		formatPath := filepath.Join(dir, ".minio.sys", "format.json")
		bytes, err := os.ReadFile(formatPath)
		if err != nil {
			// Missing format.json on a healthy MinIO host means the
			// drive isn't actually formatted yet (still healing). Do
			// not back up an absent file — that would erase a valid
			// prior snapshot.
			continue
		}
		sum := sha256.Sum256(bytes)
		sha := hex.EncodeToString(sum[:])

		existing, _ := srv.loadMinioFormatBackup(ctx, srv.nodeID, dir)
		if existing != nil && existing.FormatJSONSHA256 == sha && existing.TopologyFingerprint == fingerprint {
			continue // no drift, no write
		}

		payload := minioFormatBackup{
			NodeID:              srv.nodeID,
			DrivePath:           dir,
			TopologyGeneration:  state.Generation,
			TopologyFingerprint: fingerprint,
			FormatJSONBytes:     bytes,
			FormatJSONSHA256:    sha,
			BackedUpAtUnix:      now,
		}
		if err := srv.saveMinioFormatBackup(ctx, payload); err != nil {
			log.Printf("minio-format-backup: save %s: %v", formatPath, err)
			continue
		}
		log.Printf("minio-format-backup: backed up %s (sha=%s, topology=%s)",
			formatPath, sha[:8], fingerprint[:8])
	}
}

// purgeMinioFormatBackupForNode removes every snapshot for this node.
// Called by clearMinioSysForModeChange so an approved destructive transition
// genuinely starts fresh — otherwise restore would race with the new
// topology's first format and resurrect the old one.
func (srv *NodeAgentServer) purgeMinioFormatBackupForNode(ctx context.Context, state *config.ObjectStoreDesiredState, nodeIP string) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		log.Printf("minio-format-backup: purge etcd client: %v", err)
		return
	}
	tCtx, cancel := context.WithTimeout(ctx, minioFormatBackupEtcdTimeout)
	defer cancel()

	// Walk the drives we'd normally cover so we don't accidentally clobber
	// snapshots that belong to a different node sharing the same prefix.
	for _, dir := range minioDataDirsForState(state, nodeIP) {
		key := minioFormatBackupKey(srv.nodeID, dir)
		if _, err := cli.Delete(tCtx, key); err != nil {
			log.Printf("minio-format-backup: delete %s: %v", key, err)
			continue
		}
		log.Printf("minio-format-backup: purged etcd snapshot for %s — destructive transition acknowledged", dir)
	}
}

func (srv *NodeAgentServer) loadMinioFormatBackup(ctx context.Context, nodeID, drivePath string) (*minioFormatBackup, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, err
	}
	tCtx, cancel := context.WithTimeout(ctx, minioFormatBackupEtcdTimeout)
	defer cancel()

	resp, err := cli.Get(tCtx, minioFormatBackupKey(nodeID, drivePath))
	if err != nil {
		return nil, err
	}
	if resp == nil || len(resp.Kvs) == 0 {
		return nil, nil
	}
	var b minioFormatBackup
	if err := json.Unmarshal(resp.Kvs[0].Value, &b); err != nil {
		return nil, fmt.Errorf("decode etcd backup: %w", err)
	}
	return &b, nil
}

func (srv *NodeAgentServer) saveMinioFormatBackup(ctx context.Context, b minioFormatBackup) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return err
	}
	tCtx, cancel := context.WithTimeout(ctx, minioFormatBackupEtcdTimeout)
	defer cancel()

	payload, err := json.Marshal(b)
	if err != nil {
		return fmt.Errorf("encode backup: %w", err)
	}
	_, err = cli.Put(tCtx, minioFormatBackupKey(b.NodeID, b.DrivePath), string(payload))
	return err
}

// minioUID / minioGID return the uid/gid that owns MinIO's data dirs. The
// globular user is created by every package install (ensure_user_group step
// in every spec) — these are looked up at runtime, with a 0/0 fallback if
// the lookup ever fails (the restored file is still readable; MinIO will
// log a permission warning instead of silently failing).
func minioUID() int {
	u, err := user.Lookup("globular")
	if err != nil {
		return 0
	}
	if id, err := strconv.Atoi(u.Uid); err == nil {
		return id
	}
	return 0
}

func minioGID() int {
	g, err := user.LookupGroup("globular")
	if err != nil {
		return 0
	}
	if id, err := strconv.Atoi(g.Gid); err == nil {
		return id
	}
	return 0
}
