// Package nodeid is the SINGLE authority for deriving a Globular node's stable
// identity from its network attributes. The cluster controller (when it mints
// AssignedNodeID for a join), the node agent (its local StableNodeID fallback),
// and the resource store (peers.go) MUST all derive through here so a given node
// maps to exactly ONE node_id everywhere.
//
// Before this package existed the scheme was hand-rolled in three places and
// diverged: resource/peers.go used Utility.GenerateUUID = UUID v3/MD5 over the
// bare MAC in the DNS namespace, while the controller and node agent used
// UUID v5/SHA1 over "mac:"+mac in the Globular namespace. Same node → different
// UUIDs. Centralizing here removes that divergence.
//
// Identity-migration note: node_id is still DERIVED from the MAC/hostname, which
// are mutable, rather than minted as an opaque token and read through (the way
// the cluster membership UUID is). This package is the single baselined site for
// that deviation; making node_id a minted read-through id is a later phase. The
// Namespace and derivation grammar below are load-bearing — they are baked into
// every persisted /globular/nodes/{id} etcd key and every signed JoinPlan
// AssignedNodeID, so they MUST NOT change without a node-identity migration.
package nodeid

import (
	"sort"
	"strings"

	"github.com/google/uuid"
)

// Namespace is the fixed UUID v5 namespace for Globular node identities.
// DO NOT CHANGE — see the package doc.
var Namespace = uuid.MustParse("a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d")

// FromMAC derives the canonical node id from a MAC address: UUID v5/SHA1 over
// "mac:"+mac in Namespace. The caller supplies mac already normalized, exactly as
// the historical call sites did (this reproduces their bytes verbatim).
func FromMAC(mac string) string {
	return uuid.NewSHA1(Namespace, []byte("mac:"+mac)).String()
}

// FromHostAndIPs derives the canonical node id when no MAC is available, from the
// hostname and IPs: UUID v5/SHA1 over "host:"+hostname+"|"+join(sortedIPs,"|").
// IPs are sorted so the id is stable regardless of input order. A copy is sorted
// so the caller's slice is not mutated.
func FromHostAndIPs(hostname string, ips []string) string {
	sorted := append([]string(nil), ips...)
	sort.Strings(sorted)
	key := hostname + "|" + strings.Join(sorted, "|")
	return uuid.NewSHA1(Namespace, []byte("host:"+key)).String()
}
