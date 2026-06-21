package opsknowledge

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
	corpus "github.com/globulario/services/golang/opsknowledge"
)

// repoRelCorpus is the canonical repo-relative location of the corpus, used to
// stamp source paths deterministically regardless of the absolute compile dir.
const repoRelCorpus = "docs/operational-knowledge"

// Compile reads the operational-knowledge corpus rooted at corpusDir (the
// absolute path to docs/operational-knowledge) and returns the deterministic
// behavioral-memory seed bundle. Same corpus → identical Bundle.
func Compile(corpusDir string) (*Bundle, error) {
	files, err := corpus.LoadDir(corpusDir)
	if err != nil {
		return nil, fmt.Errorf("load corpus: %w", err)
	}
	incidents, err := loadIncidents(filepath.Join(corpusDir, "incidents"), repoRelCorpus)
	if err != nil {
		return nil, fmt.Errorf("load incidents: %w", err)
	}

	b := &builder{seen: map[string]bool{}}

	for _, f := range files {
		src, err := b.fileSource(f, corpusDir)
		if err != nil {
			return nil, err
		}
		switch f.FileKind {
		case corpus.FileKindServiceRole:
			b.fromServiceRole(f, src)
		case corpus.FileKindStage:
			b.fromStage(f, src)
		case corpus.FileKindRunbook:
			b.fromRunbook(f, src)
		}
	}
	for _, inc := range incidents {
		b.fromIncident(inc)
	}

	// Cross-cutting generative principles (Level 3) — emitted once, aggregating
	// every contributing source. Each is self-contained: it references only
	// generated catalog entries, which are emitted alongside it.
	b.emitGenerativePrinciples()

	return b.bundle(), nil
}

// builder accumulates extracted objects, de-duplicating ids.
type builder struct {
	authorities      []domain.CatalogEntry
	conditions       []domain.CatalogEntry
	forbidden        []domain.CatalogEntry
	requiredEvidence []domain.CatalogEntry
	principles       []domain.PrincipleSeed
	sources          []Source
	seen             map[string]bool // catalog id de-dup

	opevSources        []string // runbooks with observe→plan→execute→verify
	serviceRoleSources []string
	incidentSources    []string
}

func (b *builder) addAuthority(e domain.CatalogEntry) { b.add(&b.authorities, e) }
func (b *builder) addCondition(e domain.CatalogEntry) { b.add(&b.conditions, e) }
func (b *builder) addForbidden(e domain.CatalogEntry) { b.add(&b.forbidden, e) }
func (b *builder) addRequired(e domain.CatalogEntry)  { b.add(&b.requiredEvidence, e) }

func (b *builder) add(dst *[]domain.CatalogEntry, e domain.CatalogEntry) {
	if b.seen[e.ID] {
		return
	}
	b.seen[e.ID] = true
	*dst = append(*dst, e)
}

// fileSource builds the provenance record for one corpus file (raw-bytes hash).
func (b *builder) fileSource(f *corpus.File, corpusDir string) (Source, error) {
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return Source{}, fmt.Errorf("read %s: %w", f.Path, err)
	}
	sum := sha256.Sum256(data)
	rel := strings.TrimPrefix(f.Path, strings.TrimRight(corpusDir, "/")+"/")
	kind := strings.ReplaceAll(f.FileKind, "-", "_") // service-role → service_role
	src := Source{
		Ref:   SourcePrefix + kind + "." + fileSlug(f.Path),
		Kind:  kind,
		Path:  filepath.ToSlash(filepath.Join(repoRelCorpus, rel)),
		Hash:  hex.EncodeToString(sum[:]),
		Title: f.Metadata.Title,
	}
	b.sources = append(b.sources, src)
	return src, nil
}

// fromServiceRole emits one owner authority per service-role file.
func (b *builder) fromServiceRole(f *corpus.File, src Source) {
	svc := fileSlug(f.Path)
	b.addAuthority(domain.CatalogEntry{
		ID:    "authority.cluster." + svc + ".runtime_state",
		Title: "Owner authority for " + svc + " runtime state",
		Fields: map[string]string{
			"owner_kind":  "service_rpc",
			"governs":     "runtime state owned by the " + svc + " service",
			"owned_state": "the desired/observed state this service is the sole writer of",
			"source":      src.Ref,
		},
	})
	b.serviceRoleSources = append(b.serviceRoleSources, src.Ref)
}

// fromStage emits one lifecycle condition per stage file.
func (b *builder) fromStage(f *corpus.File, src Source) {
	stage := fileSlug(f.Path)
	title := f.Metadata.Title
	if title == "" {
		title = "Lifecycle stage " + stage
	}
	b.addCondition(domain.CatalogEntry{
		ID:    "condition.cluster.lifecycle." + stage,
		Title: title,
		Fields: map[string]string{
			"severity":    "info",
			"detect_spec": "cluster is operating in lifecycle stage " + stage,
			"source":      src.Ref,
		},
	})
}

