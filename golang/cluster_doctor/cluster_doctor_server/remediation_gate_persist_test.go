package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	ai_memorypb "github.com/globulario/services/golang/ai_memory/ai_memorypb"
	"google.golang.org/grpc"
)

// fakeGateMemory is a minimal in-memory stand-in for the ai-memory client,
// implementing the narrow remediationGateMemory interface. Tag matching uses AND
// logic, mirroring QueryRqst.Tags semantics.
type fakeGateMemory struct {
	mu      sync.Mutex
	mems    map[string]*ai_memorypb.Memory // id -> memory
	nextID  int
	failAll bool // when true, every RPC errors (simulates ai-memory down)
}

func newFakeGateMemory() *fakeGateMemory {
	return &fakeGateMemory{mems: map[string]*ai_memorypb.Memory{}}
}

func gateTagsContainAll(have, want []string) bool {
	set := map[string]bool{}
	for _, h := range have {
		set[h] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}

func (f *fakeGateMemory) Store(_ context.Context, in *ai_memorypb.StoreRqst, _ ...grpc.CallOption) (*ai_memorypb.StoreRsp, error) {
	if f.failAll {
		return nil, errors.New("ai-memory down")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	id := fmt.Sprintf("mem-%d", f.nextID)
	m := in.GetMemory()
	m.Id = id
	f.mems[id] = m
	return &ai_memorypb.StoreRsp{Id: id}, nil
}

func (f *fakeGateMemory) Query(_ context.Context, in *ai_memorypb.QueryRqst, _ ...grpc.CallOption) (*ai_memorypb.QueryRsp, error) {
	if f.failAll {
		return nil, errors.New("ai-memory down")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*ai_memorypb.Memory
	for _, m := range f.mems {
		if gateTagsContainAll(m.GetTags(), in.GetTags()) {
			out = append(out, m)
		}
	}
	return &ai_memorypb.QueryRsp{Memories: out, Total: int32(len(out))}, nil
}

func (f *fakeGateMemory) Update(_ context.Context, in *ai_memorypb.UpdateRqst, _ ...grpc.CallOption) (*ai_memorypb.UpdateRsp, error) {
	if f.failAll {
		return nil, errors.New("ai-memory down")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if existing, ok := f.mems[in.GetMemory().GetId()]; ok {
		if md := in.GetMemory().GetMetadata(); md != nil {
			existing.Metadata = md
		}
	}
	return &ai_memorypb.UpdateRsp{}, nil
}

func (f *fakeGateMemory) Delete(_ context.Context, in *ai_memorypb.DeleteRqst, _ ...grpc.CallOption) (*ai_memorypb.DeleteRsp, error) {
	if f.failAll {
		return nil, errors.New("ai-memory down")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.mems, in.GetId())
	return &ai_memorypb.DeleteRsp{}, nil
}

func (f *fakeGateMemory) countWithTag(tag string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, m := range f.mems {
		if gateTagsContainAll(m.GetTags(), []string{tag}) {
			n++
		}
	}
	return n
}

// TestRemediationGatePersist_RoundTrip proves the core EX-3 contract: an escalation
// persisted to ai-memory is recovered intact — so restart/failover cannot silently
// downgrade an escalated remediation back to auto-executable.
func TestRemediationGatePersist_RoundTrip(t *testing.T) {
	fake := newFakeGateMemory()
	setRemediationGateAiMemoryClient(fake)
	t.Cleanup(func() { setRemediationGateAiMemoryClient(nil) })

	key := "finding-x|SYSTEMCTL_RESTART|0"
	want := remediationGateState{CooldownRejections: 3, LastRejectionAt: 1719350000, Escalated: true}
	remediationGatePersist(context.Background(), key, want)

	got, ok := remediationGateLoad(context.Background(), key)
	if !ok {
		t.Fatal("expected loaded state, got not-found")
	}
	if got != want {
		t.Errorf("round-trip mismatch: got %+v want %+v", got, want)
	}
}

// TestRemediationGatePersist_UpsertNoDuplicate proves repeated persists update the
// same record rather than accumulating duplicates, and the latest state wins.
func TestRemediationGatePersist_UpsertNoDuplicate(t *testing.T) {
	fake := newFakeGateMemory()
	setRemediationGateAiMemoryClient(fake)
	t.Cleanup(func() { setRemediationGateAiMemoryClient(nil) })

	key := "finding-y|SYSTEMCTL_STOP|1"
	remediationGatePersist(context.Background(), key, remediationGateState{CooldownRejections: 1, LastRejectionAt: 100})
	remediationGatePersist(context.Background(), key, remediationGateState{CooldownRejections: 3, LastRejectionAt: 200, Escalated: true})

	if n := fake.countWithTag(remediationGateTag(key)); n != 1 {
		t.Errorf("expected exactly 1 memory for the gate key (upsert), got %d", n)
	}
	got, ok := remediationGateLoad(context.Background(), key)
	if !ok || got.CooldownRejections != 3 || !got.Escalated || got.LastRejectionAt != 200 {
		t.Errorf("load after upsert must reflect the latest state, got %+v ok=%v", got, ok)
	}
}

// TestRemediationGateLoad_MalformedIgnored proves the cautious-owl rule: a corrupt
// persisted record is ignored (returns not-found) so the doctor falls back to
// in-memory/default rather than behaving unpredictably on garbage.
func TestRemediationGateLoad_MalformedIgnored(t *testing.T) {
	fake := newFakeGateMemory()
	setRemediationGateAiMemoryClient(fake)
	t.Cleanup(func() { setRemediationGateAiMemoryClient(nil) })

	key := "finding-z|DELETE_CACHE_ARTIFACT|0"
	// Inject a record with the right tag but corrupt cooldown_rejections.
	_, _ = fake.Store(context.Background(), &ai_memorypb.StoreRqst{Memory: &ai_memorypb.Memory{
		Project: remediationGateMemoryProject,
		Tags:    []string{remediationGateTagBase, remediationGateTag(key)},
		Metadata: map[string]string{
			"cooldown_rejections":    "not-a-number",
			"last_rejection_at_unix": "123",
			"escalated":              "true",
		},
	}})
	if _, ok := remediationGateLoad(context.Background(), key); ok {
		t.Error("load must ignore a malformed record and report not-found")
	}
}

// TestRemediationGateDelete_RemovesMemory proves clear (success/operator path)
// removes the persisted escalation so a subsequent load finds nothing.
func TestRemediationGateDelete_RemovesMemory(t *testing.T) {
	fake := newFakeGateMemory()
	setRemediationGateAiMemoryClient(fake)
	t.Cleanup(func() { setRemediationGateAiMemoryClient(nil) })

	key := "finding-d|SYSTEMCTL_RESTART|2"
	remediationGatePersist(context.Background(), key, remediationGateState{CooldownRejections: 3, LastRejectionAt: 1, Escalated: true})
	if _, ok := remediationGateLoad(context.Background(), key); !ok {
		t.Fatal("precondition: state should be present before delete")
	}
	remediationGateDelete(context.Background(), key)
	if _, ok := remediationGateLoad(context.Background(), key); ok {
		t.Error("after delete, load must find nothing")
	}
	if n := fake.countWithTag(remediationGateTag(key)); n != 0 {
		t.Errorf("after delete, expected 0 memories, got %d", n)
	}
}

// TestRemediationGate_NilClientNoOp locks the graceful-degradation contract: with
// no ai-memory client wired, persist/load/delete are safe no-ops — the doctor must
// never become unavailable because its escalation store is unreachable.
func TestRemediationGate_NilClientNoOp(t *testing.T) {
	setRemediationGateAiMemoryClient(nil)
	key := "finding-n|SYSTEMCTL_RESTART|0"
	remediationGatePersist(context.Background(), key, remediationGateState{CooldownRejections: 3, Escalated: true}) // must not panic
	if _, ok := remediationGateLoad(context.Background(), key); ok {
		t.Error("nil client: load must report not-found")
	}
	remediationGateDelete(context.Background(), key) // must not panic
}
