// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.ingress_spec_guard
// @awareness file_role=ingress_spec_guardian_with_lkg_restore_and_delete_approval
// @awareness enforces=globular.platform:invariant.critical_state.deletion_requires_audited_intent
// @awareness enforces=globular.platform:invariant.destructive_actions.require_explicit_guard
// @awareness risk=critical
package main

// ingress_spec_guard.go — guards the ingress desired spec
// (/globular/ingress/v1/spec). Two load-bearing properties:
//
//  1. A missing spec key is treated as MISSING_STATE_WITHOUT_INTENT
//     unless an explicit ingressDeleteApproval tombstone exists
//     under the approval prefix. Without the approval, the guard
//     republishes from spec_backup — restoring a key the operator
//     never asked to delete.
//
//  2. A spec with mode=disabled must carry the full guard
//     (explicit_disabled=true + non-empty reason + generation>0)
//     before any node will act on it. The pre-execution doctor rule
//     in cluster_doctor/.../rules/destructive_action_audit.go
//     reads the same etcd key and surfaces the gap before any node
//     processes it.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_service"
)

const (
	ingressSpecKey             = "/globular/ingress/v1/spec"
	ingressSpecBackupKey       = "/globular/ingress/v1/spec_backup"
	ingressRepublishRequestKey = "/globular/ingress/v1/republish_request"

	// ingressDeleteApprovalPrefix is the etcd path prefix for explicit ingress
	// delete approvals (Case 06: MISSING_STATE_WITHOUT_INTENT_MARKER).
	// A key at /globular/ingress/v1/delete_approval/<generation> signals that
	// the operator intentionally deleted the spec for that generation.
	// Without an approval the controller always restores a missing spec.
	ingressDeleteApprovalPrefix = "/globular/ingress/v1/delete_approval/"
)

// ingressDeleteApproval is the payload written to a delete-approval key.
type ingressDeleteApproval struct {
	Generation     int64  `json:"generation"`
	ActorIdentity  string `json:"actor_identity"`
	Reason         string `json:"reason"`
	ApprovedAtUnix int64  `json:"approved_at_unix"`
}

type ingressSpecMode string

const (
	ingressModeVIPFailover ingressSpecMode = "vip_failover"
	ingressModeDisabled    ingressSpecMode = "disabled"
)

type ingressDesiredSpec struct {
	Version          string                 `json:"version"`
	Mode             ingressSpecMode        `json:"mode"`
	Generation       int64                  `json:"generation,omitempty"`
	Checksum         string                 `json:"checksum,omitempty"`
	WrittenAtUnix    int64                  `json:"written_at_unix,omitempty"`
	WriterLeaderID   string                 `json:"writer_leader_id,omitempty"`
	Source           string                 `json:"source,omitempty"`
	ExplicitDisabled bool                   `json:"explicit_disabled,omitempty"`
	Reason           string                 `json:"reason,omitempty"`
	VIPFailover      map[string]interface{} `json:"vip_failover,omitempty"`

	// Authoritative indicates the spec was written and validated by an active
	// cluster-controller leader with a known cluster topology (Case 02:
	// BOOTSTRAP_STATE_ESCAPED_TO_PRODUCTION). Bootstrap/operator-injected specs
	// without this marker are treated as tentative by consumers — they apply the
	// spec but do not treat it as final cluster intent.
	Authoritative bool `json:"authoritative,omitempty"`
}

func (srv *server) startIngressSpecGuard(ctx context.Context) {
	safeGoTracked("ingress-spec-guard", 30*time.Second, func(h *globular_service.SubsystemHandle) {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !srv.isLeader() {
					h.Tick()
					continue
				}
				if srv.consumeIngressRepublishRequest(ctx) {
					log.Printf("ingress-spec-guard: processing manual republish request")
				}
				rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				srv.ensureIngressDesiredState(rctx)
				cancel()
				h.Tick()
			}
		}
	})
}

func (srv *server) consumeIngressRepublishRequest(ctx context.Context) bool {
	kv := srv.kv
	if kv == nil {
		kv = srv.etcdClient
	}
	if kv == nil {
		return false
	}
	rctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	resp, err := kv.Get(rctx, ingressRepublishRequestKey)
	if err != nil || len(resp.Kvs) == 0 {
		return false
	}
	_, _ = kv.Delete(rctx, ingressRepublishRequestKey)
	return true
}

