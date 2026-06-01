// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.ingress
// @awareness file_role=keepalived_ingress_reconciler
// @awareness risk=high
package main

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/globular_service/lkg"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/ingress"
)

const (
	ingressSpecKey           = "/globular/ingress/v1/spec"
	ingressReconcileInterval = 30 * time.Second
	ingressLKGSubsystem      = "ingress"
	ingressLKGKey            = "spec"
)

// ingressReconcileLoop periodically reconciles the ingress configuration
// by reading the spec from etcd and applying it via the keepalived action
func (srv *NodeAgentServer) ingressReconcileLoop(ctx context.Context) {
	ih := globular_service.RegisterSubsystem("ingress-reconcile", ingressReconcileInterval)
	ticker := time.NewTicker(ingressReconcileInterval)
	defer ticker.Stop()

	// Initial reconciliation
	srv.reconcileIngress(ctx)
	ih.Tick()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			srv.reconcileIngress(ctx)
			ih.Tick()
		}
	}
}

// reconcileIngress reads the ingress spec from etcd and applies it
func (srv *NodeAgentServer) reconcileIngress(ctx context.Context) {
	if srv.nodeID == "" {
		return
	}

	// Get etcd client
	etcdClient, err := config.GetEtcdClient()
	if err != nil {
		log.Printf("ingress: failed to get etcd client: %v", err)
		return
	}

	reconcileCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Read ingress spec from etcd
	resp, err := etcdClient.Get(reconcileCtx, ingressSpecKey)
	if err != nil {
		log.Printf("ingress: failed to read spec from etcd: %v (holding last-known-good)", err)
		srv.reconcileIngressFromLKG(ctx, "DEGRADED_SPEC_MISSING", err.Error())
		return
	}

	// Missing key must not trigger destructive disable.
	var specJSON string
	if len(resp.Kvs) == 0 {
		log.Printf("ingress: spec missing in etcd (holding last-known-good)")
		srv.reconcileIngressFromLKG(ctx, "DEGRADED_SPEC_MISSING", "ingress spec missing")
		return
	} else {
		specJSON = string(resp.Kvs[0].Value)
	}

	// Validate spec
	var spec ingress.Spec
	if err := json.Unmarshal([]byte(specJSON), &spec); err != nil {
		log.Printf("ingress: invalid spec JSON: %v (holding last-known-good)", err)
		srv.reconcileIngressFromLKG(ctx, "DEGRADED_SPEC_INVALID", err.Error())
		return
	}

	// Call keepalived reconcile action
	action := actions.Get("ingress.keepalived.reconcile")
	if action == nil {
		log.Printf("ingress: keepalived action not registered")
		return
	}

	// Prepare action arguments
	args, err := structpb.NewStruct(map[string]interface{}{
		"spec_json": specJSON,
		"node_id":   srv.nodeID,
	})
	if err != nil {
		log.Printf("ingress: failed to prepare action args: %v", err)
		return
	}

	// Inject etcd client into context for status writing
	actionCtx := context.WithValue(reconcileCtx, "etcd_client", etcdClient)

	// Apply action
	result, err := action.Apply(actionCtx, args)
	if err != nil {
		log.Printf("ingress: reconciliation failed: %v", err)
		// Write error status
		srv.writeIngressErrorStatus(ctx, err)
		return
	}
	if err := lkg.StoreRaw(ingressLKGSubsystem, ingressLKGKey, spec.Generation, []byte(specJSON)); err != nil {
		log.Printf("ingress: WARNING: failed to persist last-known-good spec: %v", err)
	}

	log.Printf("ingress: %s", result)
}

