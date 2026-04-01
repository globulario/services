// globular-upgrader is a static helper binary that upgrades services which
// cannot safely upgrade themselves (e.g. node-agent). It is deployed once
// with the initial install and never upgraded through the pipeline.
//
// Usage:
//
//	globular-upgrader \
//	  --node-id <id> \
//	  --plan-id <id> \
//	  --plan-generation <gen> \
//	  --artifact <path> \
//	  --service <name> \
//	  --version <ver> \
//	  --unit <systemd-unit> \
//	  --etcd-endpoints <endpoints> \
//	  --etcd-cert <path> --etcd-key <path> --etcd-ca <path>
//
// Steps:
//  1. Stop the target systemd unit
//  2. Extract the artifact (tar.gz) into the install paths
//  3. Write a version marker
//  4. Write PLAN_SUCCEEDED to etcd
//  5. Start the target systemd unit
package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
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
		nodeID         = flag.String("node-id", "", "Node ID")
		planID         = flag.String("plan-id", "", "Plan ID to report status for")
		planGeneration = flag.Uint64("plan-generation", 0, "Plan generation")
		artifactPath   = flag.String("artifact", "", "Path to artifact tar.gz")
		service        = flag.String("service", "", "Service name (e.g. node-agent)")
		version        = flag.String("version", "", "Target version")
		unit           = flag.String("unit", "", "Systemd unit name (e.g. globular-node-agent.service)")
		etcdEndpoints  = flag.String("etcd-endpoints", "https://localhost:2379", "Comma-separated etcd endpoints")
		etcdCert       = flag.String("etcd-cert", "", "Client TLS cert for etcd")
		etcdKey        = flag.String("etcd-key", "", "Client TLS key for etcd")
		etcdCA         = flag.String("etcd-ca", "", "CA cert for etcd")
	)
	flag.Parse()

	if *nodeID == "" || *planID == "" || *artifactPath == "" || *service == "" || *unit == "" {
		flag.Usage()
		os.Exit(1)
	}

	log.SetPrefix("globular-upgrader: ")
	log.SetFlags(log.Ltime)

	// Step 1: Stop the service.
	log.Printf("stopping %s", *unit)
	if err := systemctl("stop", *unit); err != nil {
		fail(*etcdEndpoints, *etcdCert, *etcdKey, *etcdCA, *nodeID, *planID, *planGeneration,
			fmt.Sprintf("stop %s: %v", *unit, err))
	}

	// Step 2: Extract artifact.
	log.Printf("installing artifact %s for %s", *artifactPath, *service)
	if err := installArtifact(*artifactPath, *service); err != nil {
		fail(*etcdEndpoints, *etcdCert, *etcdKey, *etcdCA, *nodeID, *planID, *planGeneration,
			fmt.Sprintf("install artifact: %v", err))
	}

	// Step 3: Write version marker.
	// Must match versionutil.MarkerPath: /var/lib/globular/services/{service}/version
	if *version != "" {
		markerPath := fmt.Sprintf("/var/lib/globular/services/%s/version", *service)
		if err := os.MkdirAll(filepath.Dir(markerPath), 0o755); err == nil {
			_ = os.WriteFile(markerPath, []byte(*version+"\n"), 0o644)
		}
	}

	// Step 4: Write PLAN_SUCCEEDED to etcd.
	log.Printf("reporting plan %s succeeded", *planID)
	if err := reportSuccess(*etcdEndpoints, *etcdCert, *etcdKey, *etcdCA, *nodeID, *planID, *planGeneration); err != nil {
		log.Printf("WARNING: failed to write plan status: %v (service will still be started)", err)
	}

	// Step 5: Start the service.
	log.Printf("starting %s", *unit)
	if err := systemctl("start", *unit); err != nil {
		log.Printf("ERROR: failed to start %s: %v", *unit, err)
		os.Exit(1)
	}

	log.Printf("upgrade complete: %s → %s", *service, *version)
}

func systemctl(action, unit string) error {
	cmd := exec.Command("systemctl", action, unit)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func installArtifact(artifactPath, service string) error {
	f, err := os.Open(artifactPath)
	if err != nil {
		return fmt.Errorf("open artifact: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	binDir := "/usr/lib/globular/bin"
	systemdDir := "/etc/systemd/system"
	configDir := "/var/lib/globular"

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		name := strings.TrimLeft(hdr.Name, "./")
		var dest string
		switch {
		case strings.HasPrefix(name, "bin/"):
			dest = filepath.Join(binDir, filepath.Base(name))
		case strings.HasPrefix(name, "systemd/"), strings.HasPrefix(name, "units/"):
			dest = filepath.Join(systemdDir, filepath.Base(name))
		case strings.HasPrefix(name, "config/"):
			dest = filepath.Join(configDir, service, strings.TrimPrefix(name, "config/"))
		default:
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(dest), err)
		}
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, hdr.FileInfo().Mode())
		if err != nil {
			return fmt.Errorf("create %s: %w", dest, err)
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return fmt.Errorf("write %s: %w", dest, err)
		}
		out.Close()
	}
	return nil
}

func etcdClient(endpoints, certFile, keyFile, caFile string) (*clientv3.Client, error) {
	tlsCfg := &tls.Config{}
	if caFile != "" {
		caPEM, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read CA: %w", err)
		}
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(caPEM)
		tlsCfg.RootCAs = pool
	}
	if certFile != "" && keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("load client cert: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	return clientv3.New(clientv3.Config{
		Endpoints:   strings.Split(endpoints, ","),
		TLS:         tlsCfg,
		DialTimeout: 5 * time.Second,
	})
}

// writeStatus deleted — plan system removed.
func writeStatus(endpoints, certFile, keyFile, caFile, nodeID, planID string, generation uint64, state int, errMsg string) error { return nil }

func reportSuccess(endpoints, certFile, keyFile, caFile, nodeID, planID string, generation uint64) error {
	return writeStatus(endpoints, certFile, keyFile, caFile, nodeID, planID, generation, 0, "")
}

func fail(endpoints, certFile, keyFile, caFile, nodeID, planID string, generation uint64, errMsg string) {
	log.Printf("FAILED: %s", errMsg)
	_ = writeStatus(endpoints, certFile, keyFile, caFile, nodeID, planID, generation, 1, errMsg)
	os.Exit(1)
}
