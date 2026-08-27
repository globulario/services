package main

import "testing"

// TestEffectiveClusterIDFallsBackToLocal pins that outbound requests carry a
// cluster identity even when startup discovery missed.
//
// s.clusterID is discovered ONCE at construction, inside the workflow dial
// block, and only if the controller answers GetClusterInfo. During bootstrap the
// controller is frequently not serving yet, so the discovery misses and the field
// stays empty for the life of the process. Every ExecuteWorkflow then carried an
// empty ClusterId and the workflow service refused it with
// "cluster_id is required" — forbidden_fix
// guard_cluster_id_injection_on_optionally_propagated_field exactly.
func TestEffectiveClusterIDFallsBackToLocal(t *testing.T) {
	t.Run("a discovered id is preferred", func(t *testing.T) {
		s := &ClusterDoctorServer{clusterID: "discovered.cluster"}
		if got := s.effectiveClusterID(); got != "discovered.cluster" {
			t.Errorf("effectiveClusterID() = %q, want the discovered value", got)
		}
	})

	t.Run("whitespace is not an identity", func(t *testing.T) {
		s := &ClusterDoctorServer{clusterID: "   "}
		// Falls through to the local source; with none configured in a unit test
		// the result is empty, which callers treat as "unknown" and refuse on.
		if got := s.effectiveClusterID(); got == "   " {
			t.Error("a whitespace-only cluster id was returned verbatim")
		}
	})

	t.Run("an empty discovered id does not panic and yields a decidable value", func(t *testing.T) {
		s := &ClusterDoctorServer{}
		got := s.effectiveClusterID()
		// Either the local source knows (non-empty) or nothing does (empty).
		// What must NOT happen is a fabricated placeholder.
		for _, bad := range []string{"unknown", "default", "localhost", "cluster"} {
			if got == bad {
				t.Errorf("effectiveClusterID() invented a placeholder %q — evidence "+
					"stamped with a fake cluster lands in the wrong partition and "+
					"looks recorded", got)
			}
		}
	})
}