func (srv *NodeAgentServer) reconcileIngressFromLKG(ctx context.Context, phase, reason string) {
	lkgBytes, err := lkg.LoadRaw(ingressLKGSubsystem, ingressLKGKey)
	if err != nil || strings.TrimSpace(string(lkgBytes)) == "" {
		if err == lkg.ErrCorrupt {
			log.Printf("ingress: LKG corrupt — waiting for authoritative spec")
		}
		srv.writeIngressWaitingStatus(ctx, "WAITING_FOR_SPEC", reason)
		return
	}
	lkgJSON := string(lkgBytes)

	action := actions.Get("ingress.keepalived.reconcile")
	if action == nil {
		log.Printf("ingress: keepalived action not registered")
		return
	}
	args, err := structpb.NewStruct(map[string]interface{}{
		"spec_json": lkgJSON,
		"node_id":   srv.nodeID,
	})
	if err != nil {
		log.Printf("ingress: failed to prepare LKG action args: %v", err)
		return
	}
	actionCtx := ctx
	if ec, err := config.GetEtcdClient(); err == nil {
		actionCtx = context.WithValue(ctx, "etcd_client", ec)
	}
	if _, err := action.Apply(actionCtx, args); err != nil {
		srv.writeIngressErrorStatus(ctx, err)
		return
	}
	srv.writeIngressWaitingStatus(ctx, phase, reason)
}

// writeIngressErrorStatus writes an error status to etcd
func (srv *NodeAgentServer) writeIngressErrorStatus(ctx context.Context, err error) {
	if srv.nodeID == "" {
		return
	}

	etcdClient, etcdErr := config.GetEtcdClient()
	if etcdErr != nil {
		return
	}

	status := &ingress.NodeStatus{
		NodeID:    srv.nodeID,
		Phase:     "Error",
		VRRPState: "UNKNOWN",
		HasVIP:    false,
		LastError: err.Error(),
	}

	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := ingress.WriteStatus(writeCtx, etcdClient, srv.nodeID, status); err != nil {
		log.Printf("ingress: failed to write error status: %v", err)
	}
}

func (srv *NodeAgentServer) writeIngressWaitingStatus(ctx context.Context, phase, reason string) {
	if srv.nodeID == "" {
		return
	}
	etcdClient, err := config.GetEtcdClient()
	if err != nil {
		return
	}
	status := &ingress.NodeStatus{
		NodeID:    srv.nodeID,
		Phase:     phase,
		VRRPState: "UNKNOWN",
		HasVIP:    false,
		LastError: reason,
	}
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_ = ingress.WriteStatus(writeCtx, etcdClient, srv.nodeID, status)
}

// watchIngressSpec watches the ingress spec key in etcd for changes
// and triggers reconciliation immediately when changes occur
func (srv *NodeAgentServer) watchIngressSpec(ctx context.Context) {
	etcdClient, err := config.GetEtcdClient()
	if err != nil {
		log.Printf("ingress: failed to get etcd client for watch: %v", err)
		return
	}

	watchChan := etcdClient.Watch(ctx, ingressSpecKey)

	for {
		select {
		case <-ctx.Done():
			return
		case watchResp := <-watchChan:
			if watchResp.Err() != nil {
				log.Printf("ingress: watch error: %v", watchResp.Err())
				// Wait before retrying
				time.Sleep(5 * time.Second)
				if retryClient, retryErr := config.GetEtcdClient(); retryErr == nil {
					watchChan = retryClient.Watch(ctx, ingressSpecKey)
				}
				continue
			}

			// Trigger reconciliation on any change
			for _, event := range watchResp.Events {
				log.Printf("ingress: spec changed (type: %s), triggering reconciliation", event.Type)
				srv.reconcileIngress(ctx)
				break // Only reconcile once per watch event
			}
		}
	}
}

// StartIngressReconciliation starts both the periodic reconciliation loop
// and the watch-based reconciliation for immediate updates
func (srv *NodeAgentServer) StartIngressReconciliation(ctx context.Context) {
	// Check if etcd is available
	if _, err := config.GetEtcdClient(); err != nil {
		log.Printf("ingress: etcd client not available, skipping reconciliation: %v", err)
		return
	}

	log.Printf("ingress: starting reconciliation loop (interval: %v)", ingressReconcileInterval)

	// Start periodic reconciliation loop
	go srv.ingressReconcileLoop(ctx)

	// Start watch-based reconciliation for immediate updates
	go srv.watchIngressSpec(ctx)
}
