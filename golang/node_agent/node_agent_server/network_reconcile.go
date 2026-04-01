package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
	"google.golang.org/protobuf/types/known/structpb"
)

// deleted: plan-era function removed

func (srv *NodeAgentServer) acmeDNSPreflight(ctx context.Context, spec *cluster_controllerpb.ClusterNetworkSpec) error {
	if spec == nil || !strings.EqualFold(spec.GetProtocol(), "https") || !spec.GetAcmeEnabled() {
		return nil
	}
	if os.Getenv("GLOBULAR_ACME_PUBLIC_DNS_PREFLIGHT") != "1" {
		return nil
	}
	resolver := &net.Resolver{}
	if override := strings.TrimSpace(os.Getenv("GLOBULAR_DNS_RESOLVER")); override != "" {
		dialer := func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{}
			target := override
			if !strings.Contains(target, ":") {
				target = net.JoinHostPort(target, "53")
			}
			return d.DialContext(ctx, "udp", target)
		}
		resolver = &net.Resolver{
			PreferGo: true,
			Dial:     dialer,
		}
	}
	domains := []string{strings.TrimSpace(spec.GetClusterDomain())}
	for _, alt := range spec.GetAlternateDomains() {
		alt = strings.TrimSpace(alt)
		if alt != "" {
			domains = append(domains, alt)
		}
	}
	waitSeconds := 0
	if v := strings.TrimSpace(os.Getenv("ACME_DNS_WAIT_SECONDS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			waitSeconds = n
		}
	}
	deadline := time.Now().Add(time.Duration(waitSeconds) * time.Second)
	missing := []string{}
	for _, d := range domains {
		if d == "" {
			continue
		}
		name := "_acme-challenge." + d
		ok := false
		for {
			txt, err := resolver.LookupTXT(ctx, name)
			if err == nil && len(txt) > 0 {
				ok = true
			}
			if ok || waitSeconds == 0 || time.Now().After(deadline) {
				break
			}
			time.Sleep(time.Second)
		}
		if !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("ACME preflight failed: missing public DNS TXT record(s): %s. Create these _acme-challenge TXT records at your DNS provider and retry.", strings.Join(missing, ", "))
	}
	return nil
}

func (srv *NodeAgentServer) waitForDNSAuthoritative(ctx context.Context, spec *cluster_controllerpb.ClusterNetworkSpec) error {
	if spec == nil || strings.TrimSpace(spec.GetClusterDomain()) == "" {
		return fmt.Errorf("cluster domain required for dns readiness check")
	}
	domain := strings.TrimSpace(spec.GetClusterDomain())
	target := "gateway." + domain
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(c context.Context, network, address string) (net.Conn, error) {
			udpAddr := strings.TrimSpace(os.Getenv("GLOBULAR_DNS_UDP_ADDR"))
			if udpAddr == "" {
				udpAddr = "127.0.0.1:53"
			}
			d := net.Dialer{}
			return d.DialContext(c, "udp", udpAddr)
		},
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		_, err := resolver.LookupHost(ctx, target)
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("dns not authoritative for %s: %w", target, err)
		}
		time.Sleep(time.Second)
	}
}

// deleted: plan-era function removed

func (srv *NodeAgentServer) ensureObjectstoreLayout(ctx context.Context, domain string) error {
	log.Printf("==== ensureObjectstoreLayout CALLED ====")
	log.Printf("  domain passed: %q", domain)

	if strings.TrimSpace(domain) == "" {
		return fmt.Errorf("objectstore layout enforcement requires cluster domain, but none was provided")
	}

	handler := actions.Get("ensure_objectstore_layout")
	if handler == nil {
		log.Printf("  ERROR: ensure_objectstore_layout handler not registered")
		return errors.New("ensure_objectstore_layout handler not registered")
	}

	contractPath := strings.TrimSpace(os.Getenv("GLOBULAR_MINIO_CONTRACT_PATH"))
	envOverride := false
	if contractPath == "" {
		contractPath = strings.TrimSpace(os.Getenv("NODE_AGENT_MINIO_CONTRACT"))
		envOverride = contractPath != ""
	} else {
		envOverride = true
	}
	if contractPath == "" {
		contractPath = "/var/lib/globular/objectstore/minio.json"
	}
	log.Printf("  contract_path: %s (env override: %t)", contractPath, envOverride)

	retry := 30
	if v := strings.TrimSpace(os.Getenv("OBJECTSTORE_RETRY")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			retry = n
		}
	}
	retryDelay := 1000
	if v := strings.TrimSpace(os.Getenv("OBJECTSTORE_RETRY_DELAY_MS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			retryDelay = n
		}
	}

	if cfg := parseContractForLog(contractPath); cfg != nil {
		log.Printf("  minio endpoint=%s bucket=%s secure=%t", cfg.Endpoint, cfg.Bucket, cfg.Secure)
	}

	args, err := buildObjectstoreArgs(contractPath, domain, retry, retryDelay, true)
	if err != nil {
		log.Printf("  ERROR building args: %v", err)
		return fmt.Errorf("build args: %w", err)
	}
	if err := handler.Validate(args); err != nil {
		log.Printf("  ERROR validating: %v", err)
		return fmt.Errorf("validate ensure_objectstore_layout: %w", err)
	}
	msg, err := handler.Apply(ctx, args)
	if err != nil {
		log.Printf("  ERROR applying ensure_objectstore_layout: %v", err)
		return fmt.Errorf("apply ensure_objectstore_layout: %w", err)
	}
	log.Printf("  SUCCESS: %s", msg)
	log.Printf("==== ensureObjectstoreLayout COMPLETED ====")
	return nil
}

