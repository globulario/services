package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/globulario/awareness/graph"
)

var experienceCfg = struct {
	id            string
	goal          string
	domain        string
	capability    string
	kind          string
	strategy      string
	action        string
	rationale     string
	outcome       string
	status        string
	summary       string
	lesson        string
	nextHint      string
	obsType       string
	obsSummary    string
	obsSource     string
	obsConfidence float64
	limit         int
	format        string
	closure       string
	invariants    []string
	forbidden     []string
	avoided       []string
	files         []string
	symbols       []string
	relation      string
	target        string
}{
	kind:   "debugging_experience",
	status: "unproven",
	limit:  5,
	format: "markdown",
}

var awarenessExperienceCmd = &cobra.Command{
	Use:   "experience",
	Short: "Record and retrieve Experience Ledger entries",
}

var awarenessExperienceStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a new experience for a goal",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if strings.TrimSpace(experienceCfg.goal) == "" {
			return fmt.Errorf("--goal is required")
		}
		g, err := openExperienceGraph()
		if err != nil {
			return err
		}
		defer g.Close()
		e, err := g.CreateExperience(context.Background(), graph.ExperienceEntry{
			ID:             experienceCfg.id,
			Kind:           experienceCfg.kind,
			Domain:         experienceCfg.domain,
			Capability:     experienceCfg.capability,
			Status:         experienceCfg.status,
			Summary:        experienceCfg.summary,
			GoalOriginal:   experienceCfg.goal,
			GoalNormalized: normalizeGoal(experienceCfg.goal),
			GoalVerb:       goalVerb(experienceCfg.goal),
			GoalObject:     goalObject(experienceCfg.goal),
			StrategyID:     experienceCfg.strategy,
			CreatedBy:      "agent",
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "started experience: %s\n", e.ID)
		return nil
	},
}

var awarenessExperienceRecordAttemptCmd = &cobra.Command{
	Use:   "record-attempt",
	Short: "Record a strategy attempt in an experience",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if experienceCfg.id == "" || experienceCfg.action == "" {
			return fmt.Errorf("--experience and --action are required")
		}
		g, err := openExperienceGraph()
		if err != nil {
			return err
		}
		defer g.Close()
		a, err := g.AddExperienceAttempt(context.Background(), graph.ExperienceAttempt{
			ExperienceID: experienceCfg.id,
			StrategyID:   experienceCfg.strategy,
			Action:       experienceCfg.action,
			Rationale:    experienceCfg.rationale,
			Outcome:      experienceCfg.outcome,
			Status:       firstNonEmptyString(experienceCfg.status, "inconclusive"),
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "recorded attempt: %s\n", a.ID)
		return nil
	},
}

var awarenessExperienceAddObservationCmd = &cobra.Command{
	Use:   "add-observation",
	Short: "Attach an observation to an experience",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if experienceCfg.id == "" || experienceCfg.obsSummary == "" {
			return fmt.Errorf("--experience and --summary are required")
		}
		g, err := openExperienceGraph()
		if err != nil {
			return err
		}
		defer g.Close()
		o, err := g.AddExperienceObservation(context.Background(), graph.ExperienceObservation{
			ExperienceID: experienceCfg.id,
			Type:         firstNonEmptyString(experienceCfg.obsType, "operator_note"),
			Summary:      experienceCfg.obsSummary,
			Source:       experienceCfg.obsSource,
			Confidence:   experienceCfg.obsConfidence,
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "recorded observation: %s\n", o.ID)
		return nil
	},
}

var awarenessExperienceCloseCmd = &cobra.Command{
	Use:   "close",
	Short: "Close an experience with lesson and next-time hint",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if experienceCfg.id == "" || experienceCfg.status == "" {
			return fmt.Errorf("--experience and --status are required")
		}
		g, err := openExperienceGraph()
		if err != nil {
			return err
		}
		defer g.Close()
		score := &graph.ExperienceScorecard{}
		if strings.TrimSpace(experienceCfg.lesson) == "" {
			score.Verdict = "unproven"
		} else {
			score.Verdict = "useful"
		}
		if err := g.CloseExperience(context.Background(), experienceCfg.id, experienceCfg.status, experienceCfg.lesson, experienceCfg.nextHint, score); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "closed experience: %s\n", experienceCfg.id)
		return nil
	},
}

