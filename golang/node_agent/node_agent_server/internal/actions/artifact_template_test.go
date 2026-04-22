package actions

import "testing"

func TestReplaceTemplateVars_NodeIPVariants(t *testing.T) {
	in := "A={{.NodeIP}} B={{ .NodeIp }} C={{.nodeip}} D={{.StateDir}} E={{.Unknown}}"
	out := replaceTemplateVars(in, map[string]string{
		"nodeip":   "10.0.0.8",
		"statedir": "/var/lib/globular",
	})
	want := "A=10.0.0.8 B=10.0.0.8 C=10.0.0.8 D=/var/lib/globular E={{.Unknown}}"
	if out != want {
		t.Fatalf("unexpected render output:\n got: %q\nwant: %q", out, want)
	}
}

