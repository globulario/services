package main

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type installFailureRecordingKV struct {
	mu     sync.Mutex
	puts   int
	key    string
	value  string
	ctxErr error
}

func (r *installFailureRecordingKV) Put(ctx context.Context, key, val string, _ ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.puts++
	r.key = key
	r.value = val
	r.ctxErr = ctx.Err()
	return &clientv3.PutResponse{}, nil
}

func (r *installFailureRecordingKV) Delete(context.Context, string, ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	return &clientv3.DeleteResponse{}, nil
}

func TestWriteInstallFailureState_UsesCommitContextWhenCallerCanceled(t *testing.T) {
	actions.ActionStateDir = t.TempDir()
	t.Cleanup(func() { actions.ActionStateDir = "/var/lib/globular" })

	kv := &installFailureRecordingKV{}
	restore := config.SetWriteKVForTest(kv)
	defer restore()

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	srv := &NodeAgentServer{nodeID: "node-1"}
	resp := srv.writeInstallFailureState(canceledCtx, &node_agentpb.ApplyPackageReleaseRequest{
		PackageName: "dns",
		PackageKind: "SERVICE",
		Version:     "1.2.3",
		BuildId:     "build-1",
		OperationId: "op-1",
	}, "dns", "SERVICE", "1.2.3", "build-1", "txn-1", errors.New("commit failed after promotion"))

	if resp.GetStatus() != "failed" {
		t.Fatalf("status = %q, want failed", resp.GetStatus())
	}
	if kv.puts != 1 {
		t.Fatalf("puts = %d, want 1", kv.puts)
	}
	if kv.ctxErr != nil {
		t.Fatalf("commit write saw canceled context: %v", kv.ctxErr)
	}
	if kv.key != "/globular/nodes/node-1/packages/SERVICE/dns" {
		t.Fatalf("key = %q", kv.key)
	}
}
