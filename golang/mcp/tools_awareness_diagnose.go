// @awareness namespace=globular.platform
// @awareness component=platform_mcp.awareness_diagnose
// @awareness file_role=mcp_composer_correlating_authored_invariants_with_runtime_findings
// @awareness implements=globular.platform:intent.awareness.graph_is_compiled_context_not_authority
// @awareness implements=globular.platform:intent.awareness.mcp_bridge_exposes_safe_tools_only
// @awareness implements=globular.platform:intent.runtime_observation_must_not_mutate_desired
// @awareness risk=medium
package main

// tools_awareness_diagnose.go — diagnostic composer that correlates authored
// awareness context with currently-active runtime findings.
//
// The tool is a strict read-only composer. It does NOT:
//   - read etcd directly
//   - write any new RDF triples
//   - add any new gRPC RPCs
//   - introduce a parallel runtime schema
//
// It calls three existing gRPC services in parallel:
//   - awareness-graph.Briefing  → authored invariants/intents/failure_modes
//   - cluster_doctor.GetClusterReport → runtime findings (with invariant_id refs)
//   - cluster_controller.GetDriftReport → desired/applied drift items
//
// Correlation tiers are explicit:
//   high   — finding.invariant_id exactly matches a briefing reference id
//   medium — drift item's node_id/entity_ref matches request hint
//   low    — keyword overlap only — surfaces as `possible_related_evidence`,
//            NEVER as `correlated_findings`. We never imply causality from text alone.
//
// Safety invariants (always enforced):
//   - forbidden_conclusions list is always emitted
//   - missing runtime data is labeled (`blind_spots`), never collapsed to OK
//   - authored_context and runtime_evidence stay in separate sections
//   - per-source freshness is reported; partial-failure paths degrade gracefully

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	awarenesspb "github.com/globulario/awareness-graph/golang/pb"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	repositorypb "github.com/globulario/services/golang/repository/repositorypb"
)

// Bounds — keep the response compact regardless of upstream volume.
const (
	maxDoctorFindings    = 10
	maxDriftItems        = 10
	maxCorrelated        = 5
	maxPossibleRelated   = 5
	maxNextChecks        = 5
	keywordMinLen        = 4
	keywordMinTermsForLo = 2

	diagnoseOuterTimeout  = 30 * time.Second
	diagnoseSourceTimeout = 10 * time.Second
)

// forbiddenConclusions is a fixed list returned with every diagnose response.
// Authored awareness alone, even combined with runtime findings, must never
// support these conclusions. Listing them explicitly in every response means
// an agent can never claim it was not warned.
var forbiddenConclusions = []string{
	"Missing runtime evidence does not imply delete-desired-state intent — a missing observation can mean stale snapshot, collector failure, node offline, or mid-transition.",
	"Runtime observation is not authored truth — never rewrite invariants from doctor/drift findings.",
	"Stale or partial runtime evidence must be labeled as such, never collapsed into 'unknown' or treated as 'no problem'.",
	"Keyword overlap alone does not establish causality — possible_related_evidence is a hint, not a correlation.",
}

// registerAwarenessDiagnoseTool wires the diagnose composer into the MCP
// server. Lives in the Awareness tool group so it's enabled/disabled by the
// same flag as the briefing/impact tools.
func registerAwarenessDiagnoseTool(s *server) {
	s.register(toolDef{
		Name: "awareness_diagnose",
		Description: "Diagnostic composer — correlates authored awareness (invariants, intents, failure modes) " +
			"with currently-active runtime findings from cluster_doctor and cluster_controller. " +
			"Read-only: no etcd writes, no RDF mutations, no new schema. Always returns explicit " +
			"sections: authored_context, runtime_evidence, correlated_findings (high/medium confidence), " +
			"possible_related_evidence (keyword-only, NOT causal), forbidden_conclusions, next_checks, " +
			"blind_spots. Single-source failures degrade individual sections rather than failing the call.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"symptom": {
					Type:        "string",
					Description: "Free-form description of the symptom or task. Required.",
				},
				"file": {
					Type:        "string",
					Description: "Optional repo-relative file path. Anchors authored context to a file.",
				},
				"service_id": {
					Type:        "string",
					Description: "Optional service id (e.g. 'repository.RepositoryService') to narrow correlation.",
				},
				"node_id": {
					Type:        "string",
					Description: "Optional node id to narrow correlation to drift/findings on that node.",
				},
				"package_id": {
					Type:        "string",
					Description: "Optional package id (publisher/name) to narrow correlation.",
				},
				"mode": {
					Type:        "string",
					Description: "compact (default) or standard. Standard returns longer briefing context.",
					Enum:        []string{"compact", "standard"},
					Default:     "compact",
				},
			},
			Required: []string{"symptom"},
		},
	}, diagnoseHandler(s))
}

