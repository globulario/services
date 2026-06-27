package runtimedirs

import (
	"reflect"
	"testing"
)

func TestCanonicalRuntimeDir(t *testing.T) {
	cases := map[string]string{
		"cluster-doctor": "cluster-doctor",
		"cluster_doctor": "cluster-doctor",
		"clusterdoctor":  "cluster-doctor",
		"ai_executor":    "ai-executor",
		"ai_memory":      "ai-memory",
		"":               "",
		"  node_agent  ": "node-agent",
		"unknown_thing":  "unknown-thing", // cheap underscore→hyphen fallback
	}
	for in, want := range cases {
		if got := CanonicalRuntimeDir(in); got != want {
			t.Errorf("CanonicalRuntimeDir(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAllRuntimeDirAliases(t *testing.T) {
	got := AllRuntimeDirAliases("cluster_doctor")
	want := []string{"cluster-doctor", "cluster_doctor", "clusterdoctor"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AllRuntimeDirAliases = %v, want %v", got, want)
	}
}

func TestIsKnownRuntimeDirAlias(t *testing.T) {
	if canon, ok := IsKnownRuntimeDirAlias("ai_memory"); !ok || canon != "ai-memory" {
		t.Errorf("ai_memory: got (%q,%v), want (ai-memory,true)", canon, ok)
	}
	if canon, ok := IsKnownRuntimeDirAlias("ai-memory"); !ok || canon != "ai-memory" {
		t.Errorf("ai-memory (canonical): got (%q,%v), want (ai-memory,true)", canon, ok)
	}
	if _, ok := IsKnownRuntimeDirAlias("totally-unknown"); ok {
		t.Error("totally-unknown should not be a known alias")
	}
}

// CanonicalToLegacy must return a defensive copy — mutating the result must not
// corrupt the shared model.
func TestCanonicalToLegacyReturnsCopy(t *testing.T) {
	m := CanonicalToLegacy()
	if len(m) == 0 {
		t.Fatal("alias model is empty")
	}
	m["cluster-doctor"][0] = "MUTATED"
	delete(m, "ai-memory")
	if got := LegacyRuntimeAliases("cluster-doctor"); len(got) == 0 || got[0] != "cluster_doctor" {
		t.Errorf("shared map mutated through CanonicalToLegacy copy: %v", got)
	}
	if got := LegacyRuntimeAliases("ai-memory"); len(got) == 0 {
		t.Error("ai-memory entry deleted from shared map via copy")
	}
}
