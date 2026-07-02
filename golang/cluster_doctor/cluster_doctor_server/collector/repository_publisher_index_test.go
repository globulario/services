package collector

import (
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

func TestPublisherIndexFromManifests_GroupsInstallablePublishers(t *testing.T) {
	in := []*repopb.ArtifactManifest{
		{
			Ref:          &repopb.ArtifactRef{PublisherId: "core@globular.io", Name: "gateway", Version: "1.2.257"},
			PublishState: repopb.PublishState_PUBLISHED,
		},
		{
			Ref:          &repopb.ArtifactRef{PublisherId: "local@globule-ryzen", Name: "gateway", Version: "1.2.257"},
			PublishState: repopb.PublishState_PUBLISHED,
		},
		{
			Ref:          &repopb.ArtifactRef{PublisherId: "local@old", Name: "gateway", Version: "1.2.200"},
			PublishState: repopb.PublishState_ARCHIVED,
		},
	}

	got := publisherIndexFromManifests(in)
	if !got["gateway"]["core@globular.io"]["1.2.257"] {
		t.Fatalf("missing core publisher lane: %#v", got)
	}
	if !got["gateway"]["local@globule-ryzen"]["1.2.257"] {
		t.Fatalf("missing local publisher lane: %#v", got)
	}
	if got["gateway"]["local@old"]["1.2.200"] {
		t.Fatalf("archived artifact must not enter publisher index: %#v", got)
	}
}
