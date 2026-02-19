package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/actions"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/ingress"
)

const (
	ingressSpecKey        = "/globular/ingress/v1/spec"
	ingressReconcileInterval = 30 * time.Second
)

// ingressReconcileLoop periodically reconciles the ingress configuration
// by reading the spec from etcd and applying it via the keepalived action
func (srv *NodeAgentServer) ingressReconcileLoop(ctx context.Context) {
	ticker := time.NewTicker(ingressReconcileInterval)
	defer ticker.Stop()

	// Initial reconciliation
	srv.reconcileIngress(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			srv.reconcileIngress(ctx)
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
		log.Printf("ingress: failed to read spec from etcd: %v", err)
		return
	}

	// Default to disabled mode if no spec exists
	var specJSON string
	if len(resp.Kvs) == 0 {
		// No spec → treat as disabled mode
		spec := ingress.Spec{
			Version: "v1",
			Mode:    ingress.ModeDisabled,
		}
		specBytes, _ := json.Marshal(spec)
		specJSON = string(specBytes)
	} else {
		specJSON = string(resp.Kvs[0].Value)
	}

	// Validate spec
	var spec ingress.Spec
	if err := json.Unmarshal([]byte(specJSON), &spec); err != nil {
		log.Printf("ingress: invalid spec JSON: %v", err)
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

	log.Printf("ingress: %s", result)
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

// GetIngressStatus returns the current ingress status for this node (optional helper method)
func (srv *NodeAgentServer) GetIngressStatus(ctx context.Context) (*ingress.NodeStatus, error) {
	if srv.nodeID == "" {
		return nil, fmt.Errorf("node ID not available")
	}

	etcdClient, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("get etcd client: %w", err)
	}

	statusKey := "/globular/ingress/v1/status/" + srv.nodeID

	resp, err := etcdClient.Get(ctx, statusKey)
	if err != nil {
		return nil, fmt.Errorf("get status from etcd: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return nil, nil // No status yet
	}

	var status ingress.NodeStatus
	if err := json.Unmarshal(resp.Kvs[0].Value, &status); err != nil {
		return nil, fmt.Errorf("unmarshal status: %w", err)
	}

	return &status, nil
}