// fromRunbook emits required evidence (from success_criteria), forbidden moves
// (from procedure step warnings, each paired with the step's safe description),
// and records observe→plan→execute→verify coverage for the L3 principle.
func (b *builder) fromRunbook(f *corpus.File, src Source) {
	rb := fileSlug(f.Path)
	for _, e := range f.Entries {
		phases := map[string]bool{}
		for i, step := range e.Procedure {
			if step.Phase != "" {
				phases[strings.ToLower(step.Phase)] = true
			}
			// A step warning is a hazard; the step's description is the safe
			// action — a guaranteed generative pairing.
			if strings.TrimSpace(step.Warning) != "" && strings.TrimSpace(step.Description) != "" {
				b.addForbidden(domain.CatalogEntry{
					ID:    fmt.Sprintf("forbidden.cluster.%s.step_%d", rb, i),
					Title: firstLine(step.Warning),
					Fields: map[string]string{
						"reason":               firstLine(step.Warning),
						"recommended_behavior": firstLine(step.Description),
						"source":               src.Ref,
					},
				})
			}
		}
		// success_criteria (free-form section captured in Extra) → required evidence.
		if hasExtraList(e.Extra, "success_criteria") {
			b.addRequired(domain.CatalogEntry{
				ID:    "evidence.cluster." + rb + ".success_criteria_met",
				Title: "Success criteria met for runbook " + rb,
				Fields: map[string]string{
					"lane":      "runtime_required",
					"probe_ref": rb + ".success_criteria",
					"source":    src.Ref,
				},
			})
		}
		if phases["observe"] && phases["plan"] && phases["execute"] && phases["verify"] {
			b.opevSources = appendUnique(b.opevSources, src.Ref)
		}
	}
}

// fromIncident emits one forbidden move per incident (paired with a safe
// behavior) and records the incident for the L3 diagnostic-finding principle.
func (b *builder) fromIncident(inc incident) {
	ref := SourcePrefix + "incident." + inc.Slug
	b.sources = append(b.sources, Source{Ref: ref, Kind: "incident", Path: inc.Path, Hash: inc.Hash, Title: inc.Title})
	b.addForbidden(domain.CatalogEntry{
		ID:    "forbidden.cluster.incident." + inc.Slug,
		Title: "Repeat the failure pattern from incident: " + inc.Title,
		Fields: map[string]string{
			"reason":               "this failure pattern was observed in incident " + inc.Slug,
			"recommended_behavior": "Treat diagnostic findings as claims and build an authority chain before destructive action; verify against the owning authority first.",
			"source":               ref,
		},
	})
	b.incidentSources = appendUnique(b.incidentSources, ref)
}

