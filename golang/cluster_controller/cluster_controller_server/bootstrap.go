package main

import (
	"context"
	"net"
	"os"
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
