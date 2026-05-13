package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
	clientv3 "go.etcd.io/etcd/client/v3"
	mvccpb "go.etcd.io/etcd/api/v3/mvccpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeApprovalKV struct {
	values map[string]string
}

func (f *fakeApprovalKV) Get(_ context.Context, key string, _ ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	resp := &clientv3.GetResponse{}
	for k, v := range f.values {
		if strings.HasPrefix(k, key) {
			resp.Kvs = append(resp.Kvs, &mvccpb.KeyValue{Key: []byte(k), Value: []byte(v)})
		}
	}
	return resp, nil
}

func (f *fakeApprovalKV) Put(context.Context, string, string, ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	return &clientv3.PutResponse{}, nil
}

func (f *fakeApprovalKV) Delete(context.Context, string, ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	return &clientv3.DeleteResponse{}, nil
}

func TestDeleteCriticalReleaseRequiresApproval(t *testing.T) {
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.resources = resourcestore.NewMemStore()
	srv.setLeader(true, "leader", "127.0.0.1:1234")
	srv.kv = &fakeApprovalKV{values: map[string]string{}}

	_, err := srv.ApplyServiceRelease(context.Background(), &cluster_controllerpb.ApplyServiceReleaseRequest{
		Object: &cluster_controllerpb.ServiceRelease{
			Spec: &cluster_controllerpb.ServiceReleaseSpec{
				PublisherID: "core@globular.io",
				ServiceName: "dns",
			},
		},
	})
	if err != nil {
		t.Fatalf("apply service release: %v", err)
	}
	_, err = srv.DeleteServiceRelease(context.Background(), &cluster_controllerpb.DeleteServiceReleaseRequest{
		Name: "core@globular.io/dns",
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition without approval, got: %v", err)
	}
}

func TestDeleteCriticalReleaseRejectsGenerationMismatch(t *testing.T) {
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.resources = resourcestore.NewMemStore()
	srv.setLeader(true, "leader", "127.0.0.1:1234")

	_, err := srv.ApplyServiceRelease(context.Background(), &cluster_controllerpb.ApplyServiceReleaseRequest{
		Object: &cluster_controllerpb.ServiceRelease{
			Spec: &cluster_controllerpb.ServiceReleaseSpec{
				PublisherID: "core@globular.io",
				ServiceName: "dns",
			},
		},
	})
	if err != nil {
		t.Fatalf("apply service release: %v", err)
	}
	approval := criticalKeyDeleteApproval{
		Generation:     999,
		ActorIdentity:  "sa",
		Reason:         "test",
		ApprovedAtUnix: time.Now().Unix(),
	}
	raw, _ := json.Marshal(approval)
	srv.kv = &fakeApprovalKV{values: map[string]string{
		"/globular/approvals/delete/release/servicerelease/core@globular.io/dns/1": string(raw),
	}}
	_, err = srv.DeleteServiceRelease(context.Background(), &cluster_controllerpb.DeleteServiceReleaseRequest{
		Name: "core@globular.io/dns",
	})
	if status.Code(err) != codes.FailedPrecondition || !strings.Contains(err.Error(), "generation mismatch") {
		t.Fatalf("expected generation mismatch precondition error, got: %v", err)
	}
}

func TestDeleteNonCriticalReleaseAllowedWithoutCriticalApproval(t *testing.T) {
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.resources = resourcestore.NewMemStore()
	srv.setLeader(true, "leader", "127.0.0.1:1234")
	srv.kv = &fakeApprovalKV{values: map[string]string{}}

	_, err := srv.ApplyServiceRelease(context.Background(), &cluster_controllerpb.ApplyServiceReleaseRequest{
		Object: &cluster_controllerpb.ServiceRelease{
			Spec: &cluster_controllerpb.ServiceReleaseSpec{
				PublisherID: "core@globular.io",
				ServiceName: "echo",
			},
		},
	})
	if err != nil {
		t.Fatalf("apply service release: %v", err)
	}
	_, err = srv.DeleteServiceRelease(context.Background(), &cluster_controllerpb.DeleteServiceReleaseRequest{
		Name: "core@globular.io/echo",
	})
	if err != nil {
		t.Fatalf("non-critical release delete should be allowed: %v", err)
	}
}
