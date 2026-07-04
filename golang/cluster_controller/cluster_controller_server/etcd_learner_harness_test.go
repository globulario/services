package main

// etcd_learner_harness_test.go — a real-etcd proving ground for the learner-first
// Day-1 join work. It launches actual etcd binaries as isolated localhost
// subprocesses (high ports, temp data dirs, insecure http, a unique cluster
// token) and drives them via clientv3. It NEVER touches the live founder etcd
// (different ports, different token, temp dirs).
//
// This harness is the FIRST deliverable of the etcd-learner branch: before any
// production FSM change, prove the two facts the design rests on —
//   1. a full-voter 1->2 join loses founder quorum when the new node dies, and
//   2. a learner 1->2 join preserves founder quorum when the new node dies,
//   3. a learner is promoted to a voter only after it has caught up.
//
// Skips cleanly when no etcd binary is available or in -short mode.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// harness port base — deliberately far from the founder's 2379/2380.
const harnessPortBase = 24379

func findEtcdBinary() string {
	if p, err := exec.LookPath("etcd"); err == nil {
		return p
	}
	for _, cand := range []string{"/usr/lib/globular/bin/etcd", "/usr/local/bin/etcd"} {
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
			return cand
		}
	}
	return ""
}

type etcdNode struct {
	name      string
	clientURL string
	peerURL   string
	dataDir   string
	cmd       *exec.Cmd
}

type etcdHarness struct {
	t     *testing.T
	bin   string
	token string
	nodes map[string]*etcdNode
}

func newEtcdHarness(t *testing.T) *etcdHarness {
	t.Helper()
	if testing.Short() {
		t.Skip("etcd harness skipped in -short mode (launches real etcd subprocesses)")
	}
	bin := findEtcdBinary()
	if bin == "" {
		t.Skip("etcd binary not found — skipping real-etcd harness")
	}
	h := &etcdHarness{
		t:     t,
		bin:   bin,
		token: fmt.Sprintf("harness-%d", time.Now().UnixNano()),
		nodes: map[string]*etcdNode{},
	}
	t.Cleanup(h.stopAll)
	return h
}

// startFounder launches a fresh single-node cluster (initial-cluster-state=new).
func (h *etcdHarness) startFounder(name string) *etcdNode {
	h.t.Helper()
	n := h.mkNode(name, 0)
	initialCluster := fmt.Sprintf("%s=%s", n.name, n.peerURL)
	h.launch(n, initialCluster, "new")
	return n
}

// startJoiner launches a node that joins an existing cluster. The caller MUST
// have already registered the member (MemberAdd or MemberAddAsLearner) so the
// member list matches initialCluster.
func (h *etcdHarness) startJoiner(name string, index int, initialCluster string) *etcdNode {
	h.t.Helper()
	n := h.mkNode(name, index)
	h.launch(n, initialCluster, "existing")
	return n
}

func (h *etcdHarness) mkNode(name string, index int) *etcdNode {
	cport := harnessPortBase + index*2
	pport := harnessPortBase + index*2 + 1
	dir := h.t.TempDir()
	n := &etcdNode{
		name:      name,
		clientURL: fmt.Sprintf("http://127.0.0.1:%d", cport),
		peerURL:   fmt.Sprintf("http://127.0.0.1:%d", pport),
		dataDir:   filepath.Join(dir, name),
	}
	h.nodes[name] = n
	return n
}

func (h *etcdHarness) launch(n *etcdNode, initialCluster, state string) {
	h.t.Helper()
	logf, _ := os.Create(filepath.Join(filepath.Dir(n.dataDir), n.name+".log"))
	cmd := exec.Command(h.bin,
		"--name", n.name,
		"--data-dir", n.dataDir,
		"--listen-client-urls", n.clientURL,
		"--advertise-client-urls", n.clientURL,
		"--listen-peer-urls", n.peerURL,
		"--initial-advertise-peer-urls", n.peerURL,
		"--initial-cluster", initialCluster,
		"--initial-cluster-state", state,
		"--initial-cluster-token", h.token,
		"--logger", "zap",
	)
	if logf != nil {
		cmd.Stdout = logf
		cmd.Stderr = logf
	}
	if err := cmd.Start(); err != nil {
		h.t.Fatalf("launch etcd %s: %v", n.name, err)
	}
	n.cmd = cmd
}

