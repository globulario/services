package main

// awareness_ingest_cmd.go — `globular awareness ingest`
//
// Single entry point for feeding new knowledge into the awareness graph.
//
// Three sources:
//   --from-file    <path.yaml>      append entries to canonical YAML, then rebuild
//   --from-incident <INC-2026-XXXX> generate a candidate from an ai-memory incident
//   --from-scan                     re-run annotation scanner + rebuild
//
// Usage:
//
//	globular awareness ingest --from-file docs/awareness/my_entry.yaml
//	globular awareness ingest --from-incident INC-2026-0019
//	globular awareness ingest --from-scan
//	globular awareness ingest --from-file entry.yaml --dry-run

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"

	"github.com/globulario/services/golang/ai_memory/ai_memorypb"
	"github.com/globulario/services/golang/config"
)

// ── flags ────────────────────────────────────────────────────────────────

var (
	ingestFromFile     string
	ingestFromIncident string
	ingestFromScan     bool
	ingestDryRun       bool
	ingestNoRebuild    bool
)

var awarenessIngestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Feed new knowledge into the awareness graph",
	Long: `Add new entries to awareness YAML and rebuild the graph.

Sources (exactly one required):

  --from-file <path.yaml>
    Read a YAML file containing awareness entries (invariants, failure_modes,
    incident_patterns, or candidates). Entries are appended to the matching
    canonical YAML file based on the top-level key. Then rebuild is triggered.

  --from-incident <INC-ID>
    Generate a candidate YAML entry from an ai-memory incident record.
    Queries ai-memory for the incident by tag, extracts available fields,
    and writes a review-ready candidate to docs/awareness/candidates/.
    If ai-memory is unreachable, generates a template with the ID pre-filled.

  --from-scan
    Re-run the annotation scanner on all services and rebuild. Requires
    the awareness-graph repo as a sibling directory. This is equivalent to
    running scripts/build-awareness-graph.sh from the awareness-graph repo.`,
	RunE: runAwarenessIngest,
}

func runAwarenessIngest(cmd *cobra.Command, args []string) error {
	sources := 0
	if ingestFromFile != "" {
		sources++
	}
	if ingestFromIncident != "" {
		sources++
	}
	if ingestFromScan {
		sources++
	}
	if sources != 1 {
		return fmt.Errorf("exactly one of --from-file, --from-incident, or --from-scan is required")
	}

	switch {
	case ingestFromFile != "":
		return runIngestFromFile()
	case ingestFromIncident != "":
		return runIngestFromIncident()
	case ingestFromScan:
		return runIngestFromScan()
	}
	return nil
}

// ── from-file ────────────────────────────────────────────────────────────

// classToCanonicalFile maps awareness classes to their canonical YAML filenames.
var classToCanonicalFile = map[string]string{
	"invariants":        "invariants.yaml",
	"failure_modes":     "failure_modes.yaml",
	"incident_patterns": "incident_patterns.yaml",
	"forbidden_fixes":   "forbidden_fixes.yaml",
	"required_tests":    "required_tests.yaml",
	"candidates":        "", // stays in candidates/
}

