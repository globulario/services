package config

import (
	"context"
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
// Use this as http.Transport.DialContext to wire cluster-aware resolution
// into any Go HTTP client (including the MinIO SDK).
func ClusterDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		// Not host:port — let the default dialer handle it.
		host = addr
		port = ""
	}
	lowered := strings.ToLower(strings.TrimSuffix(host, "."))
	if strings.HasSuffix(lowered, clusterDNSSuffix) || lowered == strings.TrimPrefix(clusterDNSSuffix, ".") {
		// Resolve via cluster DNS.
		ips, err := ClusterResolver().LookupHost(ctx, host)
		if err != nil {
			return nil, err
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
