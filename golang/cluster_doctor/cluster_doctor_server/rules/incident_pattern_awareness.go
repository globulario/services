package rules

import (
	"context"
	"fmt"
	"strings"

	"github.com/globulario/awareness/graph"
	"github.com/globulario/awareness/incidentpattern"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// incidentPatternAwareness enriches doctor findings with historical incident pattern matches.
// When the cluster is showing symptoms that historically led to an incident, this rule
// surfaces the lesson before the same failure recurs.
//
// Runs only when Config.AwarenessGraphPath is set. Degrades gracefully if the graph
// is unavailable — it never blocks other doctor rules from running.
type incidentPatternAwareness struct{}

func (r incidentPatternAwareness) ID() string       { return "awareness.incident_pattern" }
func (r incidentPatternAwareness) Category() string { return "awareness" }
func (r incidentPatternAwareness) Scope() string    { return "cluster" }

func (r incidentPatternAwareness) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	if cfg.AwarenessGraphPath == "" {
		return nil
	}

	g, err := graph.Open(cfg.AwarenessGraphPath)
	if err != nil {
		return nil // degraded: awareness graph unavailable
	}
	defer g.Close()

	components := extractComponentsFromSnapshot(snap)
	if len(components) == 0 {
		return nil
	}

	req := incidentpattern.IncidentMatchRequest{
		Task:       "cluster doctor runtime diagnosis",
		Intent:     "diagnose",
		Components: components,
	}

	ctx := context.Background()
	matches, err := incidentpattern.Match(ctx, g, req)
	if err != nil || len(matches) == 0 {
		return nil
	}

	var findings []Finding
	for _, m := range matches {
		if m.Score < 0.55 {
			continue
		}
		sev := cluster_doctorpb.Severity_SEVERITY_WARN
		if m.Severity == "critical" && m.Score >= 0.80 {
			sev = cluster_doctorpb.Severity_SEVERITY_ERROR
		}

		summary := fmt.Sprintf(
			"Cluster symptoms match incident pattern %s: %s (score %.2f, %s confidence). Lesson: %s",
			m.IncidentID, m.Title, m.Score, m.Confidence, m.Lesson)

		evidence := []*cluster_doctorpb.Evidence{
			kvEvidence("awareness", "incident_pattern_match", map[string]string{
				"incident_id": m.IncidentID,
				"score":       fmt.Sprintf("%.2f", m.Score),
				"confidence":  m.Confidence,
				"lesson":      m.Lesson,
			}),
		}

		remediation := []*cluster_doctorpb.RemediationStep{
			step(1, fmt.Sprintf("Read incident %s before applying any fix.", m.IncidentID),
				fmt.Sprintf("globular awareness incident-pattern show %s", m.IncidentID)),
		}
		for i, next := range m.RecommendedNext {
			if i >= 2 {
				break
			}
			remediation = append(remediation, step(uint32(i+2), next, ""))
		}

		findings = append(findings, Finding{
			FindingID:       FindingID(r.ID(), m.IncidentID, m.PatternID),
			InvariantID:     r.ID(),
			Severity:        sev,
			Category:        r.Category(),
			EntityRef:       m.IncidentID,
			Summary:         summary,
			Evidence:        evidence,
			Remediation:     remediation,
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}

// extractComponentsFromSnapshot derives component names from installed package names
// in the snapshot inventories. Components are used as match signals for incident patterns.
func extractComponentsFromSnapshot(snap *collector.Snapshot) []string {
	seen := map[string]bool{}
	var components []string
	for _, inv := range snap.Inventories {
		if inv == nil {
			continue
		}
		for _, pkg := range inv.Components {
			// Strip version suffix — use just the service/component name.
			name := strings.Split(pkg.GetName(), "/")[0]
			if name != "" && !seen[name] {
				components = append(components, name)
				seen[name] = true
			}
		}
	}
	return components
}
