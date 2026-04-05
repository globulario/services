package main

import (
	"testing"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

func TestHardBlocklist(t *testing.T) {
	// ETCD_PUT, ETCD_DELETE, NODE_REMOVE must ALWAYS be blocked regardless
	// of risk tag. That's the projection-clauses.md Clause 8 invariant.
	cases := []struct {
		name       string
		action     cluster_doctorpb.ActionType
		risk       cluster_doctorpb.ActionRisk
		wantBlock  bool
	}{
		{"etcd_put LOW tagged still blocked", cluster_doctorpb.ActionType_ETCD_PUT, cluster_doctorpb.ActionRisk_RISK_LOW, true},
		{"etcd_put HIGH tagged still blocked", cluster_doctorpb.ActionType_ETCD_PUT, cluster_doctorpb.ActionRisk_RISK_HIGH, true},
		{"etcd_delete blocked", cluster_doctorpb.ActionType_ETCD_DELETE, cluster_doctorpb.ActionRisk_RISK_LOW, true},
		{"node_remove blocked", cluster_doctorpb.ActionType_NODE_REMOVE, cluster_doctorpb.ActionRisk_RISK_HIGH, true},
		{"systemctl_restart not blocked", cluster_doctorpb.ActionType_SYSTEMCTL_RESTART, cluster_doctorpb.ActionRisk_RISK_LOW, false},
		{"file_delete not blocked at layer 1", cluster_doctorpb.ActionType_FILE_DELETE, cluster_doctorpb.ActionRisk_RISK_LOW, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := &cluster_doctorpb.RemediationAction{ActionType: tc.action, Risk: tc.risk}
			got, _ := hardBlocked(a)
			if got != tc.wantBlock {
				t.Fatalf("hardBlocked(%s,%s) = %v, want %v", tc.action, tc.risk, got, tc.wantBlock)
			}
		})
	}
}

func TestSafeTrashAllowlist(t *testing.T) {
	cases := []struct {
		path string
		safe bool
	}{
		{"/usr/lib/globular/bin/cluster_controller_server.tmp", true},
		{"/usr/lib/globular/bin/file_server.bak", true},
		{"/usr/lib/globular/bin/cluster_controller_server", false},  // no suffix
		{"/usr/lib/globular/data/stuff.tmp", false},                  // wrong prefix
		{"/etc/globular/config.tmp", false},                          // wrong prefix
		{"/usr/lib/globular/bin/", false},                            // empty filename
		{"/tmp/whatever.tmp", false},                                 // wrong prefix
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := isSafeTrashPath(tc.path); got != tc.safe {
				t.Fatalf("isSafeTrashPath(%q) = %v, want %v", tc.path, got, tc.safe)
			}
		})
	}
}

func TestRequiresApproval(t *testing.T) {
	cases := []struct {
		name        string
		action      cluster_doctorpb.ActionType
		risk        cluster_doctorpb.ActionRisk
		params      map[string]string
		wantApprove bool
	}{
		{
			name:        "systemctl_restart on globular-* unit, LOW → no approval",
			action:      cluster_doctorpb.ActionType_SYSTEMCTL_RESTART,
			risk:        cluster_doctorpb.ActionRisk_RISK_LOW,
			params:      map[string]string{"unit": "globular-file.service"},
			wantApprove: false,
		},
		{
			name:        "systemctl_restart on non-globular unit → approval required",
			action:      cluster_doctorpb.ActionType_SYSTEMCTL_RESTART,
			risk:        cluster_doctorpb.ActionRisk_RISK_LOW,
			params:      map[string]string{"unit": "nginx.service"},
			wantApprove: true,
		},
		{
			name:        "systemctl_stop always requires approval even for globular-*",
			action:      cluster_doctorpb.ActionType_SYSTEMCTL_STOP,
			risk:        cluster_doctorpb.ActionRisk_RISK_LOW,
			params:      map[string]string{"unit": "globular-file.service"},
			wantApprove: true,
		},
		{
			name:        "file_delete in safe-trash, LOW → no approval",
			action:      cluster_doctorpb.ActionType_FILE_DELETE,
			risk:        cluster_doctorpb.ActionRisk_RISK_LOW,
			params:      map[string]string{"path": "/usr/lib/globular/bin/foo.tmp"},
			wantApprove: false,
		},
		{
			name:        "file_delete outside safe-trash → approval required",
			action:      cluster_doctorpb.ActionType_FILE_DELETE,
			risk:        cluster_doctorpb.ActionRisk_RISK_LOW,
			params:      map[string]string{"path": "/var/lib/globular/important.db"},
			wantApprove: true,
		},
		{
			name:        "RISK_HIGH always requires approval",
			action:      cluster_doctorpb.ActionType_SYSTEMCTL_RESTART,
			risk:        cluster_doctorpb.ActionRisk_RISK_HIGH,
			params:      map[string]string{"unit": "globular-file.service"},
			wantApprove: true,
		},
		{
			name:        "RISK_MEDIUM always requires approval",
			action:      cluster_doctorpb.ActionType_SYSTEMCTL_RESTART,
			risk:        cluster_doctorpb.ActionRisk_RISK_MEDIUM,
			params:      map[string]string{"unit": "globular-file.service"},
			wantApprove: true,
		},
		{
			name:        "package_reinstall requires approval",
			action:      cluster_doctorpb.ActionType_PACKAGE_REINSTALL,
			risk:        cluster_doctorpb.ActionRisk_RISK_LOW,
			params:      map[string]string{},
			wantApprove: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := &cluster_doctorpb.RemediationAction{
				ActionType: tc.action,
				Risk:       tc.risk,
				Params:     tc.params,
			}
			got, _ := requiresApproval(a)
			if got != tc.wantApprove {
				t.Fatalf("requiresApproval = %v, want %v", got, tc.wantApprove)
			}
		})
	}
}
