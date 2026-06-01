package adapter

import "context"

// StepTransport dispatches step execution requests to a remote node-agent.
// Implementations include gRPC (production) and in-memory (testing).
type StepTransport interface {
	// Dispatch sends a step request and blocks until the terminal result.
	// For async execution, the transport handles accept/progress/heartbeat
	// internally and only returns the terminal ResultEvent.
	Dispatch(ctx context.Context, req ExecuteStepRequest) (*ResultEvent, error)

	// Cancel requests cancellation of a running step attempt.
	Cancel(ctx context.Context, req CancelStepRequest) error
}

// NodeResolver maps a node ID to its agent endpoint address.
type NodeResolver interface {
	ResolveEndpoint(nodeID string) (string, error)
}

// NodeResolverFunc is a convenience adapter for NodeResolver.
type NodeResolverFunc func(nodeID string) (string, error)

func (f NodeResolverFunc) ResolveEndpoint(nodeID string) (string, error) {
	return f(nodeID)
}
