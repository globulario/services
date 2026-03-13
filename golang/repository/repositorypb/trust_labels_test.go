package repositorypb

import "testing"

func TestComputeStateLabels_Published_Empty(t *testing.T) {
	labels := ComputeStateLabels(PublishState_PUBLISHED)
	if len(labels) != 0 {
		t.Errorf("PUBLISHED should have no state labels, got %v", labels)
	}
}

func TestComputeStateLabels_Deprecated(t *testing.T) {
	labels := ComputeStateLabels(PublishState_DEPRECATED)
	if len(labels) != 1 || labels[0] != TrustLabelDeprecated {
		t.Errorf("DEPRECATED labels = %v, want [deprecated]", labels)
	}
}

func TestComputeStateLabels_Yanked(t *testing.T) {
	labels := ComputeStateLabels(PublishState_YANKED)
	if len(labels) != 1 || labels[0] != TrustLabelYanked {
		t.Errorf("YANKED labels = %v, want [yanked]", labels)
	}
}

func TestComputeStateLabels_Quarantined(t *testing.T) {
	labels := ComputeStateLabels(PublishState_QUARANTINED)
	if len(labels) != 1 || labels[0] != TrustLabelQuarantined {
		t.Errorf("QUARANTINED labels = %v, want [quarantined]", labels)
	}
}

func TestComputeStateLabels_Revoked(t *testing.T) {
	labels := ComputeStateLabels(PublishState_REVOKED)
	if len(labels) != 1 || labels[0] != TrustLabelRevoked {
		t.Errorf("REVOKED labels = %v, want [revoked]", labels)
	}
}

func TestComputeProvenanceLabels_Application(t *testing.T) {
	prov := &ProvenanceRecord{
		PrincipalType: "application",
		Subject:       "ci-bot",
		AuthMethod:    "jwt",
	}
	labels := ComputeProvenanceLabels(prov)
	found := false
	for _, l := range labels {
		if l == TrustLabelMachinePublished {
			found = true
		}
	}
	if !found {
		t.Errorf("application principal should produce machine_published label, got %v", labels)
	}
}

func TestComputeProvenanceLabels_Migration(t *testing.T) {
	prov := &ProvenanceRecord{
		Subject:       "migration",
		PrincipalType: "user",
		AuthMethod:    "none",
	}
	labels := ComputeProvenanceLabels(prov)
	found := false
	for _, l := range labels {
		if l == TrustLabelLegacy {
			found = true
		}
	}
	if !found {
		t.Errorf("migration provenance should produce legacy label, got %v", labels)
	}
}

func TestComputeProvenanceLabels_Nil(t *testing.T) {
	labels := ComputeProvenanceLabels(nil)
	if labels != nil {
		t.Errorf("nil provenance should return nil labels, got %v", labels)
	}
}

func TestComputeNamespaceLabels_Owned(t *testing.T) {
	labels := ComputeNamespaceLabels("acme", true)
	found := false
	for _, l := range labels {
		if l == TrustLabelOwned {
			found = true
		}
	}
	if !found {
		t.Errorf("owned namespace should include 'owned' label, got %v", labels)
	}
	// Should not include unclaimed.
	for _, l := range labels {
		if l == TrustLabelUnclaimedNS {
			t.Error("owned namespace should not include unclaimed_namespace label")
		}
	}
}

func TestComputeNamespaceLabels_Unclaimed(t *testing.T) {
	labels := ComputeNamespaceLabels("acme", false)
	found := false
	for _, l := range labels {
		if l == TrustLabelUnclaimedNS {
			found = true
		}
	}
	if !found {
		t.Errorf("unclaimed namespace should include 'unclaimed_namespace' label, got %v", labels)
	}
	// Should not include owned.
	for _, l := range labels {
		if l == TrustLabelOwned {
			t.Error("unclaimed namespace should not include owned label")
		}
	}
}

func TestComputeNamespaceLabels_OfficialIncludesOfficialLabel(t *testing.T) {
	labels := ComputeNamespaceLabels("globular", true)
	found := false
	for _, l := range labels {
		if l == TrustLabelOfficial {
			found = true
		}
	}
	if !found {
		t.Errorf("official namespace should include 'official' label, got %v", labels)
	}
}
