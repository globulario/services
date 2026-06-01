package adapter

import (
	"context"
	"fmt"
	"sync"
)

// MemoryTransport is an in-process StepTransport for testing and
// co-located controller/node-agent scenarios. It dispatches directly
// to a StepExecutor without network overhead.
type MemoryTransport struct {
	mu        sync.RWMutex
	executors map[string]*StepExecutor // key: nodeID
}

// NewMemoryTransport creates an in-memory transport.
func NewMemoryTransport() *MemoryTransport {
	return &MemoryTransport{
		executors: make(map[string]*StepExecutor),
	}
}

// RegisterNode adds a node-agent executor to the transport.
func (mt *MemoryTransport) RegisterNode(nodeID string, executor *StepExecutor) {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	mt.executors[nodeID] = executor
}

// Dispatch sends a step request to the target node's executor.
func (mt *MemoryTransport) Dispatch(ctx context.Context, req ExecuteStepRequest) (*ResultEvent, error) {
	nodeID := req.Identity.NodeID
	mt.mu.RLock()
	executor, ok := mt.executors[nodeID]
	mt.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("node %s not registered in memory transport", nodeID)
	}
	result := executor.Execute(ctx, req)
	return result, nil
}

// Cancel forwards a cancel request to the target node's executor.
func (mt *MemoryTransport) Cancel(ctx context.Context, req CancelStepRequest) error {
	nodeID := req.Identity.NodeID
	mt.mu.RLock()
	executor, ok := mt.executors[nodeID]
	mt.mu.RUnlock()
	if !ok {
		return fmt.Errorf("node %s not registered in memory transport", nodeID)
	}
	executor.Cancel(req)
	return nil
}