func diagnoseHandler(s *server) func(context.Context, map[string]interface{}) (interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		symptom := strings.TrimSpace(getStr(args, "symptom"))
		if symptom == "" {
			return nil, fmt.Errorf("symptom is required")
		}
		hints := requestHints{
			file:       strings.TrimSpace(getStr(args, "file")),
			serviceID:  strings.TrimSpace(getStr(args, "service_id")),
			nodeID:     strings.TrimSpace(getStr(args, "node_id")),
			packageID:  strings.TrimSpace(getStr(args, "package_id")),
			mode:       getStrDefault(args, "mode", "compact"),
			symptom:    symptom,
			keywords:   extractKeywords(symptom),
		}

		outerCtx, cancel := context.WithTimeout(ctx, diagnoseOuterTimeout)
		defer cancel()

		// Parallel collection — each source has its own goroutine. A single
		// source failure adds to tool_failures + blind_spots; the call still
		// returns whatever the other sources delivered.
		collected := collectSources(outerCtx, s, hints)

		// Build the structured response.
		response := buildDiagnoseResponse(hints, collected)
		return response, nil
	}
}

// requestHints carries the trimmed/normalized request fields into the
// collection and correlation paths.
type requestHints struct {
	symptom    string
	file       string
	serviceID  string
	nodeID     string
	packageID  string
	mode       string
	keywords   []string
}

// collectedSources holds per-source results plus per-source error/freshness
// metadata so the response builder can label degradation precisely.
//
// Conditional branches (nodeHealth/inventory/artifact) are only populated
// when the corresponding hint (node_id/package_id) is present and resolves.
type collectedSources struct {
	briefing     *awarenesspb.BriefingResponse
	briefingErr  error

	doctorReport *cluster_doctorpb.ClusterReport
	doctorErr    error

	driftReport *cluster_doctorpb.DriftReport
	driftErr    error

	// Conditional on node_id hint.
	nodeHealth        *cluster_controllerpb.GetNodeHealthDetailV1Response
	nodeHealthErr     error
	nodeHealthSkipped bool // true when node_id was not provided
	inventory         *node_agentpb.GetInventoryResponse
	inventoryErr      error
	inventorySkipped  bool

	// Conditional on package_id hint (must parse to publisher/name@version).
	artifact         *repositorypb.ExplainArtifactResponse
	artifactErr      error
	artifactSkipped  bool
	artifactSkipNote string // why it was skipped, when applicable
}

