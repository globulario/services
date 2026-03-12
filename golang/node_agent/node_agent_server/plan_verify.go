package main

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/globulario/services/golang/security"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/proto"
)

const (
	generationFile      = "/var/lib/globular/node-agent/last-generation"
	signerCacheTTL      = 1 * time.Hour
	quarantineThreshold = 3
	signerEtcdPrefix    = "globular/security/plan-signers/"
)

// signerCacheEntry holds a cached trusted signer public key.
type signerCacheEntry struct {
	pubKey    ed25519.PublicKey
	fetchedAt time.Time
}

// planRejectionTracker tracks consecutive rejections per plan_id for quarantine.
type planRejectionTracker struct {
	mu     sync.Mutex
	counts map[string]int // plan_id → consecutive rejection count
}

func newPlanRejectionTracker() *planRejectionTracker {
	return &planRejectionTracker{counts: make(map[string]int)}
}

func (t *planRejectionTracker) record(planID string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.counts[planID]++
	return t.counts[planID]
}

// clearAll resets all quarantine state.
// Called when a new plan_id is seen (controller corrected the issue).
func (t *planRejectionTracker) clearAll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.counts = make(map[string]int)
}

func (t *planRejectionTracker) isQuarantined(planID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.counts[planID] >= quarantineThreshold
}

// verifyPlan validates a plan before execution.
// Returns nil if valid, descriptive error if rejected.
func (srv *NodeAgentServer) verifyPlan(plan *planpb.NodePlan) error {
	// a. Node ID
	if plan.GetNodeId() != "" && plan.GetNodeId() != srv.nodeID {
		return fmt.Errorf("node_id mismatch: plan=%q self=%q", plan.GetNodeId(), srv.nodeID)
	}

	// b. Cluster ID
	localCID, _ := security.GetLocalClusterID()
	if localCID != "" && plan.GetClusterId() != "" && plan.GetClusterId() != localCID {
		return fmt.Errorf("cluster_id mismatch: plan=%q local=%q",
			plan.GetClusterId(), localCID)
	}

	// c. Expiry (already checked in pollPlan, but verify here too for defense in depth)
	if exp := plan.GetExpiresUnixMs(); exp > 0 {
		if uint64(time.Now().UnixMilli()) > exp {
			return fmt.Errorf("plan expired at %d (now %d)", exp, time.Now().UnixMilli())
		}
	}

	// d. Generation (replay protection)
	lastGen := loadLastAppliedGeneration()
	if plan.GetGeneration() > 0 && lastGen > 0 && plan.GetGeneration() <= lastGen {
		return fmt.Errorf("generation %d <= last applied %d (replay)",
			plan.GetGeneration(), lastGen)
	}

	// e-g. Signature verification
	sig := plan.GetSignature()
	requireSig := strings.EqualFold(os.Getenv("REQUIRE_PLAN_SIGNATURE"), "true")

	if sig == nil || len(sig.GetSig()) == 0 {
		if requireSig {
			return fmt.Errorf("unsigned plan rejected (REQUIRE_PLAN_SIGNATURE=true)")
		}
		log.Printf("WARN plan=%s: unsigned plan accepted (migration mode)", plan.GetPlanId())
		return nil
	}

	// f. Trusted signer lookup (key_id from signature, NOT issued_by)
	pubKey, err := srv.getTrustedSignerKey(sig.GetKeyId())
	if err != nil {
		return fmt.Errorf("untrusted signer key_id=%q: %w", sig.GetKeyId(), err)
	}

	// g. Deterministic verification — exact same serialization as signing
	savedSig := plan.Signature
	plan.Signature = nil
	data, err := proto.MarshalOptions{Deterministic: true}.Marshal(plan)
	plan.Signature = savedSig // always restore
	if err != nil {
		return fmt.Errorf("deterministic marshal for verification: %w", err)
	}

	if !ed25519.Verify(pubKey, data, sig.GetSig()) {
		return fmt.Errorf("Ed25519 signature verification failed (key_id=%s)", sig.GetKeyId())
	}

	// h. Desired hash (audit only — log, do NOT reject)
	if dh := plan.GetDesiredHash(); dh != "" {
		log.Printf("INFO plan=%s: desired_hash=%s (audit-only, not enforced)", plan.GetPlanId(), dh)
	}

	return nil
}

// getTrustedSignerKey fetches a signer's public key from cache or etcd.
func (srv *NodeAgentServer) getTrustedSignerKey(keyID string) (ed25519.PublicKey, error) {
	// Check cache
	srv.signerCacheMu.RLock()
	if entry, ok := srv.signerCache[keyID]; ok {
		if time.Since(entry.fetchedAt) < signerCacheTTL {
			srv.signerCacheMu.RUnlock()
			return entry.pubKey, nil
		}
	}
	srv.signerCacheMu.RUnlock()

	// Get etcd client from plan store
	var etcd *clientv3.Client
	if lps, ok := srv.planStore.(lockablePlanStore); ok {
		etcd = lps.Client()
	}
	if etcd == nil {
		return nil, fmt.Errorf("no etcd client available for signer lookup")
	}

	// Fetch from etcd
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := etcd.Get(ctx, signerEtcdPrefix+keyID)
	if err != nil {
		return nil, fmt.Errorf("etcd lookup for signer %q: %w", keyID, err)
	}
	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("signer %q not in trusted signer set", keyID)
	}

	pubKey := ed25519.PublicKey(resp.Kvs[0].Value)

	// Cache
	srv.signerCacheMu.Lock()
	if srv.signerCache == nil {
		srv.signerCache = make(map[string]signerCacheEntry)
	}
	srv.signerCache[keyID] = signerCacheEntry{pubKey: pubKey, fetchedAt: time.Now()}
	srv.signerCacheMu.Unlock()

	return pubKey, nil
}

