package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// fakeEtcdCluster is a test double for etcd client operations used by etcdMemberManager.
// It tracks members in-memory and supports simulating failures.
type fakeEtcdCluster struct {
	mu         sync.Mutex
	members    []*etcdserverpb.Member
	nextID     uint64
	addErr     error // if set, MemberAdd returns this error
	removeErr  error // if set, MemberRemove returns this error
	listErr    error // if set, MemberList returns this error
}

func newFakeEtcdCluster() *fakeEtcdCluster {
	return &fakeEtcdCluster{nextID: 1}
}

func (f *fakeEtcdCluster) addMember(name string, peerURLs []string) uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.nextID
	f.nextID++
	f.members = append(f.members, &etcdserverpb.Member{
		ID:       id,
		Name:     name,
		PeerURLs: peerURLs,
	})
	return id
}

func (f *fakeEtcdCluster) listMembers() []*etcdserverpb.Member {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*etcdserverpb.Member, len(f.members))
	copy(out, f.members)
	return out
}

// fakeEtcdMemberManager wraps etcdMemberManager but overrides the etcd client
// calls with the fake cluster. This lets us test the state machine logic
// without a real etcd.
type fakeEtcdMemberManager struct {
	etcdMemberManager
	fake *fakeEtcdCluster
}

func newFakeManager(fake *fakeEtcdCluster) *fakeEtcdMemberManager {
	return &fakeEtcdMemberManager{
		etcdMemberManager: etcdMemberManager{client: &clientv3.Client{}}, // non-nil to pass guards
		fake:              fake,
	}
}

// Override the methods that touch the real etcd client.
func (fm *fakeEtcdMemberManager) existingPeerURLSet(ctx context.Context) (map[string]bool, error) {
	if fm.fake.listErr != nil {
		return nil, fm.fake.listErr
	}
	urls := make(map[string]bool)
	for _, m := range fm.fake.listMembers() {
		for _, u := range m.PeerURLs {
			urls[u] = true
		}
	}
	return urls, nil
}

func (fm *fakeEtcdMemberManager) memberAdd(ctx context.Context, peerURL string) (uint64, error) {
	if fm.fake.addErr != nil {
		return 0, fm.fake.addErr
	}
	fm.fake.mu.Lock()
	defer fm.fake.mu.Unlock()
	// Check for duplicates.
	for _, m := range fm.fake.members {
		for _, u := range m.PeerURLs {
			if u == peerURL {
				return m.ID, nil
			}
		}
	}
	id := fm.fake.nextID
	fm.fake.nextID++
	// Added but unnamed (simulates unstarted member).
	fm.fake.members = append(fm.fake.members, &etcdserverpb.Member{
		ID:       id,
		Name:     "", // unnamed until started
		PeerURLs: []string{peerURL},
	})
	return id, nil
}

