package main

import (
	"context"
	"strings"

	"github.com/globulario/services/golang/awareness/assurance"
)

func awarenessTrustMap(st *awarenessState, matchFound bool) map[string]interface{} {
	in := assurance.ComposeInputs{MatchFound: matchFound}
	if st != nil && st.g != nil {
		if s, err := assurance.CheckStaleness(context.Background(), st.g, assurance.Options{DocsDir: st.docsDir}); err == nil {
			in.Staleness = s
		}
	}
	env := assurance.Compose(in)
	return trustEnvelopeToMap(env)
}

func trustEnvelopeToMap(env assurance.TrustEnvelope) map[string]interface{} {
	return map[string]interface{}{
		"verdict":         string(env.Verdict),
		"confidence":      string(env.Confidence),
		"freshness":       string(env.Freshness),
		"coverage":        string(env.Coverage),
		"limitations":     append([]string(nil), env.Limitations...),
		"required_action": append([]string(nil), env.RequiredActions...),
	}
}

func trustFromConfidenceCoverage(st *awarenessState, confidence, graphCoverage string, matchFound bool, blindSpots []string) map[string]interface{} {
	env := assurance.Compose(assurance.ComposeInputs{MatchFound: matchFound})
	switch strings.ToLower(graphCoverage) {
	case "checked_with_matches":
		env.Coverage = assurance.TrustCoveragePartial
	case "checked_clean", "not_checked":
		env.Coverage = assurance.TrustCoverageNone
	}
	switch strings.ToLower(confidence) {
	case "high":
		env.Confidence = assurance.ConfidenceHigh
	case "medium":
		env.Confidence = assurance.ConfidenceMedium
	case "low":
		env.Confidence = assurance.ConfidenceLow
	default:
		env.Confidence = assurance.ConfidenceNone
	}
	if st != nil && st.g != nil {
		if s, err := assurance.CheckStaleness(context.Background(), st.g, assurance.Options{DocsDir: st.docsDir}); err == nil {
			env.Freshness = assurance.Compose(assurance.ComposeInputs{MatchFound: matchFound, Staleness: s}).Freshness
			if env.Freshness != assurance.FreshnessFresh && env.Verdict == assurance.TrustUsable {
				env.Verdict = assurance.TrustStale
			}
		}
	}
	if len(blindSpots) > 0 {
		env.Limitations = append(append([]string{}, env.Limitations...), blindSpots...)
	}
	return trustEnvelopeToMap(env)
}