var awarenessExperienceSearchCmd = &cobra.Command{
	Use:   "search-similar",
	Short: "Find similar experiences by goal/domain/capability",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if strings.TrimSpace(experienceCfg.goal) == "" {
			return fmt.Errorf("--goal is required")
		}
		g, err := openExperienceGraph()
		if err != nil {
			return err
		}
		defer g.Close()
		hits, err := g.SearchSimilarExperiences(context.Background(), graph.ExperienceSearchQuery{
			Goal:            experienceCfg.goal,
			Domain:          experienceCfg.domain,
			Capability:      experienceCfg.capability,
			Files:           experienceCfg.files,
			Symbols:         experienceCfg.symbols,
			InvariantIDs:    experienceCfg.invariants,
			ForbiddenFixIDs: experienceCfg.forbidden,
			Limit:           experienceCfg.limit,
		})
		if err != nil {
			return err
		}
		if strings.EqualFold(experienceCfg.format, "json") {
			b, _ := json.MarshalIndent(hits, "", "  ")
			fmt.Fprintln(os.Stdout, string(b))
			return nil
		}
		if len(hits) == 0 {
			fmt.Fprintln(os.Stdout, "No similar experiences found.")
			return nil
		}
		fmt.Fprintln(os.Stdout, "Similar experiences:")
		for i, h := range hits {
			fmt.Fprintf(os.Stdout, "%d. %s\n   score: %.2f\n   strategy: %s\n   hint: %s\n", i+1, h.ExperienceID, h.Score, h.StrategyID, h.Hint)
		}
		return nil
	},
}

var awarenessExperienceGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a full experience record",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if experienceCfg.id == "" {
			return fmt.Errorf("--experience is required")
		}
		g, err := openExperienceGraph()
		if err != nil {
			return err
		}
		defer g.Close()
		rec, err := g.GetExperience(context.Background(), experienceCfg.id)
		if err != nil {
			return err
		}
		if rec == nil {
			fmt.Fprintln(os.Stdout, "not found")
			return nil
		}
		b, _ := json.MarshalIndent(rec, "", "  ")
		fmt.Fprintln(os.Stdout, string(b))
		return nil
	},
}

var awarenessExperiencePromoteLessonCmd = &cobra.Command{
	Use:   "promote-lesson",
	Short: "Record an invariant/forbidden-fix candidate from an experience lesson",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if experienceCfg.id == "" {
			return fmt.Errorf("--experience is required")
		}
		target, _ := cmd.Flags().GetString("to")
		if target == "" {
			return fmt.Errorf("--to is required (forbidden_fix|invariant)")
		}
		g, err := openExperienceGraph()
		if err != nil {
			return err
		}
		defer g.Close()
		res, err := g.PromoteExperienceLesson(context.Background(), experienceCfg.id, target, experienceCfg.summary)
		if err != nil {
			return err
		}
		b, _ := json.MarshalIndent(res, "", "  ")
		fmt.Fprintln(os.Stdout, string(b))
		return nil
	},
}

var awarenessExperienceSeedWorkflowDeferCmd = &cobra.Command{
	Use:   "seed-workflow-defer",
	Short: "Seed the canonical workflow defer experience (idempotent)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		g, err := openExperienceGraph()
		if err != nil {
			return err
		}
		defer g.Close()
		e, err := g.SeedWorkflowDeferExperience(context.Background())
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "seeded experience: %s\n", e.ID)
		return nil
	},
}

var awarenessExperienceLinkArtifactsCmd = &cobra.Command{
	Use:   "link-artifacts",
	Short: "Attach closure/invariants/forbidden-fixes/files/symbols to an experience",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if experienceCfg.id == "" {
			return fmt.Errorf("--experience is required")
		}
		g, err := openExperienceGraph()
		if err != nil {
			return err
		}
		defer g.Close()
		err = g.LinkExperienceArtifacts(context.Background(), experienceCfg.id, graph.ExperienceLinkInput{
			ClosureEntryID:       experienceCfg.closure,
			InvariantIDs:         experienceCfg.invariants,
			ForbiddenFixIDs:      experienceCfg.forbidden,
			AvoidedForbiddenFixs: experienceCfg.avoided,
			TouchedFiles:         experienceCfg.files,
			ChangedSymbols:       experienceCfg.symbols,
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "linked artifacts to experience: %s\n", experienceCfg.id)
		return nil
	},
}

var awarenessExperienceLinkRelationCmd = &cobra.Command{
	Use:   "link-relation",
	Short: "Link experience-to-experience relation (contradicted_by|supersedes|similar_to)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if experienceCfg.id == "" || experienceCfg.relation == "" || experienceCfg.target == "" {
			return fmt.Errorf("--experience, --relation, and --target are required")
		}
		g, err := openExperienceGraph()
		if err != nil {
			return err
		}
		defer g.Close()
		if err := g.LinkExperienceRelation(context.Background(), experienceCfg.id, experienceCfg.relation, experienceCfg.target); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "linked relation: %s --%s--> %s\n", experienceCfg.id, experienceCfg.relation, experienceCfg.target)
		return nil
	},
}

