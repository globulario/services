package main

import (
	"context"
	"strings"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"google.golang.org/grpc"
)

type retryMemoryReleaseClient struct {
	rel     *cluster_controllerpb.ServiceRelease
	items   []*cluster_controllerpb.ServiceRelease
	applied int
}

func (m *retryMemoryReleaseClient) ApplyServiceRelease(ctx context.Context, req *cluster_controllerpb.ApplyServiceReleaseRequest, opts ...grpc.CallOption) (*cluster_controllerpb.ServiceRelease, error) {
	m.applied++
	if req.Object.Meta == nil {
		req.Object.Meta = &cluster_controllerpb.ObjectMeta{}
	}
	req.Object.Meta.Generation++
	m.rel = req.Object
	return req.Object, nil
}

func (m *retryMemoryReleaseClient) GetServiceRelease(_ context.Context, req *cluster_controllerpb.GetServiceReleaseRequest, _ ...grpc.CallOption) (*cluster_controllerpb.ServiceRelease, error) {
	if m.rel != nil && m.rel.Meta != nil && m.rel.Meta.Name == req.Name {
		return m.rel, nil
	}
	return nil, context.Canceled
}

func (m *retryMemoryReleaseClient) ListServiceReleases(_ context.Context, _ *cluster_controllerpb.ListServiceReleasesRequest, _ ...grpc.CallOption) (*cluster_controllerpb.ListServiceReleasesResponse, error) {
	return &cluster_controllerpb.ListServiceReleasesResponse{Items: m.items}, nil
}

func TestFetchReleaseByPackageOrName_MatchByServiceName(t *testing.T) {
	c := &retryMemoryReleaseClient{
		items: []*cluster_controllerpb.ServiceRelease{
			{
				Meta: &cluster_controllerpb.ObjectMeta{Name: "core@globular.io/sql"},
				Spec: &cluster_controllerpb.ServiceReleaseSpec{ServiceName: "sql"},
			},
		},
	}
	rel, err := fetchReleaseByPackageOrName(context.Background(), "sql", c)
	if err != nil {
		t.Fatalf("fetchReleaseByPackageOrName: %v", err)
	}
	if rel.Meta.Name != "core@globular.io/sql" {
		t.Fatalf("unexpected release name: %s", rel.Meta.Name)
	}
}

func TestFetchReleaseByPackageOrName_Ambiguous(t *testing.T) {
	c := &retryMemoryReleaseClient{
		items: []*cluster_controllerpb.ServiceRelease{
			{Meta: &cluster_controllerpb.ObjectMeta{Name: "a/sql"}, Spec: &cluster_controllerpb.ServiceReleaseSpec{ServiceName: "sql"}},
			{Meta: &cluster_controllerpb.ObjectMeta{Name: "b/sql"}, Spec: &cluster_controllerpb.ServiceReleaseSpec{ServiceName: "sql"}},
		},
	}
	_, err := fetchReleaseByPackageOrName(context.Background(), "sql", c)
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguous error, got %v", err)
	}
}

func TestApplyReleaseUnblockSignals_AnnotationsSet(t *testing.T) {
	mc := &retryMemoryReleaseClient{
		rel: &cluster_controllerpb.ServiceRelease{
			Meta: &cluster_controllerpb.ObjectMeta{Name: "core@globular.io/sql"},
			Spec: &cluster_controllerpb.ServiceReleaseSpec{ServiceName: "sql"},
		},
	}
	oldFactory := resourcesClientFactory
	oldConnFactory := controllerConnFactory
	resourcesClientFactory = func(conn grpc.ClientConnInterface) releaseResourcesClient { return mc }
	controllerConnFactory = func() (grpc.ClientConnInterface, error) { return releaseFakeConn{}, nil }
	defer func() {
		resourcesClientFactory = oldFactory
		controllerConnFactory = oldConnFactory
	}()

	if err := applyReleaseUnblockSignals("core@globular.io/sql", "hp-01", true); err != nil {
		t.Fatalf("applyReleaseUnblockSignals: %v", err)
	}
	ann := mc.rel.Meta.Annotations
	if ann[annotationReconcileResume] != "true" {
		t.Fatalf("resume annotation missing: %#v", ann)
	}
	if ann[annotationReconcileResumeNode] != "hp-01" {
		t.Fatalf("node annotation missing: %#v", ann)
	}
	if ann[annotationReconcileDependencyPresent] != "true" {
		t.Fatalf("dependency-present annotation missing: %#v", ann)
	}
	if mc.applied != 1 {
		t.Fatalf("expected one apply, got %d", mc.applied)
	}
}

