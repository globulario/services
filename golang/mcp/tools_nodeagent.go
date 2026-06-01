package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// tokenPatterns matches sensitive values commonly found in service logs.
var tokenPatterns = []*regexp.Regexp{
	// JWT tokens (header.payload.signature)
	regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`),
	// Bearer <token>
	regexp.MustCompile(`(?i)(bearer\s+)\S+`),
	// Authorization header values
	regexp.MustCompile(`(?i)(authorization[=:]\s*)\S+`),
	// key=value for sensitive keys (token, password, secret, api_key, etc.)
	regexp.MustCompile(`(?i)((?:token|password|passwd|secret|api_key|apikey|access_key|private_key|refresh_token|session_token|auth_token)[=:]\s*)\S+`),
}

// tokenReplacements are the corresponding replacement strings.
var tokenReplacements = []string{
	"[REDACTED_JWT]",
	"${1}[REDACTED]",
	"${1}[REDACTED]",
	"${1}[REDACTED]",
}

// journalPrefix matches the typical journalctl default output prefix:
//   "Apr 07 14:23:01 globule-ryzen globular-gateway[12345]: actual message"
// We capture: (1) timestamp, (2) the rest after stripping host+unit+PID.
var journalPrefix = regexp.MustCompile(
	`^([A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})\s+\S+\s+\S+\[\d+\]:\s*(.*)$`,
)

// noisePatterns strips verbose, low-value fragments from log messages.
var noisePatterns = []*regexp.Regexp{
	// Go source locations: server.go:123
	regexp.MustCompile(`\s+\S+\.go:\d+`),
	// Full goroutine IDs, thread IDs
	regexp.MustCompile(`(?i)\s*goroutine\s+\d+`),
	// Repeated "level=info", "level=debug" etc. (structured loggers)
	regexp.MustCompile(`(?i)\blevel=\w+\s*`),
	// "ts=2026-04-07T..." redundant structured timestamp
	regexp.MustCompile(`\bts=\S+\s*`),
	// "caller=xxx.go:nn" structured caller field
	regexp.MustCompile(`\bcaller=\S+\s*`),
	// Consecutive spaces left behind by stripping
	regexp.MustCompile(`\s{2,}`),
}

// msgFingerprint extracts a canonical key from a log line for dedup.
// Strips the leading HH:MM:SS timestamp and any hex IDs / UUIDs / numbers
// so lines differing only by request-id or counter are grouped.
var (
	fpTimestamp = regexp.MustCompile(`^\d{2}:\d{2}:\d{2}\s+`)
	fpHexIDs    = regexp.MustCompile(`\b[0-9a-fA-F]{8,}\b`)
	fpUUIDs     = regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`)
	fpNumbers   = regexp.MustCompile(`\b\d{2,}\b`)
)

func msgFingerprint(line string) string {
	s := fpTimestamp.ReplaceAllString(line, "")
	s = fpUUIDs.ReplaceAllString(s, "_")
	s = fpHexIDs.ReplaceAllString(s, "_")
	s = fpNumbers.ReplaceAllString(s, "_")
	return s
}

