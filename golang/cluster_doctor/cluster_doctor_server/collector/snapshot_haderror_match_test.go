package collector

import (
	"errors"
	"testing"
)

// TestHadError_BaseNameMatchesInstanceQualified locks in the OT-3 source-name
// consolidation. Per-node fan-out sources record errors instance-qualified
// ("node_agent@globule-nuc") so MissingSources can name the failing node, but
// rules stamp evidence and gate on the BASE name ("node_agent"). Before the
// consolidation HadError did exact string equality, so HadError("node_agent", …)
// could never match a "node_agent@…" error — a silently dead reduced-harvest gate
// (objectstore_physical_overlap). A base-name query must now match any of its
// instance-qualified errors, while an instance-qualified query stays exact.
func TestHadError_BaseNameMatchesInstanceQualified(t *testing.T) {
	s := &Snapshot{}
	s.addError("node_agent@globule-nuc", "GetInventory", errors.New("context deadline exceeded"))

	tests := []struct {
		name    string
		service string
		rpc     string
		want    bool
	}{
		// The bug this fixes: base name must match an instance-qualified error.
		{"base name matches instance-qualified", "node_agent", "GetInventory", true},
		{"base name + empty rpc matches", "node_agent", "", true},
		// Instance-qualified query stays exact — the specific node still matches.
		{"exact instance-qualified matches", "node_agent@globule-nuc", "GetInventory", true},
		// A different node's instance-qualified query must NOT match.
		{"other instance does not match", "node_agent@globule-dell", "GetInventory", false},
		// No accidental widening across a prefix boundary: "node_agent" must not
		// be treated as a prefix of "node_agentd" (the "@" separator is required).
		{"prefix without @ does not match", "node_agentd", "GetInventory", false},
		// RPC still discriminates under a matched (base) service.
		{"base name wrong rpc does not match", "node_agent", "GetInfraProbe", false},
		// Unrelated service does not match.
		{"unrelated service does not match", "etcd", "GetInventory", false},
	}
	for _, tt := range tests {
		if got := s.HadError(tt.service, tt.rpc); got != tt.want {
			t.Errorf("%s: HadError(%q, %q) = %v, want %v", tt.name, tt.service, tt.rpc, got, tt.want)
		}
	}
}

// TestHadError_NonFanoutUnaffected confirms the consolidation does not change
// matching for sources that are never instance-qualified (single-source RPCs).
func TestHadError_NonFanoutUnaffected(t *testing.T) {
	s := &Snapshot{}
	s.addError("cluster_controller", "ListNodes", errors.New("unavailable"))

	if !s.HadError("cluster_controller", "ListNodes") {
		t.Error("exact match for a non-fanout source must still report the error")
	}
	if s.HadError("cluster_controller", "GetClusterHealthV1") {
		t.Error("a different rpc on the same service must not match")
	}
	if s.HadError("cluster", "ListNodes") {
		t.Error(`a base-prefix "cluster" must not match "cluster_controller" — no "@" boundary`)
	}
}
