package main

import (
	"context"
	"sync"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type operationState struct {
	mu      sync.Mutex
	last    *cluster_controllerpb.OperationEvent
	created time.Time
	done    bool
	nodeID  string
}

type operationWatcher struct {
	nodeID string
	opID   string
	ch     chan *cluster_controllerpb.OperationEvent
}

func (w *operationWatcher) matches(evt *cluster_controllerpb.OperationEvent) bool {
	if w == nil || evt == nil {
		return false
	}
	if w.nodeID != "" && w.nodeID != evt.GetNodeId() {
		return false
	}
	if w.opID != "" && w.opID != evt.GetOperationId() {
		return false
	}
	return true
}

func (srv *server) getOperationState(id string) *operationState {
	srv.opMu.Lock()
	defer srv.opMu.Unlock()
	op, ok := srv.operations[id]
	if !ok {
		op = &operationState{}
		srv.operations[id] = op
	}
	return op
}

func (srv *server) broadcastOperationEvent(evt *cluster_controllerpb.OperationEvent) {
	if evt == nil {
		return
	}
	op := srv.getOperationState(evt.GetOperationId())
	op.mu.Lock()
	if op.created.IsZero() {
		op.created = time.Now()
	}
	if op.nodeID == "" && evt.GetNodeId() != "" {
		op.nodeID = evt.GetNodeId()
	}
	op.last = evt
	if evt.GetDone() {
		op.done = true
	}
	op.mu.Unlock()
	srv.watchMu.Lock()
	for w := range srv.watchers {
		if w.matches(evt) {
			select {
			case w.ch <- evt:
			default:
			}
		}
	}
	srv.watchMu.Unlock()
}

func (srv *server) newOperationEvent(opID, nodeID string, phase cluster_controllerpb.OperationPhase, message string, percent int32, done bool, errMsg string) *cluster_controllerpb.OperationEvent {
	return &cluster_controllerpb.OperationEvent{
		OperationId: opID,
		NodeId:      nodeID,
		Phase:       phase,
		Message:     message,
		Percent:     percent,
		Done:        done,
		Error:       errMsg,
		Ts:          timestamppb.Now(),
	}
}

func (srv *server) addWatcher(w *operationWatcher) {
	srv.watchMu.Lock()
	srv.watchers[w] = struct{}{}
	srv.watchMu.Unlock()
}

func (srv *server) removeWatcher(w *operationWatcher) {
	srv.watchMu.Lock()
	if _, ok := srv.watchers[w]; ok {
		delete(srv.watchers, w)
	}
	srv.watchMu.Unlock()
}

func (srv *server) WatchOperations(req *cluster_controllerpb.WatchOperationsRequest, stream cluster_controllerpb.ClusterControllerService_WatchOperationsServer) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	ctx := stream.Context()
	w := &operationWatcher{
		nodeID: req.GetNodeId(),
		opID:   req.GetOperationId(),
		ch:     make(chan *cluster_controllerpb.OperationEvent, 8),
	}
	srv.addWatcher(w)
	defer func() {
		srv.removeWatcher(w)
		close(w.ch)
	}()
	srv.opMu.Lock()
	for _, op := range srv.operations {
		op.mu.Lock()
		last := op.last
		op.mu.Unlock()
		if last != nil && w.matches(last) {
			select {
			case w.ch <- last:
			default:
			}
		}
	}
	srv.opMu.Unlock()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case evt := <-w.ch:
			if evt == nil {
				continue
			}
			if err := stream.Send(evt); err != nil {
				return err
			}
			if evt.GetDone() {
				return nil
			}
		}
	}
}

func (srv *server) cleanupTimedOutOperations() {
	now := time.Now()
	var expired []struct {
		id     string
		nodeID string
	}
	srv.opMu.Lock()
	for id, op := range srv.operations {
		op.mu.Lock()
		done := op.done
		created := op.created
		nodeID := op.nodeID
		op.mu.Unlock()
		if done || created.IsZero() || nodeID == "" {
			continue
		}
		if now.Sub(created) > operationTimeout {
			expired = append(expired, struct {
				id     string
				nodeID string
			}{id: id, nodeID: nodeID})
		}
	}
	srv.opMu.Unlock()
	for _, entry := range expired {
		evt := srv.newOperationEvent(entry.id, entry.nodeID, cluster_controllerpb.OperationPhase_OP_FAILED, "operation timed out", 0, true, "operation timed out")
		srv.broadcastOperationEvent(evt)
	}
}

func (srv *server) startOperationCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(operationCleanupInterval)
	safeGo("operation-cleanup", func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !srv.isLeader() {
					continue
				}
				srv.cleanupTimedOutOperations()
			}
		}
	})
}
