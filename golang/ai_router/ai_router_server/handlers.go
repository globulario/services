package main

import (
	"context"
	"time"

	"github.com/globulario/services/golang/ai_router/ai_routerpb"
)

// GetRoutingPolicy returns the current routing policy.
// In Phase 0 (neutral mode): returns empty policy with confidence=0.
// The xDS watcher treats confidence=0 as "no opinion, use defaults."
func (srv *server) GetRoutingPolicy(_ context.Context, req *ai_routerpb.GetRoutingPolicyRequest) (*ai_routerpb.GetRoutingPolicyResponse, error) {
	srv.statsMu.Lock()
	srv.stats.PoliciesComputed++
	srv.stats.LastPolicyAt = time.Now()
	srv.statsMu.Unlock()

	// Check for cached policy from scoring loop (Phase 1+).
	if cached := srv.cachedPolicy.Load(); cached != nil {
		// If a specific service is requested, filter.
		if svc := req.GetServiceName(); svc != "" {
			if sp, ok := cached.Services[svc]; ok {
				return &ai_routerpb.GetRoutingPolicyResponse{
					Policy: &ai_routerpb.RoutingPolicy{
						Services:     map[string]*ai_routerpb.ServicePolicy{svc: sp},
						Generation:   cached.Generation,
						ComputedAtMs: cached.ComputedAtMs,
						Mode:         cached.Mode,
					},
				}, nil
			}
		}
		return &ai_routerpb.GetRoutingPolicyResponse{Policy: cached}, nil
	}

	// Phase 0: neutral — no opinion.
	srv.modeMu.RLock()
	mode := srv.mode
	srv.modeMu.RUnlock()

	return &ai_routerpb.GetRoutingPolicyResponse{
		Policy: &ai_routerpb.RoutingPolicy{
			Services:     map[string]*ai_routerpb.ServicePolicy{},
			Generation:   0,
			ComputedAtMs: time.Now().UnixMilli(),
			Mode:         mode,
		},
	}, nil
}

// GetStatus returns operational status.
func (srv *server) GetStatus(_ context.Context, _ *ai_routerpb.GetStatusRequest) (*ai_routerpb.GetStatusResponse, error) {
	srv.modeMu.RLock()
	mode := srv.mode
	srv.modeMu.RUnlock()

	srv.statsMu.Lock()
	stats := srv.stats
	srv.statsMu.Unlock()

	return &ai_routerpb.GetStatusResponse{
		Mode:              mode,
		Version:           srv.Version,
		UptimeSeconds:     int64(time.Since(srv.startedAt).Seconds()),
		PoliciesComputed:  stats.PoliciesComputed,
		PoliciesApplied:   stats.PoliciesApplied,
		LastPolicyAtMs:    stats.LastPolicyAt.UnixMilli(),
		LastMetricsAtMs:   stats.LastMetricsAt.UnixMilli(),
		ServicesTracked:   int32(len(srv.classifications)),
		EndpointsTracked:  0, // Phase 1: populated from collector
	}, nil
}

// SetMode switches the router between neutral, observe, and active.
func (srv *server) SetMode(_ context.Context, req *ai_routerpb.SetModeRequest) (*ai_routerpb.SetModeResponse, error) {
	srv.modeMu.Lock()
	previous := srv.mode
	srv.mode = req.GetMode()
	srv.modeMu.Unlock()

	logger.Info("mode changed",
		"previous", previous.String(),
		"current", req.GetMode().String(),
	)

	return &ai_routerpb.SetModeResponse{
		PreviousMode: previous,
		CurrentMode:  req.GetMode(),
	}, nil
}

// GetServiceClassifications returns the current service class map.
func (srv *server) GetServiceClassifications(_ context.Context, _ *ai_routerpb.GetServiceClassificationsRequest) (*ai_routerpb.GetServiceClassificationsResponse, error) {
	return &ai_routerpb.GetServiceClassificationsResponse{
		Classifications: srv.classifications,
	}, nil
}

// Stop gracefully shuts down the router.
func (srv *server) Stop(_ context.Context, _ *ai_routerpb.StopRequest) (*ai_routerpb.StopResponse, error) {
	return &ai_routerpb.StopResponse{}, srv.StopService()
}