// loadIngressSpec is the canonical typed reader for
// /globular/ingress/v1/spec. Controller-internal code MUST use this
// helper rather than calling srv.kv.Get against the raw key — the
// principle is the same one
// invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage
// enforces across services: even inside the owner, truth flows
// through a typed boundary so a future ownership move (out of the
// guard, into a dedicated service) is a single-call-site change.
//
// Return contract:
//
//	(nil, false, nil)  — spec absent, no error (caller may restore)
//	(nil, true,  err)  — read or unmarshal failed (caller logs)
//	(spec, true, nil)  — spec present and parsed
func (srv *server) loadIngressSpec(ctx context.Context) (*ingressDesiredSpec, bool, error) {
	kv := srv.kv
	if kv == nil && srv.etcdClient != nil {
		// Boot-order fallback: srv.kv may not be wired before
		// srv.etcdClient. Use the etcd client directly only when
		// it is a real non-nil pointer (assigning a nil
		// *clientv3.Client into the interface produces a non-nil
		// interface containing a nil pointer — a typed-nil that
		// fails open at runtime).
		kv = srv.etcdClient
	}
	if kv == nil {
		return nil, false, nil
	}
	resp, err := kv.Get(ctx, ingressSpecKey)
	if err != nil {
		return nil, true, err
	}
	if len(resp.Kvs) == 0 || len(resp.Kvs[0].Value) == 0 {
		return nil, false, nil
	}
	var spec ingressDesiredSpec
	if err := json.Unmarshal(resp.Kvs[0].Value, &spec); err != nil {
		return nil, true, fmt.Errorf("invalid spec json: %w", err)
	}
	return &spec, true, nil
}

func (srv *server) ensureIngressDesiredState(ctx context.Context) {
	spec, present, err := srv.loadIngressSpec(ctx)
	if err != nil {
		log.Printf("ingress-spec-guard: read spec failed: %v", err)
		if present {
			// Present-but-invalid JSON — dispatch the restore workflow
			// (failure_mode hidden_workflow.controller_ingress_spec_guard_restore_path
			// lift). Inline restoreIngressSpecFromBackup was removed
			// in favour of cluster.ingress_spec_restore for durable
			// step receipts.
			if derr := srv.dispatchIngressSpecRestore(ctx, "spec_present_but_invalid_json"); derr != nil {
				log.Printf("ingress-spec-guard: restore workflow dispatch failed: %v", derr)
			}
		}
		return
	}
	if spec != nil {
		srv.checkIngressParticipantDrift(*spec)
		normalized := srv.normalizeIngressSpec(*spec)
		if err := srv.publishIngressSpec(ctx, normalized); err != nil {
			log.Printf("ingress-spec-guard: publish normalized spec failed: %v", err)
		}
		return
	}
	// Spec is absent. Check for explicit delete approval before
	// restoring (Case 06).
	if srv.hasIngressDeleteApproval(ctx) {
		log.Printf("ingress-spec-guard: spec absent with valid delete approval — honoring operator intent, not restoring")
		return
	}
	// Dispatch the restore workflow. Replaces the inline
	// restoreIngressSpecFromBackup call. Each restore attempt now produces
	// a workflow_runs row with durable step receipts (load_backup →
	// compose_spec → publish_spec).
	if derr := srv.dispatchIngressSpecRestore(ctx, "spec_absent_no_delete_approval"); derr != nil {
		log.Printf("ingress-spec-guard: restore workflow dispatch failed: %v", derr)
	}
}

// hasIngressDeleteApproval checks whether any valid delete-approval key exists
// under ingressDeleteApprovalPrefix. A valid approval must have a non-empty
// actor identity, non-empty reason, and be less than 24 hours old.
func (srv *server) hasIngressDeleteApproval(ctx context.Context) bool {
	kv := srv.kv
	if kv == nil {
		kv = srv.etcdClient
	}
	if kv == nil {
		return false
	}
	rctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	resp, err := kv.Get(rctx, ingressDeleteApprovalPrefix, clientv3.WithPrefix())
	if err != nil || len(resp.Kvs) == 0 {
		return false
	}
	now := time.Now().Unix()
	for _, kv := range resp.Kvs {
		var approval ingressDeleteApproval
		if err := json.Unmarshal(kv.Value, &approval); err != nil {
			continue
		}
		if approval.ActorIdentity == "" || approval.Reason == "" {
			continue
		}
		age := now - approval.ApprovedAtUnix
		if age < 0 || age > 86400 {
			continue
		}
		log.Printf("ingress-spec-guard: found delete approval gen=%d actor=%q reason=%q age=%ds",
			approval.Generation, approval.ActorIdentity, approval.Reason, age)
		return true
	}
	return false
}

