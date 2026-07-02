package component_catalog

import (
	"reflect"
	"testing"
)

func TestFoundingNodeProfiles_ExcludeMediaServer(t *testing.T) {
	want := []string{"core", "control-plane", "storage"}
	if !reflect.DeepEqual(FoundingNodeProfiles, want) {
		t.Fatalf("FoundingNodeProfiles = %v, want %v", FoundingNodeProfiles, want)
	}
	for _, profile := range FoundingNodeProfiles {
		if profile == "media-server" {
			t.Fatal("FoundingNodeProfiles must not include media-server")
		}
	}
}
