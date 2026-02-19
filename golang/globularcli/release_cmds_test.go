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
	yaml := []byte(`spec: {service_name: gateway}`)
	if _, err := parseServiceRelease(yaml); err == nil {
		t.Fatalf("expected error for missing publisher_id")
	}
}

func TestApplyIdempotent(t *testing.T) {
	fc := &fakeResourcesClient{}
	resourcesClientFactory = func(conn *grpc.ClientConn) releaseResourcesClient { return fc }
	rootCfg.controllerAddr = "" // unused

	yaml := []byte(`
spec:
  publisher_id: globular
  service_name: gateway
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
	resourcesClientFactory = func(conn *grpc.ClientConn) releaseResourcesClient { return fc }
	yaml := []byte(`
spec:
  publisher_id: globular
  service_name: gateway
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
	resourcesClientFactory = func(conn *grpc.ClientConn) releaseResourcesClient { return fc }

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
	resourcesClientFactory = func(conn *grpc.ClientConn) releaseResourcesClient { return fc }

	rel, _ := fc.GetServiceRelease(context.Background(), &clustercontrollerpb.GetServiceReleaseRequest{Name: "gateway"})
	st := rel.Status

	out := captureStdout(t, func() {
		fmt.Printf("Phase:            %s\n", st.Phase)
		fmt.Printf("Resolved Version: %s\n", st.ResolvedVersion)
		if st.DesiredHash != "" {
			fmt.Printf("Desired Hash:     %s\n", st.DesiredHash)
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
		"2 total, 1 healthy",
		"n1", "n2", "stale",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q; got:\n%s", want, out)
		}
	}
}