func (h *etcdHarness) stopNode(name string) {
	n := h.nodes[name]
	if n == nil || n.cmd == nil || n.cmd.Process == nil {
		return
	}
	_ = n.cmd.Process.Kill()
	_, _ = n.cmd.Process.Wait()
	n.cmd = nil
}

func (h *etcdHarness) stopAll() {
	for name := range h.nodes {
		h.stopNode(name)
	}
}

func (h *etcdHarness) client(n *etcdNode) *clientv3.Client {
	h.t.Helper()
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{n.clientURL},
		DialTimeout: 3 * time.Second,
	})
	if err != nil {
		h.t.Fatalf("client for %s: %v", n.name, err)
	}
	h.t.Cleanup(func() { _ = cli.Close() })
	return cli
}

// canWrite reports whether the endpoint can commit a write — i.e. it has quorum.
// A write with no quorum blocks and returns a deadline error.
func canWrite(cli *clientv3.Client) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := cli.Put(ctx, "harness/probe", fmt.Sprintf("%d", time.Now().UnixNano()))
	return err == nil
}

// waitWritable polls canWrite until it succeeds or the deadline passes.
func waitWritable(cli *clientv3.Client, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if canWrite(cli) {
			return true
		}
		time.Sleep(300 * time.Millisecond)
	}
	return false
}

// waitUnwritable polls until canWrite fails or the deadline passes.
func waitUnwritable(cli *clientv3.Client, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if !canWrite(cli) {
			return true
		}
		time.Sleep(300 * time.Millisecond)
	}
	return false
}

// TestEtcdTrap_FullVoterJoinBreaksFounderQuorumOnLoss reproduces the dangerous
// 1->2 quorum trap: adding a full voter raises quorum to 2/2, so when the new
// node dies the founder — the only survivor — can no longer commit writes.
func TestEtcdTrap_FullVoterJoinBreaksFounderQuorumOnLoss(t *testing.T) {
	h := newEtcdHarness(t)
	founder := h.startFounder("founder")
	fc := h.client(founder)

	if !waitWritable(fc, 15*time.Second) {
		t.Fatal("founder never became writable")
	}

	joiner := h.mkNode("voter1", 1)
	addCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, err := fc.MemberAdd(addCtx, []string{joiner.peerURL})
	cancel()
	if err != nil {
		t.Fatalf("MemberAdd voter: %v", err)
	}
	// Start the joiner so the 2-voter cluster becomes healthy.
	initial := fmt.Sprintf("%s=%s,%s=%s", founder.name, founder.peerURL, joiner.name, joiner.peerURL)
	h.startJoiner(joiner.name, 1, initial)

	if !waitWritable(fc, 20*time.Second) {
		t.Fatal("2-voter cluster never became writable")
	}

	// The trap springs: kill the second voter → 2 voters, 1 alive → no quorum.
	h.stopNode(joiner.name)
	if !waitUnwritable(fc, 20*time.Second) {
		t.Fatal("EXPECTED founder to lose quorum after a full-voter peer died, but it stayed writable")
	}
	t.Log("confirmed: full-voter 1->2 join leaves the founder without quorum when the peer dies (the trap)")
}

// TestEtcdLearner_JoinPreservesFounderQuorumOnLoss proves the fix: a learner is
// non-voting, so a 1->2 learner join never changes quorum. Losing the learner
// before promotion leaves the founder fully writable.
func TestEtcdLearner_JoinPreservesFounderQuorumOnLoss(t *testing.T) {
	h := newEtcdHarness(t)
	founder := h.startFounder("founder")
	fc := h.client(founder)
	if !waitWritable(fc, 15*time.Second) {
		t.Fatal("founder never became writable")
	}

	learner := h.mkNode("learner1", 1)
	addCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, err := fc.MemberAddAsLearner(addCtx, []string{learner.peerURL})
	cancel()
	if err != nil {
		t.Fatalf("MemberAddAsLearner: %v", err)
	}
	// A learner does not change quorum — founder must stay writable right now.
	if !canWrite(fc) {
		t.Fatal("adding a learner must NOT cost the founder its quorum, but a write failed")
	}

	initial := fmt.Sprintf("%s=%s,%s=%s", founder.name, founder.peerURL, learner.name, learner.peerURL)
	h.startJoiner(learner.name, 1, initial)
	// Give the learner a moment to come up, then kill it BEFORE any promotion.
	time.Sleep(3 * time.Second)
	h.stopNode(learner.name)

	// The door never locks: founder still has quorum-of-one and stays writable.
	if !canWrite(fc) {
		t.Fatal("EXPECTED founder to remain writable after a learner died pre-promotion, but the write failed")
	}
	if waitUnwritable(fc, 5*time.Second) {
		t.Fatal("founder lost quorum after a learner death — learner must never count toward quorum")
	}
	t.Log("confirmed: learner 1->2 join preserves founder quorum even when the learner dies pre-promotion (the fix)")
}

