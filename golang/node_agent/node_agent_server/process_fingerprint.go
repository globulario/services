// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.process_fingerprint
// @awareness file_role=binary_sha256_proof_writer_anchored_to_pid_start_time_not_wall_clock
// @awareness implements=globular.platform:intent.node_agent.install_claim_requires_binary_proof
// @awareness risk=critical
package main

// process_fingerprint.go — proves "this PID is running THIS
// binary." InstalledUnix/UpdatedUnix anchor to the PID's
// /proc/<pid> mtime (process start), NEVER wall clock. Anchoring
// to wall clock would let a heartbeat overwrite InstalledUnix on
// every poll cycle and surface as permanent
// service.old_pid_after_upgrade findings — that was INC-2026-0016
// (commit ecdca55c). The PID-start anchor is the only way the
// 4-layer model can tell "this binary is the one the controller
// dispatched" from "this binary was on disk yesterday."
//
// Binary sha256 is the binary-integrity proof; convergence hash
// is the desired-identity hash. The two are NEVER the same value
// and MUST NOT be aliased (see installer_api.go for the
// expected_sha256 side of that boundary).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/installed_state"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// RunningBinary represents a Globular binary that is actually executing.
type RunningBinary struct {
	ServiceName string // canonical service name (e.g. "workflow", "dns")
	BinaryPath  string // absolute path to the binary
	Checksum    string // sha256 of the binary file
	PID         int
}

// checksumCache caches binary checksums keyed by path.
// Entries are invalidated when the file's modification time changes.
var checksumCache = struct {
	sync.Mutex
	entries map[string]cachedChecksum
}{entries: make(map[string]cachedChecksum)}

type cachedChecksum struct {
	checksum string
	modTime  time.Time
	size     int64
}

// DiscoverRunningBinaries scans /proc for processes whose binary lives in
// /usr/lib/globular/bin. This captures both:
//   - services running as the "globular" user
//   - node-agent running as root
//
// Checksums are cached and only recomputed when the binary changes on disk.
// Returns a map of canonical service name → RunningBinary.
func DiscoverRunningBinaries() map[string]RunningBinary {
	result := make(map[string]RunningBinary)

	entries, err := os.ReadDir("/proc")
	if err != nil {
		log.Printf("process-fingerprint: cannot read /proc: %v", err)
		return result
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		exePath, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
		if err != nil {
			continue
		}

		if !strings.HasPrefix(exePath, globularBinDir+"/") {
			continue
		}

		binName := filepath.Base(exePath)
		svcName := binaryToServiceName(binName)
		if svcName == "" {
			continue
		}

		if _, exists := result[svcName]; exists {
			continue
		}

		checksum, err := cachedSha256(exePath)
		if err != nil {
			continue
		}

		result[svcName] = RunningBinary{
			ServiceName: svcName,
			BinaryPath:  exePath,
			Checksum:    checksum,
			PID:         pid,
		}
	}

	return result
}

// cachedSha256 returns the SHA256 of a file, using a cache keyed by
// (path, modTime, size). Only recomputes when the file changes.
func cachedSha256(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	checksumCache.Lock()
	if cached, ok := checksumCache.entries[path]; ok {
		if cached.modTime.Equal(info.ModTime()) && cached.size == info.Size() {
			checksumCache.Unlock()
			return cached.checksum, nil
		}
	}
	checksumCache.Unlock()

	// Cache miss or stale — compute.
	checksum, err := sha256File(path)
	if err != nil {
		return "", err
	}

	checksumCache.Lock()
	checksumCache.entries[path] = cachedChecksum{
		checksum: checksum,
		modTime:  info.ModTime(),
		size:     info.Size(),
	}
	checksumCache.Unlock()

	return checksum, nil
}

// binaryToServiceName converts a binary filename to a canonical service name.
func binaryToServiceName(binName string) string {
	name := strings.TrimSuffix(binName, "_server")
	name = strings.TrimSuffix(name, "_service")
	if name == binName {
		return ""
	}
	name = strings.ReplaceAll(name, "_", "-")
	return name
}

