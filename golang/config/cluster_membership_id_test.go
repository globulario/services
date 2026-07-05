package config

import (
	"context"
	"errors"
	"testing"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type fakeMembershipGetter struct {
	val    string // "" + hasKV=false → absent
	hasKV  bool
	getErr error
}

func (f fakeMembershipGetter) Get(_ context.Context, _ string, _ ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	r := &clientv3.GetResponse{}
	if f.hasKV {
		r.Kvs = []*mvccpb.KeyValue{{Key: []byte(ClusterMembershipIDKey), Value: []byte(f.val)}}
	}
	return r, nil
}

// TestReadClusterMembershipID_FailClosed proves the reader never invents,
// derives, or defaults an identity: absence/emptiness → ErrClusterMembershipIDAbsent
// with an empty string, transport errors propagate, and a real value round-trips.
func TestReadClusterMembershipID_FailClosed(t *testing.T) {
	const uid = "9f1c2d3e-4a5b-6c7d-8e9f-0a1b2c3d4e5f"
	transport := errors.New("etcd unreachable")

	cases := []struct {
		name    string
		getter  fakeMembershipGetter
		wantID  string
		wantErr error
	}{
		{"absent → fail-closed", fakeMembershipGetter{hasKV: false}, "", ErrClusterMembershipIDAbsent},
		{"empty value → fail-closed", fakeMembershipGetter{hasKV: true, val: ""}, "", ErrClusterMembershipIDAbsent},
		{"whitespace value → fail-closed", fakeMembershipGetter{hasKV: true, val: "   "}, "", ErrClusterMembershipIDAbsent},
		{"transport error propagates", fakeMembershipGetter{getErr: transport}, "", transport},
		{"minted uuid round-trips", fakeMembershipGetter{hasKV: true, val: uid}, uid, nil},
		{"minted uuid trimmed", fakeMembershipGetter{hasKV: true, val: "  " + uid + "\n"}, uid, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id, err := readClusterMembershipID(context.Background(), c.getter)
			if id != c.wantID {
				t.Errorf("id = %q, want %q", id, c.wantID)
			}
			if c.wantErr == ErrClusterMembershipIDAbsent {
				if !errors.Is(err, ErrClusterMembershipIDAbsent) {
					t.Errorf("err = %v, want ErrClusterMembershipIDAbsent", err)
				}
			} else if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Errorf("err = %v, want %v", err, c.wantErr)
				}
			} else if err != nil {
				t.Errorf("unexpected err: %v", err)
			}
			// Fail-closed guarantee: on any error the id MUST be empty — never a
			// synthesized/derived/default identity (e.g. the domain).
			if err != nil && id != "" {
				t.Errorf("fail-closed violated: returned id %q alongside error %v", id, err)
			}
		})
	}
}
