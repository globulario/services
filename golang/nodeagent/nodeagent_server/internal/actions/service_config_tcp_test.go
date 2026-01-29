package actions

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

func TestServiceConfigTCPProbe(t *testing.T) {
	binDir := t.TempDir()
	stateRoot := t.TempDir()
	t.Setenv("GLOBULAR_INSTALL_BIN_DIR", binDir)
	t.Setenv("GLOBULAR_STATE_DIR", stateRoot)
	t.Setenv("GLOBULAR_PORT_RANGE", "72001-72002")

	// fake binary
	binPath := filepath.Join(binDir, "rbac_server")
	script := "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then echo '{\"Id\":\"rbac-id\",\"Address\":\"localhost:72001\"}'; fi\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	// config file
	cfgPath := filepath.Join(stateRoot, "services")
	if err := os.MkdirAll(cfgPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgPath, "rbac-id.json"), []byte(`{"Id":"rbac-id","Address":"localhost:72001","Port":72001}`), 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:72001")
	if err != nil {
		t.Skipf("listen not permitted: %v", err)
	}
	defer ln.Close()

	args, _ := structpb.NewStruct(map[string]interface{}{"service": "rbac"})
	p := serviceConfigTCPProbe{}
	if _, err := p.Apply(context.Background(), args); err != nil {
		t.Fatalf("probe should succeed: %v", err)
	}

	ln.Close()
	if _, err := p.Apply(context.Background(), args); err == nil {
		t.Fatalf("probe should fail when port closed")
	}
}
