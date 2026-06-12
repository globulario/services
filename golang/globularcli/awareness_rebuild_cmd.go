package main

// awareness_rebuild_cmd.go — `globular awareness rebuild`
//
// Single idempotent command that wraps the manual rebuild/reload pipeline:
//
//   YAML sources → yaml2nt (extractor) → embeddata/awareness.nt → Oxigraph PUT
//
// Usage:
//
//	globular awareness rebuild [flags]
//	globular awareness rebuild --check
//	globular awareness rebuild --no-runtime-reload
//	globular awareness rebuild --strict
//
// The command imports YAML awareness and intent sources using the extractor
// library (same code path as the yaml2nt CLI tool), validates the N-Triples
// output, optionally updates the embeddata seed file, and optionally reloads
// Oxigraph.

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/awareness-graph/golang/extractor"
	"github.com/spf13/cobra"
)

// ── flags ────────────────────────────────────────────────────────────────

var (
	rebuildServicesRepo string // path to services repo (default: auto-detect)
	rebuildAGRepo       string // path to awareness-graph repo (default: sibling)
	rebuildOxigraphURL  string // Oxigraph Graph Store endpoint
	rebuildCheck        bool   // --check: compare only, do not mutate
	rebuildNoReload     bool   // --no-runtime-reload: skip Oxigraph PUT
	rebuildStrict       bool   // --strict: fail on Oxigraph unavailability
)

var awarenessRebuildCmd = &cobra.Command{
	Use:   "rebuild",
	Short: "Rebuild awareness.nt from YAML sources and optionally reload Oxigraph",
	Long: `Idempotent rebuild of the awareness graph seed from docs/awareness/ YAML.

Normal mode:
  1. Scan YAML sources from both repos (awareness-graph + services)
  2. Convert to N-Triples via the extractor library
  3. Validate the output
  4. Update embeddata/awareness.nt
  5. PUT to Oxigraph if available

Check mode (--check):
  Regenerate into memory, compare with committed embeddata/awareness.nt,
  exit 1 if stale. Does not mutate any files. Suitable for CI.

The command auto-detects repo paths when run from the services checkout.
Override with --services-repo and --ag-repo if needed.`,
	RunE: runAwarenessRebuild,
}

// ── entry point ──────────────────────────────────────────────────────────

func runAwarenessRebuild(cmd *cobra.Command, args []string) error {
	// ── resolve repo paths ───────────────────────────────────────────────
	svcRepo, err := resolveServicesRepo()
	if err != nil {
		return err
	}
	agRepo, err := resolveAGRepo(svcRepo)
	if err != nil {
		return err
	}

	// ── collect input directories ────────────────────────────────────────
	inputDirs, intentDir, err := collectInputDirs(svcRepo, agRepo)
	if err != nil {
		return err
	}

	seedPath := filepath.Join(agRepo, "golang", "server", "embeddata", "awareness.nt")

	// ── generate N-Triples ───────────────────────────────────────────────
	fmt.Println("Scanning YAML sources...")
	ntBytes, totalTriples, yamlCount, err := generateNTriples(inputDirs, intentDir, svcRepo, agRepo)
	if err != nil {
		return err
	}
	fmt.Printf("  YAML files scanned: %d\n", yamlCount)
	fmt.Printf("  triples generated:  %d\n", totalTriples)

	// ── validate ─────────────────────────────────────────────────────────
	if errs := extractor.ValidateNTriples(bytes.NewReader(ntBytes)); len(errs) > 0 {
		for i, e := range errs {
			if i >= 20 {
				fmt.Fprintf(os.Stderr, "  ... %d more validation errors omitted\n", len(errs)-i)
				break
			}
			fmt.Fprintf(os.Stderr, "  validation: %s\n", e)
		}
		return fmt.Errorf("%d N-Triples validation errors — refusing to write", len(errs))
	}
	fmt.Println("  validation:         ok")

	// ── check mode ───────────────────────────────────────────────────────
	if rebuildCheck {
		return runCheckMode(ntBytes, seedPath)
	}

	// ── update embeddata ─────────────────────────────────────────────────
	updated, err := updateEmbeddata(ntBytes, seedPath)
	if err != nil {
		return err
	}
	if updated {
		fmt.Printf("  embeddata updated:  yes (%s)\n", seedPath)
	} else {
		fmt.Println("  embeddata updated:  no (already current)")
	}

	// ── Oxigraph reload ──────────────────────────────────────────────────
	if rebuildNoReload {
		fmt.Println("  Oxigraph reload:    skipped (--no-runtime-reload)")
	} else {
		err := reloadOxigraph(ntBytes)
		if err != nil {
			if rebuildStrict {
				return fmt.Errorf("Oxigraph reload failed (--strict): %w", err)
			}
			fmt.Printf("  Oxigraph reload:    skipped (%v)\n", err)
			fmt.Println("  seed rebuilt, runtime reload skipped")
		} else {
			fmt.Println("  Oxigraph reload:    ok")
		}
	}

	fmt.Println()
	fmt.Println("Done.")
	return nil
}

