package main

import (
	"strings"
	"testing"
)

const etcdDay0Yaml = `# Single-node embedded etcd defaults for Globular
name: "globular-etcd"
data-dir: "/var/lib/globular/etcd"
listen-client-urls: "https://10.10.0.11:2379"
initial-cluster: "globular-etcd=https://10.10.0.11:2380"
initial-cluster-state: "new"
initial-cluster-token: "globular-etcd-cluster"
client-transport-security:
  cert-file: "/var/lib/globular/pki/issued/services/service.crt"
`

// The founding-node case: a self-only initial-cluster with state "new" is the
// configuration that would silently bootstrap a SECOND cluster on a wiped
// restart. Both fields must converge.
func TestEtcdInitialCluster_FoundingNodeConvergesAndPromotesState(t *testing.T) {
	ring := "globular-etcd=https://10.10.0.11:2380,node-2=https://10.10.0.12:2380"
	out, changed := replaceCapturedValue([]byte(etcdDay0Yaml), etcdInitialClusterLineRe, ring, false)
	if !changed {
		t.Fatal("initial-cluster did not change")
	}
	out2, changedState := replaceCapturedValue(out, etcdClusterStateLineRe, "existing", true)
	if !changedState {
		t.Fatal("initial-cluster-state was not promoted")
	}
	got := string(out2)
	if !strings.Contains(got, "initial-cluster: "+ring+"\n") {
		t.Errorf("initial-cluster not rewritten:\n%s", got)
	}
	if !strings.Contains(got, `initial-cluster-state: "existing"`) {
		t.Errorf("state not promoted to existing:\n%s", got)
	}
	// initial-cluster-token must survive: initial-cluster is a PREFIX of it, so a
	// careless regexp would clobber the token line and break rejoin.
	if !strings.Contains(got, `initial-cluster-token: "globular-etcd-cluster"`) {
		t.Errorf("initial-cluster-token was clobbered:\n%s", got)
	}
	for _, must := range []string{
		`name: "globular-etcd"`,
		`data-dir: "/var/lib/globular/etcd"`,
		`listen-client-urls: "https://10.10.0.11:2379"`,
		"  cert-file: \"/var/lib/globular/pki/issued/services/service.crt\"",
	} {
		if !strings.Contains(got, must) {
			t.Errorf("unrelated line disturbed, missing %q", must)
		}
	}
}

// An already-correct file must report no change, so the reconciler writes
// nothing and etcd.yaml's mtime stays stable across heartbeats.
func TestEtcdInitialCluster_AlreadyCorrectIsNoOp(t *testing.T) {
	ring := "globular-etcd=https://10.10.0.11:2380"
	in := []byte("initial-cluster: \"" + ring + "\"\n")
	if _, changed := replaceCapturedValue(in, etcdInitialClusterLineRe, ring, false); changed {
		t.Error("identical value reported as changed")
	}
	st := []byte(`initial-cluster-state: "existing"` + "\n")
	if _, changed := replaceCapturedValue(st, etcdClusterStateLineRe, "existing", true); changed {
		t.Error("identical state reported as changed")
	}
}

// The token line must never be matched by the initial-cluster regexp.
func TestEtcdInitialCluster_TokenLineIsNotMatched(t *testing.T) {
	only := []byte("initial-cluster-token: \"globular-etcd-cluster\"\n")
	if loc := etcdInitialClusterLineRe.FindSubmatchIndex(only); loc != nil {
		t.Errorf("initial-cluster regexp matched the token line: %v", loc)
	}
	if loc := etcdClusterStateLineRe.FindSubmatchIndex(only); loc != nil {
		t.Errorf("state regexp matched the token line: %v", loc)
	}
}

// A file with no initial-cluster key is left alone rather than having one
// invented at an arbitrary position.
func TestEtcdInitialCluster_MissingKeyLeavesFileUntouched(t *testing.T) {
	in := []byte("name: node-9\n")
	out, changed := replaceCapturedValue(in, etcdInitialClusterLineRe, "x=y", false)
	if changed || string(out) != string(in) {
		t.Errorf("file was modified despite no initial-cluster key: %q", out)
	}
}
