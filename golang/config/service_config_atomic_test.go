package config

import (
	"context"
	"testing"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// TestSaveServiceConfiguration_AtomicDesiredAndRuntime proves the OT-3 fix: the
// service desired + runtime keys are written in a SINGLE etcd transaction (both or
// neither), closing the prior KNOWN GAP where a failed second Put left etcd with new
// desired and stale runtime — a divergence the cluster-doctor would misdiagnose.
func TestSaveServiceConfiguration_AtomicDesiredAndRuntime(t *testing.T) {
	var txns int
	var keys []string
	restore := SetTxnRunnerForTest(func(_ context.Context, ops []clientv3.Op) error {
		txns++
		for _, op := range ops {
			keys = append(keys, string(op.KeyBytes()))
		}
		return nil
	})
	defer restore()

	id := "echo.EchoService"
	if err := SaveServiceConfiguration(map[string]interface{}{"Id": id}); err != nil {
		t.Fatalf("SaveServiceConfiguration: %v", err)
	}

	if txns != 1 {
		t.Fatalf("expected desired+runtime in exactly 1 atomic transaction, got %d", txns)
	}
	wantConfig := etcdKey(id, configKey)
	wantRuntime := etcdKey(id, runtimeKey)
	var hasConfig, hasRuntime bool
	for _, k := range keys {
		switch k {
		case wantConfig:
			hasConfig = true
		case wantRuntime:
			hasRuntime = true
		}
	}
	if !hasConfig || !hasRuntime {
		t.Errorf("the transaction must contain both desired (%s) and runtime (%s) keys; got %v", wantConfig, wantRuntime, keys)
	}
}
