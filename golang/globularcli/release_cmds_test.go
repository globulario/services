package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"google.golang.org/grpc"
)

type fakeResourcesClient struct {
	applied int
	lastReq *clustercontrollerpb.ApplyServiceReleaseRequest
	err     error
}

func (f *fakeResourcesClient) ApplyServiceRelease(ctx context.Context, req *clustercontrollerpb.ApplyServiceReleaseRequest, opts ...grpc.CallOption) (*clustercontrollerpb.ServiceRelease, error) {
	f.applied++
	f.lastReq = req
	if f.err != nil {
		return nil, f.err
	}
	obj := req.GetObject()
	if obj.Meta == nil {
		obj.Meta = &clustercontrollerpb.ObjectMeta{}
	}
	obj.Meta.Generation++
	obj.Status = &clustercontrollerpb.ServiceReleaseStatus{Phase: clustercontrollerpb.ReleasePhaseAvailable}
	return obj, nil
}

// unused interface methods
func (f *fakeResourcesClient) GetServiceRelease(_ context.Context, req *clustercontrollerpb.GetServiceReleaseRequest, _ ...grpc.CallOption) (*clustercontrollerpb.ServiceRelease, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &clustercontrollerpb.ServiceRelease{
		Meta: &clustercontrollerpb.ObjectMeta{Name: req.Name, Generation: 3},
		Spec: &clustercontrollerpb.ServiceReleaseSpec{PublisherID: "globular", ServiceName: "gateway"},
		Status: &clustercontrollerpb.ServiceReleaseStatus{
			Phase:           clustercontrollerpb.ReleasePhaseAvailable,
			ResolvedVersion: "1.2.3",
			DesiredHash:     "abc123",
			Nodes: []*clustercontrollerpb.NodeReleaseStatus{
				{NodeID: "n1", InstalledVersion: "1.2.3", Phase: clustercontrollerpb.ReleasePhaseAvailable},
				{NodeID: "n2", InstalledVersion: "1.1.0", Phase: clustercontrollerpb.ReleasePhaseDegraded, ErrorMessage: "stale"},
			},
		},
	}, nil
}
func (f *fakeResourcesClient) ListServiceReleases(_ context.Context, _ *clustercontrollerpb.ListServiceReleasesRequest, _ ...grpc.CallOption) (*clustercontrollerpb.ListServiceReleasesResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &clustercontrollerpb.ListServiceReleasesResponse{
		Items: []*clustercontrollerpb.ServiceRelease{
			{
				Meta:   &clustercontrollerpb.ObjectMeta{Name: "gateway"},
				Spec:   &clustercontrollerpb.ServiceReleaseSpec{PublisherID: "globular", ServiceName: "gateway"},
				Status: &clustercontrollerpb.ServiceReleaseStatus{Phase: clustercontrollerpb.ReleasePhaseAvailable, ResolvedVersion: "1.2.3"},
			},
		},
	}, nil
}

func TestParseServiceReleaseYAML(t *testing.T) {
	yaml := []byte(`
meta:
  name: gateway
spec:
  publisher_id: globular
  service_name: gateway
  replicas:
    min: 1
  version: 1.2.3
`)
	rel, err := parseServiceRelease(yaml)
	if err != nil {
		t.Fatalf("parseServiceRelease: %v", err)
	}
	if rel.Spec.PublisherID != "globular" || rel.Spec.ServiceName != "gateway" {
		t.Fatalf("unexpected parse result: %#v", rel.Spec)
	}
}

func TestParseServiceReleaseMissingPublisher(t *testing.T) {
	yaml := []byte(`spec: {service_name: gateway, replicas: {min:1}}`)
	if _, err := parseServiceRelease(yaml); err == nil {
		t.Fatalf("expected error for missing publisher_id")
	}
}

func TestParseServiceReleaseMissingReplicas(t *testing.T) {
	yaml := []byte(`
meta: {name: gateway}
spec:
  publisher_id: globular
  service_name: gateway
  version: 1.2.3
`)
	if _, err := parseServiceRelease(yaml); err == nil {
		t.Fatalf("expected error for missing replicas.min")
	}
}