// emitGenerativePrinciples produces the Level-3 cross-cutting principles. Each is
// self-contained (references only generated catalog entries) and aggregates the
// sources that justified it. Principles are authored PROPOSED — never promoted.
func (b *builder) emitGenerativePrinciples() {
	if len(b.opevSources) > 0 {
		b.addCondition(domain.CatalogEntry{ID: "condition.cluster.recovery.in_progress", Title: "A recovery procedure is in progress",
			Fields: map[string]string{"severity": "info", "detect_spec": "an operator/agent is executing a recovery runbook"}})
		b.addForbidden(domain.CatalogEntry{ID: "forbidden.cluster.claim_recovery_before_verify", Title: "Claim recovery before the verify phase",
			Fields: map[string]string{"reason": "a command success is not recovery", "recommended_behavior": "Complete the verify phase and confirm success criteria before claiming recovery."}})
		b.addRequired(domain.CatalogEntry{ID: "evidence.cluster.recovery.verify_phase_complete", Title: "Verify phase completed",
			Fields: map[string]string{"lane": "runtime_required", "probe_ref": "recovery.verify_phase_complete"}})
		b.principles = append(b.principles, domain.PrincipleSeed{
			ID: "principle.cluster.observe_plan_execute_verify_before_recovery_claim", Title: "Complete observe → plan → execute → verify before claiming recovery",
			AppliesWhen: []string{"condition.cluster.recovery.in_progress"}, ForbiddenMoves: []string{"forbidden.cluster.claim_recovery_before_verify"},
			RequiredEvidence: []string{"evidence.cluster.recovery.verify_phase_complete"},
			RecommendedAction: "Complete observe → plan → execute → verify before claiming recovery; if a phase is missing, generate the next safe step from that phase instead of guessing a command.",
			RiskLevel:         "high", RevocationRule: "narrow if the recovery phase model changes",
			PromotionReason: "runbooks consistently encode observe→plan→execute→verify; skipping verify caused false recovery claims",
			SourceRefs:      b.opevSources, GeneratedFrom: b.opevSources,
		})
	}
	if len(b.serviceRoleSources) > 0 {
		b.addCondition(domain.CatalogEntry{ID: "condition.cluster.repair.in_progress", Title: "A repair/mutation is being attempted",
			Fields: map[string]string{"severity": "info", "detect_spec": "an agent intends to mutate runtime state"}})
		b.addForbidden(domain.CatalogEntry{ID: "forbidden.cluster.mutate_without_owner_authority", Title: "Mutate runtime state without the owning service's authority",
			Fields: map[string]string{"reason": "bypassing the owner authority corrupts the boundary", "recommended_behavior": "Route every mutation through the owning service's typed API; controllers decide, executors mutate."}})
		b.addRequired(domain.CatalogEntry{ID: "evidence.cluster.owner_authority.resolved", Title: "Owning authority resolved for the target",
			Fields: map[string]string{"lane": "runtime_required", "probe_ref": "owner_authority.resolved"}})
		b.principles = append(b.principles, domain.PrincipleSeed{
			ID: "principle.cluster.preserve_authority_boundary_during_repair", Title: "Preserve authority boundaries during repair",
			AppliesWhen: []string{"condition.cluster.repair.in_progress"}, ForbiddenMoves: []string{"forbidden.cluster.mutate_without_owner_authority"},
			RequiredEvidence: []string{"evidence.cluster.owner_authority.resolved"},
			RecommendedAction: "Preserve decide/coordinate/execute separation: resolve the owning service's authority and route every mutation through its typed API.",
			RiskLevel:         "high", RevocationRule: "narrow if the decide/coordinate/execute topology changes",
			PromotionReason: "service-role definitions consistently separate ownership from execution; collapsing them damaged the architecture",
			SourceRefs:      b.serviceRoleSources, GeneratedFrom: b.serviceRoleSources,
		})
	}
	if len(b.incidentSources) > 0 {
		b.addCondition(domain.CatalogEntry{ID: "condition.cluster.diagnostic_finding.present", Title: "A diagnostic finding is present",
			Fields: map[string]string{"severity": "info", "detect_spec": "a doctor/diagnostic produced a candidate finding"}})
		b.addForbidden(domain.CatalogEntry{ID: "forbidden.cluster.act_on_diagnostic_without_authority_chain", Title: "Act on a diagnostic finding without an authority chain",
			Fields: map[string]string{"reason": "a finding is a claim, not authority", "recommended_behavior": "Build an authority chain (finding → reference checks → installed metadata → reversible quarantine) before destructive cleanup."}})
		b.addRequired(domain.CatalogEntry{ID: "evidence.cluster.authority_chain.complete", Title: "Authority chain complete for the finding",
			Fields: map[string]string{"lane": "runtime_required", "probe_ref": "authority_chain.complete"}})
		b.principles = append(b.principles, domain.PrincipleSeed{
			ID: "principle.cluster.diagnostic_finding_is_claim_not_authority", Title: "A diagnostic finding is a claim, not authority",
			AppliesWhen: []string{"condition.cluster.diagnostic_finding.present"}, ForbiddenMoves: []string{"forbidden.cluster.act_on_diagnostic_without_authority_chain"},
			RequiredEvidence: []string{"evidence.cluster.authority_chain.complete"},
			RecommendedAction: "Treat a diagnostic finding as a claim until independently verified; build an authority chain before any destructive cleanup.",
			RiskLevel:         "irreversible", RevocationRule: "narrow if the diagnostic authority model changes",
			PromotionReason: "incidents recurred where diagnostic findings were acted on as authority and caused damage",
			SourceRefs:      b.incidentSources, GeneratedFrom: b.incidentSources,
		})
	}
}

// bundle finalizes deterministic ordering.
func (b *builder) bundle() *Bundle {
	sortEntries(b.authorities)
	sortEntries(b.conditions)
	sortEntries(b.forbidden)
	sortEntries(b.requiredEvidence)
	sort.Slice(b.principles, func(i, j int) bool { return b.principles[i].ID < b.principles[j].ID })
	sort.Slice(b.sources, func(i, j int) bool { return b.sources[i].Ref < b.sources[j].Ref })
	for i := range b.principles {
		sort.Strings(b.principles[i].SourceRefs)
		sort.Strings(b.principles[i].GeneratedFrom)
	}
	return &Bundle{
		Authorities: b.authorities, Conditions: b.conditions, ForbiddenMoves: b.forbidden,
		RequiredEvidence: b.requiredEvidence, Principles: b.principles, Sources: b.sources,
	}
}

func sortEntries(es []domain.CatalogEntry) {
	sort.Slice(es, func(i, j int) bool { return es[i].ID < es[j].ID })
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func hasExtraList(extra map[string]any, key string) bool {
	v, ok := extra[key]
	if !ok {
		return false
	}
	l, ok := v.([]any)
	return ok && len(l) > 0
}

func appendUnique(ss []string, v string) []string {
	for _, x := range ss {
		if x == v {
			return ss
		}
	}
	return append(ss, v)
}
