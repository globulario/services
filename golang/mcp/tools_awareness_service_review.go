package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/globulario/services/golang/awareness/analysis"
)

// registerReviewServiceTool registers awareness.review_service.
// It is called from registerSelfReviewTools (self_review_tool.go) to keep the
// tool group cohesive.
func registerReviewServiceTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.review_service",
		Description: "Design-level review of a named Globular service in the awareness graph. " +
			"Synthesises: proto contract, RPC authz coverage, implementation links, " +
			"invariant attachments, service dependencies, forbidden fixes, and required tests. " +
			"Input can be the service ID (e.g. 'file-service'), the proto service name " +
			"(e.g. 'file.FileService'), or the service display name.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"service": {
					Type:        "string",
					Description: "Service ID, proto service name, or display name.",
				},
				"format": {
					Type:        "string",
					Description: "Output format: 'text' (default) or 'json'.",
					Enum:        []string{"text", "json"},
				},
			},
			Required: []string{"service"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		serviceID, _ := args["service"].(string)
		if serviceID == "" {
			return map[string]interface{}{"error": "service is required"}, nil
		}
		format, _ := args["format"].(string)

		if st.g == nil {
			return map[string]interface{}{
				"error":   "awareness graph not available",
				"hint":    "run 'globular awareness build' to build the graph",
				"service": serviceID,
			}, nil
		}

		review, err := analysis.ReviewService(ctx, st.g, serviceID)
		if err != nil {
			return map[string]interface{}{
				"error":   fmt.Sprintf("review_service: %v", err),
				"service": serviceID,
			}, nil
		}

		if format == "json" {
			b, _ := json.Marshal(review)
			return json.RawMessage(b), nil
		}

		return map[string]interface{}{
			"text": renderServiceReviewText(review),
		}, nil
	})
}

// renderServiceReviewText formats a ServiceDesignReview as human-readable text.
func renderServiceReviewText(r *analysis.ServiceDesignReview) string {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Service Design Review: %s ===\n\n", r.ServiceID)

	fmt.Fprintf(&b, "Identity\n")
	if r.ProtoService != "" {
		fmt.Fprintf(&b, "  proto_service : %s\n", r.ProtoService)
	}
	if r.ProtoFile != "" {
		fmt.Fprintf(&b, "  proto_file    : %s\n", r.ProtoFile)
	}
	if r.SystemdUnit != "" {
		fmt.Fprintf(&b, "  systemd_unit  : %s\n", r.SystemdUnit)
	}
	fmt.Fprintln(&b)

	if len(r.APIContract.RPCs) > 0 {
		fmt.Fprintf(&b, "API Contract (%d RPCs)\n", len(r.APIContract.RPCs))
		for _, rpc := range r.APIContract.RPCs {
			mode := ""
			if rpc.StreamingMode != "" {
				mode = " [" + rpc.StreamingMode + "]"
			}
			authz := "NO AUTHZ"
			if rpc.HasAuthz {
				authz = fmt.Sprintf("authz: %s/%s", rpc.AuthzAction, rpc.AuthzResource)
			}
			gaps := ""
			if len(rpc.Gaps) > 0 {
				gaps = "  GAPS: [" + strings.Join(rpc.Gaps, ", ") + "]"
			}
			fmt.Fprintf(&b, "  %-45s  %s%s\n", rpc.Name+mode, authz, gaps)
		}
		fmt.Fprintln(&b)
	}

	if len(r.Dependencies) > 0 {
		fmt.Fprintf(&b, "Dependencies (%d)\n", len(r.Dependencies))
		for _, dep := range r.Dependencies {
			req := "optional"
			if dep.Required {
				req = "required"
			}
			fmt.Fprintf(&b, "  %-35s  phase=%-20s  %s\n", dep.Service, dep.Phase, req)
		}
		fmt.Fprintln(&b)
	}

	totalInv := len(r.Invariants.Critical) + len(r.Invariants.High) +
		len(r.Invariants.Medium) + len(r.Invariants.Low)
	if totalInv > 0 {
		fmt.Fprintf(&b, "Invariants (%d)\n", totalInv)
		for _, id := range r.Invariants.Critical {
			fmt.Fprintf(&b, "  [CRITICAL] %s\n", id)
		}
		for _, id := range r.Invariants.High {
			fmt.Fprintf(&b, "  [HIGH]     %s\n", id)
		}
		for _, id := range r.Invariants.Medium {
			fmt.Fprintf(&b, "  [MEDIUM]   %s\n", id)
		}
		for _, id := range r.Invariants.Low {
			fmt.Fprintf(&b, "  [LOW]      %s\n", id)
		}
		fmt.Fprintln(&b)
	}

	if len(r.ForbiddenFixes) > 0 {
		fmt.Fprintf(&b, "Forbidden Fixes (%d)\n", len(r.ForbiddenFixes))
		for _, f := range r.ForbiddenFixes {
			fmt.Fprintf(&b, "  [%s] %s\n", f.Severity, f.NodeName)
			for _, p := range f.EdgePath {
				fmt.Fprintf(&b, "    %s\n", p)
			}
		}
		fmt.Fprintln(&b)
	}

	if len(r.RequiredTests) > 0 {
		fmt.Fprintf(&b, "Required Tests (%d)\n", len(r.RequiredTests))
		for _, f := range r.RequiredTests {
			fmt.Fprintf(&b, "  [%s] %s\n", f.Severity, f.NodeName)
		}
		fmt.Fprintln(&b)
	}

	if len(r.MissingLinks) > 0 {
		fmt.Fprintf(&b, "Missing Links\n")
		for _, m := range r.MissingLinks {
			fmt.Fprintf(&b, "  ! %s\n", m)
		}
		fmt.Fprintln(&b)
	}

	if len(r.Recommendations) > 0 {
		fmt.Fprintf(&b, "Recommendations\n")
		for _, rec := range r.Recommendations {
			fmt.Fprintf(&b, "  [%s] %s\n", strings.ToUpper(rec.Priority), rec.Action)
			if rec.Evidence != "" {
				fmt.Fprintf(&b, "    evidence: %s\n", rec.Evidence)
			}
			if rec.GraphPath != "" {
				fmt.Fprintf(&b, "    graph:    %s\n", rec.GraphPath)
			}
		}
		fmt.Fprintln(&b)
	}

	if len(r.MissingLinks) == 0 && len(r.Recommendations) == 0 {
		fmt.Fprintf(&b, "No gaps or recommendations found.\n")
	}

	return b.String()
}