func TestApplyIdempotent(t *testing.T) {
	fc := &fakeResourcesClient{}
	resourcesClientFactory = func(conn grpc.ClientConnInterface) releaseResourcesClient { return fc }
	rootCfg.controllerAddr = "" // unused

	yaml := []byte(`
meta:
  name: gateway
spec:
  publisher_id: globular
  service_name: gateway
  replicas:
    min: 1
  version: 1.2.3
`)
	rel, err := parseServiceRelease(yaml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	ctx := context.Background()
	_, err = fc.ApplyServiceRelease(ctx, &clustercontrollerpb.ApplyServiceReleaseRequest{Object: rel})
	if err != nil {
		t.Fatalf("apply first: %v", err)
	}
	_, err = fc.ApplyServiceRelease(ctx, &clustercontrollerpb.ApplyServiceReleaseRequest{Object: rel})
	if err != nil {
		t.Fatalf("apply second: %v", err)
	}
	if fc.applied != 2 {
		t.Fatalf("expected 2 apply calls, got %d", fc.applied)
	}
}

func TestApplyStopsOnClientError(t *testing.T) {
	fc := &fakeResourcesClient{err: errors.New("boom")}
	resourcesClientFactory = func(conn grpc.ClientConnInterface) releaseResourcesClient { return fc }
	yaml := []byte(`
meta:
  name: gateway
spec:
  publisher_id: globular
  service_name: gateway
  replicas:
    min: 1
  version: 1.2.3
`)
	rel, err := parseServiceRelease(yaml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := fc.ApplyServiceRelease(context.Background(), &clustercontrollerpb.ApplyServiceReleaseRequest{Object: rel}); err == nil {
		t.Fatalf("expected error from client")
	}
}

// captureStdout redirects os.Stdout to a buffer and returns the captured string.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestReleaseListFormatting(t *testing.T) {
	fc := &fakeResourcesClient{}
	resourcesClientFactory = func(conn grpc.ClientConnInterface) releaseResourcesClient { return fc }

	rows := [][]string{{"NAME", "SERVICE", "PHASE", "RESOLVED_VERSION", "AGE"}}
	resp, _ := fc.ListServiceReleases(context.Background(), &clustercontrollerpb.ListServiceReleasesRequest{})
	for _, rel := range resp.Items {
		rows = append(rows, []string{
			rel.Meta.Name,
			fmt.Sprintf("%s/%s", rel.Spec.PublisherID, rel.Spec.ServiceName),
			rel.Status.Phase,
			rel.Status.ResolvedVersion,
			"-",
		})
	}
	out := captureStdout(t, func() { printTable(rows) })

	for _, want := range []string{"NAME", "gateway", "globular/gateway", "AVAILABLE", "1.2.3"} {
		if !strings.Contains(out, want) {
			t.Fatalf("list output missing %q; got:\n%s", want, out)
		}
	}
}

func TestReleaseStatusFormatting(t *testing.T) {
	fc := &fakeResourcesClient{}
	resourcesClientFactory = func(conn grpc.ClientConnInterface) releaseResourcesClient { return fc }

	rel, _ := fc.GetServiceRelease(context.Background(), &clustercontrollerpb.GetServiceReleaseRequest{Name: "gateway"})
	st := rel.Status

	out := captureStdout(t, func() {
		fmt.Printf("Phase:            %s\n", st.Phase)
		fmt.Printf("Resolved Version: %s\n", st.ResolvedVersion)
		if st.DesiredHash != "" {
			fmt.Printf("Desired Hash:     %s\n", st.DesiredHash)
		}
		desiredMin := int32(0)
		if rel.Spec != nil && rel.Spec.Replicas != nil {
			desiredMin = rel.Spec.Replicas.Min
		}
		if desiredMin > 0 {
			available := 0
			mismatch := 0
			for _, n := range st.Nodes {
				if n.Phase == clustercontrollerpb.ReleasePhaseAvailable {
					available++
				} else {
					mismatch++
				}
			}
			fmt.Printf("Replicas:         desired=%d available=%d mismatch=%d\n", desiredMin, available, mismatch)
		}
		if len(st.Nodes) > 0 {
			healthy := 0
			for _, n := range st.Nodes {
				if n.Phase == clustercontrollerpb.ReleasePhaseAvailable {
					healthy++
				}
			}
			fmt.Printf("Nodes:            %d total, %d healthy\n", len(st.Nodes), healthy)
			fmt.Printf("\n  %-12s %-12s %-12s %s\n", "NODE", "VERSION", "PHASE", "ERROR")
			for _, n := range st.Nodes {
				fmt.Printf("  %-12s %-12s %-12s %s\n", n.NodeID, n.InstalledVersion, n.Phase, n.ErrorMessage)
			}
		}
	})

	for _, want := range []string{
		"AVAILABLE", "1.2.3", "abc123",
		"n1", "n2", "stale",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q; got:\n%s", want, out)
		}
	}
}

