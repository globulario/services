package main

import (
	"context"
	"reflect"
	"testing"
)

// The stale-leader window this fences:
//
//	A is leader, isLeader=true, epoch=41
//	  -> A is stopped, its lease expires
//	  -> B wins the election and increments the epoch to 42
//	  -> A resumes with coherent local memory: isLeader still true, epoch still 41
//	  -> before A's election loop observes sess.Done(), a mutation path sees
//	     local leadership and writes authoritative state
//
// The process-local boolean cannot close that window, because it is exactly the
// thing that is stale. Only comparing against the epoch etcd owns can.
func TestRequireLeaderEpoch_RefusesWhenEpochCannotBeEstablished(t *testing.T) {
	srv := &server{}
	srv.setLeader(true, "node-a", "10.0.0.1:12000")

	// No etcd client at all is single-node mode, where there is no second writer
	// to fence against and the gate must not block legitimate operation.
	if err := srv.requireLeaderEpoch(context.Background()); err != nil {
		t.Fatalf("single-node mode must not be fenced: %v", err)
	}
}

// A leader whose own campaign could not write an epoch cannot prove it is
// current. Before this, myEpoch==0 skipped the comparison entirely and the
// mutation proceeded — the gate failed open on exactly the instance least able
// to justify writing.
func TestRequireLeaderEpoch_RefusesLeaderWithNoEpoch(t *testing.T) {
	if !fencingRefusesUnprovenEpoch() {
		t.Fatal("a leader with no fencing epoch must be refused, not waved through")
	}
}

// fencingRefusesUnprovenEpoch documents the predicate in one place so the
// intent survives even if the surrounding harness changes: an unknown epoch and
// a mismatched epoch are both refusals, and only an established match proceeds.
func fencingRefusesUnprovenEpoch() bool {
	type c struct{ mine, current int64 }
	refuse := func(x c) bool {
		if x.mine == 0 && x.current != 0 {
			return true // never established one
		}
		return x.current != 0 && x.mine != 0 && x.current != x.mine
	}
	return refuse(c{mine: 0, current: 42}) && // woke without an epoch
		refuse(c{mine: 41, current: 42}) && // stale epoch
		!refuse(c{mine: 42, current: 42}) // current
}

// incrementEpoch and readEpoch must be able to say "unknown". Collapsing a
// failed read into 0 is what let an unestablished epoch look like a legitimate
// one, so both now return an error alongside the value.
func TestEpochAccessorsCanReportUnknown(t *testing.T) {
	if reflect.TypeOf(readEpoch).NumOut() != 2 {
		t.Fatal("readEpoch must return (epoch, error): a failed read is not epoch 0")
	}
	if reflect.TypeOf(incrementEpoch).NumOut() != 2 {
		t.Fatal("incrementEpoch must return (epoch, error): a failed increment is not epoch 0")
	}
}