func runIngestFromFile() error {
	absPath, err := filepath.Abs(ingestFromFile)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	// Parse to detect top-level key.
	var doc map[string]interface{}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parse YAML: %w", err)
	}

	// Find the top-level list key.
	var topKey string
	for k := range doc {
		if _, known := classToCanonicalFile[k]; known {
			topKey = k
			break
		}
	}
	if topKey == "" {
		return fmt.Errorf("unrecognized top-level key in %s; expected one of: %s",
			absPath, "invariants, failure_modes, incident_patterns, forbidden_fixes, required_tests, candidates")
	}

	svcRepo, err := resolveServicesRepo()
	if err != nil {
		return err
	}

	// If it's a candidates file, copy to candidates/.
	if topKey == "candidates" {
		destDir := filepath.Join(svcRepo, "docs", "awareness", "candidates")
		dest := filepath.Join(destDir, filepath.Base(absPath))
		if ingestDryRun {
			fmt.Printf("[dry-run] would copy %s → %s\n", absPath, dest)
			return nil
		}
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, raw, 0o644); err != nil {
			return fmt.Errorf("write candidate: %w", err)
		}
		fmt.Printf("  candidate file written: %s\n", dest)
		fmt.Println("  (use 'globular awareness promote <id>' to promote entries)")
		return nil
	}

	// Merge entries into the canonical file.
	canonicalFile := classToCanonicalFile[topKey]
	targetPath := filepath.Join(svcRepo, "docs", "awareness", canonicalFile)

	// Load existing canonical file.
	existingRaw, err := os.ReadFile(targetPath)
	if err != nil {
		return fmt.Errorf("read canonical %s: %w", canonicalFile, err)
	}
	var existing map[string]interface{}
	if err := yaml.Unmarshal(existingRaw, &existing); err != nil {
		return fmt.Errorf("parse canonical %s: %w", canonicalFile, err)
	}

	// Extract new entries from input.
	newEntries, ok := doc[topKey].([]interface{})
	if !ok || len(newEntries) == 0 {
		return fmt.Errorf("no entries found under %q in %s", topKey, absPath)
	}

	// Check for duplicate IDs.
	existingIDs := collectIDs(existing[topKey])
	var added int
	for _, entry := range newEntries {
		m, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		if id == "" {
			return fmt.Errorf("entry missing 'id' field in %s", absPath)
		}
		if existingIDs[id] {
			fmt.Printf("  skipped (duplicate): %s\n", id)
			continue
		}
		existingList, _ := existing[topKey].([]interface{})
		existing[topKey] = append(existingList, entry)
		existingIDs[id] = true
		added++
		fmt.Printf("  added: %s\n", id)
	}

	if added == 0 {
		fmt.Println("  no new entries to add (all duplicates)")
		return nil
	}

	if ingestDryRun {
		fmt.Printf("[dry-run] would append %d entries to %s\n", added, canonicalFile)
		return nil
	}

	// Write back.
	out, err := yaml.Marshal(existing)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(targetPath, out, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", canonicalFile, err)
	}
	fmt.Printf("  wrote %d new entries to %s\n", added, targetPath)

	return maybeRebuild()
}

func collectIDs(list interface{}) map[string]bool {
	ids := make(map[string]bool)
	entries, ok := list.([]interface{})
	if !ok {
		return ids
	}
	for _, e := range entries {
		m, ok := e.(map[string]interface{})
		if !ok {
			continue
		}
		if id, ok := m["id"].(string); ok {
			ids[id] = true
		}
	}
	return ids
}

// ── from-incident ────────────────────────────────────────────────────────

var incidentIDPattern = regexp.MustCompile(`^INC-\d{4}-\d{4}$`)

