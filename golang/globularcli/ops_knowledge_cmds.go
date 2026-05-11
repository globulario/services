package main

// ops_knowledge_cmds.go — operator interface to docs/operational-knowledge/.
//
//   globular ops-knowledge validate [--dir <path>]
//   globular ops-knowledge hash --id <entry-id>
//   globular ops-knowledge seed [--dir <path>] [--seed-version <ver>]
//   globular ops-knowledge verify [--dir <path>]
//   globular ops-knowledge list [--stage day-1] [--type SKILL]
//
// validate / hash run against on-disk YAML only — no cluster connectivity.
// seed / verify / list talk to ai-memory.AiMemoryService over the cluster
// mesh and require an authenticated token (use `globular auth login` first).
//
// CRITICAL: until the ai-memory immutability layer ships, the seed command
// can also be used to OVERWRITE seed entries — preserving the contract is
// currently a convention, not a hard guarantee. The `verify` command catches
// drift, doctor invariant ops_knowledge.seed_integrity will surface it
// automatically once the runtime check is wired in cluster-doctor.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	ai_memorypb "github.com/globulario/services/golang/ai_memory/ai_memorypb"
	"github.com/globulario/services/golang/opsknowledge"
)

const (
	opsKnowledgeProject = "globular-services"
	opsKnowledgeSeedTag = "seed"
)

var opsKnowledgeCmd = &cobra.Command{
	Use:   "ops-knowledge",
	Short: "Manage the operational-knowledge seed (validate, hash, seed, verify, list)",
	Long: `Tools for the operational-knowledge YAML seed shipped with every Globular release.

` + "`validate`" + ` and ` + "`hash`" + ` run locally against on-disk YAML.
` + "`seed`" + `, ` + "`verify`" + `, and ` + "`list`" + ` talk to the AI Memory service
over the cluster mesh — run ` + "`globular auth login --user sa --password ...`" + `
first to obtain a token.`,
}

var (
	opsKnowledgeValidateDir       string
	opsKnowledgeValidateAwareness string
)

var opsKnowledgeValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate operational-knowledge YAML files against the schema",
	Long: `Lints every .yaml under <dir>/{stages,runbooks,service-roles}/ for schema
conformance, namespace rules, link integrity, and content size limits.

Returns exit code 0 if all files validate clean (warnings allowed),
exit 1 if any error-severity finding is reported.

Reads cross-file references from <awareness-dir>/{invariants,failure_modes}.yaml
to verify that links.awareness_invariants and links.awareness_failure_modes
target ids that actually exist.`,
	RunE: runOpsKnowledgeValidate,
}

var (
	opsKnowledgeHashID  string
	opsKnowledgeHashDir string
)

var opsKnowledgeHashCmd = &cobra.Command{
	Use:   "hash",
	Short: "Print the canonical SHA256 of an operational-knowledge entry",
	Long: `Computes the canonical-form SHA256 hash of a single entry by id.
This is what the build pipeline will stamp into provenance.seed_sha256.

Useful for diagnosing drift findings from cluster-doctor.`,
	RunE: runOpsKnowledgeHash,
}

var (
	opsKnowledgeSeedDir         string
	opsKnowledgeSeedVersion     string
	opsKnowledgeSeedDryRun      bool
	opsKnowledgeSeedAddr        string
)

var opsKnowledgeSeedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Push every operational-knowledge entry into AI Memory (idempotent)",
	Long: `Walks the operational-knowledge directory, computes the canonical SHA256
of every entry, and upserts each one into the AI Memory service with
metadata.source = "seed".

Idempotent: same id + same sha256 = noop; same id + new sha256 = update.

Use --dry-run to print the plan without writing.`,
	RunE: runOpsKnowledgeSeed,
}

var (
	opsKnowledgeVerifyDir  string
	opsKnowledgeVerifyAddr string
)

var opsKnowledgeVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify AI Memory seed entries match the on-disk operational-knowledge YAML",
	Long: `Reads every AI Memory entry tagged "seed", recomputes the SHA256 from the
local YAML, and reports drift (missing entries, extra entries, content
mismatch).

Returns exit 0 if everything matches, exit 1 on any drift.`,
	RunE: runOpsKnowledgeVerify,
}

var (
	opsKnowledgeListAddr  string
	opsKnowledgeListStage string
	opsKnowledgeListType  string
)

var opsKnowledgeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List operational-knowledge seed entries currently loaded in AI Memory",
	RunE:  runOpsKnowledgeList,
}

func init() {
	opsKnowledgeValidateCmd.Flags().StringVar(&opsKnowledgeValidateDir, "dir", "docs/operational-knowledge",
		"Path to the operational-knowledge directory")
	opsKnowledgeValidateCmd.Flags().StringVar(&opsKnowledgeValidateAwareness, "awareness-dir", "docs/awareness",
		"Path to the awareness directory (for link integrity checks)")
	opsKnowledgeCmd.AddCommand(opsKnowledgeValidateCmd)

	opsKnowledgeHashCmd.Flags().StringVar(&opsKnowledgeHashID, "id", "",
		"Entry id to hash (e.g. ops.day-1.minio.topology-contract). Required.")
	opsKnowledgeHashCmd.Flags().StringVar(&opsKnowledgeHashDir, "dir", "docs/operational-knowledge",
		"Path to the operational-knowledge directory")
	_ = opsKnowledgeHashCmd.MarkFlagRequired("id")
	opsKnowledgeCmd.AddCommand(opsKnowledgeHashCmd)

	opsKnowledgeSeedCmd.Flags().StringVar(&opsKnowledgeSeedDir, "dir", "docs/operational-knowledge",
		"Path to the operational-knowledge directory")
	opsKnowledgeSeedCmd.Flags().StringVar(&opsKnowledgeSeedVersion, "seed-version", "dev-untracked",
		"Seed version to stamp into provenance metadata (typically the awareness bundle version)")
	opsKnowledgeSeedCmd.Flags().BoolVar(&opsKnowledgeSeedDryRun, "dry-run", false,
		"Print what would be upserted without writing to ai-memory")
	opsKnowledgeSeedCmd.Flags().StringVar(&opsKnowledgeSeedAddr, "memory", "",
		"AI Memory service address (defaults to globular.internal mesh routing)")
	opsKnowledgeCmd.AddCommand(opsKnowledgeSeedCmd)

	opsKnowledgeVerifyCmd.Flags().StringVar(&opsKnowledgeVerifyDir, "dir", "docs/operational-knowledge",
		"Path to the operational-knowledge directory (source of truth for canonical hashes)")
	opsKnowledgeVerifyCmd.Flags().StringVar(&opsKnowledgeVerifyAddr, "memory", "",
		"AI Memory service address (defaults to globular.internal mesh routing)")
	opsKnowledgeCmd.AddCommand(opsKnowledgeVerifyCmd)

	opsKnowledgeListCmd.Flags().StringVar(&opsKnowledgeListAddr, "memory", "",
		"AI Memory service address (defaults to globular.internal mesh routing)")
	opsKnowledgeListCmd.Flags().StringVar(&opsKnowledgeListStage, "stage", "",
		"Filter by lifecycle stage: day-0|day-1|day-2|always")
	opsKnowledgeListCmd.Flags().StringVar(&opsKnowledgeListType, "type", "",
		"Filter by entry type: ARCHITECTURE|DECISION|REFERENCE|SKILL|DEBUG")
	opsKnowledgeCmd.AddCommand(opsKnowledgeListCmd)

	rootCmd.AddCommand(opsKnowledgeCmd)
}

