// globular-upgrader is a static helper binary that upgrades services which
// cannot safely upgrade themselves (e.g. node-agent). It is deployed once
// with the initial install and never upgraded through the pipeline.
//
// The caller (node-agent apply handler) has already installed the new binary
// via InstallPackage. The upgrader's job is:
//  1. Restart the systemd unit (which picks up the new binary)
//  2. Wait for the service to become active
//  3. Write installed-state to etcd (the convergence truth boundary)
//
// If restart or health check fails, installed-state is written as "failed".
//
// Usage:
//
//	globular-upgrader \
//	  --unit globular-node-agent.service \
//	  --node-id <id> \
//	  --name node-agent \
//	  --version 0.0.8 \
//	  --build 2 \
//	  --kind SERVICE \
//	  --platform linux_amd64 \
//	  --operation-id <op> \
//	  --checksum <sha256>
package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func main() {
	var (
		unit        = flag.String("unit", "", "Systemd unit name (e.g. globular-node-agent.service)")
		nodeID      = flag.String("node-id", "", "Node ID")
		name        = flag.String("name", "", "Package name (e.g. node-agent)")
		version     = flag.String("version", "", "Target version")
		build       = flag.Int64("build", 0, "Build number")
		kind        = flag.String("kind", "SERVICE", "Package kind (SERVICE, INFRASTRUCTURE, COMMAND)")
		platform    = flag.String("platform", "", "Platform (e.g. linux_amd64)")
		operationID = flag.String("operation-id", "", "Operation ID for tracing")
		checksum    = flag.String("checksum", "", "Artifact SHA256 checksum")
		buildIDFlag = flag.String("build-id", "", "Phase 2: exact artifact identity (UUIDv7)")
	)
	flag.Parse()

	if *unit == "" || *nodeID == "" || *name == "" || *version == "" {
		flag.Usage()
		os.Exit(1)
	}

	log.SetPrefix("globular-upgrader: ")
	log.SetFlags(log.Ltime)

	log.Printf("starting self-upgrade: %s → %s@%s (build %d, build_id=%s)", *unit, *name, *version, *build, *buildIDFlag)

	// Step 1: Restart the service. The new binary is already installed by
	// the caller (InstallPackage ran before LaunchUpgrader).
	log.Printf("restarting %s", *unit)
	if err := systemctl("restart", *unit); err != nil {
		errMsg := fmt.Sprintf("restart %s failed: %v", *unit, err)
		log.Printf("ERROR: %s", errMsg)
		writeInstalledState(*unit, *nodeID, *name, *version, *kind, *platform, *operationID, *checksum, *buildIDFlag, *build, "failed", errMsg)
		os.Exit(1)
	}

	// Step 2: Wait for active (up to 30s).
	log.Printf("waiting for %s to become active", *unit)
	if err := waitActive(*unit, 30*time.Second); err != nil {
		errMsg := fmt.Sprintf("%s did not become active: %v", *unit, err)
		log.Printf("ERROR: %s", errMsg)
		writeInstalledState(*unit, *nodeID, *name, *version, *kind, *platform, *operationID, *checksum, *buildIDFlag, *build, "failed", errMsg)
		os.Exit(1)
	}

	// Step 3: Write installed-state to etcd. This is the convergence truth
	// boundary — only reached after the service is confirmed running.
	log.Printf("%s is active — writing installed-state", *unit)
	writeInstalledState(*unit, *nodeID, *name, *version, *kind, *platform, *operationID, *checksum, *buildIDFlag, *build, "installed", "")

	log.Printf("self-upgrade complete: %s@%s (build %d) running and verified", *name, *version, *build)
}

func systemctl(action, unit string) error {
	cmd := exec.Command("systemctl", action, unit)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func waitActive(unit string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		err := exec.Command("systemctl", "is-active", "--quiet", unit).Run()
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout after %s", timeout)
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		return err
	}
}

