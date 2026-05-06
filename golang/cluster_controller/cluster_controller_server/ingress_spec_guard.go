package main

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

func (srv *server) ensureIngressDesiredState(ctx context.Context) {
	kv := srv.kv
	if kv == nil {
		kv = srv.etcdClient
	}
	if kv == nil {
		return
	}
	resp, err := kv.Get(ctx, ingressSpecKey)
	if err != nil {
		log.Printf("ingress-spec-guard: read spec failed: %v", err)
		return
	}
	if len(resp.Kvs) > 0 && len(resp.Kvs[0].Value) > 0 {
		var spec ingressDesiredSpec
		if err := json.Unmarshal(resp.Kvs[0].Value, &spec); err != nil {
			log.Printf("ingress-spec-guard: invalid spec json, restoring backup: %v", err)
			srv.restoreIngressSpecFromBackup(ctx)
			return
		}
		srv.checkIngressParticipantDrift(spec)
		normalized := srv.normalizeIngressSpec(spec)
		if err := srv.publishIngressSpec(ctx, normalized); err != nil {
			log.Printf("ingress-spec-guard: publish normalized spec failed: %v", err)
		}
		return
	}
	// Spec is absent. Check for explicit delete approval before restoring (Case 06).
	if srv.hasIngressDeleteApproval(ctx) {
		log.Printf("ingress-spec-guard: spec absent with valid delete approval — honoring operator intent, not restoring")
		return
	}
	srv.restoreIngressSpecFromBackup(ctx)
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

func (srv *server) restoreIngressSpecFromBackup(ctx context.Context) {
	kv := srv.kv
	if kv == nil {
		kv = srv.etcdClient
	}
	if kv == nil {
		return
	}
	b, err := kv.Get(ctx, ingressSpecBackupKey)
	if err == nil && len(b.Kvs) > 0 && len(b.Kvs[0].Value) > 0 {
		var spec ingressDesiredSpec
		if uerr := json.Unmarshal(b.Kvs[0].Value, &spec); uerr == nil {
			spec = srv.normalizeIngressSpec(spec)
			if err := srv.publishIngressSpec(ctx, spec); err == nil {
				log.Printf("ingress-spec-guard: restored ingress spec from backup")
				return
			}
		}
	}
	// No backup available. Do NOT write any spec to etcd — writing mode=disabled
	// with explicit_disabled=false is ambiguous and confusing. Instead, seed an
	// explicit disabled baseline spec so Day-0 bootstrap has authoritative intent.
	seed := srv.normalizeIngressSpec(ingressDesiredSpec{
		Mode:             ingressModeDisabled,
		ExplicitDisabled: true,
		Reason:           "day0 bootstrap default: ingress not yet configured",
	})
	if err := srv.publishIngressSpec(ctx, seed); err != nil {
		log.Printf("ingress-spec-guard: CRITICAL: missing spec+backup and failed to seed explicit disabled baseline: %v", err)
		return
	}
	log.Printf("ingress-spec-guard: seeded explicit disabled baseline spec (no backup present)")
}

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
	kv := srv.kv
	if kv == nil {
		kv = srv.etcdClient
	}
	if kv == nil {
		return fmt.Errorf("kv unavailable")
	}
	b, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	if _, err := kv.Put(ctx, ingressSpecKey, string(b)); err != nil {
		return fmt.Errorf("put ingress spec: %w", err)
	}
	if _, err := kv.Put(ctx, ingressSpecBackupKey, string(b)); err != nil {
		return fmt.Errorf("put ingress backup: %w", err)
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
