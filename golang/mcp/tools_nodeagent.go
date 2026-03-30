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

	// ── nodeagent_installed_set ─────────────────────────────────────────────
	s.register(toolDef{
		Name:        "nodeagent_installed_set",
		Description: "Set or update an installed package record on this node. Use this to register a package as installed, fix missing state, or update version/status. Writes directly to the node's etcd installed-state registry.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"name":         {Type: "string", Description: "Package name (e.g. 'mcp', 'gateway')"},
				"version":      {Type: "string", Description: "Package version (e.g. '0.0.1')"},
				"kind":         {Type: "string", Description: "Package kind", Enum: []string{"SERVICE", "APPLICATION", "AGENT", "SUBSYSTEM", "INFRASTRUCTURE", "COMMAND"}},
				"platform":     {Type: "string", Description: "Target platform (default: 'linux_amd64')"},
				"status":       {Type: "string", Description: "Package status (default: 'installed')", Enum: []string{"installed", "updating", "failed", "removing"}},
				"publisher_id": {Type: "string", Description: "Publisher identifier (default: 'core@globular.io')"},
				"checksum":     {Type: "string", Description: "Optional SHA256 checksum of the installed archive"},
				"build_number": {Type: "number", Description: "Build iteration within version (default: 0)"},
			},
			Required: []string{"name", "version", "kind"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		name := getStr(args, "name")
		version := getStr(args, "version")
		kind := strings.ToUpper(getStr(args, "kind"))
		if name == "" || version == "" || kind == "" {
			return nil, fmt.Errorf("name, version, and kind are required")
		}

		platform := getStr(args, "platform")
		if platform == "" {
			platform = "linux_amd64"
		}
		status := getStr(args, "status")
		if status == "" {
			status = "installed"
		}
		publisher := getStr(args, "publisher_id")
		if publisher == "" {
			publisher = "core@globular.io"
		}

		conn, err := s.clients.get(ctx, nodeAgentEndpoint())
		if err != nil {
			return nil, err
		}
		client := node_agentpb.NewNodeAgentServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		req := &node_agentpb.SetInstalledPackageRequest{
			Package: &node_agentpb.InstalledPackage{
				Name:        name,
				Version:     version,
				Kind:        kind,
				Platform:    platform,
				Status:      status,
				PublisherId: publisher,
				Checksum:    getStr(args, "checksum"),
				BuildNumber: int64(getInt(args, "build_number", 0)),
			},
		}

		resp, err := client.SetInstalledPackage(callCtx, req)
		if err != nil {
			return nil, fmt.Errorf("SetInstalledPackage: %w", err)
		}

		return map[string]interface{}{
			"ok":      resp.GetOk(),
			"message": resp.GetMessage(),
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

	// ── nodeagent_control_service ──────────────────────────────────────────
	s.register(toolDef{
		Name:        "nodeagent_control_service",
		Description: "Control a Globular systemd service: restart, stop, start, or check status. Only globular-* and scylla-* units are allowed. Requires admin permission.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"unit":   {Type: "string", Description: "Systemd unit name (e.g. 'globular-gateway.service', 'globular-dns', 'scylla-server.service')"},
				"action": {Type: "string", Description: "Action to perform", Enum: []string{"restart", "stop", "start", "status"}},
			},
			Required: []string{"unit", "action"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		unit := getStr(args, "unit")
		action := getStr(args, "action")
		if unit == "" || action == "" {
			return nil, fmt.Errorf("unit and action are required")
		}

		conn, err := s.clients.get(ctx, nodeAgentEndpoint())
		if err != nil {
			return nil, err
		}
		client := node_agentpb.NewNodeAgentServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 30*time.Second)
		defer cancel()

		resp, err := client.ControlService(callCtx, &node_agentpb.ControlServiceRequest{
			Unit:   unit,
			Action: action,
		})
		if err != nil {
			return nil, fmt.Errorf("ControlService: %w", err)
		}

		return map[string]interface{}{
			"ok":      resp.GetOk(),
			"unit":    resp.GetUnit(),
			"action":  resp.GetAction(),
			"state":   resp.GetState(),
			"message": resp.GetMessage(),
		}, nil
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

	// ── nodeagent_search_logs ──────────────────────────────────────────────
	s.register(toolDef{
		Name: "nodeagent_search_logs",
		Description: `Search service logs by time range, pattern, and severity. Uses journalctl under the hood with regex grep support.

Examples:
- Search for errors in the last hour: unit="globular-gateway", since="1h ago", priority="err"
- Find TLS issues: unit="globular-dns", pattern="tls|certificate|cert", since="30m ago"
- Search all severity in a time window: unit="globular-rbac", since="2026-03-25 00:00:00", until="2026-03-25 01:00:00"`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"unit":     {Type: "string", Description: "Systemd unit name (e.g. 'globular-gateway.service', 'globular-dns', 'scylla-server')"},
				"pattern":  {Type: "string", Description: "Regex pattern to search for (e.g. 'error|fail|timeout', 'tls.*cert')"},
				"since":    {Type: "string", Description: "Start of time range (e.g. '5m ago', '1h ago', '2026-03-25 00:00:00')"},
				"until":    {Type: "string", Description: "End of time range (optional, e.g. '30m ago', '2026-03-25 01:00:00')"},
				"priority": {Type: "string", Description: "Severity filter", Enum: []string{"emerg", "alert", "crit", "err", "warning", "notice", "info", "debug"}},
				"limit":    {Type: "number", Description: "Max lines to return (default 100, max 500)"},
				"case_sensitive": {Type: "boolean", Description: "Case-sensitive pattern matching (default: false)"},
			},
			Required: []string{"unit"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		unit := getStr(args, "unit")
		if unit == "" {
			return nil, fmt.Errorf("unit is required")
		}

		conn, err := s.clients.get(ctx, nodeAgentEndpoint())
		if err != nil {
			return nil, err
		}
		client := node_agentpb.NewNodeAgentServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 30*time.Second)
		defer cancel()

		resp, err := client.SearchServiceLogs(callCtx, &node_agentpb.SearchServiceLogsRequest{
			Unit:          unit,
			Pattern:       getStr(args, "pattern"),
			Since:         getStr(args, "since"),
			Until:         getStr(args, "until"),
			Priority:      getStr(args, "priority"),
			Limit:         int32(getInt(args, "limit", 100)),
			CaseSensitive: getBool(args, "case_sensitive", false),
		})
		if err != nil {
			return nil, fmt.Errorf("SearchServiceLogs: %w", err)
		}

		result := map[string]interface{}{
			"unit":        resp.GetUnit(),
			"match_count": resp.GetMatchCount(),
			"lines":       resp.GetLines(),
		}
		if resp.GetSince() != "" {
			result["since"] = resp.GetSince()
		}
		if resp.GetUntil() != "" {
			result["until"] = resp.GetUntil()
		}
		if resp.GetTruncated() {
			result["truncated"] = true
		}
		return result, nil
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
