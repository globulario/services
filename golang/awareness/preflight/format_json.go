package preflight

import (
	"encoding/json"
	"fmt"
)

// renderJSON returns the report as a canonical JSON document.
func renderJSON(r *Report) (string, error) {
	// Build the JSON-friendly shape matching the spec schema.
	type didWeFixJSON struct {
		Status          string   `json:"status"`
		MatchedPatterns []string `json:"matched_patterns"`
		FixCases        []string `json:"fix_cases"`
		RemainingGaps   []string `json:"remaining_gaps"`
	}

	type jsonReport struct {
		Task                        string                   `json:"task"`
		Classification              []TaskClass              `json:"classification"`
		MatchedAliases              []string                 `json:"matched_aliases"`
		Services                    []string                 `json:"services"`
		Packages                    []string                 `json:"packages"`
		Files                       []string                 `json:"files"`
		Invariants                  []string                 `json:"invariants"`
		FailureModes                []string                 `json:"failure_modes"`
		ForbiddenFixes              []string                 `json:"forbidden_fixes"`
		CodeSmells                  []string                 `json:"code_smells,omitempty"`
		DesignPatterns              []string                 `json:"design_patterns,omitempty"`
		AntiPatterns                []string                 `json:"anti_patterns,omitempty"`
		HashSchemas                 []string                 `json:"hash_schemas,omitempty"`
		StateTransitions            []string                 `json:"state_transitions,omitempty"`
		DidWeFix                    *DidWeFixSection         `json:"did_we_fix"`
		PackageAdmission            *PackageAdmissionSection `json:"package_admission,omitempty"`
		Cycles                      []CycleWarning           `json:"cycles"`
		RequiredTests               []string                 `json:"required_tests"`
		RequiredSearches            []string                 `json:"required_searches"`
		MatchedDecisions            []string                 `json:"matched_decisions,omitempty"`
		MatchedForbiddenAssumptions []string                 `json:"matched_forbidden_assumptions,omitempty"`
		MatchedAuthorityRules       []string                 `json:"matched_authority_rules,omitempty"`
		MatchedPreflightQuestions   []string                 `json:"matched_preflight_questions,omitempty"`
		MatchedRemediationContracts []string                 `json:"matched_remediation_contracts,omitempty"`
		RecommendedInvestigation    []string                 `json:"recommended_investigation_order"`
		AgentInstruction            string                   `json:"agent_instruction"`
		Warnings                    []string                 `json:"warnings"`
		Runtime                     *RuntimeSection          `json:"runtime,omitempty"`
		Confidence                  Confidence               `json:"confidence"`
		ConfidenceReason            string                   `json:"confidence_reason"`
		Coverage                    Coverage                 `json:"coverage"`
		BlindSpots                  []string                 `json:"blind_spots,omitempty"`
		GraphFreshness              *GraphFreshnessReport    `json:"graph_freshness,omitempty"`
		GraphAvailable              bool                     `json:"graph_available"`
		GraphMatchCount             int                      `json:"graph_match_count"`
		GraphFilteredByTrustCount   int                      `json:"graph_filtered_by_trust_count"`
		RawYAMLMatchCount           int                      `json:"raw_yaml_match_count"`
		FilteredMatches             []FilteredMatch          `json:"filtered_matches,omitempty"`
		ConfidenceFactors           ConfidenceFactors        `json:"confidence_factors"`
		SafetyStatus                SafetyStatus             `json:"safety_status"`
		DegradedMode                DegradedModePlaybook     `json:"degraded_mode"`
		RiskTier                    RiskTier                 `json:"risk_tier"`
		FastPathApplied             bool                     `json:"fast_path_applied"`
		ExperienceHints             []ExperienceHint         `json:"experience_hints,omitempty"`
		Trust                       interface{}              `json:"trust,omitempty"`
	}

	jr := jsonReport{
		Task:                      r.Task,
		Classification:            r.Classification,
		MatchedAliases:            orEmpty(r.MatchedAliases),
		Services:                  orEmpty(r.Services),
		Packages:                  orEmpty(r.Packages),
		Files:                     orEmpty(r.Files),
		Invariants:                orEmpty(r.Invariants),
		FailureModes:              orEmpty(r.FailureModes),
		ForbiddenFixes:            orEmpty(r.ForbiddenFixes),
		CodeSmells:                r.CodeSmells,
		DesignPatterns:            r.DesignPatterns,
		AntiPatterns:              r.AntiPatterns,
		HashSchemas:               r.HashSchemas,
		StateTransitions:          r.StateTransitions,
		DidWeFix:                  r.DidWeFix,
		PackageAdmission:          r.PackageAdmission,
		Cycles:                    r.Cycles,
		RequiredTests:               orEmpty(r.RequiredTests),
		RequiredSearches:            orEmpty(r.RequiredSearches),
		MatchedDecisions:            r.MatchedDecisions,
		MatchedForbiddenAssumptions: r.MatchedForbiddenAssumptions,
		MatchedAuthorityRules:       r.MatchedAuthorityRules,
		MatchedPreflightQuestions:   r.MatchedPreflightQuestions,
		MatchedRemediationContracts: r.MatchedRemediationContracts,
		RecommendedInvestigation:    orEmpty(r.RecommendedOrder),
		AgentInstruction:          r.AgentInstruction,
		Warnings:                  orEmpty(r.Warnings),
		Runtime:                   r.Runtime,
		Confidence:                r.Confidence,
		ConfidenceReason:          r.ConfidenceReason,
		Coverage:                  r.Coverage,
		BlindSpots:                r.BlindSpots,
		GraphFreshness:            r.GraphFreshness,
		GraphAvailable:            r.GraphAvailable,
		GraphMatchCount:           r.GraphMatchCount,
		GraphFilteredByTrustCount: r.GraphFilteredByTrustCount,
		RawYAMLMatchCount:         r.RawYAMLMatchCount,
		FilteredMatches:           r.FilteredMatches,
		ConfidenceFactors:         r.ConfidenceFactors,
		SafetyStatus:              r.SafetyStatus,
		DegradedMode:              r.DegradedMode,
		RiskTier:                  r.RiskTier,
		FastPathApplied:           r.FastPathApplied,
		ExperienceHints:           r.ExperienceHints,
		Trust:                     r.Trust,
	}

	b, err := json.MarshalIndent(jr, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal preflight report: %w", err)
	}
	return string(b), nil
}

// orEmpty returns a non-nil empty slice when in is nil — keeps JSON output stable.
func orEmpty(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}
