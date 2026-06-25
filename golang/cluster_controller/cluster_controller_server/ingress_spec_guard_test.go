package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestHasIngressDeleteApproval_ValidAndInvalid(t *testing.T) {
	kv := newFakeKV()
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.kv = kv

	if srv.hasIngressDeleteApproval(context.Background()) {
		t.Fatal("expected no delete approval when key is absent")
	}

	invalid := ingressDeleteApproval{
		Generation:     1,
		ActorIdentity:  "",
		Reason:         "missing actor identity",
		ApprovedAtUnix: time.Now().Unix(),
	}
	badRaw, _ := json.Marshal(invalid)
	_, _ = kv.Put(context.Background(), ingressDeleteApprovalPrefix+"1", string(badRaw))
	if srv.hasIngressDeleteApproval(context.Background()) {
		t.Fatal("expected invalid approval to be rejected")
	}

	valid := ingressDeleteApproval{
		Generation:     2,
		ActorIdentity:  "operator:test",
		Reason:         "planned ingress disable",
		ApprovedAtUnix: time.Now().Unix(),
	}
	goodRaw, _ := json.Marshal(valid)
	_, _ = kv.Put(context.Background(), ingressDeleteApprovalPrefix+"2", string(goodRaw))
	if !srv.hasIngressDeleteApproval(context.Background()) {
		t.Fatal("expected valid approval to be accepted")
	}
}

func TestEnsureIngressDesiredState_RestoreDeniedByApproval(t *testing.T) {
	kv := newFakeKV()
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.kv = kv

	backup := ingressDesiredSpec{
		Version:        "v1",
		Mode:           ingressModeVIPFailover,
		Generation:     7,
		Authoritative:  true,
		WriterLeaderID: "leader-a",
		Source:         "cluster-controller",
		VIPFailover: map[string]interface{}{
			"vip": "10.0.0.100/24",
		},
	}
	raw, _ := json.Marshal(backup)
	_, _ = kv.Put(context.Background(), ingressSpecBackupKey, string(raw))

	approval := ingressDeleteApproval{
		Generation:     7,
		ActorIdentity:  "operator:test",
		Reason:         "temporary disable",
		ApprovedAtUnix: time.Now().Unix(),
	}
	approvalRaw, _ := json.Marshal(approval)
	_, _ = kv.Put(context.Background(), ingressDeleteApprovalPrefix+"7", string(approvalRaw))

	srv.ensureIngressDesiredState(context.Background())
	resp, _ := kv.Get(context.Background(), ingressSpecKey)
	if len(resp.Kvs) != 0 {
		t.Fatal("expected ingress spec to remain absent when delete approval exists")
	}
}

// Post-2026-06-05 lift: the inline restoreIngressSpecFromBackup function was
// replaced by the cluster.ingress_spec_restore workflow. The tests below
// pin the IngressControllerConfig action handlers directly — those handlers
// are what the workflow's step actions execute. This is the correct
// regression seam after the lift; the dispatch path is owned by the
// workflow engine and tested there.

func TestIngressControllerConfig_LoadBackup_Present(t *testing.T) {
	kv := newFakeKV()
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.kv = kv

	backup := ingressDesiredSpec{Version: "v1", Mode: ingressModeVIPFailover, Generation: 3}
	raw, _ := json.Marshal(backup)
	_, _ = kv.Put(context.Background(), ingressSpecBackupKey, string(raw))

	cfg := srv.buildIngressControllerConfig()
	bytes, present, err := cfg.LoadBackup(context.Background())
	if err != nil {
		t.Fatalf("LoadBackup: %v", err)
	}
	if !present {
		t.Fatal("expected present=true when backup key exists")
	}
	if len(bytes) == 0 {
		t.Fatal("expected non-empty backup bytes")
	}
}

func TestIngressControllerConfig_LoadBackup_Absent(t *testing.T) {
	kv := newFakeKV()
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.kv = kv

	cfg := srv.buildIngressControllerConfig()
	bytes, present, err := cfg.LoadBackup(context.Background())
	if err != nil {
		t.Fatalf("LoadBackup with no backup: %v", err)
	}
	if present {
		t.Fatal("expected present=false when backup key is absent")
	}
	if bytes != nil {
		t.Fatal("expected nil bytes when backup is absent")
	}
}