func TestRunReleaseApplyDryRun(t *testing.T) {
	oldConnFactory := controllerConnFactory
	oldFactory := resourcesClientFactory
	oldFile := releaseFile
	oldDry := releaseDry
	defer func() {
		controllerConnFactory = oldConnFactory
		resourcesClientFactory = oldFactory
		releaseFile = oldFile
		releaseDry = oldDry
	}()

	controllerConnFactory = func() (grpc.ClientConnInterface, error) {
		t.Fatalf("controller should not be dialed on dry-run")
		return nil, nil
	}
	resourcesClientFactory = func(conn grpc.ClientConnInterface) releaseResourcesClient {
		t.Fatalf("resources client should not be created on dry-run")
		return nil
	}

	tmp, err := os.CreateTemp("", "release-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	content := `{"meta":{"name":"gateway"},"spec":{"publisher_id":"globular","service_name":"gateway","replicas":{"min":1},"version":"1.2.3"}}`
	if _, err := tmp.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmp.Close()

	releaseFile = tmp.Name()
	releaseDry = true

	out := captureStdout(t, func() {
		if err := runReleaseApply(nil, nil); err != nil {
			t.Fatalf("runReleaseApply dry-run: %v", err)
		}
	})
	if !strings.Contains(out, "validated ServiceRelease") {
		t.Fatalf("expected validation output, got %q", out)
	}
}

type applyCountingClient struct {
	applied int
}

func (a *applyCountingClient) ApplyServiceRelease(ctx context.Context, req *clustercontrollerpb.ApplyServiceReleaseRequest, opts ...grpc.CallOption) (*clustercontrollerpb.ServiceRelease, error) {
	a.applied++
	if req.Object.Meta == nil {
		req.Object.Meta = &clustercontrollerpb.ObjectMeta{}
	}
	req.Object.Meta.Generation++
	req.Object.Status = &clustercontrollerpb.ServiceReleaseStatus{Phase: clustercontrollerpb.ReleasePhaseAvailable}
	return req.Object, nil
}

func (*applyCountingClient) GetServiceRelease(ctx context.Context, req *clustercontrollerpb.GetServiceReleaseRequest, opts ...grpc.CallOption) (*clustercontrollerpb.ServiceRelease, error) {
	return nil, fmt.Errorf("unused")
}
func (*applyCountingClient) ListServiceReleases(ctx context.Context, req *clustercontrollerpb.ListServiceReleasesRequest, opts ...grpc.CallOption) (*clustercontrollerpb.ListServiceReleasesResponse, error) {
	return nil, fmt.Errorf("unused")
}

func TestRunReleaseApplyCallsApplyOnce(t *testing.T) {
	ac := &applyCountingClient{}
	oldConnFactory := controllerConnFactory
	oldFactory := resourcesClientFactory
	oldFile := releaseFile
	oldDry := releaseDry
	defer func() {
		controllerConnFactory = oldConnFactory
		resourcesClientFactory = oldFactory
		releaseFile = oldFile
		releaseDry = oldDry
	}()
	controllerConnFactory = func() (grpc.ClientConnInterface, error) { return fakeConn{}, nil }
	resourcesClientFactory = func(conn grpc.ClientConnInterface) releaseResourcesClient { return ac }

	tmp, err := os.CreateTemp("", "release-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	content := `{"meta":{"name":"gateway"},"spec":{"publisher_id":"globular","service_name":"gateway","replicas":{"min":1},"version":"1.2.3"}}`
	if _, err := tmp.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	releaseFile = tmp.Name()
	releaseDry = false

	if err := runReleaseApply(nil, nil); err != nil {
		t.Fatalf("runReleaseApply: %v", err)
	}
	if ac.applied != 1 {
		t.Fatalf("expected one apply call, got %d", ac.applied)
	}
}

type memoryReleaseClient struct {
	rel     *clustercontrollerpb.ServiceRelease
	applied int
}

func (m *memoryReleaseClient) ApplyServiceRelease(ctx context.Context, req *clustercontrollerpb.ApplyServiceReleaseRequest, opts ...grpc.CallOption) (*clustercontrollerpb.ServiceRelease, error) {
	m.applied++
	if req.Object.Meta == nil {
		req.Object.Meta = &clustercontrollerpb.ObjectMeta{}
	}
	req.Object.Meta.Generation++
	m.rel = req.Object
	return req.Object, nil
}

func (m *memoryReleaseClient) GetServiceRelease(ctx context.Context, req *clustercontrollerpb.GetServiceReleaseRequest, opts ...grpc.CallOption) (*clustercontrollerpb.ServiceRelease, error) {
	if m.rel == nil {
		return nil, fmt.Errorf("release %s not found", req.Name)
	}
	return m.rel, nil
}

func (m *memoryReleaseClient) ListServiceReleases(ctx context.Context, req *clustercontrollerpb.ListServiceReleasesRequest, opts ...grpc.CallOption) (*clustercontrollerpb.ListServiceReleasesResponse, error) {
	return &clustercontrollerpb.ListServiceReleasesResponse{}, nil
}

type fakeConn struct{}

func (fakeConn) Invoke(ctx context.Context, method string, args any, reply any, opts ...grpc.CallOption) error {
	return nil
}

func (fakeConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, nil
}

func TestRunReleaseScale(t *testing.T) {
	mc := &memoryReleaseClient{
		rel: &clustercontrollerpb.ServiceRelease{
			Meta: &clustercontrollerpb.ObjectMeta{Name: "gateway"},
			Spec: &clustercontrollerpb.ServiceReleaseSpec{PublisherID: "globular", ServiceName: "gateway"},
		},
	}
	oldFactory := resourcesClientFactory
	oldConnFactory := controllerConnFactory
	oldMin, oldMax := releaseMin, releaseMax
	resourcesClientFactory = func(conn grpc.ClientConnInterface) releaseResourcesClient { return mc }
	controllerConnFactory = func() (grpc.ClientConnInterface, error) { return fakeConn{}, nil }
	releaseMin = 3
	releaseMax = 5
	defer func() {
		resourcesClientFactory = oldFactory
		controllerConnFactory = oldConnFactory
		releaseMin, releaseMax = oldMin, oldMax
	}()

	if err := runReleaseScale(nil, []string{"gateway"}); err != nil {
		t.Fatalf("runReleaseScale: %v", err)
	}
	if mc.rel.Spec.Replicas == nil || mc.rel.Spec.Replicas.Min != 3 || mc.rel.Spec.Replicas.Max != 5 {
		t.Fatalf("unexpected replicas: %#v", mc.rel.Spec.Replicas)
	}
	if mc.applied != 1 {
		t.Fatalf("expected 1 apply call, got %d", mc.applied)
	}
}

func TestRunReleaseRollback(t *testing.T) {
	mc := &memoryReleaseClient{
		rel: &clustercontrollerpb.ServiceRelease{
			Meta: &clustercontrollerpb.ObjectMeta{Name: "gateway"},
			Spec: &clustercontrollerpb.ServiceReleaseSpec{
				PublisherID: "globular", ServiceName: "gateway", Version: "1.0.0", Channel: "stable",
			},
		},
	}
	oldFactory := resourcesClientFactory
	oldConnFactory := controllerConnFactory
	oldTo := releaseRollbackTo
	resourcesClientFactory = func(conn grpc.ClientConnInterface) releaseResourcesClient { return mc }
	controllerConnFactory = func() (grpc.ClientConnInterface, error) { return fakeConn{}, nil }
	releaseRollbackTo = "2.0.0"
	defer func() {
		resourcesClientFactory = oldFactory
		controllerConnFactory = oldConnFactory
		releaseRollbackTo = oldTo
	}()

	if err := runReleaseRollback(nil, []string{"gateway"}); err != nil {
		t.Fatalf("runReleaseRollback: %v", err)
	}
	if got := mc.rel.Spec.Version; got != "2.0.0" {
		t.Fatalf("expected version 2.0.0, got %s", got)
	}
	if mc.rel.Spec.Channel != "" {
		t.Fatalf("expected channel cleared, got %s", mc.rel.Spec.Channel)
	}
	if mc.applied != 1 {
		t.Fatalf("expected 1 apply call, got %d", mc.applied)
	}
}

func TestRunReleaseRollbackRequiresTarget(t *testing.T) {
	mc := &memoryReleaseClient{
		rel: &clustercontrollerpb.ServiceRelease{
			Meta: &clustercontrollerpb.ObjectMeta{Name: "gateway"},
			Spec: &clustercontrollerpb.ServiceReleaseSpec{PublisherID: "globular", ServiceName: "gateway", Version: "1.0.0"},
		},
	}
	oldFactory := resourcesClientFactory
	oldConnFactory := controllerConnFactory
	oldTo := releaseRollbackTo
	resourcesClientFactory = func(conn grpc.ClientConnInterface) releaseResourcesClient { return mc }
	controllerConnFactory = func() (grpc.ClientConnInterface, error) { return fakeConn{}, nil }
	releaseRollbackTo = ""
	defer func() {
		resourcesClientFactory = oldFactory
		controllerConnFactory = oldConnFactory
		releaseRollbackTo = oldTo
	}()

	err := runReleaseRollback(nil, []string{"gateway"})
	if err == nil || !strings.Contains(err.Error(), "rollback target required") {
		t.Fatalf("expected target error, got %v", err)
	}
}
