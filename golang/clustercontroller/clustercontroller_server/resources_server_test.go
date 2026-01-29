package main

import (
	"context"
	"testing"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/clustercontroller/resourcestore"
	"google.golang.org/grpc/metadata"
)

type fakeWatchServer struct {
	ctx    context.Context
	events []*clustercontrollerpb.WatchEvent
}

func (f *fakeWatchServer) Send(e *clustercontrollerpb.WatchEvent) error {
	f.events = append(f.events, e)
	return nil
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
	obj, err := srv.ApplyServiceDesiredVersion(context.Background(), &clustercontrollerpb.ApplyServiceDesiredVersionRequest{
		Object: &clustercontrollerpb.ServiceDesiredVersion{
			Spec: &clustercontrollerpb.ServiceDesiredVersionSpec{
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
	list, err := srv.ListServiceDesiredVersions(context.Background(), &clustercontrollerpb.ListServiceDesiredVersionsRequest{})
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
		_ = srv.Watch(&clustercontrollerpb.WatchRequest{
			Type:            "ServiceDesiredVersion",
			Prefix:          "",
			IncludeExisting: true,
		}, stream)
	}()

	waitForEvents := func(n int) bool {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if len(stream.events) >= n {
				return true
			}
			time.Sleep(20 * time.Millisecond)
		}
		return len(stream.events) >= n
	}

	if !waitForEvents(1) {
		t.Fatalf("expected at least 1 event, got %d", len(stream.events))
	}
	// Modify version
	_, err = srv.ApplyServiceDesiredVersion(context.Background(), &clustercontrollerpb.ApplyServiceDesiredVersionRequest{
		Object: &clustercontrollerpb.ServiceDesiredVersion{
			Spec: &clustercontrollerpb.ServiceDesiredVersionSpec{
				ServiceName: "gateway",
				Version:     "2.0.0",
			},
		},
	})
	if err != nil {
		t.Fatalf("ApplyServiceDesiredVersion (update): %v", err)
	}
	if !waitForEvents(2) {
		t.Fatalf("expected at least 2 events, got %d", len(stream.events))
	}
}