func collectSources(ctx context.Context, s *server, h requestHints) *collectedSources {
	out := &collectedSources{}
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 1. Awareness briefing — anchor authored context. File path takes
	//    precedence (current Briefing requires it); without a file we still
	//    send the request and let the server respond with task-only guidance.
	wg.Add(1)
	go func() {
		defer wg.Done()
		callCtx, cancel := context.WithTimeout(ctx, diagnoseSourceTimeout)
		defer cancel()

		stub, _, err := awarenessStub(callCtx, s)
		if err != nil {
			mu.Lock()
			out.briefingErr = err
			mu.Unlock()
			return
		}
		req := &awarenesspb.BriefingRequest{
			File:  h.file,
			Task:  h.symptom,
			Depth: depthForMode(h.mode),
		}
		resp, err := stub.Briefing(callCtx, req)
		mu.Lock()
		out.briefing = resp
		out.briefingErr = err
		mu.Unlock()
	}()

	// 2. Doctor cluster report — runtime findings with invariant_id refs.
	wg.Add(1)
	go func() {
		defer wg.Done()
		callCtx, cancel := context.WithTimeout(ctx, diagnoseSourceTimeout)
		defer cancel()

		conn, err := s.clients.get(callCtx, doctorEndpoint())
		if err != nil {
			mu.Lock()
			out.doctorErr = fmt.Errorf("dial doctor: %w", err)
			mu.Unlock()
			return
		}
		client := cluster_doctorpb.NewClusterDoctorServiceClient(conn)
		report, err := client.GetClusterReport(callCtx, &cluster_doctorpb.ClusterReportRequest{
			Freshness: cluster_doctorpb.FreshnessMode_FRESHNESS_CACHED,
		})
		mu.Lock()
		out.doctorReport = report
		out.doctorErr = err
		mu.Unlock()
	}()

	// 3. Drift report — desired vs applied state. Same service as doctor.
	wg.Add(1)
	go func() {
		defer wg.Done()
		callCtx, cancel := context.WithTimeout(ctx, diagnoseSourceTimeout)
		defer cancel()

		conn, err := s.clients.get(callCtx, doctorEndpoint())
		if err != nil {
			mu.Lock()
			out.driftErr = fmt.Errorf("dial doctor: %w", err)
			mu.Unlock()
			return
		}
		client := cluster_doctorpb.NewClusterDoctorServiceClient(conn)
		report, err := client.GetDriftReport(callCtx, &cluster_doctorpb.DriftReportRequest{
			Freshness: cluster_doctorpb.FreshnessMode_FRESHNESS_CACHED,
		})
		mu.Lock()
		out.driftReport = report
		out.driftErr = err
		mu.Unlock()
	}()

	// 4. Node health detail (controller-side rollup) — only if node_id given.
	if h.nodeID != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			callCtx, cancel := context.WithTimeout(ctx, diagnoseSourceTimeout)
			defer cancel()

			conn, err := s.clients.get(callCtx, controllerEndpoint())
			if err != nil {
				mu.Lock()
				out.nodeHealthErr = fmt.Errorf("dial controller: %w", err)
				mu.Unlock()
				return
			}
			client := cluster_controllerpb.NewClusterControllerServiceClient(conn)
			resp, err := client.GetNodeHealthDetailV1(callCtx, &cluster_controllerpb.GetNodeHealthDetailV1Request{
				NodeId: h.nodeID,
			})
			mu.Lock()
			out.nodeHealth = resp
			out.nodeHealthErr = err
			mu.Unlock()
		}()
	} else {
		out.nodeHealthSkipped = true
	}

	// 5. Per-node agent inventory — only if node_id given. The endpoint is
	// resolved through the controller's node registry; falls back to the
	// local node-agent if resolution fails.
	if h.nodeID != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			callCtx, cancel := context.WithTimeout(ctx, diagnoseSourceTimeout)
			defer cancel()

			ep, err := s.resolveNodeAgentEndpoint(callCtx, h.nodeID)
			if err != nil {
				mu.Lock()
				out.inventoryErr = fmt.Errorf("resolve node-agent endpoint: %w", err)
				mu.Unlock()
				return
			}
			conn, err := s.clients.get(callCtx, ep)
			if err != nil {
				mu.Lock()
				out.inventoryErr = fmt.Errorf("dial node-agent at %s: %w", ep, err)
				mu.Unlock()
				return
			}
			client := node_agentpb.NewNodeAgentServiceClient(conn)
			resp, err := client.GetInventory(callCtx, &node_agentpb.GetInventoryRequest{})
			mu.Lock()
			out.inventory = resp
			out.inventoryErr = err
			mu.Unlock()
		}()
	} else {
		out.inventorySkipped = true
	}

	// 6. Repository explain artifact — only if package_id parses cleanly
	// to publisher/name@version. We don't guess: if the hint is free-form,
	// skip this branch and record a blind_spot note so the agent can supply
	// a structured form.
	if h.packageID != "" {
		pub, name, ver, ok := parsePackageID(h.packageID)
		if !ok {
			out.artifactSkipped = true
			out.artifactSkipNote = "package_id must be of form 'publisher/name@version' to call repository.ExplainArtifact"
		} else {
			wg.Add(1)
			go func() {
				defer wg.Done()
				callCtx, cancel := context.WithTimeout(ctx, diagnoseSourceTimeout)
				defer cancel()

				conn, err := s.clients.get(callCtx, repositoryEndpoint())
				if err != nil {
					mu.Lock()
					out.artifactErr = fmt.Errorf("dial repository: %w", err)
					mu.Unlock()
					return
				}
				client := repositorypb.NewPackageRepositoryClient(conn)
				resp, err := client.ExplainArtifact(callCtx, &repositorypb.ExplainArtifactRequest{
					Ref: &repositorypb.ArtifactRef{
						PublisherId: pub,
						Name:        name,
						Version:     ver,
						Platform:    "linux_amd64",
					},
				})
				mu.Lock()
				out.artifact = resp
				out.artifactErr = err
				mu.Unlock()
			}()
		}
	} else {
		out.artifactSkipped = true
	}

	wg.Wait()
	return out
}

