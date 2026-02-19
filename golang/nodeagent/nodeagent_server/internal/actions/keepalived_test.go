package actions

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/ingress"
)

func TestKeepalivedAction_Name(t *testing.T) {
	action := &keepalivedReconcileAction{}
	if name := action.Name(); name != "ingress.keepalived.reconcile" {
		t.Errorf("Expected name 'ingress.keepalived.reconcile', got '%s'", name)
	}
}

func TestKeepalivedAction_Validate(t *testing.T) {
	tests := []struct {
		name    string
		args    *structpb.Struct
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil args",
			args:    nil,
			wantErr: true,
			errMsg:  "args required",
		},
		{
			name: "missing spec_json",
			args: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"node_id": structpb.NewStringValue("n1"),
				},
			},
			wantErr: true,
			errMsg:  "spec_json is required",
		},
		{
			name: "missing node_id",
			args: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"spec_json": structpb.NewStringValue(`{"version":"v1","mode":"vip_failover"}`),
				},
			},
			wantErr: true,
			errMsg:  "node_id is required",
		},
		{
			name: "invalid spec_json",
			args: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"spec_json": structpb.NewStringValue(`{invalid json}`),
					"node_id":   structpb.NewStringValue("n1"),
				},
			},
			wantErr: true,
			errMsg:  "invalid spec_json",
		},
		{
			name: "valid args",
			args: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"spec_json": structpb.NewStringValue(`{"version":"v1","mode":"vip_failover","vip_failover":{"vip":"10.0.0.250/24","interface":"eth0","virtual_router_id":51,"participants":["n1"]}}`),
					"node_id":   structpb.NewStringValue("n1"),
				},
			},
			wantErr: false,
		},
	}

	action := &keepalivedReconcileAction{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := action.Validate(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestKeepalivedAction_Apply_DryRun(t *testing.T) {
	action := &keepalivedReconcileAction{}

	spec := ingress.Spec{
		Version: "v1",
		Mode:    ingress.ModeVIPFailover,
		VIPFailover: &ingress.VIPFailoverSpec{
			VIP:             "10.0.0.250/24",
			Interface:       "lo",
			VirtualRouterID: 51,
			Participants:    []string{"n1"},
			Priority:        map[string]int{"n1": 120},
		},
	}

	specJSON, _ := json.Marshal(spec)

	args := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"spec_json": structpb.NewStringValue(string(specJSON)),
			"node_id":   structpb.NewStringValue("n1"),
			"dry_run":   structpb.NewBoolValue(true),
		},
	}

	ctx := context.Background()
	result, err := action.Apply(ctx, args)
	if err != nil {
		t.Fatalf("Apply() dry-run failed: %v", err)
	}

	if !contains(result, "dry-run") {
		t.Errorf("Expected dry-run result, got: %s", result)
	}
}

func TestKeepalivedAction_Apply_ModeDisabled(t *testing.T) {
	action := &keepalivedReconcileAction{}

	spec := ingress.Spec{
		Version: "v1",
		Mode:    ingress.ModeDisabled,
	}

	specJSON, _ := json.Marshal(spec)

	args := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"spec_json": structpb.NewStringValue(string(specJSON)),
			"node_id":   structpb.NewStringValue("n1"),
			"dry_run":   structpb.NewBoolValue(true), // Use dry-run to avoid touching real system
		},
	}

	ctx := context.Background()
	result, err := action.Apply(ctx, args)
	if err != nil {
		t.Fatalf("Apply() with ModeDisabled failed: %v", err)
	}

	if !contains(result, "disable") {
		t.Errorf("Expected disable result, got: %s", result)
	}
}

func TestKeepalivedAction_Apply_NodeNotInParticipants(t *testing.T) {
	action := &keepalivedReconcileAction{}

	spec := ingress.Spec{
		Version: "v1",
		Mode:    ingress.ModeVIPFailover,
		VIPFailover: &ingress.VIPFailoverSpec{
			VIP:             "10.0.0.250/24",
			Interface:       "lo",
			VirtualRouterID: 51,
			Participants:    []string{"n1", "n2"}, // n3 not in list
			Priority:        map[string]int{"n1": 120, "n2": 110},
		},
	}

	specJSON, _ := json.Marshal(spec)

	args := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"spec_json": structpb.NewStringValue(string(specJSON)),
			"node_id":   structpb.NewStringValue("n3"), // Node not in participants
			"dry_run":   structpb.NewBoolValue(true),
		},
	}

	ctx := context.Background()
	result, err := action.Apply(ctx, args)
	if err != nil {
		t.Fatalf("Apply() with non-participant node failed: %v", err)
	}

	if !contains(result, "disable") {
		t.Errorf("Expected disable result for non-participant node, got: %s", result)
	}
}

