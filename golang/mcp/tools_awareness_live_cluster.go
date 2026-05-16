package main

import (
	"context"
	"fmt"

	"github.com/globulario/awareness/livecluster"
	cluster_controllerpb "github.com/globulario/awareness/cluster_controllerpb"
	cluster_doctorpb "github.com/globulario/awareness/cluster_doctorpb"
)

// liveCollectors returns the live signal collectors the MCP server exposes.
// They dial through the shared gRPC pool with the same auth context the
// rest of the awareness tools use. Each collector degrades to "unavailable"
// on transport failure — see livecluster.CollectClusterSignals.
func liveCollectors(s *server) []livecluster.SignalCollector {
	return []livecluster.SignalCollector{
		livecluster.NewDoctorCollector("doctor", func(ctx context.Context) (cluster_doctorpb.ClusterDoctorServiceClient, func(), error) {
			conn, err := s.clients.get(ctx, doctorEndpoint())
			if err != nil {
				return nil, nil, err
			}
			return cluster_doctorpb.NewClusterDoctorServiceClient(conn), nil, nil
		}),
		livecluster.NewControllerCollector("controller", func(ctx context.Context) (cluster_controllerpb.ClusterControllerServiceClient, func(), error) {
			conn, err := s.clients.get(ctx, controllerEndpoint())
			if err != nil {
				return nil, nil, err
			}
			return cluster_controllerpb.NewClusterControllerServiceClient(conn), nil, nil
		}),
	}
}