// parsePackageID accepts "publisher/name@version" and returns the three
// fields. Returns ok=false for any other shape — the caller must record a
// blind_spot and skip the artifact branch rather than guess.
//
// Note: publisher IDs are usually email-shaped (e.g. "core@globular.io"),
// so the '@' before the version is the LAST '@' in the string, not the
// first. The '/' separating publisher from package name comes first.
func parsePackageID(s string) (publisher, name, version string, ok bool) {
	slash := strings.IndexByte(s, '/')
	at := strings.LastIndexByte(s, '@')
	if slash <= 0 || at <= slash+1 || at >= len(s)-1 {
		return "", "", "", false
	}
	publisher = strings.TrimSpace(s[:slash])
	name = strings.TrimSpace(s[slash+1 : at])
	version = strings.TrimSpace(s[at+1:])
	if publisher == "" || name == "" || version == "" {
		return "", "", "", false
	}
	return publisher, name, version, true
}

// buildDiagnoseResponse takes the collected per-source data and produces the
// final structured response. Always returns forbidden_conclusions. Labels
// blind_spots whenever a source failed or returned degraded data.
func buildDiagnoseResponse(h requestHints, c *collectedSources) map[string]interface{} {
	blindSpots := []string{}
	toolFailures := []map[string]interface{}{}

	// ── Authored context ─────────────────────────────────────────────────
	authored := map[string]interface{}{
		"coverage_status":   "unknown",
		"briefing_prose":    "",
		"referenced_ids":    []string{},
		"keyword_count":     len(h.keywords),
	}
	authoredReferenceIDs := []string{}
	if c.briefingErr != nil {
		blindSpots = append(blindSpots, "awareness-graph unreachable: authored_context empty")
		toolFailures = append(toolFailures, map[string]interface{}{
			"source": "awareness-graph",
			"error":  c.briefingErr.Error(),
		})
		authored["coverage_status"] = "degraded"
	} else if c.briefing != nil {
		authored["briefing_prose"] = c.briefing.GetProse()
		authored["referenced_ids"] = c.briefing.GetReferencedIds()
		authored["coverage_status"] = briefingStatusStr(c.briefing.GetStatus())
		authoredReferenceIDs = c.briefing.GetReferencedIds()
		if len(authoredReferenceIDs) == 0 {
			blindSpots = append(blindSpots, "briefing returned 0 referenced_ids — authored coverage may be thin for this target")
		}
	}

	// ── Runtime evidence ─────────────────────────────────────────────────
	doctorFindings := []map[string]interface{}{}
	driftItems := []map[string]interface{}{}
	freshness := map[string]interface{}{}

	if c.doctorErr != nil {
		blindSpots = append(blindSpots, "cluster_doctor unreachable: doctor_findings missing")
		toolFailures = append(toolFailures, map[string]interface{}{
			"source": "cluster_doctor",
			"error":  c.doctorErr.Error(),
		})
		freshness["doctor"] = map[string]interface{}{"status": "unavailable"}
	} else if c.doctorReport != nil {
		all := c.doctorReport.GetFindings()
		// Sort severity descending then take top N — bounded output.
		sort.SliceStable(all, func(i, j int) bool {
			return severityRank(all[i].GetSeverity()) > severityRank(all[j].GetSeverity())
		})
		for i, f := range all {
			if i >= maxDoctorFindings {
				break
			}
			doctorFindings = append(doctorFindings, map[string]interface{}{
				"finding_id":   f.GetFindingId(),
				"invariant_id": f.GetInvariantId(),
				"severity":     severityStr(f.GetSeverity()),
				"category":     f.GetCategory(),
				"summary":      f.GetSummary(),
				"entity_ref":   f.GetEntityRef(),
			})
		}
		freshness["doctor"] = freshnessPayload(c.doctorReport.GetHeader())
		if len(all) > maxDoctorFindings {
			blindSpots = append(blindSpots, fmt.Sprintf("doctor returned %d findings; only top %d by severity included", len(all), maxDoctorFindings))
		}
	}

	if c.driftErr != nil {
		blindSpots = append(blindSpots, "cluster_controller drift report unavailable")
		toolFailures = append(toolFailures, map[string]interface{}{
			"source": "cluster_controller.drift",
			"error":  c.driftErr.Error(),
		})
		freshness["drift"] = map[string]interface{}{"status": "unavailable"}
	} else if c.driftReport != nil {
		all := c.driftReport.GetItems()
		for i, item := range all {
			if i >= maxDriftItems {
				break
			}
			driftItems = append(driftItems, map[string]interface{}{
				"node_id":  item.GetNodeId(),
				"entity":   item.GetEntityRef(),
				"category": driftCategoryStr(item.GetCategory()),
				"desired":  item.GetDesired(),
				"actual":   item.GetActual(),
			})
		}
		freshness["drift"] = freshnessPayload(c.driftReport.GetHeader())
		if int(c.driftReport.GetTotalDriftCount()) > maxDriftItems {
			blindSpots = append(blindSpots, fmt.Sprintf("drift report has %d items; only first %d included", c.driftReport.GetTotalDriftCount(), maxDriftItems))
		}
	}

	// ── Optional per-node sections ───────────────────────────────────────
	var nodeHealth, inventory map[string]interface{}
	if !c.nodeHealthSkipped {
		if c.nodeHealthErr != nil {
			blindSpots = append(blindSpots, "node health detail unavailable for node_id")
			toolFailures = append(toolFailures, map[string]interface{}{
				"source": "cluster_controller.node_health",
				"error":  c.nodeHealthErr.Error(),
			})
		} else if c.nodeHealth != nil {
			checks := make([]map[string]interface{}, 0, len(c.nodeHealth.GetChecks()))
			for _, ch := range c.nodeHealth.GetChecks() {
				checks = append(checks, map[string]interface{}{
					"subsystem": ch.GetSubsystem(),
					"ok":        ch.GetOk(),
					"reason":    ch.GetReason(),
				})
			}
			nodeHealth = map[string]interface{}{
				"overall_status":     c.nodeHealth.GetOverallStatus(),
				"healthy":            c.nodeHealth.GetHealthy(),
				"last_error":         c.nodeHealth.GetLastError(),
				"inventory_complete": c.nodeHealth.GetInventoryComplete(),
				"checks":             checks,
			}
		}
	}
	if !c.inventorySkipped {
		if c.inventoryErr != nil {
			blindSpots = append(blindSpots, "node-agent inventory unavailable for node_id")
			toolFailures = append(toolFailures, map[string]interface{}{
				"source": "node_agent.inventory",
				"error":  c.inventoryErr.Error(),
			})
		} else if c.inventory != nil && c.inventory.GetInventory() != nil {
			inv := c.inventory.GetInventory()
			identity := map[string]interface{}{}
			if id := inv.GetIdentity(); id != nil {
				identity = map[string]interface{}{
					"hostname":      id.GetHostname(),
					"agent_version": id.GetAgentVersion(),
				}
			}
			inventory = map[string]interface{}{
				"identity":   identity,
				"unit_count": len(inv.GetUnits()),
			}
		}
	}

	// ── Optional repository artifact section ─────────────────────────────
	var artifact map[string]interface{}
	if !c.artifactSkipped {
		if c.artifactErr != nil {
			blindSpots = append(blindSpots, "repository.ExplainArtifact unavailable for package_id")
			toolFailures = append(toolFailures, map[string]interface{}{
				"source": "repository.explain_artifact",
				"error":  c.artifactErr.Error(),
			})
		} else if c.artifact != nil {
			artifact = map[string]interface{}{
				"installable":         c.artifact.GetInstallable(),
				"recommended_action":  c.artifact.GetRecommendedAction(),
				"artifact_state":      c.artifact.GetArtifactState(),
				"blob_present":        c.artifact.GetBlobPresent(),
				"ledger_present":      c.artifact.GetLedgerPresent(),
				"manifest_present":    c.artifact.GetManifestPresent(),
				"signature_status":    c.artifact.GetSignatureStatus(),
				"detail":              c.artifact.GetDetail(),
				"repairable":          c.artifact.GetRepairable(),
			}
		}
	} else if c.artifactSkipNote != "" {
		// Skip note (e.g. malformed package_id) — visible to the agent so
		// it can rerun with structured input. Not a tool failure.
		blindSpots = append(blindSpots, c.artifactSkipNote)
	}

	runtime := map[string]interface{}{
		"doctor_findings": doctorFindings,
		"drift_items":     driftItems,
		"freshness":       freshness,
	}
	if nodeHealth != nil {
		runtime["node_health"] = nodeHealth
	}
	if inventory != nil {
		runtime["node_inventory"] = inventory
	}
	if artifact != nil {
		runtime["repository_artifact"] = artifact
	}

	// ── Correlation ──────────────────────────────────────────────────────
	correlated, possibleRelated, corrBlindSpots := correlate(
		authoredReferenceIDs,
		doctorFindings,
		driftItems,
		h,
	)
	blindSpots = append(blindSpots, corrBlindSpots...)

	// ── Status ──────────────────────────────────────────────────────────
	status := computeStatus(c, authoredReferenceIDs, correlated)

	// ── Next checks ─────────────────────────────────────────────────────
	nextChecks := computeNextChecks(h, c, correlated)

	return map[string]interface{}{
		"status":  status,
		"symptom": h.symptom,
		"hints": map[string]interface{}{
			"file":       h.file,
			"service_id": h.serviceID,
			"node_id":    h.nodeID,
			"package_id": h.packageID,
			"mode":       h.mode,
		},
		"authored_context":          authored,
		"runtime_evidence":          runtime,
		"correlated_findings":       correlated,
		"possible_related_evidence": possibleRelated,
		"forbidden_conclusions":     forbiddenConclusions,
		"next_checks":               nextChecks,
		"blind_spots":               blindSpots,
		"tool_failures":             toolFailures,
	}
}

