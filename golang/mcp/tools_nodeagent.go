package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

func registerNodeAgentTools(s *server) {

	// ── nodeagent_get_inventory ─────────────────────────────────────────────
	s.register(toolDef{
		Name:        "nodeagent_get_inventory",
		Description: "Get the node agent inventory including host identity and systemd unit statuses.",
		InputSchema: inputSchema{
			Type:       "object",
			Properties: map[string]propSchema{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, nodeAgentEndpoint())
		if err != nil {
			return nil, err
		}
		client := node_agentpb.NewNodeAgentServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.GetInventory(callCtx, &node_agentpb.GetInventoryRequest{})
		if err != nil {
			return nil, fmt.Errorf("GetInventory: %w", err)
		}

		inv := resp.GetInventory()
		if inv == nil {
			return map[string]interface{}{"identity": nil, "units": []interface{}{}}, nil
		}

		identity := map[string]interface{}{}
		if id := inv.GetIdentity(); id != nil {
			identity["hostname"] = id.GetHostname()
			identity["domain"] = id.GetDomain()
			identity["ips"] = id.GetIps()
			identity["os"] = id.GetOs()
			identity["arch"] = id.GetArch()
			identity["agent_version"] = id.GetAgentVersion()
		}

		units := make([]map[string]interface{}, 0, len(inv.GetUnits()))
		for _, u := range inv.GetUnits() {
			units = append(units, map[string]interface{}{
				"name":    u.GetName(),
				"state":   u.GetState(),
				"details": u.GetDetails(),
			})
		}

		return map[string]interface{}{
			"identity": identity,
			"units":    units,
		}, nil
	})

	// ── nodeagent_list_installed_packages ───────────────────────────────────
	s.register(toolDef{
		Name:        "nodeagent_list_installed_packages",
		Description: "List all packages installed on this node, with optional kind filter (SERVICE, APPLICATION, INFRASTRUCTURE, COMMAND, etc.).",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"kind": {
					Type:        "string",
					Description: "Optional package kind filter (e.g. SERVICE, APPLICATION, INFRASTRUCTURE, COMMAND).",
					Enum:        []string{"SERVICE", "APPLICATION", "AGENT", "SUBSYSTEM", "INFRASTRUCTURE", "COMMAND"},
				},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, nodeAgentEndpoint())
		if err != nil {
			return nil, err
		}
		client := node_agentpb.NewNodeAgentServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		req := &node_agentpb.ListInstalledPackagesRequest{
			Kind: strings.ToUpper(getStr(args, "kind")),
		}

		resp, err := client.ListInstalledPackages(callCtx, req)
		if err != nil {
			return nil, fmt.Errorf("ListInstalledPackages: %w", err)
		}

		pkgs := make([]map[string]interface{}, 0, len(resp.GetPackages()))
		for _, p := range resp.GetPackages() {
			pkgs = append(pkgs, normalizeInstalledPackage(p))
		}

		return map[string]interface{}{
			"count":    len(pkgs),
			"packages": pkgs,
		}, nil
	})

	// ── nodeagent_get_plan_status ───────────────────────────────────────────
	s.register(toolDef{
		Name:        "nodeagent_get_plan_status",
		Description: "Get the execution status of a node plan (convergence operation). Returns plan state, step statuses, and events.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"plan_id": {
					Type:        "string",
					Description: "Optional plan or operation ID. If omitted, returns the latest plan status.",
				},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, nodeAgentEndpoint())
		if err != nil {
			return nil, err
		}
		client := node_agentpb.NewNodeAgentServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		req := &node_agentpb.GetPlanStatusV1Request{
			OperationId: getStr(args, "plan_id"),
		}

		resp, err := client.GetPlanStatusV1(callCtx, req)
		if err != nil {
			return nil, fmt.Errorf("GetPlanStatusV1: %w", err)
		}

		st := resp.GetStatus()
		if st == nil {
			return map[string]interface{}{"status": "no plan found"}, nil
		}

		steps := make([]map[string]interface{}, 0, len(st.GetSteps()))
		for _, step := range st.GetSteps() {
			steps = append(steps, map[string]interface{}{
				"id":      step.GetId(),
				"state":   step.GetState().String(),
				"attempt": step.GetAttempt(),
				"started": fmtTime(int64(step.GetStartedUnixMs())),
				"finished": fmtTime(int64(step.GetFinishedUnixMs())),
				"message": step.GetMessage(),
			})
		}

		events := make([]map[string]interface{}, 0, len(st.GetEvents()))
		for _, ev := range st.GetEvents() {
			events = append(events, map[string]interface{}{
				"timestamp": fmtTime(int64(ev.GetTsUnixMs())),
				"level":     ev.GetLevel(),
				"message":   ev.GetMsg(),
				"step_id":   ev.GetStepId(),
			})
		}

		result := map[string]interface{}{
			"plan_id":        st.GetPlanId(),
			"node_id":        st.GetNodeId(),
			"generation":     st.GetGeneration(),
			"state":          st.GetState().String(),
			"current_step":   st.GetCurrentStepId(),
			"error_message":  st.GetErrorMessage(),
			"error_step_id":  st.GetErrorStepId(),
			"started":        fmtTime(int64(st.GetStartedUnixMs())),
			"finished":       fmtTime(int64(st.GetFinishedUnixMs())),
			"steps":          steps,
			"events":         events,
		}

		return result, nil
	})

	// ── nodeagent_get_installed_package ─────────────────────────────────────
	s.register(toolDef{
		Name:        "nodeagent_get_installed_package",
		Description: "Get details of a specific installed package by name.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"name": {
					Type:        "string",
					Description: "Package name (required).",
				},
				"kind": {
					Type:        "string",
					Description: "Optional package kind filter (e.g. SERVICE, INFRASTRUCTURE).",
					Enum:        []string{"SERVICE", "APPLICATION", "AGENT", "SUBSYSTEM", "INFRASTRUCTURE", "COMMAND"},
				},
			},
			Required: []string{"name"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		name := getStr(args, "name")
		if name == "" {
			return nil, fmt.Errorf("missing required argument: name")
		}

		conn, err := s.clients.get(ctx, nodeAgentEndpoint())
		if err != nil {
			return nil, err
		}
		client := node_agentpb.NewNodeAgentServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		req := &node_agentpb.GetInstalledPackageRequest{
			Name: name,
			Kind: strings.ToUpper(getStr(args, "kind")),
		}

		resp, err := client.GetInstalledPackage(callCtx, req)
		if err != nil {
			return nil, fmt.Errorf("GetInstalledPackage: %w", err)
		}

		pkg := resp.GetPackage()
		if pkg == nil {
			return map[string]interface{}{"error": "package not found"}, nil
		}

		return normalizeInstalledPackage(pkg), nil
	})

	// ── nodeagent_get_service_logs ─────────────────────────────────────────
	s.register(toolDef{
		Name:        "nodeagent_get_service_logs",
		Description: "Read recent journalctl log output for a Globular systemd service. Unit name must start with 'globular-'. Returns up to 200 lines. Use this to investigate service failures, startup errors, or runtime issues without SSH access.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"unit": {Type: "string", Description: "Systemd unit name (must start with 'globular-', e.g. 'globular-gateway.service')"},
				"lines": {Type: "number", Description: "Number of log lines to return (default 50, max 200)"},
				"priority": {
					Type:        "string",
					Description: "Optional journalctl priority filter",
					Enum:        []string{"emerg", "alert", "crit", "err", "warning", "notice", "info", "debug"},
				},
			},
			Required: []string{"unit"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		unit := getStr(args, "unit")
		if unit == "" {
			return nil, fmt.Errorf("unit is required")
		}
		if !strings.HasPrefix(unit, "globular-") {
			return nil, fmt.Errorf("unit must start with 'globular-'")
		}

		lines := getInt(args, "lines", 50)
		priority := getStr(args, "priority")

		conn, err := s.clients.get(ctx, nodeAgentEndpoint())
		if err != nil {
			return nil, err
		}
		client := node_agentpb.NewNodeAgentServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 15*time.Second)
		defer cancel()

		resp, err := client.GetServiceLogs(callCtx, &node_agentpb.GetServiceLogsRequest{
			Unit:     unit,
			Lines:    int32(lines),
			Priority: priority,
		})
		if err != nil {
			return nil, fmt.Errorf("GetServiceLogs: %w", err)
		}

		return map[string]interface{}{
			"unit":       resp.GetUnit(),
			"line_count": resp.GetLineCount(),
			"lines":      resp.GetLines(),
		}, nil
	})

	// ── nodeagent_get_certificate_status ───────────────────────────────────
	s.register(toolDef{
		Name:        "nodeagent_get_certificate_status",
		Description: "Returns TLS certificate status for the node: server cert and CA cert details including subject, issuer, SANs, expiry date, days until expiry, chain validity, and SHA256 fingerprint. Use this to diagnose TLS issues or check certificate rotation needs.",
		InputSchema: inputSchema{
			Type:       "object",
			Properties: map[string]propSchema{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, nodeAgentEndpoint())
		if err != nil {
			return nil, err
		}
		client := node_agentpb.NewNodeAgentServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.GetCertificateStatus(callCtx, &node_agentpb.GetCertificateStatusRequest{})
		if err != nil {
			return nil, fmt.Errorf("GetCertificateStatus: %w", err)
		}

		result := map[string]interface{}{}

		if sc := resp.GetServerCert(); sc != nil {
			result["server_cert"] = normalizeCertInfo(sc)
		}
		if ca := resp.GetCaCert(); ca != nil {
			result["ca_cert"] = normalizeCertInfo(ca)
		}

		return result, nil
	})
}

func normalizeCertInfo(c *node_agentpb.CertificateInfo) map[string]interface{} {
	return map[string]interface{}{
		"subject":           c.GetSubject(),
		"issuer":            c.GetIssuer(),
		"sans":              c.GetSans(),
		"not_before":        c.GetNotBefore(),
		"not_after":         c.GetNotAfter(),
		"days_until_expiry": c.GetDaysUntilExpiry(),
		"chain_valid":       c.GetChainValid(),
		"fingerprint":       c.GetFingerprint(),
	}
}

// normalizeInstalledPackage converts a protobuf InstalledPackage to a normalized map.
func normalizeInstalledPackage(p *node_agentpb.InstalledPackage) map[string]interface{} {
	return map[string]interface{}{
		"name":         p.GetName(),
		"version":      p.GetVersion(),
		"publisher":    p.GetPublisherId(),
		"platform":     p.GetPlatform(),
		"kind":         p.GetKind(),
		"status":       p.GetStatus(),
		"checksum":     p.GetChecksum(),
		"installed_at": fmtTime(p.GetInstalledUnix()),
		"updated_at":   fmtTime(p.GetUpdatedUnix()),
	}
}