// --- Generation persistence ---

func loadLastAppliedGeneration() uint64 {
	data, err := os.ReadFile(generationFile)
	if err != nil {
		return 0 // fresh install → accept any generation
	}
	gen, _ := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	return gen
}

// saveLastAppliedGeneration writes the generation to local file.
// MUST be called ONLY after full successful plan completion —
// never on partial execution, rejection, or start.
func saveLastAppliedGeneration(gen uint64) error {
	dir := filepath.Dir(generationFile)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	return os.WriteFile(generationFile, []byte(strconv.FormatUint(gen, 10)+"\n"), 0600)
}

// --- Rejection reporting ---

// reportPlanRejection writes rejection to etcd status and tracks for quarantine.
func (srv *NodeAgentServer) reportPlanRejection(plan *planpb.NodePlan, reason error) {
	planID := plan.GetPlanId()
	gen := plan.GetGeneration()

	// 1. Write rejection to etcd plan status (Gap 7: use PLAN_REJECTED, not PLAN_FAILED)
	rejState := planpb.PlanState_PLAN_REJECTED
	count := srv.rejectionTracker.record(planID)
	if count >= quarantineThreshold {
		rejState = planpb.PlanState_PLAN_QUARANTINED
	}
	rejStatus := &planpb.NodePlanStatus{
		PlanId:         planID,
		State:          rejState,
		ErrorMessage:   fmt.Sprintf("verification rejected: %s", reason.Error()),
		Generation:     gen,
		FinishedUnixMs: uint64(time.Now().UnixMilli()),
	}
	if srv.planStore != nil {
		if err := srv.planStore.PutStatus(context.Background(), srv.nodeID, rejStatus); err != nil {
			log.Printf("WARN plan=%s: failed to write rejection status to etcd: %v", planID, err)
		}
	}

	// 2. Report to controller via RPC (best-effort, async)
	if srv.controllerClient != nil {
		go func() {
			rctx, rcancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer rcancel()
			_, err := srv.controllerClient.ReportPlanRejection(rctx,
				&cluster_controllerpb.ReportPlanRejectionRequest{
					NodeId:           srv.nodeID,
					PlanId:           planID,
					Generation:       gen,
					Reason:           reason.Error(),
					RejectedAtUnixMs: uint64(time.Now().UnixMilli()),
				})
			if err != nil {
				log.Printf("WARN: failed to report plan rejection to controller: %v", err)
			}
		}()
	}

	// 3. Structured log
	log.Printf("ERROR event=plan.rejected plan_id=%s generation=%d node_id=%s reason=%s",
		planID, gen, srv.nodeID, reason.Error())

	// 4. Log quarantine status (tracking already done above for status selection)
	if count >= quarantineThreshold {
		log.Printf("WARN plan=%s: quarantined after %d consecutive rejections", planID, count)
	}
}

// RotateNodeToken handles token rotation requests from the controller/CLI.
func (srv *NodeAgentServer) RotateNodeToken(ctx context.Context, req *node_agentpb.RotateNodeTokenRequest) (*node_agentpb.RotateNodeTokenResponse, error) {
	if req.GetNewToken() == "" || req.GetNewPrincipal() == "" {
		return nil, fmt.Errorf("token and principal required")
	}
	if err := srv.storeNodeToken(req.GetNewToken(), req.GetNewPrincipal()); err != nil {
		return nil, fmt.Errorf("store token: %w", err)
	}
	log.Printf("node token rotated: principal=%s", req.GetNewPrincipal())
	return &node_agentpb.RotateNodeTokenResponse{Ok: true}, nil
}

// storeNodeToken persists the node-scoped identity token to local filesystem.
func (srv *NodeAgentServer) storeNodeToken(token, principal string) error {
	tokenDir := "/var/lib/globular/tokens"
	if err := os.MkdirAll(tokenDir, 0750); err != nil {
		return fmt.Errorf("create token dir: %w", err)
	}
	tokenPath := filepath.Join(tokenDir, "node_token")
	if err := os.WriteFile(tokenPath, []byte(token), 0600); err != nil {
		return fmt.Errorf("write node token: %w", err)
	}
	principalPath := filepath.Join(tokenDir, "node_principal")
	if err := os.WriteFile(principalPath, []byte(principal), 0600); err != nil {
		return fmt.Errorf("write node principal: %w", err)
	}
	return nil
}
