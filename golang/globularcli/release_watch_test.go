package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type fakeWatchClient struct {
	events []*clustercontrollerpb.WatchEvent
	idx    int
}

func (f *fakeWatchClient) Recv() (*clustercontrollerpb.WatchEvent, error) {
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
	events := []*clustercontrollerpb.WatchEvent{
		{ServiceRelease: &clustercontrollerpb.ServiceRelease{Status: &clustercontrollerpb.ServiceReleaseStatus{Phase: "AVAILABLE", ResolvedVersion: "1.0.0"}}},
		{ServiceRelease: &clustercontrollerpb.ServiceRelease{Status: &clustercontrollerpb.ServiceReleaseStatus{Phase: "DEGRADED", ResolvedVersion: "1.0.0"}}},
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
	watchStream clustercontrollerpb.ResourcesService_WatchClient
	watchCalled int
}

func (m *mockResourcesClient) Watch(ctx context.Context, in *clustercontrollerpb.WatchRequest, opts ...grpc.CallOption) (clustercontrollerpb.ResourcesService_WatchClient, error) {
	m.watchCalled++
	return m.watchStream, nil
}
