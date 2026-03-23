package main

import (
	"context"
	"net"
	"os"
	"strings"

	"github.com/globulario/services/golang/config"
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
// the Globular service registry.
func registryHost(_ string) string {
	return config.GetRoutableIPv4()
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