func buildObjectstoreArgs(contractPath, domain string, retry int, retryDelayMs int, strict bool) (*structpb.Struct, error) {
	fields := map[string]interface{}{
		"contract_path":    contractPath,
		"domain":           domain,
		"create_sentinels": true,
		"sentinel_name":    ".keep",
		"retry":            int64(retry),
		"retry_delay_ms":   int64(retryDelayMs),
		"strict_contract":  strict,
	}
	return structpb.NewStruct(fields)
}

type minioContractLog struct {
	Endpoint string
	Bucket   string
	Secure   bool
}

func parseContractForLog(path string) *minioContractLog {
	f, err := os.Open(path)
	if err != nil {
		log.Printf("  WARN: cannot read contract %s: %v", path, err)
		return nil
	}
	defer f.Close()
	var cfg config.MinioProxyConfig
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		log.Printf("  WARN: cannot parse contract %s: %v", path, err)
		return nil
	}
	return &minioContractLog{
		Endpoint: cfg.Endpoint,
		Bucket:   cfg.Bucket,
		Secure:   cfg.Secure,
	}
}

func isAllowedRenderTarget(target string) bool {
	if target == "" {
		return false
	}
	if !filepath.IsAbs(target) {
		return false
	}
	clean := filepath.Clean(target)
	if strings.Contains(clean, "..") {
		return false
	}
	allowed := []string{
		"/var/lib/globular/",
		"/run/globular/",
		"/etc/globular/",
		"/etc/systemd/system/",
		"/etc/scylla/",
	}
	for _, prefix := range allowed {
		if clean == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(clean, prefix) {
			return true
		}
	}
	return false
}

func (srv *NodeAgentServer) writeNetworkSpecSnapshot(data string) error {
	if strings.TrimSpace(data) == "" {
		return nil
	}
	path := filepath.Join(config.GetRuntimeConfigDir(), "cluster_network_spec.json")
	return writeAtomicFile(path, []byte(data), 0o600)
}

// deleted: plan-era function removed

// deleted: plan-era function removed

func orderRestartUnits(units []string) []string {
	priority := map[string]int{
		"globular-etcd.service":      1,
		"globular-minio.service":     2,
		"scylladb.service":           3,
		"globular-dns.service":       4,
		"globular-discovery.service": 5,
		"globular-xds.service":       6,
		"globular-envoy.service":     7,
		"globular-gateway.service":   8,
		"globular-storage.service":   9,
	}
	seen := map[string]struct{}{}
	type pair struct {
		unit string
		p    int
	}
	var ordered []pair
	for _, u := range units {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		p := 100
		if v, ok := priority[strings.ToLower(u)]; ok {
			p = v
		}
		ordered = append(ordered, pair{unit: u, p: p})
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].p == ordered[j].p {
			return ordered[i].unit < ordered[j].unit
		}
		return ordered[i].p < ordered[j].p
	})
	out := make([]string, 0, len(ordered))
	for _, p := range ordered {
		out = append(out, p.unit)
	}
	return out
}

func resolveUnits(units []string, exists func(string) bool) []string {
	aliasMap := map[string][]string{
		"globular-envoy.service":     {"envoy.service", "globular-envoy.service"},
		"globular-gateway.service":   {"gateway.service", "globular-gateway.service"},
		"globular-xds.service":       {"xds.service", "globular-xds.service"},
		"globular-etcd.service":      {"etcd.service", "globular-etcd.service"},
		"globular-minio.service":     {"minio.service", "globular-minio.service"},
		"globular-dns.service":       {"dns.service", "globular-dns.service"},
		"globular-discovery.service": {"discovery.service", "globular-discovery.service"},
		"globular-storage.service":   {"storage.service", "globular-storage.service"},
	}
	resolved := []string{}
	seen := map[string]struct{}{}
	for _, u := range units {
		original := strings.TrimSpace(u)
		if original == "" {
			continue
		}
		effective := original
		for canon, aliases := range aliasMap {
			match := strings.EqualFold(canon, original)
			if !match {
				for _, a := range aliases {
					if strings.EqualFold(a, original) {
						match = true
						break
					}
				}
			}
			if match {
				for _, cand := range append([]string{canon}, aliases...) {
					if exists != nil && exists(cand) {
						effective = cand
						break
					}
				}
				break
			}
		}
		if _, ok := seen[effective]; ok {
			continue
		}
		seen[effective] = struct{}{}
		if effective != original {
			log.Printf("nodeagent: resolved unit %s -> %s", original, effective)
		}
		resolved = append(resolved, effective)
	}
	return orderRestartUnits(resolved)
}

// deleted: plan-era function removed

func (srv *NodeAgentServer) applyNetworkOverlay(target, data string) error {
	if strings.TrimSpace(data) == "" {
		return nil
	}
	if err := writeAtomicFile(target, []byte(data), 0o644); err != nil {
		return fmt.Errorf("write network overlay %s: %w", target, err)
	}
	if err := mergeNetworkIntoConfig(config.GetAdminConfigPath(), data); err != nil {
		return fmt.Errorf("merge network overlay: %w", err)
	}
	return nil
}
