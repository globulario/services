package main

import (
	"context"
	"testing"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestBootstrapLeadershipSingle(t *testing.T) {
	srv := newServer(defaultClusterControllerConfig(), "", "", nil, nil)
	bootstrapLeadership(context.Background(), srv, nil, "127.0.0.1:7777")
	if !srv.isLeader() {
		t.Fatalf("expected leader in single-node mode")
	}
}

// We cannot start a real election without etcd, but ensure follower init occurs.
func TestBootstrapLeadershipFollowerInit(t *testing.T) {
	srv := newServer(defaultClusterControllerConfig(), "", "", nil, nil)
	srv.setLeader(true, "pre", "")
	startLeaderElectionFn, startLeaderWatcherFn = func(ctx context.Context, cli *clientv3.Client, srv *server, addr string) {
	}, func(ctx context.Context, cli *clientv3.Client, srv *server) {
	}
	defer func() {
		startLeaderElectionFn = startLeaderElection
		startLeaderWatcherFn = startLeaderWatcher
	}()
	bootstrapLeadership(context.Background(), srv, &clientv3.Client{}, "addr")
	if srv.isLeader() {
		t.Fatalf("expected follower after bootstrap with etcd client placeholder")
	}
}