// sha256File computes the SHA256 hash of a file.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// peerChecksumLookup resolves unknown-version services by comparing their
// binary checksum against entrypoint_checksum metadata in other nodes'
// installed_state records. This handles manually-copied binaries (scp)
// without depending on the repository service.
//
// Safe for the heartbeat path: only reads etcd (already in the critical path
// via Phase 2) and computes checksums with caching. No repository calls.
func (srv *NodeAgentServer) peerChecksumLookup(ctx context.Context, versions, buildIDs map[string]string) {
	// Collect services that need resolution.
	var unknowns []string
	for svc, ver := range versions {
		if ver == "unknown" || ver == "" {
			unknowns = append(unknowns, svc)
		}
	}
	if len(unknowns) == 0 {
		return
	}

	// Load all installed packages across all nodes (already cached by etcd client).
	// Build a map: entrypoint_checksum → (version, build_id, service_name).
	type peerInfo struct {
		version string
		buildID string
		nodeID  string
	}
	checksumIndex := make(map[string]peerInfo)

	for _, kind := range []string{"SERVICE", "APPLICATION"} {
		allPkgs, err := installed_state.ListAllNodes(ctx, kind, "")
		if err != nil {
			continue
		}
		for _, pkg := range allPkgs {
			if pkg.GetNodeId() == srv.nodeID {
				continue // skip self
			}
			cksum := pkg.GetMetadata()["entrypoint_checksum"]
			if cksum == "" || pkg.GetVersion() == "" || pkg.GetVersion() == "unknown" {
				continue
			}
			canon := canonicalServiceName(pkg.GetName())
			if canon == "" {
				continue
			}
			// Key by checksum — first match wins.
			if _, exists := checksumIndex[cksum]; !exists {
				checksumIndex[cksum] = peerInfo{
					version: pkg.GetVersion(),
					buildID: pkg.GetBuildId(),
					nodeID:  pkg.GetNodeId(),
				}
			}
		}
	}

	if len(checksumIndex) == 0 {
		return // no peers have entrypoint checksums yet
	}

	// For each unknown service, compute the local binary checksum and look up.
	resolved := 0
	for _, svc := range unknowns {
		binName := strings.ReplaceAll(svc, "-", "_") + "_server"
		binPath := filepath.Join(globularBinDir, binName)

		cksum, err := cachedSha256(binPath)
		if err != nil {
			continue // binary not on disk or unreadable
		}

		peer, found := checksumIndex[cksum]
		if !found {
			continue
		}

		versions[svc] = peer.version
		if peer.buildID != "" {
			buildIDs[svc] = peer.buildID
		}

		// Also write the installed_state record so future heartbeats
		// don't need to repeat the lookup.
		//
		// Non-install writer: preserve install-receipt fields. Without
		// this, every peer-checksum resolution clobbers the canonical
		// install receipt (unit_file_sha256, binary_sha256, installed_by)
		// and the heartbeat's installed_state-first drift authority
		// falls back to legacy_sidecar migration.
		next := &node_agentpb.InstalledPackage{
			NodeId:      srv.nodeID,
			Name:        svc,
			Version:     peer.version,
			Kind:        "SERVICE",
			Status:      "installed",
			UpdatedUnix: time.Now().Unix(),
			BuildId:     peer.buildID,
			Metadata:    map[string]string{"entrypoint_checksum": cksum},
		}
		existing, _ := installed_state.GetInstalledPackage(ctx, srv.nodeID, "SERVICE", svc)
		PreserveInstallReceiptMetadata(existing, next)
		_ = installed_state.WriteInstalledPackage(ctx, next)

		log.Printf("nodeagent: peer-checksum resolved %s → version=%s from node=%s (checksum=%s)",
			svc, peer.version, peer.nodeID, cksum[:16])
		resolved++
	}

	if resolved > 0 {
		log.Printf("nodeagent: peer-checksum resolved %d/%d unknown services", resolved, len(unknowns))
	}
}