// correlate produces:
//   - correlated_findings: high (invariant id match) and medium (hint match) only
//   - possible_related_evidence: low confidence (keyword overlap only) labeled
//     explicitly as NOT causal
//   - blindSpots: notes when correlation is weak so the agent can see it
func correlate(
	authoredIDs []string,
	doctorFindings, driftItems []map[string]interface{},
	h requestHints,
) (correlated, possibleRelated []map[string]interface{}, blindSpots []string) {
	// Build the set of bare invariant IDs from authored references.
	// Briefing returns "invariant:foo" / "failure_mode:foo" — normalize to bare.
	authoredBare := map[string]string{}
	for _, ref := range authoredIDs {
		bare, class := splitClassID(ref)
		if bare == "" {
			continue
		}
		authoredBare[bare] = class
	}

	// ── High & medium ──────────────────────────────────────────────────
	highSeen := map[string]bool{}
	medSeen := map[string]bool{}

	for _, f := range doctorFindings {
		fid, _ := f["finding_id"].(string)
		invID, _ := f["invariant_id"].(string)
		entityRef, _ := f["entity_ref"].(string)
		summary, _ := f["summary"].(string)
		sev, _ := f["severity"].(string)

		// High: invariant_id matches an authored reference id (bare).
		if invID != "" {
			if class, ok := authoredBare[invID]; ok && !highSeen[fid] {
				correlated = append(correlated, map[string]interface{}{
					"finding_id":           fid,
					"severity":             sev,
					"summary":              summary,
					"matched_awareness_id": class + ":" + invID,
					"match_reason":         "invariant_id_overlap",
					"confidence":           "high",
				})
				highSeen[fid] = true
				continue
			}
		}

		// Medium: request hint (node/service/package id) matches the finding's
		// EntityRef field.
		if hint := hintMatch(h, "", entityRef); hint != "" && !medSeen[fid] {
			correlated = append(correlated, map[string]interface{}{
				"finding_id":           fid,
				"severity":             sev,
				"summary":              summary,
				"matched_awareness_id": "",
				"match_reason":         hint,
				"confidence":           "medium",
			})
			medSeen[fid] = true
			continue
		}

		// Low: keyword overlap only — possible_related_evidence (NOT correlated).
		if matchedKeywords := keywordOverlap(h.keywords, summary); len(matchedKeywords) >= keywordMinTermsForLo {
			possibleRelated = append(possibleRelated, map[string]interface{}{
				"finding_id":          fid,
				"severity":            sev,
				"summary":             summary,
				"matched_keywords":    matchedKeywords,
				"match_reason":        "keyword_overlap_only",
				"confidence":          "low",
				"causal_implication":  false,
				"warning":             "keyword overlap is a hint, not a correlation",
			})
		}
	}

	// Drift items have no invariant_id, only NodeId + EntityRef. They match
	// at medium tier via hints, or surface as possible_related on keyword.
	for _, d := range driftItems {
		node, _ := d["node_id"].(string)
		entity, _ := d["entity"].(string)
		actual, _ := d["actual"].(string)
		desired, _ := d["desired"].(string)
		cat, _ := d["category"].(string)
		summary := fmt.Sprintf("drift %s on %s/%s (desired=%s, actual=%s)", cat, node, entity, desired, actual)

		key := node + "|" + entity + "|" + cat
		if hint := hintMatch(h, node, entity); hint != "" && !medSeen[key] {
			correlated = append(correlated, map[string]interface{}{
				"finding_id":           "drift:" + key,
				"severity":             "warning",
				"summary":              summary,
				"matched_awareness_id": "",
				"match_reason":         hint,
				"confidence":           "medium",
			})
			medSeen[key] = true
			continue
		}
		if matchedKeywords := keywordOverlap(h.keywords, summary); len(matchedKeywords) >= keywordMinTermsForLo {
			possibleRelated = append(possibleRelated, map[string]interface{}{
				"finding_id":         "drift:" + key,
				"severity":           "warning",
				"summary":            summary,
				"matched_keywords":   matchedKeywords,
				"match_reason":       "keyword_overlap_only",
				"confidence":         "low",
				"causal_implication": false,
				"warning":            "keyword overlap is a hint, not a correlation",
			})
		}
	}

	// Bound + sort by confidence descending.
	sort.SliceStable(correlated, func(i, j int) bool {
		return confidenceRank(correlated[i]["confidence"].(string)) > confidenceRank(correlated[j]["confidence"].(string))
	})
	if len(correlated) > maxCorrelated {
		correlated = correlated[:maxCorrelated]
	}
	if len(possibleRelated) > maxPossibleRelated {
		possibleRelated = possibleRelated[:maxPossibleRelated]
	}

	// Weak-signal label — never invent a link to fill the response.
	highCount := 0
	for _, c := range correlated {
		if c["confidence"] == "high" {
			highCount++
		}
	}
	if highCount == 0 && len(correlated) <= 1 {
		blindSpots = append(blindSpots, "weak_authored_runtime_correlation: 0 high-confidence matches and ≤1 medium match — diagnosis is tentative")
	}
	return correlated, possibleRelated, blindSpots
}