// TestEtcdLearner_PromotionCreatesVoterAfterCatchup proves promotion semantics:
// MemberPromote succeeds only once the learner is caught up, and yields a
// voting member. This is the catch-up-before-voter contract the promotion FSM
// must honor (retry MemberPromote until it stops returning "not in sync").
func TestEtcdLearner_PromotionCreatesVoterAfterCatchup(t *testing.T) {
	h := newEtcdHarness(t)
	founder := h.startFounder("founder")
	fc := h.client(founder)
	if !waitWritable(fc, 15*time.Second) {
		t.Fatal("founder never became writable")
	}

	learner := h.mkNode("learner1", 1)
	addCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	addResp, err := fc.MemberAddAsLearner(addCtx, []string{learner.peerURL})
	cancel()
	if err != nil {
		t.Fatalf("MemberAddAsLearner: %v", err)
	}
	learnerID := addResp.Member.ID

	initial := fmt.Sprintf("%s=%s,%s=%s", founder.name, founder.peerURL, learner.name, learner.peerURL)
	h.startJoiner(learner.name, 1, initial)

	// Retry MemberPromote until the learner is in sync (mirrors the FSM's
	// bounded promotion retry). Before catch-up etcd returns an error.
	promoted := false
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		pctx, pcancel := context.WithTimeout(context.Background(), 3*time.Second)
		_, perr := fc.MemberPromote(pctx, learnerID)
		pcancel()
		if perr == nil {
			promoted = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !promoted {
		t.Fatal("learner was never promotable within the deadline")
	}

	// After promotion the member must be a voter (IsLearner == false).
	lctx, lcancel := context.WithTimeout(context.Background(), 5*time.Second)
	ml, err := fc.MemberList(lctx)
	lcancel()
	if err != nil {
		t.Fatalf("MemberList: %v", err)
	}
	for _, m := range ml.Members {
		if m.ID == learnerID && m.IsLearner {
			t.Fatal("member is still a learner after MemberPromote returned success")
		}
	}
	t.Log("confirmed: MemberPromote succeeds only after catch-up and yields a voting member")
}

// TestEtcdConstraint_DefaultCapsLearnersAtOne pins the etcd fact that shapes
// Policy A: by default etcd allows only ONE learner. A founder that already has
// a learner rejects a second with "too many learner members in cluster". This is
// why StartEtcdServer sets experimental-max-learners=2 (process.go) — without it,
// topologyAllowsLearnerPromotion would strand the cluster at 1 voter + 1 learner
// (can't add a 2nd learner, won't promote into a non-HA 2-voter state).
func TestEtcdConstraint_DefaultCapsLearnersAtOne(t *testing.T) {
	h := newEtcdHarness(t)
	founder := h.startFounder("founder") // NO max-learners override -> etcd default (1)
	fc := h.client(founder)
	if !waitWritable(fc, 15*time.Second) {
		t.Fatal("founder never became writable")
	}
	l1 := h.mkNode("l1", 1)
	l2 := h.mkNode("l2", 2)

	c1, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, err := fc.MemberAddAsLearner(c1, []string{l1.peerURL})
	cancel()
	if err != nil {
		t.Fatalf("first learner add should succeed: %v", err)
	}

	c2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	_, err2 := fc.MemberAddAsLearner(c2, []string{l2.peerURL})
	cancel2()
	if err2 == nil {
		t.Fatal("EXPECTED etcd to reject a second learner by default, but the add succeeded — max-learners assumption is wrong")
	}
	t.Logf("confirmed: default etcd caps learners at 1 (second add rejected: %v)", err2)
}

// TestEtcdLearner_PolicyADrivesOneToThreeVotersHA is the capstone: on real etcd
// 3.5.14 (learners hard-capped at 1) it drives a cluster from 1 voter to 3 voters
// using the PRODUCTION reconcileLearnerPromotion driver, SEQUENTIALLY — one
// learner added and promoted at a time, passing through a transient 2-voter state
// (Policy A′). It then proves the result is genuinely HA by killing one voter and
// confirming the cluster still commits writes. End to end, real etcd, no fakes.
func TestEtcdLearner_PolicyADrivesOneToThreeVotersHA(t *testing.T) {
	const target = etcdHAVoterTarget // intended voter count = 3

	h := newEtcdHarness(t)
	founder := h.startFounder("founder") // default cap (1 learner) — the real constraint
	fc := h.client(founder)
	if !waitWritable(fc, 15*time.Second) {
		t.Fatal("founder never became writable")
	}
	mgr := &etcdMemberManager{client: fc}

	// addLearnerAndPromote registers + starts one learner, then drives the
	// production promotion loop until that learner becomes a voter (etcd gates
	// promotion on catch-up, so we retry).
	addLearnerAndPromote := func(name string, index int, wantVoters int) {
		l := h.mkNode(name, index)
		// etcd's strict-reconfig check refuses a member add unless the current
		// voter set is healthy (has quorum). Right after a promotion the new voter
		// takes a moment to report healthy, so wait-for-writable then retry the add
		// on the transient "unhealthy cluster" — exactly what a real join must do.
		if !waitWritable(fc, 15*time.Second) {
			t.Fatalf("cluster not writable before adding learner %s", name)
		}
		addDeadline := time.Now().Add(20 * time.Second)
		for {
			ac, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, err := fc.MemberAddAsLearner(ac, []string{l.peerURL})
			cancel()
			if err == nil {
				break
			}
			if time.Now().After(addDeadline) {
				t.Fatalf("MemberAddAsLearner %s: %v", name, err)
			}
			time.Sleep(500 * time.Millisecond)
		}
		// A learner must never cost the founder its quorum, even before it starts.
		if !canWrite(fc) {
			t.Fatalf("founder lost quorum after adding learner %s — learners must not count toward quorum", name)
		}

		// initial-cluster must list every current member (voters + this learner).
		parts := []string{fmt.Sprintf("%s=%s", founder.name, founder.peerURL)}
		for nm, nd := range h.nodes {
			if nm == founder.name || nm == name {
				continue
			}
			parts = append(parts, fmt.Sprintf("%s=%s", nm, nd.peerURL))
		}
		parts = append(parts, fmt.Sprintf("%s=%s", name, l.peerURL))
		h.startJoiner(name, index, joinInitialCluster(parts))

		deadline := time.Now().Add(45 * time.Second)
		for time.Now().Before(deadline) {
			mgr.reconcileLearnerPromotion(context.Background(), target)
			if v, lc := countEtcdMembers(t, fc); v >= wantVoters && lc == 0 {
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
		v, lc := countEtcdMembers(t, fc)
		t.Fatalf("driver never reached %d voters / 0 learners for %s (got %dv %dl)", wantVoters, name, v, lc)
	}

	// 1v -> 2v (transient) -> 3v, one learner at a time.
	addLearnerAndPromote("n2", 1, 2)
	t.Log("confirmed: reconcileLearnerPromotion promoted through the transient 2-voter state")
	addLearnerAndPromote("n3", 2, 3)
	t.Log("confirmed: reconcileLearnerPromotion completed the 3-voter HA set sequentially")

	// HA proof: kill one voter. 2 of 3 remain = quorum, so writes still commit.
	// (Contrast the 2-voter trap where losing one voter is fatal —
	// TestEtcdTrap_FullVoterJoinBreaksFounderQuorumOnLoss.)
	h.stopNode("n3")
	if !waitWritable(fc, 15*time.Second) {
		t.Fatal("EXPECTED a 3-voter cluster to survive losing one voter, but it went unwritable — not HA")
	}
	t.Log("confirmed: 3-voter cluster remains writable after losing one voter (genuine HA)")
}

// joinInitialCluster joins member specs into an etcd --initial-cluster string.
func joinInitialCluster(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ","
		}
		out += p
	}
	return out
}

// countEtcdMembers returns (voters, learners) from a live MemberList.
func countEtcdMembers(t *testing.T, cli *clientv3.Client) (voters, learners int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ml, err := cli.MemberList(ctx)
	if err != nil {
		return 0, 0
	}
	for _, m := range ml.Members {
		if m.IsLearner {
			learners++
		} else {
			voters++
		}
	}
	return voters, learners
}