func runIngestFromIncident() error {
	incID := ingestFromIncident
	if !incidentIDPattern.MatchString(incID) {
		return fmt.Errorf("incident ID must match INC-YYYY-NNNN pattern, got %q", incID)
	}

	svcRepo, err := resolveServicesRepo()
	if err != nil {
		return err
	}

	// Try to fetch from ai-memory.
	title, content, tags, metadata := queryIncidentFromMemory(incID)

	// Generate candidate YAML.
	candidate := buildIncidentCandidate(incID, title, content, tags, metadata)

	candidateYAML, err := yaml.Marshal(map[string]interface{}{
		"candidates": []interface{}{candidate},
	})
	if err != nil {
		return fmt.Errorf("marshal candidate: %w", err)
	}

	fileName := fmt.Sprintf("%s.yaml", strings.ToLower(strings.ReplaceAll(incID, "-", "_")))
	destPath := filepath.Join(svcRepo, "docs", "awareness", "candidates", fileName)

	if ingestDryRun {
		fmt.Println("[dry-run] generated candidate:")
		fmt.Println(string(candidateYAML))
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(destPath, candidateYAML, 0o644); err != nil {
		return fmt.Errorf("write candidate: %w", err)
	}
	fmt.Printf("  candidate written: %s\n", destPath)
	fmt.Println("  review the entry, then run: globular awareness promote <id>")
	return nil
}

func queryIncidentFromMemory(incID string) (title, content string, tags []string, metadata map[string]string) {
	// Best-effort: try to reach ai-memory. If unavailable, return empty.
	addr := config.ResolveServiceAddr("ai_memory.AiMemoryService", "globular.internal:443")
	if addr == "" {
		fmt.Println("  ai-memory: address not resolved, generating template only")
		return "", "", nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try insecure first (local dev), then let it fail gracefully.
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		fmt.Printf("  ai-memory: unreachable (%v), generating template only\n", err)
		return "", "", nil, nil
	}
	defer conn.Close()

	client := ai_memorypb.NewAiMemoryServiceClient(conn)
	rsp, err := client.Query(ctx, &ai_memorypb.QueryRqst{
		Project:    "globular-services",
		Tags:       []string{incID},
		TextSearch: incID,
		Limit:      1,
	})
	if err != nil {
		fmt.Printf("  ai-memory: query failed (%v), generating template only\n", err)
		return "", "", nil, nil
	}

	if len(rsp.GetMemories()) == 0 {
		fmt.Printf("  ai-memory: no entry found for %s, generating template only\n", incID)
		return "", "", nil, nil
	}

	m := rsp.GetMemories()[0]
	fmt.Printf("  ai-memory: found %q (%s)\n", m.GetTitle(), m.GetId())
	return m.GetTitle(), m.GetContent(), m.GetTags(), m.GetMetadata()
}

func buildIncidentCandidate(incID, title, content string, tags []string, metadata map[string]string) map[string]interface{} {
	// Build a candidate entry from what we have.
	candidateID := "pat." + strings.ToLower(strings.ReplaceAll(incID, "-", "_"))

	if title == "" {
		title = incID + ": (fill in one-line summary)"
	}

	candidate := map[string]interface{}{
		"id":    candidateID,
		"class": "incident_pattern",
		"label": title,
		"status":          "candidate",
		"review_required": true,
		"discovered_from": incID,
		"confidence":      "medium",
	}

	// Extract component from tags or metadata.
	component := ""
	if c, ok := metadata["component"]; ok {
		component = c
	}
	for _, t := range tags {
		if t != "incident" && t != incID {
			if component == "" {
				component = t
			}
		}
	}

	if content != "" {
		candidate["evidence"] = content
		// Try to extract root_cause if present in structured content.
		if idx := strings.Index(content, "## Root Cause"); idx >= 0 {
			rest := content[idx+len("## Root Cause"):]
			if end := strings.Index(rest, "\n## "); end > 0 {
				candidate["root_cause"] = strings.TrimSpace(rest[:end])
			}
		}
	} else {
		candidate["evidence"] = fmt.Sprintf("(extracted from ai-memory incident %s — fill in details)", incID)
	}

	if component != "" {
		candidate["risk"] = "medium"
		candidate["source_file"] = fmt.Sprintf("golang/%s/ (verify path)", component)
	} else {
		candidate["risk"] = "medium"
		candidate["source_file"] = "(fill in affected file path)"
	}

	return candidate
}

// ── from-scan ────────────────────────────────────────────────────────────

func runIngestFromScan() error {
	svcRepo, err := resolveServicesRepo()
	if err != nil {
		return err
	}
	agRepo, err := resolveAGRepo(svcRepo)
	if err != nil {
		return err
	}

	scriptPath := filepath.Join(agRepo, "scripts", "build-awareness-graph.sh")
	if _, sErr := os.Stat(scriptPath); sErr != nil {
		return fmt.Errorf("build script not found: %s", scriptPath)
	}

	if ingestDryRun {
		fmt.Printf("[dry-run] would run: %s\n", scriptPath)
		return nil
	}

	fmt.Println("Running annotation scanner + rebuild...")
	cmd := exec.Command("bash", scriptPath)
	cmd.Dir = agRepo
	cmd.Env = append(os.Environ(), "SERVICES_REPO="+svcRepo)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build-awareness-graph.sh failed: %w", err)
	}

	fmt.Println()
	fmt.Println("Done. Annotation scan + rebuild complete.")
	return nil
}

// ── shared ───────────────────────────────────────────────────────────────

func maybeRebuild() error {
	if ingestNoRebuild {
		fmt.Println("  rebuild: skipped (--no-rebuild)")
		return nil
	}
	fmt.Println()
	fmt.Println("Triggering rebuild...")
	// Reuse the rebuild logic from awareness_rebuild_cmd.go.
	rebuildNoReload = false
	rebuildCheck = false
	rebuildStrict = false
	return runAwarenessRebuild(nil, nil)
}

// ── wiring ───────────────────────────────────────────────────────────────

func init() {
	awarenessIngestCmd.Flags().StringVar(&ingestFromFile, "from-file", "",
		"Path to a YAML file with awareness entries to ingest")
	awarenessIngestCmd.Flags().StringVar(&ingestFromIncident, "from-incident", "",
		"Incident ID (INC-YYYY-NNNN) to generate a candidate from ai-memory")
	awarenessIngestCmd.Flags().BoolVar(&ingestFromScan, "from-scan", false,
		"Re-run annotation scanner on all services and rebuild")
	awarenessIngestCmd.Flags().BoolVar(&ingestDryRun, "dry-run", false,
		"Validate and show what would happen; do not modify files")
	awarenessIngestCmd.Flags().BoolVar(&ingestNoRebuild, "no-rebuild", false,
		"Skip the automatic rebuild after ingestion")

	awarenessCmd.AddCommand(awarenessIngestCmd)
}