func (fm *fakeEtcdMemberManager) memberRemove(ctx context.Context, memberID uint64) error {
	if fm.fake.removeErr != nil {
		return fm.fake.removeErr
	}
	fm.fake.mu.Lock()
	defer fm.fake.mu.Unlock()
	for i, m := range fm.fake.members {
		if m.ID == memberID {
			fm.fake.members = append(fm.fake.members[:i], fm.fake.members[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("member %d not found", memberID)
}

func (fm *fakeEtcdMemberManager) memberIsHealthy(ctx context.Context, peerURL string) bool {
	fm.fake.mu.Lock()
	defer fm.fake.mu.Unlock()
	for _, m := range fm.fake.members {
		if m.Name == "" {
			continue
		}
		for _, u := range m.PeerURLs {
			if u == peerURL {
				return true
			}
		}
	}
	return false
}

// startMember simulates the etcd service starting: sets the member name.
func (f *fakeEtcdCluster) startMember(peerURL, name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, m := range f.members {
		for _, u := range m.PeerURLs {
			if u == peerURL {
				m.Name = name
				return
			}
		}
	}
}

// --- Helper builders ---

func makeNode(id, hostname, ip string, profiles []string, units []unitStatusRecord) *nodeState {
	return &nodeState{
		NodeID:   id,
		Identity: storedIdentity{Hostname: hostname, Ips: []string{ip}},
		Profiles: profiles,
		Units:    units,
	}
}

func etcdUnit(state string) unitStatusRecord {
	return unitStatusRecord{Name: "globular-etcd.service", State: state}
}

// --- Tests ---

func TestNodeIsPreparedForEtcdJoin(t *testing.T) {
	emptyURLs := map[string]bool{}

	tests := []struct {
		name     string
		node     *nodeState
		existing map[string]bool
		want     bool
	}{
		{
			name: "prepared: has profile, unit, routable IP",
			node: makeNode("n1", "host1", "10.0.0.2", []string{"core"}, []unitStatusRecord{etcdUnit("inactive")}),
			existing: emptyURLs,
			want:     true,
		},
		{
			name: "not prepared: no etcd profile",
			node: makeNode("n1", "host1", "10.0.0.2", []string{"gateway"}, []unitStatusRecord{etcdUnit("inactive")}),
			existing: emptyURLs,
			want:     false,
		},
		{
			name: "not prepared: no unit file",
			node: makeNode("n1", "host1", "10.0.0.2", []string{"core"}, nil),
			existing: emptyURLs,
			want:     false,
		},
		{
			name: "not prepared: localhost IP only",
			node: makeNode("n1", "host1", "127.0.0.1", []string{"core"}, []unitStatusRecord{etcdUnit("inactive")}),
			existing: emptyURLs,
			want:     false,
		},
		{
			name: "not prepared: empty IP",
			node: makeNode("n1", "host1", "", []string{"core"}, []unitStatusRecord{etcdUnit("inactive")}),
			existing: emptyURLs,
			want:     false,
		},
		{
			name:     "not prepared: already in member list",
			node:     makeNode("n1", "host1", "10.0.0.2", []string{"core"}, []unitStatusRecord{etcdUnit("inactive")}),
			existing: map[string]bool{"https://10.0.0.2:2380": true},
			want:     false,
		},
		{
			name: "not prepared: mid-join (member_added phase)",
			node: func() *nodeState {
				n := makeNode("n1", "host1", "10.0.0.2", []string{"core"}, []unitStatusRecord{etcdUnit("inactive")})
				n.EtcdJoinPhase = EtcdJoinMemberAdded
				return n
			}(),
			existing: emptyURLs,
			want:     false,
		},
		{
			name: "not prepared: mid-join (started phase)",
			node: func() *nodeState {
				n := makeNode("n1", "host1", "10.0.0.2", []string{"core"}, []unitStatusRecord{etcdUnit("active")})
				n.EtcdJoinPhase = EtcdJoinStarted
				return n
			}(),
			existing: emptyURLs,
			want:     false,
		},
		{
			name: "prepared: failed phase allows retry",
			node: func() *nodeState {
				n := makeNode("n1", "host1", "10.0.0.2", []string{"core"}, []unitStatusRecord{etcdUnit("inactive")})
				n.EtcdJoinPhase = EtcdJoinFailed
				return n
			}(),
			existing: emptyURLs,
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nodeIsPreparedForEtcdJoin(tt.node, tt.existing)
			if got != tt.want {
				t.Errorf("nodeIsPreparedForEtcdJoin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNodeRoutableIP(t *testing.T) {
	tests := []struct {
		name string
		node *nodeState
		want string
	}{
		{"nil node", nil, ""},
		{"no IPs", &nodeState{Identity: storedIdentity{Ips: nil}}, ""},
		{"loopback only", &nodeState{Identity: storedIdentity{Ips: []string{"127.0.0.1"}}}, ""},
		{"ipv6 loopback", &nodeState{Identity: storedIdentity{Ips: []string{"::1"}}}, ""},
		{"routable", &nodeState{Identity: storedIdentity{Ips: []string{"10.0.0.5"}}}, "10.0.0.5"},
		{"loopback then routable", &nodeState{Identity: storedIdentity{Ips: []string{"127.0.0.1", "10.0.0.5"}}}, "10.0.0.5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nodeRoutableIP(tt.node)
			if got != tt.want {
				t.Errorf("nodeRoutableIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestEtcdJoin_SingleNodePlusPreparedNode tests the happy path:
// single-node cluster + new prepared node → MemberAdd + eventual start → verified.
func TestEtcdJoin_SingleNodePlusPreparedNode(t *testing.T) {
	fake := newFakeEtcdCluster()
	// Bootstrap node is already a member.
	fake.addMember("globule-ryzen", []string{"https://10.0.0.10:2380"})

	mgr := newFakeManager(fake)

	bootstrap := makeNode("n1", "globule-ryzen", "10.0.0.10", []string{"core"}, []unitStatusRecord{etcdUnit("active")})
	bootstrap.EtcdJoinPhase = EtcdJoinVerified

	newNode := makeNode("n2", "globule-dell", "10.0.0.20", []string{"core"}, []unitStatusRecord{etcdUnit("inactive")})

	nodes := []*nodeState{bootstrap, newNode}

	// Phase 1: prepared → member_added
	dirty := reconcileJoinWithFake(mgr, nodes)
	if !dirty {
		t.Fatal("expected dirty after MemberAdd")
	}
	if newNode.EtcdJoinPhase != EtcdJoinMemberAdded {
		t.Fatalf("expected member_added, got %s", newNode.EtcdJoinPhase)
	}
	if newNode.EtcdMemberID == 0 {
		t.Fatal("expected non-zero EtcdMemberID")
	}

	// Phase 2: member_added → started (simulate etcd service starting)
	newNode.Units = []unitStatusRecord{etcdUnit("active")}
	dirty = reconcileJoinWithFake(mgr, nodes)
	if !dirty {
		t.Fatal("expected dirty after etcd started")
	}
	if newNode.EtcdJoinPhase != EtcdJoinStarted {
		t.Fatalf("expected started, got %s", newNode.EtcdJoinPhase)
	}

	// Phase 3: started → verified (simulate member appearing with name)
	fake.startMember("https://10.0.0.20:2380", "globule-dell")
	dirty = reconcileJoinWithFake(mgr, nodes)
	if !dirty {
		t.Fatal("expected dirty after verification")
	}
	if newNode.EtcdJoinPhase != EtcdJoinVerified {
		t.Fatalf("expected verified, got %s", newNode.EtcdJoinPhase)
	}
	if newNode.EtcdJoinError != "" {
		t.Fatalf("expected no error, got %q", newNode.EtcdJoinError)
	}
}

// TestEtcdJoin_TimeoutRollback tests: MemberAdd + start failure → rollback removes member.
func TestEtcdJoin_TimeoutRollback(t *testing.T) {
	fake := newFakeEtcdCluster()
	fake.addMember("globule-ryzen", []string{"https://10.0.0.10:2380"})

	mgr := newFakeManager(fake)

	bootstrap := makeNode("n1", "globule-ryzen", "10.0.0.10", []string{"core"}, []unitStatusRecord{etcdUnit("active")})
	bootstrap.EtcdJoinPhase = EtcdJoinVerified

	newNode := makeNode("n2", "globule-dell", "10.0.0.20", []string{"core"}, []unitStatusRecord{etcdUnit("inactive")})

	nodes := []*nodeState{bootstrap, newNode}

	// MemberAdd succeeds.
	reconcileJoinWithFake(mgr, nodes)
	if newNode.EtcdJoinPhase != EtcdJoinMemberAdded {
		t.Fatalf("expected member_added, got %s", newNode.EtcdJoinPhase)
	}
	memberID := newNode.EtcdMemberID

	// Verify member was added to fake cluster.
	members := fake.listMembers()
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}

	// Simulate timeout: set EtcdJoinStartedAt far in the past.
	newNode.EtcdJoinStartedAt = time.Now().Add(-etcdJoinTimeout - time.Minute)

	dirty := reconcileJoinWithFake(mgr, nodes)
	if !dirty {
		t.Fatal("expected dirty after timeout rollback")
	}
	if newNode.EtcdJoinPhase != EtcdJoinFailed {
		t.Fatalf("expected failed, got %s", newNode.EtcdJoinPhase)
	}
	if newNode.EtcdJoinError == "" {
		t.Fatal("expected error message after rollback")
	}
	if !strings.Contains(newNode.EtcdJoinError, "timeout") {
		t.Fatalf("expected timeout in error, got %q", newNode.EtcdJoinError)
	}

	// Verify member was removed from fake cluster.
	members = fake.listMembers()
	found := false
	for _, m := range members {
		if m.ID == memberID {
			found = true
		}
	}
	if found {
		t.Fatal("expected rolled-back member to be removed from cluster")
	}
}

// TestEtcdJoin_AlreadyMember tests: node already in member list → idempotent no-op (verified).
func TestEtcdJoin_AlreadyMember(t *testing.T) {
	fake := newFakeEtcdCluster()
	fake.addMember("globule-ryzen", []string{"https://10.0.0.10:2380"})

	mgr := newFakeManager(fake)

	// Node is already in the member list but phase is none.
	node := makeNode("n1", "globule-ryzen", "10.0.0.10", []string{"core"}, []unitStatusRecord{etcdUnit("active")})

	nodes := []*nodeState{node}
	dirty := reconcileJoinWithFake(mgr, nodes)
	if !dirty {
		t.Fatal("expected dirty when setting verified")
	}
	if node.EtcdJoinPhase != EtcdJoinVerified {
		t.Fatalf("expected verified for existing member, got %s", node.EtcdJoinPhase)
	}
}

// TestEtcdJoin_PackageInstalledUnitInactive_AllowedWhenPrepared tests that
// a node with the package installed but unit inactive is allowed into the
// join flow (not blocked forever like the old active-state gate).
func TestEtcdJoin_PackageInstalledUnitInactive_AllowedWhenPrepared(t *testing.T) {
	fake := newFakeEtcdCluster()
	fake.addMember("bootstrap", []string{"https://10.0.0.10:2380"})

	mgr := newFakeManager(fake)

	bootstrap := makeNode("n1", "bootstrap", "10.0.0.10", []string{"core"}, []unitStatusRecord{etcdUnit("active")})
	bootstrap.EtcdJoinPhase = EtcdJoinVerified

	// Unit exists but is inactive — this was previously blocked.
	newNode := makeNode("n2", "new-node", "10.0.0.20", []string{"compute"}, []unitStatusRecord{etcdUnit("inactive")})

	nodes := []*nodeState{bootstrap, newNode}
	reconcileJoinWithFake(mgr, nodes)

	if newNode.EtcdJoinPhase != EtcdJoinMemberAdded {
		t.Fatalf("expected member_added (unit inactive should be allowed), got %s", newNode.EtcdJoinPhase)
	}
}

// TestEtcdJoin_LocalhostPeerURL_NeverEmitted tests that localhost/loopback
// IPs are never used as peer URLs.
func TestEtcdJoin_LocalhostPeerURL_NeverEmitted(t *testing.T) {
	fake := newFakeEtcdCluster()
	fake.addMember("bootstrap", []string{"https://10.0.0.10:2380"})

	mgr := newFakeManager(fake)

	bootstrap := makeNode("n1", "bootstrap", "10.0.0.10", []string{"core"}, []unitStatusRecord{etcdUnit("active")})
	bootstrap.EtcdJoinPhase = EtcdJoinVerified

	// Node with only loopback IP — should NOT be added.
	loopbackNode := makeNode("n2", "loopback", "127.0.0.1", []string{"core"}, []unitStatusRecord{etcdUnit("inactive")})

	nodes := []*nodeState{bootstrap, loopbackNode}
	reconcileJoinWithFake(mgr, nodes)

	if loopbackNode.EtcdJoinPhase != EtcdJoinNone {
		t.Fatalf("expected none (loopback should be rejected), got %s", loopbackNode.EtcdJoinPhase)
	}

	// Verify no member was added with localhost URL.
	for _, m := range fake.listMembers() {
		for _, u := range m.PeerURLs {
			if strings.Contains(u, "127.0.0.1") || strings.Contains(u, "localhost") {
				t.Fatalf("localhost peer URL emitted: %s", u)
			}
		}
	}
}

// TestEtcdJoin_ControllerRestartMidJoin tests that a controller restart
// mid-join (node is in member_added phase) resumes safely.
func TestEtcdJoin_ControllerRestartMidJoin(t *testing.T) {
	fake := newFakeEtcdCluster()
	fake.addMember("bootstrap", []string{"https://10.0.0.10:2380"})
	// Simulate: MemberAdd was already called before restart (unnamed member in cluster).
	memberID := fake.addMember("", []string{"https://10.0.0.20:2380"})
	// Set name to "" to simulate unstarted.
	fake.mu.Lock()
	for _, m := range fake.members {
		if m.ID == memberID {
			m.Name = ""
		}
	}
	fake.mu.Unlock()

	mgr := newFakeManager(fake)

	bootstrap := makeNode("n1", "bootstrap", "10.0.0.10", []string{"core"}, []unitStatusRecord{etcdUnit("active")})
	bootstrap.EtcdJoinPhase = EtcdJoinVerified

	// Simulate persisted state: node was in member_added phase before restart.
	newNode := makeNode("n2", "new-node", "10.0.0.20", []string{"core"}, []unitStatusRecord{etcdUnit("inactive")})
	newNode.EtcdJoinPhase = EtcdJoinMemberAdded
	newNode.EtcdJoinStartedAt = time.Now().Add(-30 * time.Second) // recent, not timed out
	newNode.EtcdMemberID = memberID

	nodes := []*nodeState{bootstrap, newNode}

	// Should stay in member_added (etcd not yet running), no re-add attempted.
	dirty := reconcileJoinWithFake(mgr, nodes)
	if dirty {
		t.Fatal("expected no state change — node is waiting for etcd start")
	}
	if newNode.EtcdJoinPhase != EtcdJoinMemberAdded {
		t.Fatalf("expected member_added (waiting for start), got %s", newNode.EtcdJoinPhase)
	}

	// Now simulate etcd starting.
	newNode.Units = []unitStatusRecord{etcdUnit("active")}
	dirty = reconcileJoinWithFake(mgr, nodes)
	if !dirty {
		t.Fatal("expected dirty after etcd started")
	}
	if newNode.EtcdJoinPhase != EtcdJoinStarted {
		t.Fatalf("expected started, got %s", newNode.EtcdJoinPhase)
	}
}

// TestEtcdJoin_NoConcurrentJoins tests that only one node joins at a time.
func TestEtcdJoin_NoConcurrentJoins(t *testing.T) {
	fake := newFakeEtcdCluster()
	fake.addMember("bootstrap", []string{"https://10.0.0.10:2380"})

	mgr := newFakeManager(fake)

	bootstrap := makeNode("n1", "bootstrap", "10.0.0.10", []string{"core"}, []unitStatusRecord{etcdUnit("active")})
	bootstrap.EtcdJoinPhase = EtcdJoinVerified

	node2 := makeNode("n2", "node2", "10.0.0.20", []string{"core"}, []unitStatusRecord{etcdUnit("inactive")})
	node3 := makeNode("n3", "node3", "10.0.0.30", []string{"core"}, []unitStatusRecord{etcdUnit("inactive")})

	nodes := []*nodeState{bootstrap, node2, node3}

	// First reconcile: one node should start joining.
	reconcileJoinWithFake(mgr, nodes)

	joinCount := 0
	for _, n := range []*nodeState{node2, node3} {
		if n.EtcdJoinPhase == EtcdJoinMemberAdded {
			joinCount++
		}
	}
	if joinCount != 1 {
		t.Fatalf("expected exactly 1 node joining at a time, got %d", joinCount)
	}
}

// reconcileJoinWithFake drives the state machine using the fake manager's overridden methods.
// Since we can't use polymorphism with the struct methods, we duplicate the core logic
// using the fake's methods directly.
func reconcileJoinWithFake(fm *fakeEtcdMemberManager, nodes []*nodeState) bool {
	ctx := context.Background()

	existingURLs, err := fm.existingPeerURLSet(ctx)
	if err != nil {
		return false
	}

	now := time.Now()
	dirty := false

	for _, node := range nodes {
		if node == nil {
			continue
		}
		if !nodeHasProfile(&memberNode{Profiles: node.Profiles}, profilesForEtcd) {
			continue
		}

		switch node.EtcdJoinPhase {
		case EtcdJoinNone, EtcdJoinFailed:
			// Check if already a member (before prepared check, which rejects existing members).
			ip := nodeRoutableIP(node)
			if ip != "" {
				peerURL := fmt.Sprintf("https://%s:2380", ip)
				if existingURLs[peerURL] {
					node.EtcdJoinPhase = EtcdJoinVerified
					node.EtcdJoinError = ""
					dirty = true
					continue
				}
			}
			if !nodeIsPreparedForEtcdJoin(node, existingURLs) {
				continue
			}
			if fm.etcdMemberManager.anyNodeMidJoin(nodes, node.NodeID) {
				continue
			}
			peerURL := fmt.Sprintf("https://%s:2380", ip)
			memberID, err := fm.memberAdd(ctx, peerURL)
			if err != nil {
				node.EtcdJoinPhase = EtcdJoinFailed
				node.EtcdJoinError = err.Error()
				dirty = true
				continue
			}
			node.EtcdJoinPhase = EtcdJoinMemberAdded
			node.EtcdJoinStartedAt = now
			node.EtcdJoinError = ""
			node.EtcdMemberID = memberID
			dirty = true

		case EtcdJoinMemberAdded:
			if nodeHasEtcdRunning(node) {
				node.EtcdJoinPhase = EtcdJoinStarted
				dirty = true
				continue
			}
			if now.Sub(node.EtcdJoinStartedAt) > etcdJoinTimeout {
				if node.EtcdMemberID != 0 {
					if err := fm.memberRemove(ctx, node.EtcdMemberID); err == nil {
						node.EtcdJoinError = "timeout waiting for etcd service to start"
					} else {
						node.EtcdJoinError = fmt.Sprintf("timeout; rollback failed: %v", err)
					}
				} else {
					node.EtcdJoinError = "timeout waiting for etcd service to start"
				}
				node.EtcdJoinPhase = EtcdJoinFailed
				node.EtcdMemberID = 0
				dirty = true
			}

		case EtcdJoinStarted:
			ip := nodeRoutableIP(node)
			peerURL := fmt.Sprintf("https://%s:2380", ip)
			if fm.memberIsHealthy(ctx, peerURL) {
				node.EtcdJoinPhase = EtcdJoinVerified
				node.EtcdJoinError = ""
				node.EtcdMemberID = 0
				dirty = true
				continue
			}
			if now.Sub(node.EtcdJoinStartedAt) > etcdJoinTimeout {
				if node.EtcdMemberID != 0 {
					if err := fm.memberRemove(ctx, node.EtcdMemberID); err == nil {
						node.EtcdJoinError = "timeout waiting for etcd member to become healthy"
					} else {
						node.EtcdJoinError = fmt.Sprintf("timeout; rollback failed: %v", err)
					}
				} else {
					node.EtcdJoinError = "timeout waiting for etcd member to become healthy"
				}
				node.EtcdJoinPhase = EtcdJoinFailed
				node.EtcdMemberID = 0
				dirty = true
			}

		case EtcdJoinVerified:
			// no-op
		}
	}
	return dirty
}
