package behavioral_backfill

import (
	"context"

	ai_memorypb "github.com/globulario/services/golang/ai_memory/ai_memorypb"
)

// rpcSource reads memories through the AiMemoryService Query RPC. Run applies the
// type/tag/since/agent filters client-side, so this source only needs to page by
// project.
type rpcSource struct {
	client ai_memorypb.AiMemoryServiceClient
}

// NewRPCSource wraps an AiMemoryService client as a MemorySource.
func NewRPCSource(client ai_memorypb.AiMemoryServiceClient) MemorySource {
	return &rpcSource{client: client}
}

func (r *rpcSource) Query(ctx context.Context, f MemoryFilter) ([]*ai_memorypb.Memory, error) {
	limit := int32(f.Limit)
	if limit <= 0 {
		limit = 1000
	}
	rsp, err := r.client.Query(ctx, &ai_memorypb.QueryRqst{Project: f.Project, Limit: limit})
	if err != nil {
		return nil, err
	}
	return rsp.GetMemories(), nil
}