var awarenessExperiencePromotionCheckCmd = &cobra.Command{
	Use:   "promotion-check",
	Short: "Evaluate whether an experience is ready for lesson promotion",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if experienceCfg.id == "" {
			return fmt.Errorf("--experience is required")
		}
		g, err := openExperienceGraph()
		if err != nil {
			return err
		}
		defer g.Close()
		res, err := g.EvaluatePromotionReadiness(context.Background(), experienceCfg.id)
		if err != nil {
			return err
		}
		b, _ := json.MarshalIndent(res, "", "  ")
		fmt.Fprintln(os.Stdout, string(b))
		return nil
	},
}

func openExperienceGraph() (*graph.Graph, error) {
	repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
	if err != nil {
		return nil, err
	}
	dbPath := awareCfg.dbPath
	if dbPath == "" {
		dbPath = filepath.Join(repoRoot, ".globular", "awareness", "graph.db")
	}
	return graph.Open(dbPath)
}

func normalizeGoal(goal string) string {
	g := strings.ToLower(strings.TrimSpace(goal))
	r := strings.NewReplacer("_", " ", ".", " ", "-", " ", "/", " ", "  ", " ")
	return strings.Join(strings.Fields(r.Replace(g)), " ")
}

func goalVerb(goal string) string {
	parts := strings.Fields(normalizeGoal(goal))
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func goalObject(goal string) string {
	parts := strings.Fields(normalizeGoal(goal))
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts[1:], "_")
}

