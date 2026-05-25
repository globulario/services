package main

import (
	"fmt"
	"strings"
	"time"
)

// Group0Voter is a single entry in Scylla Raft Group 0.
type Group0Voter struct {
	// ServerID is the UUID from system.raft_group0_members. In Scylla this
	// equals the node's host_id in system.local / system.peers.
	ServerID string
	// PeerAddr is the gossip IP of this voter, resolved by cross-referencing
	// system.peers.host_id against ServerID. Empty when the voter is unknown
	// to gossip — the strongest available staleness signal.
	PeerAddr string
	// CanVote is from the can_vote column, if present in this Scylla version.
	// Nil means the column is not available; stale detection falls back to
	// the gossip cross-reference only.
	CanVote *bool
	// InGossip is true when ServerID matches system.local.host_id or any
	// system.peers.host_id. False means gossip no longer knows this node.
	InGossip bool
	// StaleReason is non-empty when this voter is suspected stale/dead.
	// "not_in_gossip" — host_id absent from local and all peers.
	// "can_vote=false" — Scylla reports the voter cannot participate.
	StaleReason string
}

// IsStale returns true when the voter has a staleness reason.
func (v Group0Voter) IsStale() bool { return v.StaleReason != "" }

// Group0View is the structured result of a Raft Group 0 membership inspection.
// It is attached to DDLPreflightResult when Layer 3 executes successfully.
type Group0View struct {
	Voters      []Group0Voter
	SourceTable string
	TotalVoters int
	StaleVoters int
	CheckedAt   time.Time
}

// collectGroup0View cross-references raw Group 0 membership with gossip to
// determine per-voter staleness. It is pure logic — no I/O.
//
// localHostID is from system.local.host_id.
// peers is from system.peers with Peer (IP) and HostID populated.
// members is from system.raft_group0_members (already queried).
func collectGroup0View(members []group0Member, localHostID string, peers []peerSchemaRow) Group0View {
	// Build host_id → IP map from gossip. Local node maps to "local".
	hostToAddr := map[string]string{}
	if localHostID != "" {
		hostToAddr[localHostID] = "local"
	}
	for _, p := range peers {
		if p.HostID != "" {
			hostToAddr[p.HostID] = p.Peer
		}
	}

	view := Group0View{
		SourceTable: "system.raft_group0_members",
		CheckedAt:   time.Now(),
	}
	for _, m := range members {
		voter := Group0Voter{
			ServerID: m.ServerID,
			CanVote:  m.CanVote,
		}
		if addr, ok := hostToAddr[m.ServerID]; ok {
			voter.InGossip = true
			if addr != "local" {
				voter.PeerAddr = addr
			}
		}
		// Stale detection: gossip absence is the primary signal;
		// can_vote=false is secondary (requires Scylla 5.x+ column).
		if !voter.InGossip {
			voter.StaleReason = "not_in_gossip"
		} else if m.CanVote != nil && !*m.CanVote {
			voter.StaleReason = "can_vote=false"
		}
		view.Voters = append(view.Voters, voter)
	}
	view.TotalVoters = len(view.Voters)
	for _, v := range view.Voters {
		if v.IsStale() {
			view.StaleVoters++
		}
	}
	return view
}

// Group0FindingText generates a human-readable operator finding for the given
// DDL preflight result. Used in log messages, doctor findings, and alerts.
//
// Two canonical messages (from Phase D spec):
//
//	"Scylla DDL blocked: Raft Group 0 contains dead/stale voter."
//	"Scylla DDL blocked: Raft Group 0 voter health cannot be proven."
func Group0FindingText(r DDLPreflightResult) string {
	var sb strings.Builder
	switch r.Reason {
	case DDLPreflightOK:
		voters := 0
		if r.Group0 != nil {
			voters = r.Group0.TotalVoters
		}
		fmt.Fprintf(&sb, "Scylla DDL preflight passed: Raft Group 0 is healthy (%d voters, source: %s).",
			voters, group0Source(r))

	case DDLPreflightUnknown:
		sb.WriteString("Scylla DDL blocked: Raft Group 0 voter health cannot be proven.")
		if detail, ok := r.Details["reason"]; ok {
			fmt.Fprintf(&sb, " (%s)", detail)
		}
		sb.WriteString("\n  Action: upgrade Scylla to a version that exposes system.raft_group0_members, or verify voter health manually.")

	case DDLPreflightGroup0StaleVoter:
		sb.WriteString("Scylla DDL blocked: Raft Group 0 contains dead/stale voter.")
		if r.Group0 != nil {
			for _, v := range r.Group0.Voters {
				if !v.IsStale() {
					continue
				}
				if v.PeerAddr != "" {
					fmt.Fprintf(&sb, "\n  Stale voter: %s (%s) reason=%s", v.ServerID, v.PeerAddr, v.StaleReason)
				} else {
					fmt.Fprintf(&sb, "\n  Stale voter: %s reason=%s (not in gossip)", v.ServerID, v.StaleReason)
				}
			}
			fmt.Fprintf(&sb, "\n  Source: %s", r.Group0.SourceTable)
		}
		sb.WriteString("\n  Action: remove stale voter before issuing DDL. Use 'nodetool removenode <host_id>' or the Scylla admin API.")

	case DDLPreflightGroup0Unavailable:
		sb.WriteString("Scylla DDL blocked: Raft Group 0 membership is unavailable (table empty or unreachable).")
		if r.Group0 != nil && r.Group0.SourceTable != "" {
			fmt.Fprintf(&sb, " Source: %s.", r.Group0.SourceTable)
		}

	case DDLPreflightSchemaAgreementUnavailable:
		sb.WriteString("Scylla DDL blocked: schema agreement is not established across all peers.")
		if peer, ok := r.Details["first_disagreeing_peer"]; ok {
			fmt.Fprintf(&sb, " Disagreeing peer: %s", peer)
			if pv, ok2 := r.Details["peer_schema_version"]; ok2 {
				fmt.Fprintf(&sb, " (schema_version=%s vs local=%s)", pv, r.Details["local_schema_version"])
			}
		}

	case DDLPreflightScyllaUnavailable:
		sb.WriteString("Scylla DDL blocked: Scylla is unreachable or not responding.")
		if errMsg, ok := r.Details["error"]; ok {
			fmt.Fprintf(&sb, " Error: %s", errMsg)
		}

	default:
		fmt.Fprintf(&sb, "Scylla DDL blocked: %s", r.Reason)
	}
	return sb.String()
}

func group0Source(r DDLPreflightResult) string {
	if r.Group0 != nil && r.Group0.SourceTable != "" {
		return r.Group0.SourceTable
	}
	return "unknown"
}
