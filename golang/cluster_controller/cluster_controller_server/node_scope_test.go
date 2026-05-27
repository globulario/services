package main

import (
	"context"
	"os"
	"testing"

	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ctxWithSubject creates a context with an AuthContext carrying the given subject.
func ctxWithSubject(subject string) context.Context {
	ac := &security.AuthContext{Subject: subject}
	return ac.ToContext(context.Background())
}

// --- ReportNodeStatus enforcement ---

func TestEnforceNodeScope_OwnNode_ReportNodeStatus(t *testing.T) {
	// node_abc reporting status for abc → allowed
	ctx := ctxWithSubject("node_abc")
	err := enforceNodeScope(ctx, "abc", "/clustercontroller.ClusterControllerService/ReportNodeStatus")
	if err != nil {
		t.Errorf("own-node ReportNodeStatus should be allowed: %v", err)
	}
}

func TestEnforceNodeScope_CrossNode_ReportNodeStatus(t *testing.T) {
	// node_abc reporting status for xyz → denied
	ctx := ctxWithSubject("node_abc")
	err := enforceNodeScope(ctx, "xyz", "/clustercontroller.ClusterControllerService/ReportNodeStatus")
	if err == nil {
		t.Fatal("cross-node ReportNodeStatus should be denied")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Errorf("expected PermissionDenied, got: %v", err)
	}
}

// --- ReportPlanRejection enforcement ---

func TestEnforceNodeScope_OwnNode_ReportPlanRejection(t *testing.T) {
	// node_abc reporting plan rejection for abc → allowed
	ctx := ctxWithSubject("node_abc")
	err := enforceNodeScope(ctx, "abc", "/clustercontroller.ClusterControllerService/ReportPlanRejection")
	if err != nil {
		t.Errorf("own-node ReportPlanRejection should be allowed: %v", err)
	}
}

func TestEnforceNodeScope_CrossNode_ReportPlanRejection(t *testing.T) {
	// node_abc reporting plan rejection for xyz → denied
	ctx := ctxWithSubject("node_abc")
	err := enforceNodeScope(ctx, "xyz", "/clustercontroller.ClusterControllerService/ReportPlanRejection")
	if err == nil {
		t.Fatal("cross-node ReportPlanRejection should be denied")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Errorf("expected PermissionDenied, got: %v", err)
	}
}

// --- SA deprecation/enforcement at handler layer ---

func TestEnforceNodeScope_SA_DeprecationMode(t *testing.T) {
	// sa on node-agent path with DEPRECATE_SA_NODE_AUTH=true → allowed with warning
	os.Setenv("DEPRECATE_SA_NODE_AUTH", "true")
	os.Unsetenv("REQUIRE_NODE_IDENTITY")
	defer os.Unsetenv("DEPRECATE_SA_NODE_AUTH")

	ctx := ctxWithSubject("sa")
	err := enforceNodeScope(ctx, "some-node", "/clustercontroller.ClusterControllerService/ReportNodeStatus")
	if err != nil {
		t.Errorf("sa in deprecation mode should be allowed (with warning): %v", err)
	}
}

func TestEnforceNodeScope_SA_EnforcementMode(t *testing.T) {
	// Intent: etcd.is_source_of_truth — REQUIRE_NODE_IDENTITY is now
	// controlled exclusively via etcd config, not env vars. Without etcd
	// config, the flag defaults to false and sa is allowed. This test
	// verifies that the env var is NOT consulted (security fix).
	t.Setenv("REQUIRE_NODE_IDENTITY", "true")

	ctx := ctxWithSubject("sa")
	err := enforceNodeScope(ctx, "some-node", "/clustercontroller.ClusterControllerService/ReportNodeStatus")
	if err != nil {
		t.Fatalf("sa should be ALLOWED when REQUIRE_NODE_IDENTITY is set only via env var (etcd config is the sole authority): %v", err)
	}
	// When etcd config sets RequireNodeIdentity=true, sa would be rejected.
	// That path is tested via integration tests with a real config backend.
}

// --- Non-node principal on node-only path ---

func TestEnforceNodeScope_NonNodePrincipal(t *testing.T) {
	// "operator" or "controller" on node-only path → allowed (admin/service principals pass)
	ctx := ctxWithSubject("globular-controller")
	err := enforceNodeScope(ctx, "some-node", "/clustercontroller.ClusterControllerService/ReportNodeStatus")
	if err != nil {
		t.Errorf("controller principal should be allowed: %v", err)
	}
}

func TestEnforceNodeScope_AnonymousPrincipal(t *testing.T) {
	// anonymous (empty subject) → rejected
	ctx := ctxWithSubject("")
	err := enforceNodeScope(ctx, "some-node", "/clustercontroller.ClusterControllerService/ReportNodeStatus")
	if err == nil {
		t.Fatal("anonymous principal should be rejected")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Errorf("expected PermissionDenied, got: %v", err)
	}
}

// --- No auth context (interceptor handles) ---

func TestEnforceNodeScope_NoAuthContext(t *testing.T) {
	// No AuthContext in context → nil (let interceptor handle)
	err := enforceNodeScope(context.Background(), "some-node", "/clustercontroller.ClusterControllerService/ReportNodeStatus")
	if err != nil {
		t.Errorf("no AuthContext should return nil (interceptor handles auth): %v", err)
	}
}