// writeInstalledState writes the installed package record directly to etcd.
// This is the ONLY place where self-update success is recorded — matching
// the convergence truth invariant.
//
// The record MUST be receipt-complete. The canonical install path stamps
// installed_state.metadata.unit_file_sha256 (see node_agent internal/
// installreceipt) so the doctor can compare the on-disk systemd unit against
// the recorded expectation. Because node-agent cannot restart itself, its
// unit receipt is only ever written here — so this writer must stamp it too.
// Omitting it (the pre-fix behaviour) left node-agent permanently reporting
// unit_file_drift, "healed by re-install" that never runs any path but this
// one. We recompute the unit hash from the just-restarted on-disk unit
// (ground truth) using the identical raw-file sha256 as installreceipt and
// checkUnitHashDrift (meta.identity_computation_must_be_invariant), and
// read-merge any existing receipt metadata so proof fields stamped by the
// original install survive (meta.write_creates_completion_obligation).
func writeInstalledState(unit, nodeID, name, version, kind, platform, operationID, checksum, buildID string, buildNumber int64, status, errMsg string) {
	etcdKey := fmt.Sprintf("/globular/nodes/%s/packages/%s/%s", nodeID, kind, name)

	cli, err := newEtcdClient()
	if err != nil {
		log.Printf("WARNING: etcd connect failed: %v (installed-state NOT written)", err)
		return
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Read-merge: preserve receipt metadata the canonical install stamped
	// (binary_sha256, entrypoint_checksum, proof_*, …) rather than clobbering
	// it with a truncated record.
	metadata := mergeReceiptMetadata(readExistingMetadata(ctx, cli, etcdKey), unit, status, errMsg)

	now := time.Now().Unix()
	pkg := map[string]interface{}{
		"nodeId":        nodeID,
		"name":          name,
		"version":       version,
		"kind":          kind,
		"status":        status,
		"updatedUnix":   fmt.Sprintf("%d", now),
		"installedUnix": fmt.Sprintf("%d", now),
		"buildNumber":   fmt.Sprintf("%d", buildNumber),
	}
	if platform != "" {
		pkg["platform"] = platform
	}
	if operationID != "" {
		pkg["operationId"] = operationID
	}
	if checksum != "" {
		pkg["checksum"] = checksum
	}
	if buildID != "" {
		pkg["buildId"] = buildID
	}

	if len(metadata) > 0 {
		pkg["metadata"] = metadata
	}

	data, err := json.Marshal(pkg)
	if err != nil {
		log.Printf("WARNING: marshal installed-state failed: %v", err)
		return
	}

	if _, err := cli.Put(ctx, etcdKey, string(data)); err != nil {
		log.Printf("WARNING: etcd put %s failed: %v", etcdKey, err)
		return
	}
	log.Printf("installed-state written: %s = %s (unit receipt: %v)", etcdKey, status, metadata[unitFileSha256Key] != nil)
}

// Receipt metadata keys — must match node_agent/.../installreceipt constants
// (KeyUnitFileSha256, KeyUnitFilePath). The doctor's unit_receipt_drift rule
// reads these literal keys; keep them aligned.
const (
	unitFileSha256Key = "unit_file_sha256"
	unitFilePathKey   = "unit_file_path"
)

// systemdUnitDir is the directory holding rendered systemd unit files. A var
// (not a const) so tests can point it at a temp dir.
var systemdUnitDir = "/etc/systemd/system"

// mergeReceiptMetadata computes the installed-state receipt metadata for a
// self-upgrade write, starting from the existing (preserved) metadata.
//
//   - status=="installed": the on-disk unit is the new version's unit, so the
//     receipt is recomputed to match reality and any stale error is cleared.
//   - otherwise: existing metadata is preserved and the failure reason recorded
//     under "error" (the receipt is NOT invalidated by a failed restart).
func mergeReceiptMetadata(metadata map[string]interface{}, unit, status, errMsg string) map[string]interface{} {
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	if status == "installed" {
		delete(metadata, "error")
		if unitPath, unitSha := hashUnitFile(unit); unitSha != "" {
			metadata[unitFileSha256Key] = unitSha
			metadata[unitFilePathKey] = unitPath
		} else {
			log.Printf("WARNING: could not hash unit %s — installed-state will lack unit receipt", unit)
		}
	} else if errMsg != "" {
		metadata["error"] = errMsg
	}
	return metadata
}

// hashUnitFile returns the on-disk systemd unit path and its lowercase-hex
// sha256, computed over the raw file bytes — the identical computation used by
// installreceipt.Stamp and node-agent's checkUnitHashDrift so the produced and
// verified hashes cannot diverge (meta.identity_computation_must_be_invariant).
// Returns ("", "") if the unit name is empty or the file cannot be read.
func hashUnitFile(unit string) (path, sha string) {
	if strings.TrimSpace(unit) == "" {
		return "", ""
	}
	path = filepath.Join(systemdUnitDir, unit)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	sum := sha256.Sum256(data)
	return path, hex.EncodeToString(sum[:])
}

// readExistingMetadata returns the current record's metadata map (string keys/
// values) so it can be preserved across the self-upgrade write. Returns an
// empty, non-nil map when the key is absent or has no metadata.
func readExistingMetadata(ctx context.Context, cli *clientv3.Client, etcdKey string) map[string]interface{} {
	out := map[string]interface{}{}
	resp, err := cli.Get(ctx, etcdKey)
	if err != nil || len(resp.Kvs) == 0 {
		return out
	}
	var existing map[string]interface{}
	if err := json.Unmarshal(resp.Kvs[0].Value, &existing); err != nil {
		return out
	}
	if md, ok := existing["metadata"].(map[string]interface{}); ok {
		for k, v := range md {
			out[k] = v
		}
	}
	return out
}

// etcdEndpointsFile is the cluster-rendered list of etcd member URLs.
// Written by the controller during reconciliation. One URL per line.
const etcdEndpointsFile = "/var/lib/globular/config/etcd_endpoints"

func newEtcdClient() (*clientv3.Client, error) {
	// Read endpoints from the cluster config file (same source the node-agent uses).
	endpoints := readEndpointsFile(etcdEndpointsFile)
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("etcd endpoints not configured: %s is missing or empty — cannot write installed-state without cluster connection", etcdEndpointsFile)
	}

	certFile := "/var/lib/globular/pki/issued/services/service.crt"
	keyFile := "/var/lib/globular/pki/issued/services/service.key"
	caFile := "/var/lib/globular/pki/ca.crt"

	cfg := clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	}

	if _, err := os.Stat(caFile); err == nil {
		tlsCfg := &tls.Config{}
		caPEM, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read CA: %w", err)
		}
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(caPEM)
		tlsCfg.RootCAs = pool

		if _, err := os.Stat(certFile); err == nil {
			cert, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				return nil, fmt.Errorf("load client cert: %w", err)
			}
			tlsCfg.Certificates = []tls.Certificate{cert}
		}
		cfg.TLS = tlsCfg
	}

	return clientv3.New(cfg)
}

// readEndpointsFile reads a newline-separated list of URLs from a file.
func readEndpointsFile(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var eps []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			eps = append(eps, line)
		}
	}
	return eps
}