// hintMatch returns a non-empty reason string when one of the request's
// targeting hints (node/service/package id) appears in the finding's node
// or entity_ref. Empty string means no hint match.
func hintMatch(h requestHints, node, entityRef string) string {
	if h.nodeID != "" && (eqFold(h.nodeID, node) || containsFold(entityRef, h.nodeID)) {
		return "node_hint"
	}
	if h.serviceID != "" && containsFold(entityRef, h.serviceID) {
		return "service_hint"
	}
	if h.packageID != "" && containsFold(entityRef, h.packageID) {
		return "package_hint"
	}
	return ""
}

// computeStatus rolls the per-source state into a single status value an
// agent can branch on without inspecting blind_spots.
func computeStatus(c *collectedSources, authoredIDs []string, correlated []map[string]interface{}) string {
	allFailed := c.briefingErr != nil && c.doctorErr != nil && c.driftErr != nil
	if allFailed {
		return "degraded"
	}
	anyFailed := c.briefingErr != nil || c.doctorErr != nil || c.driftErr != nil
	if anyFailed {
		return "partial"
	}
	if len(authoredIDs) == 0 && len(correlated) == 0 {
		return "empty"
	}
	return "ok"
}

// computeNextChecks suggests up to N concrete existing tools/commands.
// We never recommend destructive actions; only read-only or operator-approved
// runbook entry points.
func computeNextChecks(h requestHints, c *collectedSources, correlated []map[string]interface{}) []string {
	checks := []string{}
	// Always: re-read briefing fresh + verify cluster health.
	if c.briefingErr == nil && h.file != "" {
		checks = append(checks, fmt.Sprintf("Re-run awareness.briefing(file=%q, depth=standard) to confirm authored coverage", h.file))
	}
	if c.doctorErr == nil && len(correlated) > 0 {
		checks = append(checks, "Call cluster_get_doctor_report(freshness=fresh) to confirm findings are still current")
	}
	if c.driftErr == nil {
		checks = append(checks, "Call cluster_get_drift_report to inspect full drift inventory if convergence is suspected")
	}
	if h.nodeID != "" {
		checks = append(checks, fmt.Sprintf("Call cluster_get_node_full_status(node_id=%q) for per-node detail", h.nodeID))
	}
	if h.packageID != "" {
		checks = append(checks, fmt.Sprintf("Call repository_explain_artifact(name=%q) to verify manifest+blob presence", h.packageID))
	}
	if len(checks) > maxNextChecks {
		checks = checks[:maxNextChecks]
	}
	return checks
}

