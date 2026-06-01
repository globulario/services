package main

import (
	"context"
	"net"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/cluster_controller_server/projections"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ResolveNode answers "who is this node?" given any of node_id / hostname /
// mac / ip. See docs/architecture/projection-clauses.md for the contract.
//
// Resolution order per Clause 3 (Reader-Fallback):
//
//	1. Detect identifier shape (UUID → node_id, contains ':' → mac, dotted-
//	   quad → ip, else hostname).
//	2. Try the scylla node_identity projection first (one partition read).
//	3. On miss/error, fall back to a scan over srv.state.Nodes (source of
//	   truth, always correct but O(n)).
//	4. The response's `source` field reflects where the answer came from so
//	   the caller knows the confidence.
//
// Clause 5 (Scoped Query) is satisfied trivially: this RPC always returns
// exactly zero or one node.
func (srv *server) ResolveNode(ctx context.Context, req *cluster_controllerpb.ResolveNodeRequest) (*cluster_controllerpb.ResolveNodeResponse, error) {
	ident := strings.TrimSpace(req.GetIdentifier())
	if ident == "" {
		return nil, status.Error(codes.InvalidArgument, "identifier is required")
	}

	kind := detectIdentifierKind(ident)

	// Try the projection first — single partition read, fastest path.
	if srv.nodeIdentityProj != nil {
		if id := srv.resolveFromProjection(ctx, ident, kind); id != nil {
			return &cluster_controllerpb.ResolveNodeResponse{
				Identity: toProtoIdentity(id, "scylla"),
			}, nil
		}
	}

	// Fallback: scan in-memory state. Always correct, always available.
	if id := srv.resolveFromState(ident, kind); id != nil {
		return &cluster_controllerpb.ResolveNodeResponse{
			Identity: toProtoIdentity(id, "cluster-controller"),
		}, nil
	}

	return nil, status.Errorf(codes.NotFound, "no node matches %q", ident)
}

// identifierKind classifies how the caller's identifier should be interpreted.
type identifierKind int

const (
	kindUnknown identifierKind = iota
	kindNodeID                 // UUID
	kindMAC                    // xx:xx:xx:xx:xx:xx
	kindIP                     // dotted-quad or IPv6
	kindHostname               // anything else
)

// detectIdentifierKind picks the lookup table based purely on the shape of
// the identifier. Purity matters (Clause 10) — same input always yields
// the same lookup plan, which makes the resolver easy to reason about.
func detectIdentifierKind(s string) identifierKind {
	if _, err := uuid.Parse(s); err == nil {
		return kindNodeID
	}
	// MAC addresses use colons and parse cleanly as hw addrs.
	if strings.Count(s, ":") == 5 {
		if _, err := net.ParseMAC(s); err == nil {
			return kindMAC
		}
	}
	if ip := net.ParseIP(s); ip != nil {
		return kindIP
	}
	return kindHostname
}

// resolveFromProjection hits the scylla read model. Returns nil on miss or
// error (logged, swallowed) so the caller can fall back cleanly.
func (srv *server) resolveFromProjection(ctx context.Context, ident string, kind identifierKind) *projections.NodeIdentity {
	p := srv.nodeIdentityProj

	switch kind {
	case kindNodeID:
		id, err := p.GetByNodeID(ctx, ident)
		if err != nil {
			return nil
		}
		return id
	case kindHostname:
		nodeID, err := p.GetNodeIDByHostname(ctx, ident)
		if err != nil || nodeID == "" {
			return nil
		}
		id, err := p.GetByNodeID(ctx, nodeID)
		if err != nil {
			return nil
		}
		return id
	case kindMAC:
		nodeID, err := p.GetNodeIDByMAC(ctx, ident)
		if err != nil || nodeID == "" {
			return nil
		}
		id, err := p.GetByNodeID(ctx, nodeID)
		if err != nil {
			return nil
		}
		return id
	case kindIP:
		nodeID, err := p.GetNodeIDByIP(ctx, ident)
		if err != nil || nodeID == "" {
			return nil
		}
		id, err := p.GetByNodeID(ctx, nodeID)
		if err != nil {
			return nil
		}
		return id
	}
	return nil
}

// resolveFromState scans the in-memory cluster state. This is always
// available and always correct — it IS the source of truth, just with
// O(n) lookup. Used as a fallback when scylla is missing/stale.
func (srv *server) resolveFromState(ident string, kind identifierKind) *projections.NodeIdentity {
	srv.lock("resolve")
	defer srv.unlock()
	if srv.state == nil {
		return nil
	}

	for _, node := range srv.state.Nodes {
		if nodeMatchesIdentifier(node, ident, kind) {
			return nodeToIdentity(node)
		}
	}
	return nil
}

// nodeMatchesIdentifier checks whether a nodeState satisfies the given
// identifier. MAC matching falls through to labels[node.mac] since the
// stored identity struct doesn't carry MAC directly.
func nodeMatchesIdentifier(node *nodeState, ident string, kind identifierKind) bool {
	switch kind {
	case kindNodeID:
		return node.NodeID == ident
	case kindHostname:
		return node.Identity.Hostname == ident
	case kindMAC:
		return strings.EqualFold(node.Metadata["node.mac"], ident)
	case kindIP:
		for _, ip := range node.Identity.Ips {
			if ip == ident {
				return true
			}
		}
	}
	return false
}

// nodeToIdentity extracts the projection row from a full nodeState. The
// projector and the fallback path MUST produce byte-identical rows for the
// same input (Clause 10) — this function is the one authority for that
// mapping.
func nodeToIdentity(node *nodeState) *projections.NodeIdentity {
	macs := make([]string, 0, 1)
	if mac := node.Metadata["node.mac"]; mac != "" {
		macs = append(macs, mac)
	}
	ips := make([]string, 0, len(node.Identity.Ips))
	for _, ip := range node.Identity.Ips {
		if ip != "" {
			ips = append(ips, ip)
		}
	}
	labels := make([]string, 0, len(node.Profiles))
	labels = append(labels, node.Profiles...)

	observed := node.LastSeen
	if observed.IsZero() {
		observed = node.ReportedAt
	}
	var observedAt int64
	if !observed.IsZero() {
		observedAt = observed.Unix()
	} else {
		observedAt = time.Now().Unix()
	}

	return &projections.NodeIdentity{
		NodeID:     node.NodeID,
		Hostname:   node.Identity.Hostname,
		IPs:        ips,
		MACs:       macs,
		Labels:     labels,
		ObservedAt: observedAt,
	}
}

// toProtoIdentity converts the internal projection row into the proto-level
// response shape. The `source` field is set by the caller — NEVER inferred
// from the data itself (Clause 4).
func toProtoIdentity(id *projections.NodeIdentity, source string) *cluster_controllerpb.NodeIdentityProjection {
	return &cluster_controllerpb.NodeIdentityProjection{
		NodeId:     id.NodeID,
		Hostname:   id.Hostname,
		Ips:        id.IPs,
		Macs:       id.MACs,
		Labels:     id.Labels,
		Source:     source,
		ObservedAt: id.ObservedAt,
	}
}
