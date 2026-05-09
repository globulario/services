package semanticdiff

import "strings"

// layerTerms maps lowercase token fragments to a layer.
var layerTerms = []struct {
	term  string
	layer string
}{
	{"desiredstate", LayerDesired},
	{"desiredversion", LayerDesired},
	{"desiredrelease", LayerDesired},
	{"desiredconfig", LayerDesired},
	{"desiredspec", LayerDesired},
	{"intendedstate", LayerDesired},
	{"wantedstate", LayerDesired},
	{"targetstate", LayerDesired},
	{"desired", LayerDesired},

	{"installedstate", LayerInstalled},
	{"installedversion", LayerInstalled},
	{"installedrelease", LayerInstalled},
	{"installedspec", LayerInstalled},
	{"observedstate", LayerInstalled},
	{"appliedstate", LayerInstalled},
	{"committedstate", LayerInstalled},
	{"installed", LayerInstalled},

	{"runtimestate", LayerRuntime},
	{"livestate", LayerRuntime},
	{"systemdstate", LayerRuntime},
	{"actualstate", LayerRuntime},
	{"heartbeat", LayerRuntime},
	{"readiness", LayerRuntime},
	{"runtime", LayerRuntime},

	{"artifactmetadata", LayerArtifact},
	{"manifest", LayerArtifact},
	{"artifact", LayerArtifact},
	{"blobdigest", LayerArtifact},
	{"buildid", LayerArtifact},
	{"release", LayerArtifact},
}

// IdentifyLayer returns the layer a given identifier belongs to.
// Returns LayerUnknown when the identifier doesn't map to a known layer.
func IdentifyLayer(identifier string) string {
	lower := strings.ToLower(strings.ReplaceAll(identifier, "_", ""))
	for _, lt := range layerTerms {
		if strings.Contains(lower, lt.term) {
			return lt.layer
		}
	}
	return LayerUnknown
}

type forbiddenPair struct{ from, to, kind, reason string }

var forbiddenTransitions = []forbiddenPair{
	{LayerDesired, LayerInstalled, "desired_state_promoted_to_installed_without_proof",
		"Installed state must be backed by apply proof, generation match, or authoritative transaction."},
	{LayerArtifact, LayerInstalled, "artifact_metadata_treated_as_installed",
		"Artifact metadata may not produce Installed state without resolved build_id and install action."},
	{LayerRuntime, LayerDesired, "runtime_state_promoted_to_desired",
		"Runtime observations may produce drift findings but must not directly rewrite Desired."},
	{LayerInstalled, LayerDesired, "installed_state_treated_as_desired",
		"Installed state cannot rewrite Desired without an explicit controller decision."},
}

// ForbiddenTransition returns true when moving from → to is forbidden.
func ForbiddenTransition(from, to string) bool {
	for _, fp := range forbiddenTransitions {
		if fp.from == from && fp.to == to {
			return true
		}
	}
	return false
}

// TransitionKind returns the atom kind and reason for a layer transition.
func TransitionKind(from, to string) (kind, reason string) {
	for _, fp := range forbiddenTransitions {
		if fp.from == from && fp.to == to {
			return fp.kind, fp.reason
		}
	}
	return "state_layer_bypass", "Unknown layer transition."
}

// ServiceFromSymbol infers which service owns a changed symbol based on naming conventions.
func ServiceFromSymbol(filePath, symbol string) string {
	path := strings.ToLower(filePath)
	switch {
	case strings.Contains(path, "cluster_controller"):
		return "cluster-controller"
	case strings.Contains(path, "node_agent"):
		return "node-agent"
	case strings.Contains(path, "workflow"):
		return "workflow-service"
	case strings.Contains(path, "repository"):
		return "repository"
	case strings.Contains(path, "cluster_doctor") || strings.Contains(path, "awareness"):
		return "doctor/awareness"
	default:
		return ""
	}
}
