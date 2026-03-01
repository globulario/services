package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type fakeWatchClient struct {
	events []*cluster_controllerpb.WatchEvent
	idx    int
}

func (f *fakeWatchClient) Recv() (*cluster_controllerpb.WatchEvent, error) {
	if f.idx >= len(f.events) {
		return nil, errors.New("done")
	}
	ev := f.events[f.idx]
	f.idx++
	return ev, nil
}
func (f *fakeWatchClient) Header() (metadata.MD, error) { return nil, nil }
func (f *fakeWatchClient) Trailer() metadata.MD         { return nil }
func (f *fakeWatchClient) CloseSend() error             { return nil }
func (f *fakeWatchClient) Context() context.Context     { return context.Background() }
func (f *fakeWatchClient) SendMsg(m interface{}) error  { return nil }
func (f *fakeWatchClient) RecvMsg(m interface{}) error  { return nil }

func TestWatchReleasePrintsEvents(t *testing.T) {
	events := []*cluster_controllerpb.WatchEvent{
		{ServiceRelease: &cluster_controllerpb.ServiceRelease{Status: &cluster_controllerpb.ServiceReleaseStatus{Phase: "AVAILABLE", ResolvedVersion: "1.0.0"}}},
		{ServiceRelease: &cluster_controllerpb.ServiceRelease{Status: &cluster_controllerpb.ServiceReleaseStatus{Phase: "DEGRADED", ResolvedVersion: "1.0.0"}}},
	}
	fakeStream := &fakeWatchClient{events: events}

	client := &mockResourcesClient{watchStream: fakeStream}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	buf := captureStdout(t, func() {
		if err := watchReleaseOnce(ctx, "rel1", client, false); err == nil {
			// expected error after stream ends
		} else if err.Error() != "done" {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if client.watchCalled == 0 {
		t.Fatalf("expected watch to be called")
	}
	if !strings.Contains(buf, "AVAILABLE") || !strings.Contains(buf, "DEGRADED") {
		t.Fatalf("expected both events in output, got: %s", buf)
	}
}

// mockResourcesClient implements only Watch for tests.
type mockResourcesClient struct {
	watchStream cluster_controllerpb.ResourcesService_WatchClient
	watchCalled int
}

func (m *mockResourcesClient) Watch(ctx context.Context, in *cluster_controllerpb.WatchRequest, opts ...grpc.CallOption) (cluster_controllerpb.ResourcesService_WatchClient, error) {
	m.watchCalled++
	return m.watchStream, nil
}
