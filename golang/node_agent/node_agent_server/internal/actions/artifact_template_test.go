package actions

import (
	"testing"

	"github.com/globulario/services/golang/unitrender"
)

func TestReplaceTemplateVars_NodeIPVariants(t *testing.T) {
	in := "A={{.NodeIP}} B={{ .NodeIp }} C={{.nodeip}} D={{.StateDir}} E={{.Unknown}}"
	out := string(unitrender.RenderBytes([]byte(in), unitrender.Inputs{
		NodeIP:   "10.0.0.8",
		StateDir: "/var/lib/globular",
	}))
	want := "A=10.0.0.8 B=10.0.0.8 C=10.0.0.8 D=/var/lib/globular E={{.Unknown}}"
	if out != want {
		t.Fatalf("unexpected render output:\n got: %q\nwant: %q", out, want)
	}
}