// compactLogLines redacts secrets, strips journalctl noise, deduplicates by
// message fingerprint, and shortens timestamps.
// Returns compressed lines + a count of suppressed duplicates.
func compactLogLines(raw []string) ([]string, int) {
	// Phase 1: redact + strip prefix + strip noise
	type entry struct {
		ts  string // "HH:MM:SS" or ""
		msg string // cleaned message body
	}
	cleaned := make([]entry, 0, len(raw))
	for _, line := range raw {
		if line == "" || line == "-- No entries --" {
			continue
		}
		s := line
		// Redact tokens
		for j, pat := range tokenPatterns {
			s = pat.ReplaceAllString(s, tokenReplacements[j])
		}
		// Strip journalctl prefix → "HH:MM:SS message"
		ts := ""
		if m := journalPrefix.FindStringSubmatch(s); m != nil {
			raw := m[1]
			if idx := strings.LastIndex(raw, " "); idx >= 0 {
				ts = raw[idx+1:]
			}
			s = m[2]
		}
		// Strip verbose noise fragments
		for _, np := range noisePatterns {
			s = np.ReplaceAllString(s, " ")
		}
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		cleaned = append(cleaned, entry{ts: ts, msg: s})
	}

	// Phase 2: deduplicate by message fingerprint (preserves first occurrence order)
	type group struct {
		first int // index of first occurrence
		count int
		tsMin string // earliest timestamp
		tsMax string // latest timestamp
	}
	seen := make(map[string]*group)
	order := make([]string, 0, len(cleaned))

	for i, e := range cleaned {
		fp := msgFingerprint(e.msg)
		if g, ok := seen[fp]; ok {
			g.count++
			if e.ts != "" {
				g.tsMax = e.ts
			}
		} else {
			g := &group{first: i, count: 1, tsMin: e.ts, tsMax: e.ts}
			seen[fp] = g
			order = append(order, fp)
		}
	}

	// Phase 3: emit compact output
	out := make([]string, 0, len(order))
	suppressed := 0
	for _, fp := range order {
		g := seen[fp]
		e := cleaned[g.first]
		var line string
		if g.count > 1 {
			suppressed += g.count - 1
			timeRange := ""
			if g.tsMin != "" && g.tsMax != "" && g.tsMin != g.tsMax {
				timeRange = g.tsMin + ".." + g.tsMax + " "
			} else if g.tsMin != "" {
				timeRange = g.tsMin + " "
			}
			line = fmt.Sprintf("%s%s (×%d)", timeRange, e.msg, g.count)
		} else {
			if e.ts != "" {
				line = e.ts + " " + e.msg
			} else {
				line = e.msg
			}
		}
		out = append(out, line)
	}
	return out, suppressed
}

