package main

import (
	"context"
	"errors"
	"testing"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/globulario/services/golang/installed_state"
)

func TestKindCriticalKeyPrereqsService(t *testing.T) {
	keys := kindCriticalKeyPrereqs["SERVICE"]
	if len(keys) == 0 {
		t.Fatal("SERVICE kind should have at least one critical key prereq")
	}
	found := false
	for _, k := range keys {
		if k == "/globular/system/config" {
			found = true
		}
	}
	if !found {
		t.Errorf("SERVICE kind prereqs %v should include /globular/system/config", keys)
	}
}

func TestKindCriticalKeyPrereqsWorkload(t *testing.T) {
	keys := kindCriticalKeyPrereqs["WORKLOAD"]
	if len(keys) == 0 {
		t.Fatal("WORKLOAD kind should have at least one critical key prereq")
	}
}

func TestKindCriticalKeyPrereqsInfrastructure(t *testing.T) {
	keys := kindCriticalKeyPrereqs["INFRASTRUCTURE"]
	if len(keys) != 0 {
		t.Errorf("INFRASTRUCTURE kind should have no prereqs (it creates config); got %v", keys)
	}
}

func TestKindCriticalKeyPrereqsCommand(t *testing.T) {
	keys := kindCriticalKeyPrereqs["COMMAND"]
	if len(keys) != 0 {
		t.Errorf("COMMAND kind should have no prereqs; got %v", keys)
	}
}

func TestPackageCriticalKeyPrereqsKeepalived(t *testing.T) {
	keys := packageCriticalKeyPrereqs["keepalived"]
	found := false
	for _, k := range keys {
		if k == "/globular/ingress/v1/spec" {
			found = true
		}
	}
	if !found {
		t.Errorf("keepalived prereqs %v should include /globular/ingress/v1/spec", keys)
	}
}

func TestPackageCriticalKeyPrereqsEnvoy(t *testing.T) {
	keys := packageCriticalKeyPrereqs["envoy"]
	found := false
	for _, k := range keys {
		if k == "/globular/ingress/v1/spec" {
			found = true
		}
	}
	if !found {
		t.Errorf("envoy prereqs %v should include /globular/ingress/v1/spec", keys)
	}
}

func TestCriticalKeyBlockActionID(t *testing.T) {
	id := criticalKeyBlockActionID("node-abc", "SERVICE", "rbac")
	expected := "controller/node-abc/SERVICE/rbac/critical_key_block"
	if id != expected {
		t.Errorf("criticalKeyBlockActionID = %q, want %q", id, expected)
	}
}

func TestCriticalKeyPrereqsMissingNoPrereqs(t *testing.T) {
	// INFRASTRUCTURE packages have no prereqs — returns "" without hitting etcd.
	missing, checkErr := criticalKeyPrereqStatus(nil, "etcd", "INFRASTRUCTURE")
	if missing != "" {
		t.Errorf("INFRASTRUCTURE pkg should have no prereqs, got missing=%q", missing)
	}
	if checkErr != nil {
		t.Errorf("INFRASTRUCTURE pkg should not check etcd, got checkErr=%v", checkErr)
	}
}

func TestCriticalKeyPrereqStatus_EtcdClientError(t *testing.T) {
	orig := criticalKeyGetEtcdClient
	t.Cleanup(func() { criticalKeyGetEtcdClient = orig })
	criticalKeyGetEtcdClient = func() (*clientv3.Client, error) {
		return nil, errors.New("dial etcd: timeout")
	}

	missing, checkErr := criticalKeyPrereqStatus(context.Background(), "rbac", "SERVICE")
	if missing != "" {
		t.Fatalf("expected no missing key when client creation fails, got %q", missing)
	}
	if checkErr == nil {
		t.Fatal("expected checkErr on etcd client error")
	}
}

func TestWriteCriticalKeyBlock_CheckErrorPayload(t *testing.T) {
	orig := criticalKeyWriteResult
	t.Cleanup(func() { criticalKeyWriteResult = orig })

	var captured *installed_state.ConvergenceResultV1
	criticalKeyWriteResult = func(ctx context.Context, r *installed_state.ConvergenceResultV1) error {
		captured = r
		return nil
	}

	writeCriticalKeyBlock(context.Background(), []string{"node-1"}, "rbac", "SERVICE", "", errors.New("tls: bad certificate"))
	if captured == nil {
		t.Fatal("expected convergence result to be written")
	}
	if captured.Outcome != installed_state.OutcomeBlockedCriticalKeyMissing {
		t.Fatalf("outcome=%s, want %s", captured.Outcome, installed_state.OutcomeBlockedCriticalKeyMissing)
	}
	if captured.ReasonCode != "critical_key_check_error" {
		t.Fatalf("reason_code=%q, want critical_key_check_error", captured.ReasonCode)
	}
	if captured.UnblockPolicy != "check_error_retry_after_backoff" {
		t.Fatalf("unblock_policy=%q, want check_error_retry_after_backoff", captured.UnblockPolicy)
	}
	if captured.Evidence["check_error"] == "" {
		t.Fatal("expected check_error evidence")
	}
}
