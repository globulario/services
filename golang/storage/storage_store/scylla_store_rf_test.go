package storage_store

import "testing"

// distinctHostCount is the cluster-size signal used to pick the keyspace
// replication factor. It must count UNIQUE hosts (ignoring port) from the
// configured topology — replacing the gossip-dependent `system.peers` query that
// undercounts under multi-host routing and sets RF too low (under-replication).
func TestDistinctHostCount(t *testing.T) {
	cases := []struct {
		name  string
		hosts []string
		want  int
	}{
		{"empty", nil, 0},
		{"single host no port", []string{"10.0.0.63"}, 1},
		{"single host with port", []string{"10.0.0.63:9042"}, 1},
		{"three distinct", []string{"10.0.0.63:9042", "10.0.0.8:9042", "10.0.0.20:9042"}, 3},
		{"dedupe same host different port", []string{"10.0.0.63:9042", "10.0.0.63:19042"}, 1},
		{"ignores blanks", []string{"10.0.0.63:9042", "", "  ", "10.0.0.8:9042"}, 2},
		{"ipv6 with port", []string{"[::1]:9042"}, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := distinctHostCount(c.hosts); got != c.want {
				t.Errorf("distinctHostCount(%v) = %d, want %d", c.hosts, got, c.want)
			}
		})
	}
}