func runOpsKnowledgeValidate(cmd *cobra.Command, args []string) error {
	dir, err := filepath.Abs(opsKnowledgeValidateDir)
	if err != nil {
		return fmt.Errorf("resolve --dir: %w", err)
	}
	awarenessDir, err := filepath.Abs(opsKnowledgeValidateAwareness)
	if err != nil {
		return fmt.Errorf("resolve --awareness-dir: %w", err)
	}

	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("operational-knowledge dir not found at %s — pass --dir to point elsewhere", dir)
	}

	files, err := opsknowledge.LoadDir(dir)
	if err != nil {
		return fmt.Errorf("load yaml files: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no yaml files found under %s", dir)
	}

	refs, err := opsknowledge.LoadRefsFromAwareness(awarenessDir, dir)
	if err != nil {
		// Don't fail hard — link integrity checks just won't run if awareness
		// dir is unreachable. Warn the operator and continue with file/entry
		// rules.
		fmt.Fprintf(os.Stderr, "warning: could not load awareness refs (%v) — link integrity checks skipped\n", err)
		refs = &opsknowledge.Refs{
			InvariantIDs:   map[string]bool{},
			FailureModeIDs: map[string]bool{},
			RunbookPaths:   map[string]bool{},
			SeenEntryIDs:   map[string]string{},
		}
	}

	var allFindings []opsknowledge.Finding
	for _, f := range files {
		allFindings = append(allFindings, opsknowledge.Validate(f, refs)...)
	}

	// Group by file for human-friendly output.
	byFile := map[string][]opsknowledge.Finding{}
	for _, fnd := range allFindings {
		rel, err := filepath.Rel(dir, fnd.Path)
		if err != nil {
			rel = fnd.Path
		}
		byFile[rel] = append(byFile[rel], fnd)
	}

	// Print findings, sorted by file path.
	paths := make([]string, 0, len(byFile))
	for p := range byFile {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	totalErr, totalWarn := 0, 0
	for _, p := range paths {
		fmt.Fprintf(cmd.OutOrStdout(), "── %s\n", p)
		for _, fnd := range byFile[p] {
			label := strings.ToUpper(string(fnd.Severity))
			if fnd.EntryID != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s [%s] %s — %s\n", label, fnd.EntryID, fnd.Code, fnd.Message)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — %s\n", label, fnd.Code, fnd.Message)
			}
			switch fnd.Severity {
			case opsknowledge.SevError:
				totalErr++
			case opsknowledge.SevWarn:
				totalWarn++
			}
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\n── summary\n  files validated: %d\n  errors: %d\n  warnings: %d\n",
		len(files), totalErr, totalWarn)

	if totalErr > 0 {
		return fmt.Errorf("%d error-severity findings — fix the YAML before shipping", totalErr)
	}
	return nil
}

func runOpsKnowledgeHash(cmd *cobra.Command, args []string) error {
	dir, err := filepath.Abs(opsKnowledgeHashDir)
	if err != nil {
		return fmt.Errorf("resolve --dir: %w", err)
	}
	files, err := opsknowledge.LoadDir(dir)
	if err != nil {
		return fmt.Errorf("load yaml files: %w", err)
	}
	for _, f := range files {
		for _, e := range f.Entries {
			if e.ID == opsKnowledgeHashID {
				h, err := opsknowledge.HashEntry(e)
				if err != nil {
					return fmt.Errorf("hash entry: %w", err)
				}
				rel, _ := filepath.Rel(dir, f.Path)
				fmt.Fprintf(cmd.OutOrStdout(), "id:     %s\nfile:   %s\nsha256: %s\n", e.ID, rel, h)
				return nil
			}
		}
	}
	return fmt.Errorf("entry id %q not found in any file under %s", opsKnowledgeHashID, dir)
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// entryToMemory converts an opsknowledge.Entry into the AI Memory protobuf
// row that gets stored. The entry's id IS the Memory id (not auto-generated)
// so subsequent runs upsert deterministically.
func entryToMemory(e opsknowledge.Entry, seedVersion, seedSHA256 string) *ai_memorypb.Memory {
	memType := ai_memorypb.MemoryType_REFERENCE // safe default
	if v, ok := ai_memorypb.MemoryType_value[e.Type]; ok {
		memType = ai_memorypb.MemoryType(v)
	}

	// Tag every seed entry with "seed" for cheap query filtering.
	tags := append([]string{}, e.Tags...)
	if !containsString(tags, opsKnowledgeSeedTag) {
		tags = append(tags, opsKnowledgeSeedTag)
	}

	return &ai_memorypb.Memory{
		Id:         e.ID, // deterministic id, not auto-generated
		Project:    opsKnowledgeProject,
		Type:       memType,
		Tags:       tags,
		Title:      e.Title,
		Content:    e.Content,
		AgentId:    "ops-knowledge-seeder",
		RelatedIds: append([]string{}, e.RelatedIDs...),
		Metadata: map[string]string{
			"source":       opsknowledge.ProvenanceSourceSeed,
			"seed_version": seedVersion,
			"seed_sha256":  seedSHA256,
			"immutable":    "true",
		},
	}
}

func containsString(s []string, want string) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}