func TestIngressControllerConfig_ComposeRestoreSpec_RestoresBackup(t *testing.T) {
	kv := newFakeKV()
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.kv = kv

	backup := ingressDesiredSpec{
		Version:    "v1",
		Mode:       ingressModeVIPFailover,
		Generation: 5,
		VIPFailover: map[string]interface{}{
			"vip": "10.0.0.100/24",
		},
	}
	raw, _ := json.Marshal(backup)

	cfg := srv.buildIngressControllerConfig()
	specBytes, source, err := cfg.ComposeRestoreSpec(context.Background(), true, raw)
	if err != nil {
		t.Fatalf("ComposeRestoreSpec: %v", err)
	}
	if source != "backup" {
		t.Fatalf("source = %q, want %q", source, "backup")
	}
	var got ingressDesiredSpec
	if err := json.Unmarshal(specBytes, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Mode != ingressModeVIPFailover {
		t.Fatalf("Mode = %q, want %q", got.Mode, ingressModeVIPFailover)
	}
	if got.Generation != backup.Generation+1 {
		t.Fatalf("Generation = %d, want %d (normalize must bump)", got.Generation, backup.Generation+1)
	}
}

func TestIngressControllerConfig_ComposeRestoreSpec_SeedsWhenNoBackup(t *testing.T) {
	kv := newFakeKV()
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.kv = kv

	cfg := srv.buildIngressControllerConfig()
	specBytes, source, err := cfg.ComposeRestoreSpec(context.Background(), false, nil)
	if err != nil {
		t.Fatalf("ComposeRestoreSpec: %v", err)
	}
	if source != "seed" {
		t.Fatalf("source = %q, want %q", source, "seed")
	}
	var got ingressDesiredSpec
	if err := json.Unmarshal(specBytes, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Mode != ingressModeDisabled {
		t.Fatalf("Mode = %q, want %q", got.Mode, ingressModeDisabled)
	}
	if !got.ExplicitDisabled {
		t.Fatal("expected ExplicitDisabled=true for Day-0 seeded baseline")
	}
}

func TestIngressControllerConfig_ComposeRestoreSpec_SeedsWhenBackupUnparseable(t *testing.T) {
	kv := newFakeKV()
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.kv = kv

	cfg := srv.buildIngressControllerConfig()
	_, source, err := cfg.ComposeRestoreSpec(context.Background(), true, []byte("{not valid json"))
	if err != nil {
		t.Fatalf("ComposeRestoreSpec: %v", err)
	}
	if source != "seed" {
		t.Fatalf("source = %q, want %q (unparseable backup must fall back to seed)", source, "seed")
	}
}

func TestIngressControllerConfig_PublishRestoreSpec_WritesBothKeys(t *testing.T) {
	// publishIngressSpec now writes the spec + backup atomically through the
	// guarded transaction primitive (RT-3), so capture the committed txn ops
	// instead of probing srv.kv.
	var committed []string
	restore := config.SetTxnRunnerForTest(func(_ context.Context, ops []clientv3.Op) error {
		for _, op := range ops {
			committed = append(committed, string(op.KeyBytes()))
		}
		return nil
	})
	defer restore()

	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)

	spec := srv.normalizeIngressSpec(ingressDesiredSpec{
		Mode:             ingressModeDisabled,
		ExplicitDisabled: true,
		Reason:           "test",
	})
	specBytes, _ := json.Marshal(spec)

	cfg := srv.buildIngressControllerConfig()
	if err := cfg.PublishRestoreSpec(context.Background(), specBytes); err != nil {
		t.Fatalf("PublishRestoreSpec: %v", err)
	}

	var live, backup bool
	for _, k := range committed {
		switch k {
		case ingressSpecKey:
			live = true
		case ingressSpecBackupKey:
			backup = true
		}
	}
	if !live {
		t.Fatal("expected live spec key to be written in the guarded txn")
	}
	if !backup {
		t.Fatal("expected backup spec key to be written in the guarded txn")
	}
}
