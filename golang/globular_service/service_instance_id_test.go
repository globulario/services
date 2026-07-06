// service_instance_id_test.go: pins the Phase-4 identity contract for the single
// service-instance-id authority. The historical seed name:version:mac churned the
// id on every upgrade (version) and NIC change (mac), orphaning
// /globular/services/{id}. ServiceInstanceID must be deterministic,
// version-independent, and derive its node part through the nodeid authority.
package globular_service

import (
	"testing"

	"github.com/globulario/services/golang/nodeid"
	Utility "github.com/globulario/utility"
)

// TestServiceInstanceID_DeterministicAndVersionFree: same (name, mac) always yields
// the same id, and the id is exactly GenerateUUID(name + ":" + nodeid.FromMAC(mac)) —
// no version input exists, so an upgrade cannot change it. This is the regression
// guard against re-introducing a version-derived (churning) id.
func TestServiceInstanceID_DeterministicAndVersionFree(t *testing.T) {
	const (
		name = "rbac.RbacService"
		mac  = "aa:bb:cc:dd:ee:ff"
	)
	want := Utility.GenerateUUID(name + ":" + nodeid.FromMAC(mac))

	got1 := ServiceInstanceID(name, mac)
	got2 := ServiceInstanceID(name, mac)
	if got1 != got2 {
		t.Fatalf("not deterministic: %q vs %q", got1, got2)
	}
	if got1 != want {
		t.Fatalf("id must derive through nodeid authority: got %q want %q", got1, want)
	}

	// Whitespace around inputs must not change identity (mirrors ensureDescribeID trim).
	if s := ServiceInstanceID("  "+name+"  ", "  "+mac+"  "); s != want {
		t.Errorf("trimmed inputs must yield same id: got %q want %q", s, want)
	}
}

// TestServiceInstanceID_Discriminates: a different service name OR a different node
// (mac) yields a different id — so (service-name, node) uniqueness is preserved.
func TestServiceInstanceID_Discriminates(t *testing.T) {
	base := ServiceInstanceID("sql.SqlService", "aa:bb:cc:dd:ee:01")
	if byName := ServiceInstanceID("log.LogService", "aa:bb:cc:dd:ee:01"); byName == base {
		t.Error("different service name must produce a different id")
	}
	if byMac := ServiceInstanceID("sql.SqlService", "aa:bb:cc:dd:ee:02"); byMac == base {
		t.Error("different node (mac) must produce a different id")
	}
}