// ── N-Triples generation ─────────────────────────────────────────────────

func generateNTriples(inputDirs []string, intentDir string, svcRepo, agRepo string) (ntBytes []byte, totalTriples int, yamlCount int, err error) {
	var buf bytes.Buffer
	opts := extractor.ImportDirOptions{
		StripPathPrefixes: []string{agRepo, svcRepo},
	}

	for _, dir := range inputDirs {
		emitter, report, dirErr := extractor.ImportAwarenessDirWithOpts(dir, &buf, opts)
		if dirErr != nil {
			return nil, 0, 0, fmt.Errorf("import %s: %w", dir, dirErr)
		}
		totalTriples += emitter.Triples
		yamlCount += len(report.Files)
	}

	if intentDir != "" {
		emitter, report, dirErr := extractor.ImportAwarenessDirWithOpts(intentDir, &buf, opts)
		if dirErr != nil {
			return nil, 0, 0, fmt.Errorf("import intent %s: %w", intentDir, dirErr)
		}
		totalTriples += emitter.Triples
		yamlCount += len(report.Files)
	}

	return buf.Bytes(), totalTriples, yamlCount, nil
}

// ── embeddata update ─────────────────────────────────────────────────────

func updateEmbeddata(ntBytes []byte, seedPath string) (updated bool, err error) {
	// Compare by content hash to ensure idempotency.
	newHash := sha256.Sum256(ntBytes)

	existing, readErr := os.ReadFile(seedPath)
	if readErr == nil {
		oldHash := sha256.Sum256(existing)
		if newHash == oldHash {
			return false, nil // no change
		}
	}
	// Ensure parent directory exists.
	if mkErr := os.MkdirAll(filepath.Dir(seedPath), 0o755); mkErr != nil {
		return false, fmt.Errorf("mkdir embeddata: %w", mkErr)
	}
	if wErr := os.WriteFile(seedPath, ntBytes, 0o644); wErr != nil {
		return false, fmt.Errorf("write embeddata: %w", wErr)
	}
	return true, nil
}

// ── check mode ───────────────────────────────────────────────────────────

func runCheckMode(ntBytes []byte, seedPath string) error {
	fmt.Println()
	fmt.Println("Check mode: comparing with committed embeddata...")

	committed, err := os.ReadFile(seedPath)
	if err != nil {
		return fmt.Errorf("read committed seed: %w", err)
	}

	newHash := sha256.Sum256(ntBytes)
	oldHash := sha256.Sum256(committed)

	if newHash == oldHash {
		fmt.Println("  status: fresh (no changes)")
		return nil
	}

	newLines := bytes.Count(ntBytes, []byte("\n"))
	oldLines := bytes.Count(committed, []byte("\n"))
	fmt.Fprintf(os.Stderr, "  STALE: embeddata/awareness.nt\n")
	fmt.Fprintf(os.Stderr, "    committed: %d lines, sha256:%x\n", oldLines, oldHash)
	fmt.Fprintf(os.Stderr, "    generated: %d lines, sha256:%x\n", newLines, newHash)
	fmt.Fprintf(os.Stderr, "\nRun 'globular awareness rebuild' and commit the result.\n")
	os.Exit(1)
	return nil // unreachable
}

