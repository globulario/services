package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"
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

func TestEnsureIngressDesiredState_RestoreWithoutApproval(t *testing.T) {
	kv := newFakeKV()
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.kv = kv

	backup := ingressDesiredSpec{
		Version:        "v1",
		Mode:           ingressModeVIPFailover,
		Generation:     3,
		Authoritative:  true,
		WriterLeaderID: "leader-b",
		Source:         "cluster-controller",
		VIPFailover: map[string]interface{}{
			"vip": "10.0.0.100/24",
		},
	}
	raw, _ := json.Marshal(backup)
	_, _ = kv.Put(context.Background(), ingressSpecBackupKey, string(raw))

	srv.ensureIngressDesiredState(context.Background())
	resp, _ := kv.Get(context.Background(), ingressSpecKey)
	if len(resp.Kvs) == 0 {
		t.Fatal("expected ingress spec restore when approval is absent")
	}
}

func TestEnsureIngressDesiredState_NoSpecNoBackup_SeedsExplicitDisabled(t *testing.T) {
	kv := newFakeKV()
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.kv = kv

	srv.ensureIngressDesiredState(context.Background())

	resp, _ := kv.Get(context.Background(), ingressSpecKey)
	if len(resp.Kvs) == 0 {
		t.Fatal("expected ingress spec to be seeded when spec+backup are both absent")
	}
	var spec ingressDesiredSpec
	if err := json.Unmarshal(resp.Kvs[0].Value, &spec); err != nil {
		t.Fatalf("unmarshal spec: %v", err)
	}
	if spec.Mode != ingressModeDisabled {
		t.Fatalf("expected mode=disabled, got %q", spec.Mode)
	}
	if !spec.ExplicitDisabled {
		t.Fatal("expected ExplicitDisabled=true for Day-0 seeded baseline")
	}

	backupResp, _ := kv.Get(context.Background(), ingressSpecBackupKey)
	if len(backupResp.Kvs) == 0 {
		t.Fatal("expected ingress backup key to be seeded alongside spec")
	}
}