// loadAllSeedEntries walks the directory and returns every entry plus its
// canonical hash, in path-stable order. Used by seed and verify.
func loadAllSeedEntries(dir string) ([]seedItem, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, fmt.Errorf("operational-knowledge dir not found at %s — pass --dir to point elsewhere", abs)
	}
	files, err := opsknowledge.LoadDir(abs)
	if err != nil {
		return nil, err
	}
	var items []seedItem
	for _, f := range files {
		for _, e := range f.Entries {
			h, err := opsknowledge.HashEntry(e)
			if err != nil {
				return nil, fmt.Errorf("hash %s: %w", e.ID, err)
			}
			rel, _ := filepath.Rel(abs, f.Path)
			items = append(items, seedItem{Entry: e, Hash: h, RelPath: rel})
		}
	}
	return items, nil
}

type seedItem struct {
	Entry   opsknowledge.Entry
	Hash    string
	RelPath string
}

// ── seed ────────────────────────────────────────────────────────────────────

func runOpsKnowledgeSeed(cmd *cobra.Command, args []string) error {
	items, err := loadAllSeedEntries(opsKnowledgeSeedDir)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return fmt.Errorf("no entries found under %s", opsKnowledgeSeedDir)
	}

	if opsKnowledgeSeedDryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "── dry-run: would upsert %d entries (seed_version=%s)\n",
			len(items), opsKnowledgeSeedVersion)
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tTYPE\tSHA256")
		for _, it := range items {
			fmt.Fprintf(w, "%s\t%s\t%s\n", it.Entry.ID, it.Entry.Type, it.Hash[:12])
		}
		w.Flush()
		return nil
	}

	cc, err := dialGRPC(pick(opsKnowledgeSeedAddr, "globular.internal"))
	if err != nil {
		return fmt.Errorf("connect to ai-memory: %w (run `globular auth login --user sa --password ...` first)", err)
	}
	defer cc.Close()
	client := ai_memorypb.NewAiMemoryServiceClient(cc)

	var stored, updated, skipped, failed int

	for _, it := range items {
		mem := entryToMemory(it.Entry, opsKnowledgeSeedVersion, it.Hash)

		// Per-RPC context — sharing one deadline across 64 RPCs would expire
		// the late ones even on a healthy cluster.
		existing, getErr := client.Get(ctxWithTimeout(), &ai_memorypb.GetRqst{
			Id:      it.Entry.ID,
			Project: opsKnowledgeProject,
		})

		if getErr == nil && existing != nil && existing.GetMemory() != nil {
			existingHash := existing.GetMemory().GetMetadata()["seed_sha256"]
			if existingHash == it.Hash {
				skipped++
				continue
			}
			// Hash drifted — update.
			if _, err := client.Update(ctxWithTimeout(), &ai_memorypb.UpdateRqst{Memory: mem}); err != nil {
				failed++
				fmt.Fprintf(cmd.ErrOrStderr(), "  FAIL %s: update: %v\n", it.Entry.ID, err)
				continue
			}
			updated++
			continue
		}

		// Store fresh.
		if _, err := client.Store(ctxWithTimeout(), &ai_memorypb.StoreRqst{Memory: mem}); err != nil {
			failed++
			fmt.Fprintf(cmd.ErrOrStderr(), "  FAIL %s: store: %v\n", it.Entry.ID, err)
			continue
		}
		stored++
	}

	fmt.Fprintf(cmd.OutOrStdout(),
		"── seed complete\n  stored:  %d\n  updated: %d\n  skipped: %d (already up-to-date)\n  failed:  %d\n  total:   %d\n",
		stored, updated, skipped, failed, len(items))
	if failed > 0 {
		return fmt.Errorf("%d entries failed to upsert", failed)
	}
	return nil
}

// ── verify ──────────────────────────────────────────────────────────────────

