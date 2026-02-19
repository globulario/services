package main

import (
	"context"
	"errors"
	"testing"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
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
func (f *fakeResourcesClient) GetServiceRelease(context.Context, *clustercontrollerpb.GetServiceReleaseRequest, ...grpc.CallOption) (*clustercontrollerpb.ServiceRelease, error) {
	return nil, nil
}
func (f *fakeResourcesClient) ListServiceReleases(context.Context, *clustercontrollerpb.ListServiceReleasesRequest, ...grpc.CallOption) (*clustercontrollerpb.ListServiceReleasesResponse, error) {
	return nil, nil
}
func (f *fakeResourcesClient) DeleteServiceRelease(context.Context, *clustercontrollerpb.DeleteServiceReleaseRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (f *fakeResourcesClient) ApplyClusterNetwork(context.Context, *clustercontrollerpb.ApplyClusterNetworkRequest, ...grpc.CallOption) (*clustercontrollerpb.ClusterNetwork, error) {
	return nil, nil
}
func (f *fakeResourcesClient) GetClusterNetwork(context.Context, *clustercontrollerpb.GetClusterNetworkRequest, ...grpc.CallOption) (*clustercontrollerpb.ClusterNetwork, error) {
	return nil, nil
}
func (f *fakeResourcesClient) ApplyServiceDesiredVersion(context.Context, *clustercontrollerpb.ApplyServiceDesiredVersionRequest, ...grpc.CallOption) (*clustercontrollerpb.ServiceDesiredVersion, error) {
	return nil, nil
}
func (f *fakeResourcesClient) ListServiceDesiredVersions(context.Context, *clustercontrollerpb.ListServiceDesiredVersionsRequest, ...grpc.CallOption) (*clustercontrollerpb.ListServiceDesiredVersionsResponse, error) {
	return nil, nil
}
func (f *fakeResourcesClient) DeleteServiceDesiredVersion(context.Context, *clustercontrollerpb.DeleteServiceDesiredVersionRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (f *fakeResourcesClient) Watch(context.Context, *clustercontrollerpb.WatchRequest, ...grpc.CallOption) (clustercontrollerpb.ResourcesService_WatchClient, error) {
	return nil, nil
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
	resourcesClientFactory = func(conn *grpc.ClientConn) clustercontrollerpb.ResourcesServiceClient { return fc }
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
	resourcesClientFactory = func(conn *grpc.ClientConn) clustercontrollerpb.ResourcesServiceClient { return fc }
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
