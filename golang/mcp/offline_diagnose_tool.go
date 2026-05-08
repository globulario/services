package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/globulario/services/golang/awareness/preflight"
)

// offlineEvent is a single extracted log event.
type offlineEvent struct {
	Source  string `json:"source"`
	Pattern string `json:"pattern"`
	Text    string `json:"text"`
	Time    string `json:"time,omitempty"`
}

// offlineTimeline is an ordered event for the timeline output.
type offlineTimeline struct {
	Time   string `json:"time"`
	Source string `json:"source"`
	Event  string `json:"event"`
}

// offlineFailureModeMatch is a failure mode scored by offline evidence.
type offlineFailureModeMatch struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	MatchScore float64 `json:"match_score"`
}

// offlineInvariantMatch is an invariant suspected to be violated.
type offlineInvariantMatch struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Violated bool   `json:"violated"`
}

// logPattern represents a detectable log event pattern.
type logPattern struct {
	PatternID  string
	Keywords   []string // any keyword triggers a match
	EventLabel string
}

var offlineLogPatterns = []logPattern{
	{
		PatternID:  "etcd_nospace",
		Keywords:   []string{"NOSPACE", "mvcc: database space exceeded", "etcd disk"},
		EventLabel: "etcd_disk_pressure",
	},
	{
		PatternID:  "leader_instability",
		Keywords:   []string{"lost leader", "elected leader", "leader changed", "is now leader", "lost quorum"},
		EventLabel: "leader_instability",
	},
	{
		PatternID:  "port_squatting",
		Keywords:   []string{"address already in use", "bind: address already in use", "port in use"},
		EventLabel: "port_in_use",
	},
	{
		PatternID:  "unknown_grpc_service",
		Keywords:   []string{"unknown service", "unknown gRPC service", "unimplemented"},
		EventLabel: "unknown_grpc_service",
	},
	{
		PatternID:  "connection_refused",
		Keywords:   []string{"connection refused"},
		EventLabel: "connection_refused",
	},
	{
		PatternID:  "deadline_exceeded",
		Keywords:   []string{"context deadline exceeded", "deadline exceeded"},
		EventLabel: "deadline_exceeded",
	},
	{
		PatternID:  "connection_reset",
		Keywords:   []string{"connection reset by peer", "EOF", "broken pipe"},
		EventLabel: "network_reset",
	},
	{
		PatternID:  "restart_storm",
		Keywords:   []string{"restart loop", "start-limit-hit", "start operation timed out", "Too many restarts"},
		EventLabel: "restart_storm",
	},
	{
		PatternID:  "permission_denied",
		Keywords:   []string{"permission denied", "PermissionDenied", "Unauthenticated"},
		EventLabel: "auth_failure",
	},
	{
		PatternID:  "tls_problem",
		Keywords:   []string{"certificate expired", "certificate mismatch", "invalid certificate", "tls: failed", "x509:"},
		EventLabel: "tls_problem",
	},
	{
		PatternID:  "artifact_integrity",
		Keywords:   []string{"checksum mismatch", "build mismatch", "artifact integrity", "digest mismatch"},
		EventLabel: "artifact_integrity",
	},
	{
		PatternID:  "minio_issue",
		Keywords:   []string{"healing", "offline disk", "drive offline", "MinIO", "minio"},
		EventLabel: "minio_disk_issue",
	},
	{
		PatternID:  "scylladb_issue",
		Keywords:   []string{"Scylla timeout", "scylla timeout", "Scylla unavailable", "scylla connection refused", "ScyllaDB"},
		EventLabel: "scylladb_issue",
	},
	{
		PatternID:  "workflow_stuck",
		Keywords:   []string{"workflow stuck", "workflow blocked", "workflow timeout", "workflow failed"},
		EventLabel: "workflow_stuck",
	},
	{
		PatternID:  "systemd_notify_failure",
		Keywords:   []string{"activating", "start operation timed out", "READY=1", "sd_notify"},
		EventLabel: "systemd_notify_failure",
	},
}

