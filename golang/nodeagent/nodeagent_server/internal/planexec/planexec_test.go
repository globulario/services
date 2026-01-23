package planexec

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/actions"
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/globulario/services/golang/plan/versionutil"
	"google.golang.org/protobuf/types/known/structpb"
)

type successProbeAction struct{}

func (successProbeAction) Name() string                         { return "probe.success" }
func (successProbeAction) Validate(args *structpb.Struct) error { return nil }
func (successProbeAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	return "ok", nil
}

func init() {
	actions.Register(successProbeAction{})
}

func TestReconcilePlanSucceedsWhenProbesPass(t *testing.T) {
	plan := &planpb.NodePlan{
		NodeId: "node-1",
		Spec: &planpb.PlanSpec{
			SuccessProbes: []*planpb.Probe{
				{Type: "probe.success"},
			},
		},
	}
	runner := NewRunner("node-1", nil)
	status, err := runner.ReconcilePlan(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("ReconcilePlan returned error: %v", err)
	}
	if status.GetState() != planpb.PlanState_PLAN_SUCCEEDED {
		t.Fatalf("expected PLAN_SUCCEEDED got %v", status.GetState())
	}
}

func TestDetectServiceVersionUsesMarkerFile(t *testing.T) {
	tmp := t.TempDir()
	oldBase := versionutil.BaseDir()
	versionutil.SetBaseDir(tmp)
	defer versionutil.SetBaseDir(oldBase)

	const svc = "my-service"
	path := versionutil.MarkerPath(svc)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir marker dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	got, err := detectServiceVersion(svc)
	if err != nil {
		t.Fatalf("detectServiceVersion error: %v", err)
	}
	if got != "1.2.3" {
		t.Fatalf("expected version 1.2.3 got %q", got)
	}
}