// restoreIngressSpecFromBackup was removed in the 2026-06-05 lift to the
// cluster.ingress_spec_restore workflow. Its three logical steps
// (load_backup, compose_spec, publish_spec) are now declared in
// golang/workflow/definitions/cluster.ingress_spec_restore.yaml and
// implemented in workflow_ingress_restore.go::buildIngressControllerConfig.
// The guard tick at ensureIngressDesiredState() dispatches the workflow.
//
// See failure_mode hidden_workflow.controller_ingress_spec_guard_restore_path
// for the audit finding that motivated the lift.

func (srv *server) normalizeIngressSpec(spec ingressDesiredSpec) ingressDesiredSpec {
	if strings.TrimSpace(spec.Version) == "" {
		spec.Version = "v1"
	}
	if spec.Mode == "" {
		spec.Mode = ingressModeDisabled
	}
	spec.WrittenAtUnix = time.Now().Unix()
	spec.Source = "cluster-controller"
	if leaderID, _ := srv.leaderID.Load().(string); leaderID != "" {
		spec.WriterLeaderID = leaderID
	}
	spec.Generation++
	// Mark as authoritative: this spec was written by an active leader with a
	// known cluster topology (Case 02: BOOTSTRAP_STATE_ESCAPED_TO_PRODUCTION).
	spec.Authoritative = true
	spec.Checksum = ingressSpecChecksum(spec)
	return spec
}

func ingressSpecChecksum(spec ingressDesiredSpec) string {
	copySpec := spec
	copySpec.Checksum = ""
	b, _ := json.Marshal(copySpec)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (srv *server) publishIngressSpec(ctx context.Context, spec ingressDesiredSpec) error {
	const writerID = "cluster-controller"
	if err := ValidateCriticalKeyWrite(ingressSpecKey, writerID); err != nil {
		return err
	}
	if err := ValidateCriticalKeyWrite(ingressSpecBackupKey, writerID); err != nil {
		return err
	}
	b, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	// Atomic, owner-guarded write of the ingress spec + its backup (RT-3): both
	// keys commit together or neither does, through the transaction primitive,
	// which also checks the controller's registered writer identity against the
	// owner table. The two keys were previously written as separate non-atomic
	// Puts — making the write atomic keeps the backup consistent with the spec for
	// restoreIngressSpecFromBackup (the explicit ValidateCriticalKeyWrite checks
	// above are retained as the always-on guard).
	if err := config.RunTxnWithClass(ctx, config.CriticalWrite,
		config.PutOp(ingressSpecKey, b),
		config.PutOp(ingressSpecBackupKey, b),
	); err != nil {
		return fmt.Errorf("publish ingress spec: %w", err)
	}
	return nil
}

func (srv *server) ingressParticipantsLocked() []string {
	if srv.state == nil {
		return nil
	}
	var ids []string
	for _, n := range srv.state.Nodes {
		if n == nil || n.NodeID == "" {
			continue
		}
		if !nodeHasProfile(&memberNode{Profiles: n.Profiles}, []string{"control-plane"}) {
			continue
		}
		ids = append(ids, n.NodeID)
	}
	sort.Strings(ids)
	return ids
}

// checkIngressParticipantDrift compares the participant list in the ingress spec
// against the current set of gateway nodes and logs a warning on drift.
// It does NOT auto-update the spec — changing participants is a topology safety
// action that requires explicit operator intent.
func (srv *server) checkIngressParticipantDrift(spec ingressDesiredSpec) {
	if spec.Mode != ingressModeVIPFailover || spec.VIPFailover == nil {
		return
	}
	rawParticipants, ok := spec.VIPFailover["participants"]
	if !ok {
		return
	}
	// VIPFailover is map[string]interface{}; participants is []interface{} after JSON decode.
	participantSlice, ok := rawParticipants.([]interface{})
	if !ok {
		return
	}
	specParticipants := make([]string, 0, len(participantSlice))
	for _, p := range participantSlice {
		if s, ok := p.(string); ok {
			specParticipants = append(specParticipants, s)
		}
	}
	sort.Strings(specParticipants)

	srv.lock("ingress-participant-drift-check")
	clusterParticipants := srv.ingressParticipantsLocked()
	srv.unlock()

	// Build sets and compare.
	specSet := map[string]bool{}
	for _, p := range specParticipants {
		specSet[p] = true
	}
	clusterSet := map[string]bool{}
	for _, p := range clusterParticipants {
		clusterSet[p] = true
	}
	if len(specSet) == len(clusterSet) {
		allMatch := true
		for p := range clusterSet {
			if !specSet[p] {
				allMatch = false
				break
			}
		}
		if allMatch {
			return
		}
	}
	log.Printf("ingress-spec-guard: WARNING: spec participants do not match current gateway nodes — spec has %v, cluster has %v",
		specParticipants, clusterParticipants)
}