// ── helpers ────────────────────────────────────────────────────────────

// splitClassID separates a class-qualified id like "invariant:foo.bar" into
// bare id + class. Returns ("","") on malformed input.
func splitClassID(ref string) (bare, class string) {
	i := strings.IndexByte(ref, ':')
	if i <= 0 || i >= len(ref)-1 {
		return "", ""
	}
	return ref[i+1:], ref[:i]
}

// extractKeywords lowercases the symptom and returns distinct alphanumeric
// tokens of length ≥ keywordMinLen. Used for low-confidence text matching only.
func extractKeywords(symptom string) []string {
	seen := map[string]bool{}
	out := []string{}
	cur := strings.Builder{}
	flush := func() {
		w := cur.String()
		cur.Reset()
		if len(w) < keywordMinLen {
			return
		}
		if seen[w] {
			return
		}
		seen[w] = true
		out = append(out, w)
	}
	for _, r := range strings.ToLower(symptom) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			cur.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}

// keywordOverlap returns the keywords from h that appear in text.
func keywordOverlap(keywords []string, text string) []string {
	if len(keywords) == 0 || text == "" {
		return nil
	}
	lower := strings.ToLower(text)
	var matched []string
	for _, k := range keywords {
		if strings.Contains(lower, k) {
			matched = append(matched, k)
		}
	}
	return matched
}

func eqFold(a, b string) bool      { return strings.EqualFold(a, b) }
func containsFold(s, sub string) bool {
	if sub == "" {
		return false
	}
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

func severityRank(s cluster_doctorpb.Severity) int {
	switch s {
	case cluster_doctorpb.Severity_SEVERITY_CRITICAL:
		return 4
	case cluster_doctorpb.Severity_SEVERITY_ERROR:
		return 3
	case cluster_doctorpb.Severity_SEVERITY_WARN:
		return 2
	case cluster_doctorpb.Severity_SEVERITY_INFO:
		return 1
	}
	return 0
}

func confidenceRank(c string) int {
	switch c {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	}
	return 0
}

func depthForMode(mode string) string {
	if mode == "standard" {
		return "standard"
	}
	return "compact"
}

func getStrDefault(args map[string]interface{}, key, def string) string {
	if v, ok := args[key].(string); ok && v != "" {
		return v
	}
	return def
}