// fmDirectNextDiagnostics maps high-severity failure mode IDs to first-line diagnostic
// commands that an operator should run immediately. These surface in recommended_next_diagnostics
// without requiring a follow-up awareness.explain_symptom call.
var fmDirectNextDiagnostics = map[string][]string{
	"etcd.nospace_alarm": {
		"etcdctl alarm list  — verify NOSPACE is active",
		"etcdctl compact $(etcdctl endpoint status --write-out=json | jq -r '.[0].Status.header.revision')  — compact revision history",
		"etcdctl defrag --endpoints=<all>  — release disk space below quota",
		"etcdctl alarm disarm  — ONLY after disk usage is below quota",
		"etcdctl endpoint health  — verify all members healthy",
		"etcdctl endpoint status --write-out=table  — confirm stable leader",
	},
	"etcd.leader_instability": {
		"etcdctl endpoint status --write-out=table  — watch leader_id for stability",
		"wait for no 'lost leader'/'elected leader' messages for ≥60s before acting",
		"do NOT restart etcd or controller until quorum is stable",
	},
	"controller.lease_expired_due_to_etcd_instability": {
		"watch controller logs for 'acquired leader lease' — do not intervene before",
		"do NOT restart controller before etcd is stable (etcdctl alarm list must be empty)",
	},
	"workflow.dispatch_timeout_due_to_control_plane_instability": {
		"do NOT restart workflow service — fix etcd and controller first",
		"globular workflow list-runs --status blocked  — check after control plane stabilises",
	},
}

// timestamp patterns for timeline parsing.
var (
	rfc3339RE    = regexp.MustCompile(`(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z?)`)
	commonLogRE  = regexp.MustCompile(`(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})`)
	journalctlRE = regexp.MustCompile(`([A-Z][a-z]{2}\s+\d{1,2} \d{2}:\d{2}:\d{2}(?:\.\d+)?)`)
)

func registerOfflineDiagnoseTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name:        "awareness.offline_diagnose",
		Description: "Parse log text inputs (journalctl, systemd, etcdctl, docker-compose) for known failure patterns. Works without live cluster access. Returns evidence, timeline, suspected failure modes, and invariants.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"logs_dir":             {Type: "string", Description: "Directory containing log files to scan."},
				"journalctl_text":      {Type: "string", Description: "Raw journalctl output text."},
				"systemd_status":       {Type: "string", Description: "Output of systemctl status."},
				"etcdctl_output":       {Type: "string", Description: "Output of etcdctl endpoint/member/status commands."},
				"docker_compose_logs":  {Type: "string", Description: "Output of docker compose logs."},
				"workflow_receipts_dir": {Type: "string", Description: "Optional path to workflow receipt files."},
				"doctor_report_file":   {Type: "string", Description: "Optional path to a doctor report YAML file."},
				"service":              {Type: "string", Description: "Optional service name to filter events."},
				"node":                 {Type: "string", Description: "Optional node ID to filter events."},
				"time_window":          {Type: "string", Description: "Optional time window, e.g. '1h'."},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		_ = ctx

		logsDir := strArg(args, "logs_dir")
		journalText := strArg(args, "journalctl_text")
		systemdStatus := strArg(args, "systemd_status")
		etcdOutput := strArg(args, "etcdctl_output")
		dockerLogs := strArg(args, "docker_compose_logs")
		workflowDir := strArg(args, "workflow_receipts_dir")
		doctorFile := strArg(args, "doctor_report_file")
		serviceFilter := strArg(args, "service")

		// Collect all text sources.
		type textSource struct {
			name string
			text string
		}
		var sources []textSource

		if journalText != "" {
			sources = append(sources, textSource{"journalctl", journalText})
		}
		if systemdStatus != "" {
			sources = append(sources, textSource{"systemd_status", systemdStatus})
		}
		if etcdOutput != "" {
			sources = append(sources, textSource{"etcdctl", etcdOutput})
		}
		if dockerLogs != "" {
			sources = append(sources, textSource{"docker_compose", dockerLogs})
		}

		// Load log files from directory.
		if logsDir != "" {
			_ = filepath.Walk(logsDir, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				if strings.HasSuffix(path, ".log") || strings.HasSuffix(path, ".txt") {
					data, readErr := os.ReadFile(path)
					if readErr == nil {
						sources = append(sources, textSource{filepath.Base(path), string(data)})
					}
				}
				return nil
			})
		}

		// Load workflow receipts.
		if workflowDir != "" {
			_ = filepath.Walk(workflowDir, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".json") {
					data, readErr := os.ReadFile(path)
					if readErr == nil {
						sources = append(sources, textSource{"workflow_receipt", string(data)})
					}
				}
				return nil
			})
		}

		// Load doctor report.
		if doctorFile != "" {
			data, err := os.ReadFile(doctorFile)
			if err == nil {
				sources = append(sources, textSource{"doctor_report", string(data)})
			}
		}

		// --- Blank-input case ---
		var blindSpots []string
		if len(sources) == 0 {
			blindSpots = append(blindSpots, "no log inputs provided — all sources empty")
			return map[string]interface{}{
				"evidence":                    []offlineEvent{},
				"timeline":                    []offlineTimeline{},
				"suspected_failure_modes":     []offlineFailureModeMatch{},
				"suspected_invariants":        []offlineInvariantMatch{},
				"confidence":                  "unknown",
				"blind_spots":                 blindSpots,
				"recommended_next_diagnostics": []string{
					"provide journalctl_text or logs_dir",
					"run awareness.runtime_snapshot if cluster is partially up",
				},
			}, nil
		}

		// --- Extract events from all sources ---
		var events []offlineEvent
		patternHits := make(map[string]int) // patternID → count

		for _, src := range sources {
			scanner := bufio.NewScanner(strings.NewReader(src.text))
			for scanner.Scan() {
				line := scanner.Text()
				if serviceFilter != "" && !strings.Contains(strings.ToLower(line), strings.ToLower(serviceFilter)) {
					continue
				}
				ts := extractTimestamp(line)
				for _, pat := range offlineLogPatterns {
					if lineMatchesPattern(line, pat) {
						patternHits[pat.PatternID]++
						events = append(events, offlineEvent{
							Source:  src.name,
							Pattern: pat.PatternID,
							Text:    truncate(strings.TrimSpace(line), 200),
							Time:    ts,
						})
						// Deduplicate: stop after first matching pattern per line
						break
					}
				}
			}
		}

		// Deduplicate events (keep first N per patternID to avoid noise).
		events = deduplicateEvents(events, 5)

		// --- Build timeline ---
		timeline := buildTimeline(events)

		// --- Match against failure modes ---
		docsDir := st.docsDir
		var suspectedFMs []offlineFailureModeMatch
		var suspectedInvs []offlineInvariantMatch

		if docsDir != "" {
			fmDetails := loadFailureModeDetails(filepath.Join(docsDir, "failure_modes.yaml"))
			invDetails := loadInvariantDetails(filepath.Join(docsDir, "invariants.yaml"))

			// Primary failure-mode scoring: symptom fields only (title + symptoms list).
			// Scoring only symptoms prevents false positives from background terms such as
			// "etcd" or "leader" that appear in other failure modes' root_cause text but
			// are not observable symptoms of that failure mode.
			suspectedFMs = symptomBasedFMMatch(events, fmDetails)

			// Raw blob matching: always runs for invariants (no symptom fields), and
			// for failure modes only as fallback when no FM has matching symptom text.
			taskTerms := buildTaskFromEvents(events)
			rawMatches := preflight.RawKnowledgeFallback(taskTerms, nil, docsDir)
			for _, m := range rawMatches {
				score := float64(m.Score) / float64(len(offlineLogPatterns))
				if score > 1.0 {
					score = 1.0
				}
				switch m.Kind {
				case "failure_mode":
					if len(suspectedFMs) == 0 {
						title := ""
						if d, ok := fmDetails[m.ID]; ok {
							title, _ = d["title"].(string)
						}
						suspectedFMs = append(suspectedFMs, offlineFailureModeMatch{
							ID: m.ID, Title: title, MatchScore: score,
						})
					}
				case "invariant":
					title := ""
					if d, ok := invDetails[m.ID]; ok {
						title, _ = d["title"].(string)
					}
					suspectedInvs = append(suspectedInvs, offlineInvariantMatch{
						ID: m.ID, Title: title, Violated: true,
					})
				}
			}

			// Direct pattern boost: raises score for pattern-specific FM matches.
			// Scores are updated (not skipped) when already present from symptom matching.
			for pID := range patternHits {
				directMatches := directPatternToFailureMode(pID, fmDetails)
				for _, fm := range directMatches {
					updated := false
					for i := range suspectedFMs {
						if suspectedFMs[i].ID == fm.ID {
							if fm.MatchScore > suspectedFMs[i].MatchScore {
								suspectedFMs[i].MatchScore = fm.MatchScore
							}
							updated = true
							break
						}
					}
					if !updated {
						suspectedFMs = append(suspectedFMs, fm)
					}
				}
			}
		} else {
			blindSpots = append(blindSpots, "docs dir not configured — failure mode matching unavailable")
		}

		// Sort failure modes by score descending.
		sort.Slice(suspectedFMs, func(i, j int) bool {
			return suspectedFMs[i].MatchScore > suspectedFMs[j].MatchScore
		})
		if len(suspectedFMs) > 8 {
			suspectedFMs = suspectedFMs[:8]
		}

		// --- Compute confidence ---
		confidence := offlineConfidence(len(events), len(suspectedFMs), docsDir != "")
		if len(sources) == 0 {
			confidence = "unknown"
		}

		// Add blind spots.
		if docsDir == "" {
			blindSpots = append(blindSpots, "no runtime sources available — docs dir missing")
		}
		if logsDir == "" && journalText == "" && systemdStatus == "" {
			blindSpots = append(blindSpots, "no log text or logs_dir provided; evidence is thin")
		}
		if len(events) == 0 {
			blindSpots = append(blindSpots, "no known patterns matched in provided logs")
		}

		// --- Build next diagnostics ---
		// For failure modes with direct diagnostic commands, surface them immediately
		// so the operator doesn't need a follow-up awareness.explain_symptom call.
		// For other modes, point to explain_symptom as before.
		var nextDiag []string
		seen := make(map[string]bool)
		addDiag := func(d string) {
			if !seen[d] {
				seen[d] = true
				nextDiag = append(nextDiag, d)
			}
		}
		for _, fm := range suspectedFMs {
			if fm.MatchScore <= 0.3 {
				continue
			}
			if direct, ok := fmDirectNextDiagnostics[fm.ID]; ok {
				for _, cmd := range direct {
					addDiag(cmd)
				}
			} else {
				addDiag(fmt.Sprintf("run awareness.explain_symptom for failure mode %q", fm.ID))
			}
			if len(nextDiag) >= 8 {
				break
			}
		}
		if len(nextDiag) == 0 {
			nextDiag = append(nextDiag, "run awareness.runtime_snapshot if cluster is partially up")
		}

		return map[string]interface{}{
			"evidence":                    events,
			"timeline":                    timeline,
			"suspected_failure_modes":     suspectedFMs,
			"suspected_invariants":        suspectedInvs,
			"confidence":                  confidence,
			"blind_spots":                 blindSpots,
			"recommended_next_diagnostics": nextDiag,
		}, nil
	})
}

