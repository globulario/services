package main

// ops_knowledge_export.go — the write-back rung of the memory loop.
//
//   globular ops-knowledge export [--tag promote] [--out <file>] [--memory <addr>]
//
// Runtime ai-memory is node-local (ai_memory keyspace, RF=1 on single-node) and is
// LOST on clean-node. The git-tracked operational-knowledge corpus is the durability
// boundary that reseeds on every node at Day-0. This command promotes durable runtime
// memories INTO that corpus so a lesson learned once survives a wipe.
//
// Contract for a memory to be exportable (the agent sets these when recording it):
//   - tag it with the export tag (default "promote")
//   - metadata.promote_id = ops.<stage>.<topic>.<verb-noun>  (stable, ops.* namespaced)
//   - metadata.lifecycle  = day-0|day-1|day-2|always         (optional; default "always")
//
// Output is a schema-valid <file_kind> YAML file ready to commit. It does NOT write
// to ai-memory and does NOT commit — review, then run `globular ops-knowledge validate`
// and commit. provenance.seed_version / seed_sha256 are stamped later by the build.

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	ai_memorypb "github.com/globulario/services/golang/ai_memory/ai_memorypb"
	"github.com/globulario/services/golang/opsknowledge"
)

var (
	opsKnowledgeExportTag      string
	opsKnowledgeExportOut      string
	opsKnowledgeExportAddr     string
	opsKnowledgeExportFileKind string
	opsKnowledgeExportTitle    string
)

var opsKnowledgeExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Promote durable runtime ai-memory entries into the operational-knowledge seed",
	Long: `Queries AI Memory for entries carrying the export tag (default "promote") and
serializes them into a schema-valid operational-knowledge YAML file, so a lesson
learned at runtime survives a clean install and reseeds at Day-0.

Each memory MUST carry metadata.promote_id = ops.<...> (stable, ops.* namespaced)
so the seed entry has a deterministic id; optionally metadata.lifecycle to set the
lifecycle stage (default "always").

This writes a candidate file only — it does not push to ai-memory or commit. After
export, run ` + "`globular ops-knowledge validate`" + ` and commit the result.`,
	RunE: runOpsKnowledgeExport,
}

func init() {
	opsKnowledgeExportCmd.Flags().StringVar(&opsKnowledgeExportTag, "tag", "promote",
		"AI Memory tag selecting entries to promote")
	opsKnowledgeExportCmd.Flags().StringVar(&opsKnowledgeExportOut, "out", "",
		"Output YAML path (default: stdout)")
	opsKnowledgeExportCmd.Flags().StringVar(&opsKnowledgeExportAddr, "memory", "",
		"AI Memory service address (defaults to globular.internal mesh routing)")
	opsKnowledgeExportCmd.Flags().StringVar(&opsKnowledgeExportFileKind, "file-kind", opsknowledge.FileKindStage,
		"Corpus file_kind for the output: stage|runbook|service-role")
	opsKnowledgeExportCmd.Flags().StringVar(&opsKnowledgeExportTitle, "title", "",
		"metadata.title for the generated file (default: derived from the tag)")
	opsKnowledgeCmd.AddCommand(opsKnowledgeExportCmd)
}