// resolveNodeAgentEndpoint returns the gRPC endpoint for a node's agent.
// If nodeID is empty, returns the local node-agent endpoint.
// If nodeID is provided, queries the cluster-controller for the node's
// agent endpoint and returns it for direct dial.
func (s *server) resolveNodeAgentEndpoint(ctx context.Context, nodeID string) (string, error) {
	if nodeID == "" {
		return nodeAgentEndpoint(), nil
	}

	// Query the controller for the node's agent endpoint.
	conn, err := s.clients.get(ctx, controllerEndpoint())
	if err != nil {
		return "", fmt.Errorf("connect to controller: %w", err)
	}
	client := cluster_controllerpb.NewClusterControllerServiceClient(conn)

	callCtx, cancel := context.WithTimeout(authCtx(ctx), 5*time.Second)
	defer cancel()

	resp, err := client.ListNodes(callCtx, &cluster_controllerpb.ListNodesRequest{})
	if err != nil {
		return "", fmt.Errorf("ListNodes: %w", err)
	}

	for _, node := range resp.GetNodes() {
		if node.GetNodeId() == nodeID {
			ep := node.GetAgentEndpoint()
			if ep != "" {
				return ep, nil
			}
				return "", fmt.Errorf("node %s has no agent endpoint registered in etcd", nodeID)
		}
	}
	return "", fmt.Errorf("node %s not found", nodeID)
}

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
		Description: "Read recent journalctl log output for a Globular systemd service. Unit name must start with 'globular-'. Returns compact, deduplicated output. Start with the default (10 lines) and only increase if a summary tool shows more context is needed.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"unit":    {Type: "string", Description: "Systemd unit name (must start with 'globular-', e.g. 'globular-gateway.service')"},
				"lines":   {Type: "number", Description: "Number of raw lines to fetch (default 10, max 50). Output may be fewer after dedup."},
				"node_id": {Type: "string", Description: "Optional: query a remote node's logs by node ID. If omitted, reads from the local node."},
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

		lines := getInt(args, "lines", 10)
		if lines > 50 {
			lines = 50
		}
		priority := getStr(args, "priority")
		nodeID := getStr(args, "node_id")

		endpoint, err := s.resolveNodeAgentEndpoint(ctx, nodeID)
		if err != nil {
			return nil, fmt.Errorf("resolve node agent: %w", err)
		}

		conn, err := s.clients.get(ctx, endpoint)
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

		compact, suppressed := compactLogLines(resp.GetLines())
		result := map[string]interface{}{
			"unit":       resp.GetUnit(),
			"line_count": len(compact),
			"lines":      compact,
		}
		if suppressed > 0 {
			result["duplicates_collapsed"] = suppressed
		}
		return result, nil
	})

	// ── nodeagent_search_logs ──────────────────────────────────────────────
	s.register(toolDef{
		Name: "nodeagent_search_logs",
		Description: `Search service logs by time range, pattern, and severity. Returns compact, deduplicated output with secrets redacted. Always use filters to narrow results.

Examples:
- Errors in the last hour: unit="globular-gateway", since="1h ago", priority="err"
- TLS issues: unit="globular-dns", pattern="tls|certificate|cert", since="30m ago"
- Time window: unit="globular-rbac", since="2026-03-25 00:00:00", until="2026-03-25 01:00:00"`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"unit":     {Type: "string", Description: "Systemd unit name (e.g. 'globular-gateway.service', 'globular-dns', 'scylla-server')"},
				"node_id":  {Type: "string", Description: "Optional: query a remote node's logs by node ID. If omitted, reads from the local node."},
				"pattern":  {Type: "string", Description: "Regex pattern to search for (e.g. 'error|fail|timeout', 'tls.*cert')"},
				"since":    {Type: "string", Description: "Start of time range (e.g. '5m ago', '1h ago', '2026-03-25 00:00:00')"},
				"until":    {Type: "string", Description: "End of time range (optional, e.g. '30m ago', '2026-03-25 01:00:00')"},
				"priority": {Type: "string", Description: "Severity filter", Enum: []string{"emerg", "alert", "crit", "err", "warning", "notice", "info", "debug"}},
				"limit":    {Type: "number", Description: "Max lines to fetch (default 50, max 200). Output may be fewer after dedup."},
				"case_sensitive": {Type: "boolean", Description: "Case-sensitive pattern matching (default: false)"},
			},
			Required: []string{"unit"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		unit := getStr(args, "unit")
		if unit == "" {
			return nil, fmt.Errorf("unit is required")
		}

		nodeID := getStr(args, "node_id")
		endpoint, err := s.resolveNodeAgentEndpoint(ctx, nodeID)
		if err != nil {
			return nil, fmt.Errorf("resolve node agent: %w", err)
		}

		conn, err := s.clients.get(ctx, endpoint)
		if err != nil {
			return nil, err
		}
		client := node_agentpb.NewNodeAgentServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 30*time.Second)
		defer cancel()

		limit := getInt(args, "limit", 50)
		if limit > 200 {
			limit = 200
		}

		resp, err := client.SearchServiceLogs(callCtx, &node_agentpb.SearchServiceLogsRequest{
			Unit:          unit,
			Pattern:       getStr(args, "pattern"),
			Since:         getStr(args, "since"),
			Until:         getStr(args, "until"),
			Priority:      getStr(args, "priority"),
			Limit:         int32(limit),
			CaseSensitive: getBool(args, "case_sensitive", false),
		})
		if err != nil {
			return nil, fmt.Errorf("SearchServiceLogs: %w", err)
		}

		compact, suppressed := compactLogLines(resp.GetLines())
		result := map[string]interface{}{
			"unit":        resp.GetUnit(),
			"match_count": resp.GetMatchCount(),
			"lines":       compact,
		}
		if suppressed > 0 {
			result["duplicates_collapsed"] = suppressed
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