// lineMatchesPattern returns true if the line contains any of the pattern's keywords.
func lineMatchesPattern(line string, pat logPattern) bool {
	lower := strings.ToLower(line)
	for _, kw := range pat.Keywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// extractTimestamp attempts to extract an ISO8601 or common log timestamp from the line.
func extractTimestamp(line string) string {
	// Try RFC3339 first (most precise).
	if m := rfc3339RE.FindString(line); m != "" {
		return m
	}
	// Try common log format.
	if m := commonLogRE.FindString(line); m != "" {
		t, err := time.Parse("2006/01/02 15:04:05", m)
		if err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	// Try journalctl/systemd format.
	if m := journalctlRE.FindString(line); m != "" {
		// Approximate — no year available.
		year := time.Now().UTC().Year()
		full := fmt.Sprintf("%d %s", year, m)
		// Try several formats.
		for _, layout := range []string{
			"2006 Jan  2 15:04:05",
			"2006 Jan 02 15:04:05",
			"2006 Jan  2 15:04:05.000",
			"2006 Jan 02 15:04:05.000",
		} {
			t, err := time.Parse(layout, full)
			if err == nil {
				return t.UTC().Format(time.RFC3339)
			}
		}
	}
	return ""
}

// deduplicateEvents keeps at most maxPerPattern events per patternID.
func deduplicateEvents(events []offlineEvent, maxPerPattern int) []offlineEvent {
	counts := make(map[string]int)
	out := make([]offlineEvent, 0, len(events))
	for _, e := range events {
		if counts[e.Pattern] < maxPerPattern {
			out = append(out, e)
			counts[e.Pattern]++
		}
	}
	return out
}

// buildTimeline creates a time-ordered list of events that have timestamps.
func buildTimeline(events []offlineEvent) []offlineTimeline {
	var tl []offlineTimeline
	for _, e := range events {
		if e.Time == "" {
			continue
		}
		tl = append(tl, offlineTimeline{
			Time:   e.Time,
			Source: e.Source,
			Event:  e.Pattern + ": " + truncate(e.Text, 80),
		})
	}
	sort.Slice(tl, func(i, j int) bool {
		return tl[i].Time < tl[j].Time
	})
	return tl
}

// buildTaskFromEvents creates a search string from extracted event patterns.
func buildTaskFromEvents(events []offlineEvent) string {
	seen := make(map[string]bool)
	var terms []string
	for _, e := range events {
		if !seen[e.Pattern] {
			seen[e.Pattern] = true
			terms = append(terms, e.Pattern)
			// Also include a snippet of the actual text for richer matching.
			words := strings.Fields(e.Text)
			for _, w := range words {
				if len(w) > 4 && !seen[w] {
					seen[w] = true
					terms = append(terms, w)
				}
			}
		}
	}
	return strings.Join(terms, " ")
}

// ---------------------------------------------------------------------------
// Symptom-only failure mode matching
// ---------------------------------------------------------------------------

// offlineStopWords are common English words excluded from event keyword extraction.
var offlineStopWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "from": true,
	"that": true, "this": true, "into": true, "when": true, "not": true,
	"are": true, "was": true, "has": true, "all": true, "any": true,
	"been": true, "will": true, "make": true, "fix": true, "safe": true,
	"via": true, "its": true, "but": true, "due": true, "per": true,
}

// extractFMSymptomsText returns lowercased title + symptom strings for a failure mode.
// root_cause and architecture_fix are deliberately excluded — they contain background
// context that mentions unrelated systems and causes false-positive matches.
func extractFMSymptomsText(fm map[string]interface{}) string {
	var parts []string
	if title, ok := fm["title"].(string); ok && title != "" {
		parts = append(parts, title)
	}
	if symptoms, ok := fm["symptoms"].([]interface{}); ok {
		for _, s := range symptoms {
			if st, ok := s.(string); ok {
				parts = append(parts, st)
			}
		}
	}
	return strings.ToLower(strings.Join(parts, " "))
}

// buildEventKeywords extracts significant lowercase keywords from detected events.
// Pattern ID segments ("etcd_nospace" → "etcd", "nospace") and meaningful log
// words (len ≥ 3, not stop words) are included.
func buildEventKeywords(events []offlineEvent) []string {
	seen := make(map[string]bool)
	var terms []string
	add := func(w string) {
		w = strings.ToLower(strings.Trim(w, ".,;:\"'()[]{}!?-/\\"))
		if len(w) < 3 || offlineStopWords[w] || seen[w] {
			return
		}
		seen[w] = true
		terms = append(terms, w)
	}
	for _, e := range events {
		for _, part := range strings.FieldsFunc(e.Pattern, func(r rune) bool { return r == '_' }) {
			add(part)
		}
		for _, word := range strings.Fields(e.Text) {
			add(word)
		}
	}
	return terms
}

// symptomBasedFMMatch scores failure modes against event keywords using only
// symptom text and title — not root_cause, architecture_fix, or related_services.
// Score = fraction of event keywords that appear in the FM's symptom corpus.
// Returns nil when no events or no failure modes have symptom text.
func symptomBasedFMMatch(events []offlineEvent, fmDetails map[string]map[string]interface{}) []offlineFailureModeMatch {
	if len(events) == 0 || len(fmDetails) == 0 {
		return nil
	}
	keywords := buildEventKeywords(events)
	if len(keywords) == 0 {
		return nil
	}
	var out []offlineFailureModeMatch
	for fmID, detail := range fmDetails {
		corpus := extractFMSymptomsText(detail)
		if corpus == "" {
			continue
		}
		matched := 0
		for _, kw := range keywords {
			if strings.Contains(corpus, kw) {
				matched++
			}
		}
		if matched == 0 {
			continue
		}
		score := float64(matched) / float64(len(keywords))
		if score > 1.0 {
			score = 1.0
		}
		title, _ := detail["title"].(string)
		out = append(out, offlineFailureModeMatch{ID: fmID, Title: title, MatchScore: score})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].MatchScore > out[j].MatchScore
	})
	return out
}