func runOpsKnowledgeExport(cmd *cobra.Command, args []string) error {
	cc, err := dialGRPC(pick(opsKnowledgeExportAddr, "globular.internal"))
	if err != nil {
		return fmt.Errorf("connect to ai-memory: %w (run `globular auth login --user sa --password ...` first)", err)
	}
	defer func() { _ = cc.Close() }()
	client := ai_memorypb.NewAiMemoryServiceClient(cc)

	resp, err := client.Query(ctxWithTimeout(), &ai_memorypb.QueryRqst{
		Project: opsKnowledgeProject,
		Tags:    []string{opsKnowledgeExportTag},
		Limit:   1000,
	})
	if err != nil {
		return fmt.Errorf("query ai-memory (tag=%s): %w", opsKnowledgeExportTag, err)
	}
	memories := resp.GetMemories()
	if len(memories) == 0 {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
			"no memories tagged %q in project %q — nothing to export.\n"+
				"Tag a durable memory with %q and set metadata.promote_id=ops.<...> first.\n",
			opsKnowledgeExportTag, opsKnowledgeProject, opsKnowledgeExportTag)
		return nil
	}

	var entries []opsknowledge.Entry
	var skipped int
	for _, m := range memories {
		e, err := memoryToOpsEntry(m)
		if err != nil {
			skipped++
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "  SKIP %s: %v\n", m.GetId(), err)
			continue
		}
		if len(e.Content) > opsknowledge.MaxEntryContentBytes {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
				"  WARN %s: content is %d bytes (cap %d) — trim before committing\n",
				e.ID, len(e.Content), opsknowledge.MaxEntryContentBytes)
		}
		entries = append(entries, e)
	}
	if len(entries) == 0 {
		return fmt.Errorf("no exportable entries (%d skipped) — set metadata.promote_id=ops.<...> on the tagged memories", skipped)
	}

	// Deterministic ordering by id so re-runs produce stable diffs.
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })

	title := opsKnowledgeExportTitle
	if strings.TrimSpace(title) == "" {
		title = fmt.Sprintf("Promoted from runtime ai-memory (tag=%s)", opsKnowledgeExportTag)
	}
	file := opsknowledge.File{
		SchemaVersion: 1,
		FileKind:      opsKnowledgeExportFileKind,
		Metadata: opsknowledge.Metadata{
			Title: title,
			Description: "Durable lessons promoted from runtime ai-memory into the Day-0 seed. " +
				"Runtime ai-memory is node-local and lost on clean-node; this corpus is the " +
				"durability boundary that reseeds on every node. provenance.seed_version / " +
				"seed_sha256 are stamped by the build.",
		},
		Entries: entries,
	}

	out, err := yaml.Marshal(file)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	if strings.TrimSpace(opsKnowledgeExportOut) == "" {
		if _, err := cmd.OutOrStdout().Write(out); err != nil {
			return fmt.Errorf("write stdout: %w", err)
		}
	} else {
		if err := os.WriteFile(opsKnowledgeExportOut, out, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", opsKnowledgeExportOut, err)
		}
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
			"── exported %d entr%s to %s (%d skipped)\n"+
				"   next: globular ops-knowledge validate  &&  git add %s && commit\n",
			len(entries), plural(len(entries)), opsKnowledgeExportOut, skipped, opsKnowledgeExportOut)
	}
	return nil
}

// memoryToOpsEntry maps a runtime AI Memory row to a schema-valid seed Entry.
// Returns an error when the memory lacks a stable ops.* id (the one thing the
// exporter cannot safely invent).
func memoryToOpsEntry(m *ai_memorypb.Memory) (opsknowledge.Entry, error) {
	id := strings.TrimSpace(m.GetMetadata()["promote_id"])
	if id == "" && strings.HasPrefix(m.GetId(), opsknowledge.IDPrefix) {
		id = m.GetId()
	}
	if !strings.HasPrefix(id, opsknowledge.IDPrefix) {
		return opsknowledge.Entry{}, fmt.Errorf("no ops.* id: set metadata.promote_id=ops.<stage>.<topic>.<verb-noun>")
	}

	life := strings.TrimSpace(m.GetMetadata()["lifecycle"])
	if !isLifecycleStage(life) {
		life = opsknowledge.StageAlways
	}

	// First tag MUST be the lifecycle stage; carry the rest minus control tags.
	tags := []string{life}
	for _, t := range m.GetTags() {
		t = strings.TrimSpace(t)
		if t == "" || t == life || t == opsKnowledgeExportTag {
			continue
		}
		tags = append(tags, t)
	}

	phases := []string{opsknowledge.StageDay0, opsknowledge.StageDay1, opsknowledge.StageDay2}
	if life != opsknowledge.StageAlways {
		phases = []string{life}
	}

	return opsknowledge.Entry{
		ID:          id,
		Type:        memTypeToOpsType(m.GetType()),
		Title:       m.GetTitle(),
		Tags:        tags,
		AppliesWhen: opsknowledge.AppliesWhen{ClusterPhases: phases},
		Content:     m.GetContent(),
		Provenance: opsknowledge.Provenance{
			Source:    opsknowledge.ProvenanceSourceSeed,
			Immutable: true,
		},
	}, nil
}

// memTypeToOpsType maps an AI Memory MemoryType to a valid operational-knowledge
// entry type. Types with no seed analog (FEEDBACK, SESSION, USER, PROJECT, SCRATCH)
// fall back to REFERENCE.
func memTypeToOpsType(t ai_memorypb.MemoryType) string {
	switch ai_memorypb.MemoryType_name[int32(t)] {
	case opsknowledge.TypeArchitecture:
		return opsknowledge.TypeArchitecture
	case opsknowledge.TypeDecision:
		return opsknowledge.TypeDecision
	case opsknowledge.TypeDebug:
		return opsknowledge.TypeDebug
	case opsknowledge.TypeReference:
		return opsknowledge.TypeReference
	case opsknowledge.TypeSkill:
		return opsknowledge.TypeSkill
	default:
		return opsknowledge.TypeReference
	}
}

func isLifecycleStage(s string) bool {
	switch s {
	case opsknowledge.StageDay0, opsknowledge.StageDay1, opsknowledge.StageDay2, opsknowledge.StageAlways:
		return true
	}
	return false
}

func plural(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
