// @awareness namespace=globular.platform
// @awareness component=platform_config
// @awareness file_role=cluster_dns_config
// @awareness risk=medium
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// clusterDNSSuffix is the suffix we resolve via the Globular cluster DNS
// daemon. Everything outside this suffix is delegated to the system resolver
// (so external names still work through /etc/resolv.conf).
const clusterDNSSuffix = ".globular.internal"

var (
	clusterResolverOnce sync.Once
	clusterResolver     *net.Resolver
)

// ClusterResolver returns a net.Resolver that resolves *.globular.internal
// via the Globular DNS daemons whose IPs are stored in etcd at
// /globular/cluster/dns/hosts (Tier-0: DNS can't itself be resolved via DNS).
// Falls back to the local node's routable IP if etcd is unreachable. Falls
// back to the system resolver for every non-cluster name.
//
// Services use this resolver to reach cluster endpoints (minio.globular.internal,
// controller.globular.internal, etc.) without needing /etc/resolv.conf to be
// reconfigured — host-level DNS stays unchanged.
func ClusterResolver() *net.Resolver {
	clusterResolverOnce.Do(func() {
		var dnsServers []string
		if hosts, err := GetDNSHosts(); err == nil && len(hosts) > 0 {
			for _, h := range hosts {
				dnsServers = append(dnsServers, h+":53")
			}
		}
		if len(dnsServers) == 0 {
			// Last-resort fallback: the local node likely runs DNS.
			dnsServers = []string{GetRoutableIPv4() + ":53"}
		}
		var idx int
		var idxMu sync.Mutex
		clusterResolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				// Round-robin across DNS servers so load is spread and a
				// single down DNS node doesn't block resolution.
				idxMu.Lock()
				target := dnsServers[idx%len(dnsServers)]
				idx++
				idxMu.Unlock()
				d := net.Dialer{Timeout: 2 * time.Second}
				return d.DialContext(ctx, "udp", target)
			},
		}
	})
	return clusterResolver
}

// ClusterDialContext is a net.Dialer DialContext function that resolves
// cluster names (*.globular.internal) via Globular DNS and dials them.
// All other hostnames fall through to the system resolver.
//
// When the cluster DNS service is unavailable, ClusterDialContext falls back to
// the etcd-backed DNS records map (/globular/cluster/dns/records) written by the
// DNS reconciler, then to Tier-0 service-specific host lists. This allows MinIO,
// Scylla, and other cluster services to remain reachable even during a DNS outage.
//
// Use this as http.Transport.DialContext to wire cluster-aware resolution
// into any Go HTTP client (including the MinIO SDK).
func ClusterDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		// Not host:port — let the default dialer handle it.
		host = addr
		port = ""
	}
	lowered := strings.ToLower(strings.TrimSuffix(host, "."))
	if strings.HasSuffix(lowered, clusterDNSSuffix) || lowered == strings.TrimPrefix(clusterDNSSuffix, ".") {
		// For tier-0 services, prefer direct etcd-backed resolution first to avoid
		// DNS self-dependency during recovery.
		var ips []string
		if isTier0ClusterServiceHost(host) {
			if tier0, tErr := resolveClusterNameFromEtcd(host); tErr == nil && len(tier0) > 0 {
				ips = tier0
			}
		}
		if len(ips) == 0 {
			// Resolve via cluster DNS.
			ips, err = ClusterResolver().LookupHost(ctx, host)
			if err != nil {
				// DNS service is unreachable — fall back to etcd-backed records.
				log.Printf("cluster-dial: DNS lookup failed for %q (%v); trying etcd fallback", host, err)
				ips, err = resolveClusterNameFromEtcd(host)
				if err != nil {
					return nil, fmt.Errorf("cluster-dial: DNS and etcd fallback both failed for %q: %w", host, err)
				}
			}
		}
		var lastErr error
		d := net.Dialer{Timeout: 5 * time.Second}
		for _, ip := range ips {
			target := ip
			if port != "" {
				target = net.JoinHostPort(ip, port)
			}
			conn, derr := d.DialContext(ctx, network, target)
			if derr == nil {
				return conn, nil
			}
			lastErr = derr
		}
		if lastErr != nil {
			return nil, lastErr
		}
	}
	// Fall through to system resolver for non-cluster names.
	d := net.Dialer{Timeout: 5 * time.Second}
	return d.DialContext(ctx, network, addr)
}

func isTier0ClusterServiceHost(host string) bool {
	normalized := strings.ToLower(strings.TrimSuffix(strings.TrimSuffix(host, "."), clusterDNSSuffix))
	switch normalized {
	case "minio", "scylla", "dns":
		return true
	default:
		return false
	}
}

// resolveClusterNameFromEtcd is the DNS fallback when the cluster DNS service is
// unavailable. It tries two sources in order:
//  1. /globular/cluster/dns/records — a hostname→IP map written by the DNS reconciler
//  2. Tier-0 service-specific host lists (minio/scylla/dns) from individual etcd keys
//
// Invariant: recovery.must_not_depend_on_dns_only
func resolveClusterNameFromEtcd(host string) ([]string, error) {
	// 1. Try the full DNS records map (mirrors what the DNS service would return).
	if ips, err := lookupDNSRecordFromEtcd(host); err == nil && len(ips) > 0 {
		return ips, nil
	}

	// 2. Tier-0 host lists for known service prefixes.
	normalized := strings.ToLower(strings.TrimSuffix(strings.TrimSuffix(host, "."), clusterDNSSuffix))
	switch normalized {
	case "minio":
		return GetMinioHosts()
	case "scylla":
		return GetScyllaHosts()
	case "dns":
		return GetDNSHosts()
	}
	return nil, fmt.Errorf("no etcd fallback for %q", host)
}

// lookupDNSRecordFromEtcd reads /globular/cluster/dns/records from etcd (a
// hostname→IP map written by the cluster controller's DNS reconciler) and returns
// the single IP for host. This is the same data the DNS service serves but stored
// directly in etcd, so it remains queryable even when the DNS daemon is down.
func lookupDNSRecordFromEtcd(host string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cli, err := GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("dns records: etcd unavailable: %w", err)
	}
	resp, err := cli.Get(ctx, "/globular/cluster/dns/records")
	if err != nil || len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("dns records: not found in etcd")
	}
	var records map[string]string
	if err := json.Unmarshal(resp.Kvs[0].Value, &records); err != nil {
		return nil, fmt.Errorf("dns records: parse error: %w", err)
	}
	normalized := strings.ToLower(strings.TrimSuffix(host, "."))
	if ip, ok := records[normalized]; ok {
		return []string{ip}, nil
	}
	return nil, fmt.Errorf("dns records: no entry for %q", host)
}
