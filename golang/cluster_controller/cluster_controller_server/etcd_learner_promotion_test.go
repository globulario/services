package main

import (
	"context"
	"errors"
	"testing"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// fakeEtcdAPI implements etcdClientAPI for FSM unit tests. Only the methods
// the promotion driver uses carry behavior; the rest are inert stubs.
type fakeEtcdAPI struct {
	members      []*etcdserverpb.Member
	promoteErr   error    // when set, MemberPromote fails (simulates "not in sync")
	promoteCalls []uint64 // IDs successfully promoted
}

func mkVoter(id uint64) *etcdserverpb.Member   { return &etcdserverpb.Member{ID: id, IsLearner: false} }
func mkLearner(id uint64) *etcdserverpb.Member { return &etcdserverpb.Member{ID: id, IsLearner: true} }

func (f *fakeEtcdAPI) MemberList(ctx context.Context) (*clientv3.MemberListResponse, error) {
	return &clientv3.MemberListResponse{Members: f.members}, nil
}

func (f *fakeEtcdAPI) MemberPromote(ctx context.Context, id uint64) (*clientv3.MemberPromoteResponse, error) {
	if f.promoteErr != nil {
		return nil, f.promoteErr
	}
	for _, m := range f.members {
		if m.ID == id {
			m.IsLearner = false
		}
	}
	f.promoteCalls = append(f.promoteCalls, id)
	return &clientv3.MemberPromoteResponse{}, nil
}

// Inert stubs for the rest of etcdClientAPI.
func (f *fakeEtcdAPI) Endpoints() []string { return nil }
func (f *fakeEtcdAPI) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	return &clientv3.GetResponse{}, nil
}
func (f *fakeEtcdAPI) MemberAdd(ctx context.Context, peerAddrs []string) (*clientv3.MemberAddResponse, error) {
	return &clientv3.MemberAddResponse{}, nil
}
func (f *fakeEtcdAPI) MemberAddAsLearner(ctx context.Context, peerAddrs []string) (*clientv3.MemberAddResponse, error) {
	return &clientv3.MemberAddResponse{}, nil
}
func (f *fakeEtcdAPI) MemberRemove(ctx context.Context, id uint64) (*clientv3.MemberRemoveResponse, error) {
	return &clientv3.MemberRemoveResponse{}, nil
}
func (f *fakeEtcdAPI) MemberUpdate(ctx context.Context, id uint64, peerAddrs []string) (*clientv3.MemberUpdateResponse, error) {
	return &clientv3.MemberUpdateResponse{}, nil
}
func (f *fakeEtcdAPI) Status(ctx context.Context, endpoint string) (*clientv3.StatusResponse, error) {
	return &clientv3.StatusResponse{}, nil
}

func TestTopologyAllowsLearnerPromotion(t *testing.T) {
	cases := []struct {
		voters, learners, target int
		want                     bool
		note                     string
	}{
		{1, 0, 3, false, "FounderOnly — nothing to promote"},
		{1, 1, 3, true, "promote 1->2v: a transient step toward the 3-voter target (Policy A′)"},
		{2, 1, 3, true, "finish the 3-voter transition"},
		{3, 0, 3, false, "already at HA target — nothing to promote"},
		{3, 1, 3, false, "at target; do not overshoot to 4 voters"},
		{1, 1, 2, false, "target is not HA — never grow toward a 2-voter settle"},
		{1, 1, 1, false, "single-node intent; keep the learner"},
		{2, 0, 3, false, "no learner to promote"},
		{1, 2, 3, true, "1v+2l would promote (unreachable on 3.5.14, but logically toward HA)"},
	}
	for _, c := range cases {
		if got := topologyAllowsLearnerPromotion(c.voters, c.learners, c.target); got != c.want {
			t.Errorf("topologyAllowsLearnerPromotion(%d,%d,%d) = %v, want %v (%s)",
				c.voters, c.learners, c.target, got, c.want, c.note)
		}
	}
}

func TestReconcileLearnerPromotion_PolicyA(t *testing.T) {
	ctx := context.Background()
	const target = 3

	t.Run("1v+1l promotes to 2v as a transient step toward 3", func(t *testing.T) {
		f := &fakeEtcdAPI{members: []*etcdserverpb.Member{mkVoter(1), mkLearner(2)}}
		m := &etcdMemberManager{client: f}
		if !m.reconcileLearnerPromotion(ctx, target) {
			t.Fatal("expected a promotion toward the 3-voter target")
		}
		if len(f.promoteCalls) != 1 {
			t.Fatalf("expected exactly one promotion; got %v", f.promoteCalls)
		}
	})

	t.Run("2v+1l promotes the last learner to reach 3v HA", func(t *testing.T) {
		f := &fakeEtcdAPI{members: []*etcdserverpb.Member{mkVoter(1), mkVoter(2), mkLearner(3)}}
		m := &etcdMemberManager{client: f}
		if !m.reconcileLearnerPromotion(ctx, target) {
			t.Fatal("expected a promotion to complete the 3-voter set")
		}
		if len(f.promoteCalls) != 1 {
			t.Fatalf("expected exactly one promotion; got %v", f.promoteCalls)
		}
	})

	t.Run("1v+1l with a non-HA target does not promote (never settle at 2v)", func(t *testing.T) {
		f := &fakeEtcdAPI{members: []*etcdserverpb.Member{mkVoter(1), mkLearner(2)}}
		m := &etcdMemberManager{client: f}
		if m.reconcileLearnerPromotion(ctx, 2) {
			t.Fatal("must not grow toward a 2-voter (non-HA) target")
		}
		if len(f.promoteCalls) != 0 {
			t.Fatalf("MemberPromote must not be called; got %v", f.promoteCalls)
		}
	})

	t.Run("not-caught-up learner is not promoted, retries next cycle", func(t *testing.T) {
		f := &fakeEtcdAPI{
			members:    []*etcdserverpb.Member{mkVoter(1), mkLearner(2)},
			promoteErr: errors.New("etcdserver: can only promote a learner member which is in sync with leader"),
		}
		m := &etcdMemberManager{client: f}
		if m.reconcileLearnerPromotion(ctx, target) {
			t.Fatal("must not report a promotion when the learner is not in sync")
		}
		if len(f.promoteCalls) != 0 {
			t.Fatalf("no learner should be recorded promoted; got %v", f.promoteCalls)
		}
	})

	t.Run("3v+0l does not promote (already at HA target)", func(t *testing.T) {
		f := &fakeEtcdAPI{members: []*etcdserverpb.Member{mkVoter(1), mkVoter(2), mkVoter(3)}}
		m := &etcdMemberManager{client: f}
		if m.reconcileLearnerPromotion(ctx, target) {
			t.Fatal("expected no promotion at the HA target")
		}
		if len(f.promoteCalls) != 0 {
			t.Fatalf("MemberPromote must not be called; got %v", f.promoteCalls)
		}
	})
}