func runOpsKnowledgeVerify(cmd *cobra.Command, args []string) error {
	items, err := loadAllSeedEntries(opsKnowledgeVerifyDir)
	if err != nil {
		return err
	}
	localHashByID := map[string]string{}
	localTitleByID := map[string]string{}
	for _, it := range items {
		localHashByID[it.Entry.ID] = it.Hash
		localTitleByID[it.Entry.ID] = it.Entry.Title
	}

	cc, err := dialGRPC(pick(opsKnowledgeVerifyAddr, "globular.internal"))
	if err != nil {
		return fmt.Errorf("connect to ai-memory: %w (run `globular auth login --user sa --password ...` first)", err)
	}
	defer cc.Close()
	client := ai_memorypb.NewAiMemoryServiceClient(cc)

	resp, err := client.Query(ctxWithTimeout(), &ai_memorypb.QueryRqst{
		Project: opsKnowledgeProject,
		Tags:    []string{opsKnowledgeSeedTag},
		Limit:   1000,
	})
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}

	remoteByID := map[string]*ai_memorypb.Memory{}
	for _, m := range resp.GetMemories() {
		remoteByID[m.GetId()] = m
	}

	var missing, extra, drift, ok []string
	// Each local: present remotely with matching hash?
	for id, hash := range localHashByID {
		rm, present := remoteByID[id]
		if !present {
			missing = append(missing, id)
			continue
		}
		if rm.GetMetadata()["seed_sha256"] != hash {
			drift = append(drift, id)
			continue
		}
		ok = append(ok, id)
	}
	// Each remote: was it in the local set?
	for id := range remoteByID {
		if _, present := localHashByID[id]; !present {
			extra = append(extra, id)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	sort.Strings(drift)

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "── verify report\n  in_sync: %d\n  missing in ai-memory (need seed): %d\n  drifted (ai-memory hash != local): %d\n  extra in ai-memory (no local source): %d\n",
		len(ok), len(missing), len(drift), len(extra))
	if len(missing) > 0 {
		fmt.Fprintln(w, "\n  Missing:")
		for _, id := range missing {
			fmt.Fprintf(w, "    + %s — %s\n", id, localTitleByID[id])
		}
	}
	if len(drift) > 0 {
		fmt.Fprintln(w, "\n  Drifted:")
		for _, id := range drift {
			fmt.Fprintf(w, "    ~ %s\n      local:    %s\n      ai-memory: %s\n",
				id, localHashByID[id], remoteByID[id].GetMetadata()["seed_sha256"])
		}
	}
	if len(extra) > 0 {
		fmt.Fprintln(w, "\n  Extra (not in local YAML — could be a retired entry):")
		for _, id := range extra {
			fmt.Fprintf(w, "    - %s — %s\n", id, remoteByID[id].GetTitle())
		}
	}

	if len(missing)+len(drift)+len(extra) > 0 {
		return fmt.Errorf("verify failed: %d missing, %d drifted, %d extra",
			len(missing), len(drift), len(extra))
	}
	return nil
}

// ── list ────────────────────────────────────────────────────────────────────

func runOpsKnowledgeList(cmd *cobra.Command, args []string) error {
	cc, err := dialGRPC(pick(opsKnowledgeListAddr, "globular.internal"))
	if err != nil {
		return fmt.Errorf("connect to ai-memory: %w (run `globular auth login --user sa --password ...` first)", err)
	}
	defer cc.Close()
	client := ai_memorypb.NewAiMemoryServiceClient(cc)

	tags := []string{opsKnowledgeSeedTag}
	if opsKnowledgeListStage != "" {
		tags = append(tags, opsKnowledgeListStage)
	}
	req := &ai_memorypb.QueryRqst{
		Project: opsKnowledgeProject,
		Tags:    tags,
		Limit:   1000,
	}
	if opsKnowledgeListType != "" {
		if v, ok := ai_memorypb.MemoryType_value[strings.ToUpper(opsKnowledgeListType)]; ok {
			req.Type = ai_memorypb.MemoryType(v)
		}
	}

	resp, err := client.Query(ctxWithTimeout(), req)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}

	memories := resp.GetMemories()
	if len(memories) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No seed entries currently loaded in ai-memory. Run `globular ops-knowledge seed` first.")
		return nil
	}
	sort.Slice(memories, func(i, j int) bool { return memories[i].GetId() < memories[j].GetId() })

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTYPE\tSEED_VER\tSHA256\tTITLE")
	for _, m := range memories {
		md := m.GetMetadata()
		typ := strings.TrimPrefix(m.GetType().String(), "MEMORY_")
		title := m.GetTitle()
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		hash := md["seed_sha256"]
		if len(hash) > 12 {
			hash = hash[:12]
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", m.GetId(), typ, md["seed_version"], hash, title)
	}
	w.Flush()
	fmt.Fprintf(cmd.OutOrStdout(), "\n  total: %d entries (filter: tags=%s, type=%s)\n",
		len(memories), strings.Join(tags, ","), opsKnowledgeListType)
	return nil
}
