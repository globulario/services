package config

import (
	"context"
	"testing"
)

// TestSaveDiskCandidate_RoutesThroughGovernedPrimitive proves the RT-2 Surface-A
// migration: the node-self disk-candidate write goes through the governed
// PutRuntimeWithClass primitive (write-class policy + owner-ownership guard),
// not a bare cli.Put. The write KV override records the call.
func TestSaveDiskCandidate_RoutesThroughGovernedPrimitive(t *testing.T) {
	kv := &fakeKV{}
	restore := SetWriteKVForTest(kv)
	defer restore()

	if err := SaveDiskCandidate(context.Background(), &DiskCandidate{NodeID: "n1", DiskID: "d1"}); err != nil {
		t.Fatalf("SaveDiskCandidate: %v", err)
	}
	if kv.callCount() != 1 {
		t.Errorf("expected SaveDiskCandidate to route through the governed primitive (1 Put), got %d", kv.callCount())
	}
}
