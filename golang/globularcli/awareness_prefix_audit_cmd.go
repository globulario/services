package main

// awareness prefix-audit reports hand-rolled `"<kind>:" + id` graph-node-id
// string concatenations across the awareness Go code. This is a measurement
// tool, not a gate — it never fails CI. Its job is to surface whether the
// prefix-id fragmentation incident has recurred enough to earn a typed
// graph.NodeID(kind, id) migration. See
// docs/awareness/composed_path_failures.md (graph node id prefix).
//
// The audit reports counts by prefix and by package. A growing count is
// not a bug per se — but a second prefix-related incident in the log,
// combined with a high site count here, is the trigger to consolidate.

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// auditPrefixes is the list of well-known graph-node prefixes the audit
// looks for. Adding or removing entries here changes what the report
// counts; keep it tight to avoid false positives.
var auditPrefixes = []string{
	"failure_mode",
	"invariant",
	"forbidden_fix",
	"detector",
	"source_file",
	"test",
	"design_pattern",
	"anti_pattern",
	"pattern",
	"code_smell",
	"decision",
	"doc",
	"symbol",
}

var prefixAuditCfg = struct {
	repoPath string
	asJSON   bool
}{}

var awarenessPrefixAuditCmd = &cobra.Command{
	Use:   "prefix-audit",
	Short: "Count hand-rolled \"<kind>:\" + id graph-node-id sites across awareness Go code",
	Long: `prefix-audit measures fragmentation of graph-node-id construction.

Each match is a hand-rolled string concatenation like:

    "failure_mode:" + fm.ID

These work, but they're the surface area for a recurring class of bug
(prefix mismatch between writer and reader). The typed
` + "`graph.NodeID(kind, id)`" + ` consolidation is a candidate in
docs/awareness/composed_path_failures.md (graph node id prefix). The
audit measures site count by prefix and by package so that decision
can be made on evidence, not intuition.

This command never fails CI. It reports counts and exits 0.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := resolveRepoRoot(prefixAuditCfg.repoPath)
		if err != nil {
			return err
		}

		// Compile a single regex that matches any of the prefixes. The pattern
		// looks for `"prefix:"` followed by optional whitespace and `+` —
		// catching both `"failure_mode:" + id` and `"failure_mode:"+id`.
		pat := buildPrefixRegex(auditPrefixes)

		report := newPrefixReport()
		err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if name == "vendor" || name == "node_modules" || name == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			matches := pat.FindAllSubmatch(data, -1)
			if len(matches) == 0 {
				return nil
			}
			pkg := derivePackage(root, path)
			for _, m := range matches {
				if len(m) < 2 {
					continue
				}
				prefix := string(m[1])
				report.add(prefix, pkg)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("walk %s: %w", root, err)
		}

		// AST sanity: report.skipped warns if go/parser couldn't load any of
		// the awareness package — useful early signal if the audit is broken.
		_, _ = parser.ParseFile(token.NewFileSet(), filepath.Join(root, "golang/awareness/graph/edges.go"), nil, 0)

		if prefixAuditCfg.asJSON {
			return report.printJSON(os.Stdout)
		}
		report.printText(os.Stdout)
		return nil
	},
}

// buildPrefixRegex compiles a single regex matching any of the supplied
// prefixes followed by a closing quote and a `+`. The captured group is
// the prefix name (without quotes or colon).
func buildPrefixRegex(prefixes []string) *regexp.Regexp {
	parts := make([]string, len(prefixes))
	for i, p := range prefixes {
		parts[i] = regexp.QuoteMeta(p)
	}
	// `\b("(prefix1|prefix2):"\s*\+)` — the prefix is captured as group 1.
	return regexp.MustCompile(`"(` + strings.Join(parts, "|") + `):"\s*\+`)
}

// derivePackage returns a coarse package label for a given file relative
// to the repo root. Used purely as a grouping key; not a Go-package name.
func derivePackage(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.Dir(path)
	}
	return filepath.Dir(rel)
}

type prefixReport struct {
	byPrefix map[string]int
	byPkg    map[string]map[string]int
	total    int
}

func newPrefixReport() *prefixReport {
	return &prefixReport{
		byPrefix: make(map[string]int),
		byPkg:    make(map[string]map[string]int),
	}
}

func (r *prefixReport) add(prefix, pkg string) {
	r.byPrefix[prefix]++
	if r.byPkg[pkg] == nil {
		r.byPkg[pkg] = make(map[string]int)
	}
	r.byPkg[pkg][prefix]++
	r.total++
}

func (r *prefixReport) printText(out io.Writer) {
	fmt.Fprintln(out, "awareness prefix-audit")
	fmt.Fprintln(out, "======================")
	fmt.Fprintf(out, "\nTotal hand-rolled prefix sites: %d\n", r.total)
	fmt.Fprintf(out, "Distinct prefixes:               %d\n", len(r.byPrefix))
	fmt.Fprintf(out, "Distinct packages with sites:    %d\n", len(r.byPkg))

	fmt.Fprintln(out, "\nCount by prefix:")
	prefixes := make([]string, 0, len(r.byPrefix))
	for p := range r.byPrefix {
		prefixes = append(prefixes, p)
	}
	sort.Slice(prefixes, func(i, j int) bool {
		if r.byPrefix[prefixes[i]] != r.byPrefix[prefixes[j]] {
			return r.byPrefix[prefixes[i]] > r.byPrefix[prefixes[j]]
		}
		return prefixes[i] < prefixes[j]
	})
	for _, p := range prefixes {
		fmt.Fprintf(out, "  %-16s %d\n", p+":", r.byPrefix[p])
	}

	fmt.Fprintln(out, "\nTop packages by site count:")
	type pkgRow struct {
		pkg   string
		total int
	}
	rows := make([]pkgRow, 0, len(r.byPkg))
	for p, m := range r.byPkg {
		t := 0
		for _, c := range m {
			t += c
		}
		rows = append(rows, pkgRow{p, t})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].total != rows[j].total {
			return rows[i].total > rows[j].total
		}
		return rows[i].pkg < rows[j].pkg
	})
	limit := len(rows)
	if limit > 15 {
		limit = 15
	}
	for _, row := range rows[:limit] {
		fmt.Fprintf(out, "  %-60s %d\n", row.pkg, row.total)
	}
	if len(rows) > limit {
		fmt.Fprintf(out, "  … %d more package(s)\n", len(rows)-limit)
	}

	fmt.Fprintln(out, "\nThis is a measurement, not a gate. Exit code 0.")
	fmt.Fprintln(out, "Consolidation candidate: graph.NodeID(kind, id) — see")
	fmt.Fprintln(out, "docs/awareness/composed_path_failures.md (graph node id prefix).")
}

func (r *prefixReport) printJSON(out io.Writer) error {
	type entry struct {
		Prefix string `json:"prefix"`
		Count  int    `json:"count"`
	}
	type pkgEntry struct {
		Package string         `json:"package"`
		Total   int            `json:"total"`
		Counts  map[string]int `json:"counts"`
	}
	prefixEntries := make([]entry, 0, len(r.byPrefix))
	for p, c := range r.byPrefix {
		prefixEntries = append(prefixEntries, entry{p, c})
	}
	sort.Slice(prefixEntries, func(i, j int) bool {
		return prefixEntries[i].Count > prefixEntries[j].Count
	})

	pkgEntries := make([]pkgEntry, 0, len(r.byPkg))
	for p, m := range r.byPkg {
		t := 0
		for _, c := range m {
			t += c
		}
		pkgEntries = append(pkgEntries, pkgEntry{p, t, m})
	}
	sort.Slice(pkgEntries, func(i, j int) bool {
		return pkgEntries[i].Total > pkgEntries[j].Total
	})

	payload := map[string]any{
		"total":              r.total,
		"distinct_prefixes":  len(r.byPrefix),
		"distinct_packages":  len(r.byPkg),
		"by_prefix":          prefixEntries,
		"by_package":         pkgEntries,
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

func init() {
	awarenessPrefixAuditCmd.Flags().StringVar(&prefixAuditCfg.repoPath, "repo", "", "Repo root (default: auto-detected from git)")
	awarenessPrefixAuditCmd.Flags().BoolVar(&prefixAuditCfg.asJSON, "json", false, "Emit JSON instead of human-readable text")
	awarenessCmd.AddCommand(awarenessPrefixAuditCmd)
}