// directPatternToFailureMode maps known pattern IDs to failure mode IDs by keyword similarity.
func directPatternToFailureMode(patternID string, fmDetails map[string]map[string]interface{}) []offlineFailureModeMatch {
	// Keywords must be specific enough to discriminate the target failure mode
	// from unrelated entries sharing generic terms like "etcd" or "leader".
	// Use terms present in the failure mode's symptoms, not just root_cause/fix text.
	keywordMap := map[string][]string{
		"etcd_nospace":           {"nospace", "mvcc", "alarm"},       // etcd-disk-specific; absent from objectstore
		"leader_instability":     {"quorum", "election", "heartbeat"}, // etcd-election-specific; absent from objectstore
		"port_squatting":         {"port", "squatting", "orphan", "cgroup"},
		"unknown_grpc_service":   {"port", "squatting", "grpc", "unknown"},
		"restart_storm":          {"restart", "storm", "loop"},
		"minio_issue":            {"minio", "objectstore", "artifact"},
		"scylladb_issue":         {"scylla", "database"},
		"workflow_stuck":         {"workflow", "dispatch", "converging"},
		"tls_problem":            {"tls", "certificate"},
		"artifact_integrity":     {"artifact", "checksum"},
		"systemd_notify_failure": {"systemd", "notify", "orphan", "port"},
	}
	keywords, ok := keywordMap[patternID]
	if !ok {
		return nil
	}

	var out []offlineFailureModeMatch
	for fmID, detail := range fmDetails {
		blobBytes, _ := marshalToString(detail)
		blob := strings.ToLower(blobBytes)
		matched := 0
		for _, kw := range keywords {
			if strings.Contains(blob, kw) {
				matched++
			}
		}
		if matched >= 2 {
			title, _ := detail["title"].(string)
			score := float64(matched) / float64(len(keywords))
			out = append(out, offlineFailureModeMatch{
				ID:         fmID,
				Title:      title,
				MatchScore: score,
			})
		}
	}
	return out
}

func marshalToString(v interface{}) (string, error) {
	if m, ok := v.(map[string]interface{}); ok {
		var parts []string
		for k, val := range m {
			parts = append(parts, fmt.Sprintf("%s %v", k, val))
		}
		return strings.Join(parts, " "), nil
	}
	return fmt.Sprintf("%v", v), nil
}

// alreadyInFMList is kept for potential future use by other callers.
func alreadyInFMList(list []offlineFailureModeMatch, id string) bool {
	for _, m := range list {
		if m.ID == id {
			return true
		}
	}
	return false
}

func offlineConfidence(eventCount, fmCount int, hasDocs bool) string {
	if eventCount == 0 {
		return "unknown"
	}
	if eventCount > 3 && fmCount > 0 && hasDocs {
		return "medium"
	}
	if eventCount > 0 && hasDocs {
		return "low"
	}
	return "low"
}
