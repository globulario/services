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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
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
		writeInstalledState(*nodeID, *name, *version, *kind, *platform, *operationID, *checksum, *buildIDFlag, *build, "failed", errMsg)
		os.Exit(1)
	}

	// Step 2: Wait for active (up to 30s).
	log.Printf("waiting for %s to become active", *unit)
	if err := waitActive(*unit, 30*time.Second); err != nil {
		errMsg := fmt.Sprintf("%s did not become active: %v", *unit, err)
		log.Printf("ERROR: %s", errMsg)
		writeInstalledState(*nodeID, *name, *version, *kind, *platform, *operationID, *checksum, *buildIDFlag, *build, "failed", errMsg)
		os.Exit(1)
	}

	// Step 3: Write installed-state to etcd. This is the convergence truth
	// boundary — only reached after the service is confirmed running.
	log.Printf("%s is active — writing installed-state", *unit)
	writeInstalledState(*nodeID, *name, *version, *kind, *platform, *operationID, *checksum, *buildIDFlag, *build, "installed", "")

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
func writeInstalledState(nodeID, name, version, kind, platform, operationID, checksum, buildID string, buildNumber int64, status, errMsg string) {
	etcdKey := fmt.Sprintf("/globular/nodes/%s/packages/%s/%s", nodeID, kind, name)

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
	if errMsg != "" {
		pkg["metadata"] = map[string]string{"error": errMsg}
	}

	data, err := json.Marshal(pkg)
	if err != nil {
		log.Printf("WARNING: marshal installed-state failed: %v", err)
		return
	}

	cli, err := newEtcdClient()
	if err != nil {
		log.Printf("WARNING: etcd connect failed: %v (installed-state NOT written)", err)
		return
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := cli.Put(ctx, etcdKey, string(data)); err != nil {
		log.Printf("WARNING: etcd put %s failed: %v", etcdKey, err)
		return
	}
	log.Printf("installed-state written: %s = %s", etcdKey, status)
}

// etcdEndpointsFile is the cluster-rendered list of etcd member URLs.
// Written by the controller during reconciliation. One URL per line.
const etcdEndpointsFile = "/var/lib/globular/config/etcd_endpoints"

func newEtcdClient() (*clientv3.Client, error) {
	// Read endpoints from the cluster config file (same source the node-agent uses).
	endpoints := readEndpointsFile(etcdEndpointsFile)
	if len(endpoints) == 0 {
		// Fallback to localhost (works on control-plane nodes where etcd runs locally).
		endpoints = []string{"https://localhost:2379"}
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
