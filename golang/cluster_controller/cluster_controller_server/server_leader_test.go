package main

import (
	"context"
	"strings"
	"testing"
)

func TestRequireLeaderReturnsLeaderAddr(t *testing.T) {
	srv := newServer(defaultClusterControllerConfig(), "", "", nil, nil)
	srv.setLeader(false, "", "")
	srv.leaderAddr.Store("1.2.3.4:1234")

	err := srv.requireLeader(context.Background())
	if err == nil {
		t.Fatalf("expected error when not leader")
	}
	if !strings.Contains(err.Error(), "leader_addr=1.2.3.4:1234") {
		t.Fatalf("expected leader address in error, got %v", err)
	}
}
