package main

import (
	"context"
	"net"
	"os"
	"os/exec"
	"strings"

	clientv3 "go.etcd.io/etcd/client/v3"
)

var (
	startLeaderElectionFn = startLeaderElection
	startLeaderWatcherFn  = startLeaderWatcher
)

func bootstrapLeadership(ctx context.Context, srv *server, etcdClient *clientv3.Client, leaderAddr string) {
	if etcdClient == nil {
		srv.setLeader(true, "single", leaderAddr)
		return
	}
	srv.setLeader(false, "", "")
	startLeaderWatcherFn(ctx, etcdClient, srv)
	go startLeaderElectionFn(ctx, etcdClient, srv, leaderAddr)
}

// registryHost returns the routable IPv4 address for this node, suitable for
// the Globular service registry. Hostnames are resolved to IPv4 to avoid
// issues with IPv6 link-local addresses that Envoy cannot route to.
func registryHost(leaderAddr string) string {
	host, _, err := net.SplitHostPort(leaderAddr)
	if err != nil || host == "" {
		host, _ = os.Hostname()
	}
	// If it's already an IP, use it directly
	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			return ip4.String()
		}
	}
	// Resolve hostname to IPv4
	if addrs, err := net.LookupIP(host); err == nil {
		for _, a := range addrs {
			if ip4 := a.To4(); ip4 != nil {
				return ip4.String()
			}
		}
	}
	// Fallback: detect routable IP via routing table
	if out, err := exec.Command("ip", "route", "get", "8.8.8.8").Output(); err == nil {
		for i, f := range strings.Fields(string(out)) {
			if f == "src" && i+1 < len(strings.Fields(string(out))) {
				return strings.Fields(string(out))[i+1]
			}
		}
	}
	return "127.0.0.1"
}

// resolveLeaderAddr turns a listen address into an advertise/leader address.
func resolveLeaderAddr(listenAddr string) string {
	addr := strings.TrimSpace(listenAddr)
	if addr == "" {
		return addr
	}
	if strings.HasPrefix(addr, ":") {
		host, _ := os.Hostname()
		return net.JoinHostPort(host, strings.TrimPrefix(addr, ":"))
	}
	return addr
}
