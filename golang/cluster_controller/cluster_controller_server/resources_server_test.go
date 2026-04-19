package main

import (
	"context"
	"sync"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
	"google.golang.org/grpc/metadata"
)

type fakeWatchServer struct {
	ctx    context.Context
	mu     sync.Mutex
	events []*cluster_controllerpb.WatchEvent
}

func (f *fakeWatchServer) Send(e *cluster_controllerpb.WatchEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, e)
	return nil
}

func (f *fakeWatchServer) eventCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.events)
}

func (f *fakeWatchServer) getEvents() []*cluster_controllerpb.WatchEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*cluster_controllerpb.WatchEvent, len(f.events))
	copy(out, f.events)
	return out
}

// Implement grpc.ServerStream minimal methods.
func (f *fakeWatchServer) SetHeader(md metadata.MD) error  { return nil }
func (f *fakeWatchServer) SendHeader(md metadata.MD) error { return nil }
func (f *fakeWatchServer) SetTrailer(md metadata.MD)       {}
func (f *fakeWatchServer) Context() context.Context        { return f.ctx }
func (f *fakeWatchServer) SendMsg(m interface{}) error     { return nil }
func (f *fakeWatchServer) RecvMsg(m interface{}) error     { return nil }

func TestResourcesServiceApplyListWatch(t *testing.T) {
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.resources = resourcestore.NewMemStore()
	srv.setLeader(true, "leader", "127.0.0.1:1234")

	// Apply service desired version
	obj, err := srv.ApplyServiceDesiredVersion(context.Background(), &cluster_controllerpb.ApplyServiceDesiredVersionRequest{
		Object: &cluster_controllerpb.ServiceDesiredVersion{
			Spec: &cluster_controllerpb.ServiceDesiredVersionSpec{
				ServiceName: "gateway",
				Version:     "1.0.0",
			},
		},
	})
	if err != nil {
		t.Fatalf("ApplyServiceDesiredVersion: %v", err)
	}
	if obj.Meta == nil || obj.Meta.Generation != 1 {
		t.Fatalf("expected generation=1, got %+v", obj.Meta)
	}
	if rv := obj.Meta.ResourceVersion; rv == "" {
		t.Fatalf("expected resource_version set")
	}

	// List returns the item
	list, err := srv.ListServiceDesiredVersions(context.Background(), &cluster_controllerpb.ListServiceDesiredVersionsRequest{})
	if err != nil {
		t.Fatalf("ListServiceDesiredVersions: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].Meta.Name != "gateway" {
		t.Fatalf("expected gateway item, got %+v", list.Items)
	}

	// Watch include_existing + modification
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &fakeWatchServer{ctx: ctx}
	go func() {
		_ = srv.Watch(&cluster_controllerpb.WatchRequest{
			Type:            "ServiceDesiredVersion",
			Prefix:          "",
			IncludeExisting: true,
		}, stream)
	}()

	waitForEvents := func(n int) bool {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if stream.eventCount() >= n {
				return true
			}
			time.Sleep(20 * time.Millisecond)
		}
		return stream.eventCount() >= n
	}

	if !waitForEvents(1) {
		t.Fatalf("expected at least 1 event, got %d", stream.eventCount())
	}
	// Modify version
	_, err = srv.ApplyServiceDesiredVersion(context.Background(), &cluster_controllerpb.ApplyServiceDesiredVersionRequest{
		Object: &cluster_controllerpb.ServiceDesiredVersion{
			Spec: &cluster_controllerpb.ServiceDesiredVersionSpec{
				ServiceName: "gateway",
				Version:     "2.0.0",
			},
		},
	})
	if err != nil {
		t.Fatalf("ApplyServiceDesiredVersion (update): %v", err)
	}
	if !waitForEvents(2) {
		t.Fatalf("expected at least 2 events, got %d", stream.eventCount())
	}
}
