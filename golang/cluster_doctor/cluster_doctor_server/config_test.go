package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadConfigNormalizesLoopback locks in the contract exposed by
// docs/endpoint_resolution_policy.md: controller_endpoint/workflow_endpoint
// loaded from disk with a loopback IP literal must be rewritten to
// "localhost" before any dial is attempted. The cluster service cert
// has DNS:localhost in its SAN but not IP:127.0.0.1, so a dial against
// "127.0.0.1:12000" fails TLS verification.
//
// This is a regression test: the same deployed config that caused the
// original bug (127.0.0.1:12000 written by old packaging) must now
// round-trip to localhost:12000.
func TestLoadConfigNormalizesLoopback(t *testing.T) {
	cases := []struct {
		name               string
		json               string
		wantController     string
		wantWorkflow       string
	}{
		{
			name:           "loopback-ipv4",
			json:           `{"controller_endpoint":"127.0.0.1:12000","workflow_endpoint":"127.0.0.1:10220"}`,
			wantController: "localhost:12000",
			wantWorkflow:   "localhost:10220",
		},
		{
			name:           "loopback-ipv6-bracketed",
			json:           `{"controller_endpoint":"[::1]:12000","workflow_endpoint":"[::1]:10220"}`,
			wantController: "localhost:12000",
			wantWorkflow:   "localhost:10220",
		},
		{
			name:           "localhost-passthrough",
			json:           `{"controller_endpoint":"localhost:12000","workflow_endpoint":"localhost:10220"}`,
			wantController: "localhost:12000",
			wantWorkflow:   "localhost:10220",
		},
		{
			name:           "remote-host-passthrough",
			json:           `{"controller_endpoint":"controller.globular.internal:12000","workflow_endpoint":"wf.globular.internal:10220"}`,
			wantController: "controller.globular.internal:12000",
			wantWorkflow:   "wf.globular.internal:10220",
		},
		{
			name:           "empty-controller-fills-default",
			json:           `{"controller_endpoint":""}`,
			wantController: "localhost:12000", // default
			wantWorkflow:   "localhost:10220", // default (originally 127.0.0.1:10220)
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.json")
			if err := os.WriteFile(path, []byte(tc.json), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}
			cfg, err := loadConfig(path)
			if err != nil {
				t.Fatalf("loadConfig: %v", err)
			}
			if cfg.ControllerEndpoint != tc.wantController {
				t.Errorf("ControllerEndpoint = %q, want %q", cfg.ControllerEndpoint, tc.wantController)
			}
			if cfg.WorkflowEndpoint != tc.wantWorkflow {
				t.Errorf("WorkflowEndpoint = %q, want %q", cfg.WorkflowEndpoint, tc.wantWorkflow)
			}
		})
	}
}