func firstNonEmptyString(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func init() {
	awarenessExperienceStartCmd.Flags().StringVar(&experienceCfg.id, "id", "", "Experience ID (optional)")
	awarenessExperienceStartCmd.Flags().StringVar(&experienceCfg.goal, "goal", "", "Goal statement")
	awarenessExperienceStartCmd.Flags().StringVar(&experienceCfg.domain, "domain", "", "Domain (workflow, cluster, frontend, etc)")
	awarenessExperienceStartCmd.Flags().StringVar(&experienceCfg.capability, "capability", "", "Capability ID")
	awarenessExperienceStartCmd.Flags().StringVar(&experienceCfg.kind, "kind", "debugging_experience", "Experience kind")
	awarenessExperienceStartCmd.Flags().StringVar(&experienceCfg.summary, "summary", "", "Short summary")
	awarenessExperienceStartCmd.Flags().StringVar(&experienceCfg.strategy, "strategy", "", "Primary strategy ID")
	awarenessExperienceStartCmd.Flags().StringVar(&experienceCfg.status, "status", "unproven", "Initial status")

	awarenessExperienceRecordAttemptCmd.Flags().StringVar(&experienceCfg.id, "experience", "", "Experience ID")
	awarenessExperienceRecordAttemptCmd.Flags().StringVar(&experienceCfg.strategy, "strategy", "", "Strategy ID")
	awarenessExperienceRecordAttemptCmd.Flags().StringVar(&experienceCfg.action, "action", "", "Attempt action")
	awarenessExperienceRecordAttemptCmd.Flags().StringVar(&experienceCfg.rationale, "rationale", "", "Rationale")
	awarenessExperienceRecordAttemptCmd.Flags().StringVar(&experienceCfg.outcome, "outcome", "", "Outcome summary")
	awarenessExperienceRecordAttemptCmd.Flags().StringVar(&experienceCfg.status, "status", "inconclusive", "Attempt status")

	awarenessExperienceAddObservationCmd.Flags().StringVar(&experienceCfg.id, "experience", "", "Experience ID")
	awarenessExperienceAddObservationCmd.Flags().StringVar(&experienceCfg.obsType, "type", "operator_note", "Observation type")
	awarenessExperienceAddObservationCmd.Flags().StringVar(&experienceCfg.obsSummary, "summary", "", "Observation summary")
	awarenessExperienceAddObservationCmd.Flags().StringVar(&experienceCfg.obsSource, "source", "", "Observation source")
	awarenessExperienceAddObservationCmd.Flags().Float64Var(&experienceCfg.obsConfidence, "confidence", 0.7, "Observation confidence")

	awarenessExperienceCloseCmd.Flags().StringVar(&experienceCfg.id, "experience", "", "Experience ID")
	awarenessExperienceCloseCmd.Flags().StringVar(&experienceCfg.status, "status", "", "Final status")
	awarenessExperienceCloseCmd.Flags().StringVar(&experienceCfg.lesson, "lesson", "", "Reusable lesson")
	awarenessExperienceCloseCmd.Flags().StringVar(&experienceCfg.nextHint, "next-time-hint", "", "Next-time hint")

	awarenessExperienceSearchCmd.Flags().StringVar(&experienceCfg.goal, "goal", "", "Goal text")
	awarenessExperienceSearchCmd.Flags().StringVar(&experienceCfg.domain, "domain", "", "Domain filter")
	awarenessExperienceSearchCmd.Flags().StringVar(&experienceCfg.capability, "capability", "", "Capability filter")
	awarenessExperienceSearchCmd.Flags().StringArrayVar(&experienceCfg.files, "file", nil, "Changed file path hint (repeatable)")
	awarenessExperienceSearchCmd.Flags().StringArrayVar(&experienceCfg.symbols, "symbol", nil, "Changed symbol hint (repeatable)")
	awarenessExperienceSearchCmd.Flags().StringArrayVar(&experienceCfg.invariants, "invariant", nil, "Nearby invariant hint (repeatable)")
	awarenessExperienceSearchCmd.Flags().StringArrayVar(&experienceCfg.forbidden, "forbidden-fix", nil, "Relevant forbidden-fix hint (repeatable)")
	awarenessExperienceSearchCmd.Flags().IntVar(&experienceCfg.limit, "limit", 5, "Max results")
	awarenessExperienceSearchCmd.Flags().StringVar(&experienceCfg.format, "format", "markdown", "Output format: markdown|json")

	awarenessExperienceGetCmd.Flags().StringVar(&experienceCfg.id, "experience", "", "Experience ID")
	awarenessExperiencePromoteLessonCmd.Flags().StringVar(&experienceCfg.id, "experience", "", "Experience ID")
	awarenessExperiencePromoteLessonCmd.Flags().StringVar(&experienceCfg.summary, "summary", "", "Candidate summary (optional override)")
	awarenessExperiencePromoteLessonCmd.Flags().String("to", "", "Promotion target: forbidden_fix|invariant")

	awarenessExperienceLinkArtifactsCmd.Flags().StringVar(&experienceCfg.id, "experience", "", "Experience ID")
	awarenessExperienceLinkArtifactsCmd.Flags().StringVar(&experienceCfg.closure, "closure-entry", "", "Closure entry ID")
	awarenessExperienceLinkArtifactsCmd.Flags().StringArrayVar(&experienceCfg.invariants, "invariant", nil, "Invariant ID (repeatable)")
	awarenessExperienceLinkArtifactsCmd.Flags().StringArrayVar(&experienceCfg.forbidden, "forbidden-fix", nil, "Forbidden-fix ID produced/candidate (repeatable)")
	awarenessExperienceLinkArtifactsCmd.Flags().StringArrayVar(&experienceCfg.avoided, "avoided-forbidden-fix", nil, "Forbidden-fix ID explicitly avoided (repeatable)")
	awarenessExperienceLinkArtifactsCmd.Flags().StringArrayVar(&experienceCfg.files, "file", nil, "Touched file path (repeatable)")
	awarenessExperienceLinkArtifactsCmd.Flags().StringArrayVar(&experienceCfg.symbols, "symbol", nil, "Changed symbol (repeatable)")
	awarenessExperienceLinkRelationCmd.Flags().StringVar(&experienceCfg.id, "experience", "", "Source experience ID")
	awarenessExperienceLinkRelationCmd.Flags().StringVar(&experienceCfg.relation, "relation", "", "Relation: contradicted_by|supersedes|similar_to")
	awarenessExperienceLinkRelationCmd.Flags().StringVar(&experienceCfg.target, "target", "", "Target experience ID")
	awarenessExperiencePromotionCheckCmd.Flags().StringVar(&experienceCfg.id, "experience", "", "Experience ID")

	awarenessExperienceCmd.AddCommand(
		awarenessExperienceStartCmd,
		awarenessExperienceRecordAttemptCmd,
		awarenessExperienceAddObservationCmd,
		awarenessExperienceCloseCmd,
		awarenessExperienceSearchCmd,
		awarenessExperienceGetCmd,
		awarenessExperiencePromoteLessonCmd,
		awarenessExperienceSeedWorkflowDeferCmd,
		awarenessExperienceLinkArtifactsCmd,
		awarenessExperienceLinkRelationCmd,
		awarenessExperiencePromotionCheckCmd,
	)
	awarenessCmd.AddCommand(awarenessExperienceCmd)
}
