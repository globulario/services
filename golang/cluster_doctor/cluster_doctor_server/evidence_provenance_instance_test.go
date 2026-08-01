package main

import (
	"testing"

	"github.com/globulario/services/golang/evidence"
)

func TestInferEvidenceSource_InstanceQualifiedNodeAgent(t *testing.T) {
	for _, service := range []string{"node_agent@n1", "node-agent@n2"} {
		if got := inferEvidenceSource(service, "GetInventory"); got != evidence.SourceServiceLog {
			t.Fatalf("inferEvidenceSource(%q) = %q, want %q", service, got, evidence.SourceServiceLog)
		}
	}
}
