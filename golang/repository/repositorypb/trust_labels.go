package repositorypb

// trust_labels.go — machine-readable trust signal computation for artifacts.
//
// Trust labels separate "owned namespace" from "verified publisher" and provide
// a structured vocabulary for UI, CLI, and policy evaluation.

import "strings"

// TrustLabel represents a machine-readable trust signal for an artifact.
type TrustLabel string

const (
	TrustLabelOwned            TrustLabel = "owned"               // namespace has a claimed owner
	TrustLabelVerifiedNS       TrustLabel = "verified_namespace"   // namespace is verified (future)
	TrustLabelTrustedCI        TrustLabel = "trusted_ci"           // published via trusted CI relationship
	TrustLabelOfficial         TrustLabel = "official"             // from an official/system namespace
	TrustLabelDeprecated       TrustLabel = "deprecated"           // deprecated state
	TrustLabelYanked           TrustLabel = "yanked"               // yanked state
	TrustLabelQuarantined      TrustLabel = "quarantined"          // under security review
	TrustLabelRevoked          TrustLabel = "revoked"              // permanently revoked
	TrustLabelMachinePublished TrustLabel = "machine_published"    // published by APPLICATION (not human)
	TrustLabelLegacy           TrustLabel = "legacy"               // migrated without provenance
	TrustLabelUnclaimedNS      TrustLabel = "unclaimed_namespace"  // namespace not yet claimed by a user
)

// ComputeStateLabels returns trust labels derived from the publish state alone.
func ComputeStateLabels(state PublishState) []TrustLabel {
	var labels []TrustLabel
	switch state {
	case PublishState_DEPRECATED:
		labels = append(labels, TrustLabelDeprecated)
	case PublishState_YANKED:
		labels = append(labels, TrustLabelYanked)
	case PublishState_QUARANTINED:
		labels = append(labels, TrustLabelQuarantined)
	case PublishState_REVOKED:
		labels = append(labels, TrustLabelRevoked)
	}
	return labels
}

// ComputeProvenanceLabels returns trust labels derived from provenance metadata.
func ComputeProvenanceLabels(prov *ProvenanceRecord) []TrustLabel {
	if prov == nil {
		return nil
	}
	var labels []TrustLabel
	if prov.PrincipalType == "application" {
		labels = append(labels, TrustLabelMachinePublished)
	}
	if prov.Subject == "migration" && prov.AuthMethod == "none" {
		labels = append(labels, TrustLabelLegacy)
	}
	return labels
}

// officialPrefixes are namespace prefixes that indicate official/system packages.
var officialPrefixes = []string{"globular", "system", "core"}

// IsOfficialNamespace returns true if the namespace is under an official/system prefix.
func IsOfficialNamespace(publisherID string) bool {
	for _, p := range officialPrefixes {
		if publisherID == p {
			return true
		}
		if strings.HasPrefix(publisherID, p+".") || strings.HasPrefix(publisherID, p+"-") || strings.HasPrefix(publisherID, p+"@") {
			return true
		}
	}
	return false
}

// IsVerifiedPublisher returns true if a publisher satisfies the v1 verified publisher rule:
//   - Official/system namespace (globular, system, core prefixes), OR
//   - Namespace has been explicitly claimed by a real user (hasRealOwner=true)
func IsVerifiedPublisher(publisherID string, hasRealOwner bool) bool {
	if IsOfficialNamespace(publisherID) {
		return true
	}
	return hasRealOwner
}

// ComputeNamespaceLabels returns trust labels derived from namespace ownership status.
// hasOwner indicates whether the namespace has a claimed (non-migration) owner.
func ComputeNamespaceLabels(publisherID string, hasOwner bool) []TrustLabel {
	var labels []TrustLabel
	if hasOwner {
		labels = append(labels, TrustLabelOwned)
	} else {
		labels = append(labels, TrustLabelUnclaimedNS)
	}
	if IsOfficialNamespace(publisherID) {
		labels = append(labels, TrustLabelOfficial)
	}
	return labels
}