// ── Oxigraph reload ──────────────────────────────────────────────────────

func reloadOxigraph(ntBytes []byte) error {
	endpoint, err := normalizeOxigraphURL(rebuildOxigraphURL)
	if err != nil {
		return fmt.Errorf("invalid oxigraph-url: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(ntBytes))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/n-triples")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func normalizeOxigraphURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("scheme must be http or https")
	}
	if u.Host == "" {
		return "", fmt.Errorf("host is required")
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = "/store"
	}
	if strings.HasSuffix(u.Path, "/query") {
		u.Path = strings.TrimSuffix(u.Path, "/query") + "/store"
	}
	if u.RawQuery == "" {
		u.RawQuery = "default"
	}
	return u.String(), nil
}

// ── repo path resolution ─────────────────────────────────────────────────

func resolveServicesRepo() (string, error) {
	if rebuildServicesRepo != "" {
		return filepath.Abs(rebuildServicesRepo)
	}
	// Walk up from cwd looking for docs/awareness/namespaces.yaml (services repo marker).
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, sErr := os.Stat(filepath.Join(dir, "docs", "awareness", "namespaces.yaml")); sErr == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("cannot find services repo (no docs/awareness/namespaces.yaml in ancestors)\n  set --services-repo or run from inside the services checkout")
}

func resolveAGRepo(svcRepo string) (string, error) {
	if rebuildAGRepo != "" {
		return filepath.Abs(rebuildAGRepo)
	}
	// Default: sibling of the services repo.
	candidate := filepath.Join(filepath.Dir(svcRepo), "awareness-graph")
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		if _, yamlErr := os.Stat(filepath.Join(candidate, "docs", "awareness")); yamlErr == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("cannot find awareness-graph repo (checked %s)\n  set --ag-repo or clone it as a sibling of the services repo", candidate)
}

// collectInputDirs returns the awareness YAML directories to scan and the
// intent directory. This mirrors the yaml2nt invocation in
// build-awareness-graph.sh: AG/docs/awareness + SVC/docs/awareness +
// SVC/docs/awareness/generated, plus SVC/docs/intent.
func collectInputDirs(svcRepo, agRepo string) (inputDirs []string, intentDir string, err error) {
	dirs := []string{
		filepath.Join(agRepo, "docs", "awareness"),
		filepath.Join(svcRepo, "docs", "awareness"),
		filepath.Join(svcRepo, "docs", "awareness", "generated"),
	}
	for _, d := range dirs {
		if _, sErr := os.Stat(d); sErr != nil {
			return nil, "", fmt.Errorf("input directory not found: %s", d)
		}
	}
	intent := filepath.Join(svcRepo, "docs", "intent")
	if _, sErr := os.Stat(intent); sErr != nil {
		intent = "" // intent is optional
	}
	return dirs, intent, nil
}

// ── wiring ───────────────────────────────────────────────────────────────

func init() {
	awarenessRebuildCmd.Flags().StringVar(&rebuildServicesRepo, "services-repo", "",
		"Path to services repo (default: auto-detect from cwd)")
	awarenessRebuildCmd.Flags().StringVar(&rebuildAGRepo, "ag-repo", "",
		"Path to awareness-graph repo (default: sibling of services repo)")
	awarenessRebuildCmd.Flags().StringVar(&rebuildOxigraphURL, "oxigraph-url", "http://localhost:7878/store?default",
		"Oxigraph Graph Store endpoint")
	awarenessRebuildCmd.Flags().BoolVar(&rebuildCheck, "check", false,
		"Compare generated output with committed embeddata; exit 1 if stale (CI mode)")
	awarenessRebuildCmd.Flags().BoolVar(&rebuildNoReload, "no-runtime-reload", false,
		"Skip Oxigraph PUT after updating embeddata")
	awarenessRebuildCmd.Flags().BoolVar(&rebuildStrict, "strict", false,
		"Fail if Oxigraph is unavailable (default: warn and continue)")

	awarenessCmd.AddCommand(awarenessRebuildCmd)
}