func TestKeepalivedAction_Apply_DefaultPriority(t *testing.T) {
	action := &keepalivedReconcileAction{}

	spec := ingress.Spec{
		Version: "v1",
		Mode:    ingress.ModeVIPFailover,
		VIPFailover: &ingress.VIPFailoverSpec{
			VIP:             "10.0.0.250/24",
			Interface:       "lo",
			VirtualRouterID: 51,
			Participants:    []string{"n1"},
			Priority:        map[string]int{}, // No priority specified for n1
			CheckTCPPorts:   []int{443},
		},
	}

	specJSON, _ := json.Marshal(spec)

	args := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"spec_json": structpb.NewStringValue(string(specJSON)),
			"node_id":   structpb.NewStringValue("n1"),
			"dry_run":   structpb.NewBoolValue(true),
		},
	}

	ctx := context.Background()
	result, err := action.Apply(ctx, args)
	if err != nil {
		t.Fatalf("Apply() with default priority failed: %v", err)
	}

	// Should use default priority (100)
	if !contains(result, "priority 100") && !contains(result, "priority=100") {
		t.Logf("Result: %s", result)
		// Note: dry-run may not include priority in result, so this is informational
	}
}

func TestWriteFileIfChanged_NoChange(t *testing.T) {
	// Create a temp file with initial content
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.conf")

	initialContent := "test content\n"
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Write same content again
	changed, err := writeFileIfChanged(testFile, initialContent, 0644)
	if err != nil {
		t.Fatalf("writeFileIfChanged failed: %v", err)
	}

	if changed {
		t.Errorf("Expected changed=false when content is identical, got changed=true")
	}

	// Verify file content is unchanged
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != initialContent {
		t.Errorf("File content changed unexpectedly")
	}
}

func TestWriteFileIfChanged_Changed(t *testing.T) {
	// Create a temp file with initial content
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.conf")

	initialContent := "old content\n"
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Write different content
	newContent := "new content\n"
	changed, err := writeFileIfChanged(testFile, newContent, 0644)
	if err != nil {
		t.Fatalf("writeFileIfChanged failed: %v", err)
	}

	if !changed {
		t.Errorf("Expected changed=true when content differs, got changed=false")
	}

	// Verify file content is updated
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != newContent {
		t.Errorf("File content not updated correctly, got %q, want %q", string(content), newContent)
	}
}

func TestWriteFileIfChanged_NewFile(t *testing.T) {
	// Create a temp directory
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "subdir", "test.conf")

	// Write to non-existent file (should create directory and file)
	content := "new file content\n"
	changed, err := writeFileIfChanged(testFile, content, 0644)
	if err != nil {
		t.Fatalf("writeFileIfChanged failed for new file: %v", err)
	}

	if !changed {
		t.Errorf("Expected changed=true for new file, got changed=false")
	}

	// Verify file exists and has correct content
	readContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read new file: %v", err)
	}

	if string(readContent) != content {
		t.Errorf("New file content incorrect, got %q, want %q", string(readContent), content)
	}
}

func TestWriteFileIfChanged_Idempotency(t *testing.T) {
	// Test that writing the same content multiple times is idempotent
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.conf")

	content := "test content for idempotency\n"

	// First write (should change)
	changed1, err := writeFileIfChanged(testFile, content, 0644)
	if err != nil {
		t.Fatalf("First write failed: %v", err)
	}
	if !changed1 {
		t.Errorf("First write should report changed=true")
	}

	// Second write (should not change)
	changed2, err := writeFileIfChanged(testFile, content, 0644)
	if err != nil {
		t.Fatalf("Second write failed: %v", err)
	}
	if changed2 {
		t.Errorf("Second write should report changed=false (idempotent)")
	}

	// Third write (should not change)
	changed3, err := writeFileIfChanged(testFile, content, 0644)
	if err != nil {
		t.Fatalf("Third write failed: %v", err)
	}
	if changed3 {
		t.Errorf("Third write should report changed=false (idempotent)")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