func registerAwarenessLiveClusterTools(s *server, st *awarenessState) {
	s.register(toolDef{
		Name:        "awareness.live.collect_signals",
		Description: "Collect live cluster signals (service health, errors, convergence, incidents) from all registered sources. Returns a point-in-time snapshot.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"session_id":       {Type: "string", Description: "Session ID for correlation"},
				"task":             {Type: "string", Description: "What task is being checked"},
				"files":            {Type: "array", Items: &propSchema{Type: "string"}, Description: "Source files being changed — used to derive components/services"},
				"components":       {Type: "array", Items: &propSchema{Type: "string"}, Description: "Explicit component names to scope the check"},
				"services":         {Type: "array", Items: &propSchema{Type: "string"}, Description: "Explicit service names to scope the check"},
				"lookback_hours":   {Type: "integer", Description: "Error lookback window in hours (default 24)"},
				"require_live_data": {Type: "boolean", Description: "If true, block when no live sources are available"},
				"cluster_id":       {Type: "string", Description: "Cluster ID (stored with snapshot for retrieval)"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		req := livecluster.CollectSignalsRequest{
			ClusterID:       strArg(args, "cluster_id"),
			SessionID:       strArg(args, "session_id"),
			Task:            strArg(args, "task"),
			Files:           strSliceArg(args, "files"),
			Components:      strSliceArg(args, "components"),
			Services:        strSliceArg(args, "services"),
			LookbackHours:   intArgDefault(args, "lookback_hours", 24),
			RequireLiveData: boolArg(args, "require_live_data"),
		}
		snap, err := livecluster.CollectClusterSignals(authCtx(ctx), req, liveCollectors(s))
		if err != nil {
			return nil, err
		}
		st2 := livecluster.NewStore(st.g)
		_ = st2.StoreClusterSignalSnapshot(ctx, snap)
		return map[string]interface{}{
			"snapshot_id": snap.ID,
			"status":      snap.Status,
			"summary":     snap.Summary,
			"services":    len(snap.Services),
			"errors":      len(snap.Errors),
			"convergence": len(snap.Convergence),
			"incidents":   len(snap.Incidents),
			"sources":     len(snap.Sources),
		}, nil
	})

	s.register(toolDef{
		Name:        "awareness.live.preflight",
		Description: "Run live preflight — combines static awareness graph context with live cluster signals to produce a verdict (allow / allow_with_warnings / block / unknown) before a code edit.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"task":              {Type: "string", Description: "What you are about to do"},
				"files":             {Type: "array", Items: &propSchema{Type: "string"}, Description: "Files you plan to change"},
				"components":        {Type: "array", Items: &propSchema{Type: "string"}, Description: "Components in scope (derived from files if omitted)"},
				"services":          {Type: "array", Items: &propSchema{Type: "string"}, Description: "Services in scope"},
				"session_id":        {Type: "string", Description: "Session ID for correlation"},
				"static_result_id":  {Type: "string", Description: "ID of the static preflight result to associate"},
				"lookback_hours":    {Type: "integer", Description: "Error lookback window in hours (default 24)"},
				"require_live_data": {Type: "boolean", Description: "If true, block when no live sources are available"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		req := livecluster.LivePreflightRequest{
			SessionID:       strArg(args, "session_id"),
			Task:            strArg(args, "task"),
			Files:           strSliceArg(args, "files"),
			Components:      strSliceArg(args, "components"),
			Services:        strSliceArg(args, "services"),
			StaticResultID:  strArg(args, "static_result_id"),
			LookbackHours:   intArgDefault(args, "lookback_hours", 24),
			RequireLiveData: boolArg(args, "require_live_data"),
		}
		st2 := livecluster.NewStore(st.g)
		r, err := livecluster.RunLivePreflight(authCtx(ctx), st.g, st2, liveCollectors(s), req)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"result_id":     r.ID,
			"verdict":       r.Verdict,
			"severity":      r.Severity,
			"summary":       r.Summary,
			"blockers":      r.Blockers,
			"warnings":      r.Warnings,
			"confirmations": r.Confirmations,
			"snapshot_id":   r.SignalSnapshotID,
			"live_section":  livecluster.FormatLiveSection(r),
		}, nil
	})

	s.register(toolDef{
		Name:        "awareness.live.latest_snapshot",
		Description: "Retrieve the most recently stored cluster signal snapshot for a cluster ID.",
		InputSchema: inputSchema{
			Type:     "object",
			Required: []string{"cluster_id"},
			Properties: map[string]propSchema{
				"cluster_id": {Type: "string", Description: "Cluster ID to look up"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		clusterID := strArg(args, "cluster_id")
		if clusterID == "" {
			return nil, fmt.Errorf("cluster_id is required")
		}
		st2 := livecluster.NewStore(st.g)
		snap, err := st2.GetLatestClusterSignalSnapshot(ctx, clusterID)
		if err != nil {
			return nil, err
		}
		return snap, nil
	})

	s.register(toolDef{
		Name:        "awareness.live.service_health",
		Description: "Get service health states from the latest stored cluster signal snapshot.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"cluster_id": {Type: "string", Description: "Cluster ID (default: uses any available snapshot)"},
				"services":   {Type: "array", Items: &propSchema{Type: "string"}, Description: "Filter to these service names (empty = all)"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		clusterID := strArg(args, "cluster_id")
		filterServices := strSliceArg(args, "services")

		st2 := livecluster.NewStore(st.g)
		snap, err := st2.GetLatestClusterSignalSnapshot(ctx, clusterID)
		if err != nil {
			return map[string]interface{}{
				"error":   err.Error(),
				"message": "no snapshot available — run awareness.live.collect_signals first",
			}, nil
		}

		services := snap.Services
		if len(filterServices) > 0 {
			var filtered []livecluster.ServiceLiveState
			for _, svc := range services {
				for _, name := range filterServices {
					if svc.ServiceName == name || svc.Component == name {
						filtered = append(filtered, svc)
						break
					}
				}
			}
			services = filtered
		}

		unhealthy := 0
		for _, svc := range services {
			if svc.Health != "healthy" && svc.Health != "unknown" {
				unhealthy++
			}
		}
		return map[string]interface{}{
			"snapshot_id":      snap.ID,
			"snapshot_status":  snap.Status,
			"collected_at":     snap.CollectedAt,
			"services":         services,
			"total":            len(services),
			"unhealthy_count":  unhealthy,
		}, nil
	})

	s.register(toolDef{
		Name:        "awareness.live.convergence",
		Description: "Get runtime convergence states from the latest stored cluster signal snapshot.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"cluster_id": {Type: "string", Description: "Cluster ID"},
				"components": {Type: "array", Items: &propSchema{Type: "string"}, Description: "Filter to these component names (empty = all)"},
				"status_filter": {Type: "string", Description: "Only return entries with this convergence status (e.g. stuck, diverged, pending)"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if st.g == nil {
			return nil, fmt.Errorf("awareness graph unavailable")
		}
		clusterID := strArg(args, "cluster_id")
		filterComps := strSliceArg(args, "components")
		statusFilter := strArg(args, "status_filter")

		st2 := livecluster.NewStore(st.g)
		snap, err := st2.GetLatestClusterSignalSnapshot(ctx, clusterID)
		if err != nil {
			return map[string]interface{}{
				"error":   err.Error(),
				"message": "no snapshot available — run awareness.live.collect_signals first",
			}, nil
		}

		convergence := snap.Convergence
		if len(filterComps) > 0 {
			var filtered []livecluster.RuntimeConvergenceState
			for _, c := range convergence {
				for _, name := range filterComps {
					if c.Component == name {
						filtered = append(filtered, c)
						break
					}
				}
			}
			convergence = filtered
		}
		if statusFilter != "" {
			var filtered []livecluster.RuntimeConvergenceState
			for _, c := range convergence {
				if c.ConvergenceStatus == statusFilter {
					filtered = append(filtered, c)
				}
			}
			convergence = filtered
		}

		stuck := 0
		for _, c := range convergence {
			if c.ConvergenceStatus == "stuck" || c.ConvergenceStatus == "blocked" || c.ConvergenceStatus == "diverged" {
				stuck++
			}
		}
		return map[string]interface{}{
			"snapshot_id":  snap.ID,
			"collected_at": snap.CollectedAt,
			"convergence":  convergence,
			"total":        len(convergence),
			"stuck_count":  stuck,
		}, nil
	})
}
