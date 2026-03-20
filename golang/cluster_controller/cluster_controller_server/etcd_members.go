package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// etcdMemberManager handles automatic etcd cluster membership changes.
// It queries the live etcd cluster to determine current members and adds
// new members before their etcd instance starts.
type etcdMemberManager struct {
	client *clientv3.Client
}

func newEtcdMemberManager(client *clientv3.Client) *etcdMemberManager {
	return &etcdMemberManager{client: client}
}

// snapshotEtcdMembers queries the live etcd cluster and returns the current
// member state. This is used by renderEtcdConfig to set initial-cluster-state.
func (m *etcdMemberManager) snapshotEtcdMembers(ctx context.Context) (*etcdMemberState, error) {
	if m == nil || m.client == nil {
		return &etcdMemberState{Bootstrapped: false}, nil
	}

	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := m.client.MemberList(tctx)
	if err != nil {
		return nil, fmt.Errorf("etcd member list: %w", err)
	}

	state := &etcdMemberState{
		Bootstrapped:   len(resp.Members) > 0,
		MemberPeerURLs: make(map[string]string, len(resp.Members)),
	}
	for _, member := range resp.Members {
		name := member.Name
		if name == "" {
			// Unstarted member (added but not yet started) — use first peer URL.
			if len(member.PeerURLs) > 0 {
				name = member.PeerURLs[0]
			}
			continue
		}
		if len(member.PeerURLs) > 0 {
			state.MemberPeerURLs[name] = member.PeerURLs[0]
		}
	}
	return state, nil
}

// reconcileMembers ensures that all desired etcd nodes are registered as etcd
// members. For each node that has an etcd profile but isn't yet an etcd member,
// it calls MemberAdd. This must be called BEFORE dispatching plans to new nodes.
//
// Returns the list of node IPs that were newly added as members.
func (m *etcdMemberManager) reconcileMembers(ctx context.Context, desiredEtcdNodes []memberNode) ([]string, error) {
	if m == nil || m.client == nil {
		return nil, nil
	}

	tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := m.client.MemberList(tctx)
	if err != nil {
		return nil, fmt.Errorf("etcd member list: %w", err)
	}

	// Build set of existing peer URLs.
	existingPeerURLs := make(map[string]bool)
	for _, member := range resp.Members {
		for _, purl := range member.PeerURLs {
			existingPeerURLs[purl] = true
		}
	}

	var added []string
	for _, node := range desiredEtcdNodes {
		ip := node.IP
		if ip == "" {
			continue
		}
		peerURL := fmt.Sprintf("https://%s:2380", ip)
		if existingPeerURLs[peerURL] {
			continue // already a member
		}

		// Add as new member.
		addCtx, addCancel := context.WithTimeout(ctx, 10*time.Second)
		_, err := m.client.MemberAdd(addCtx, []string{peerURL})
		addCancel()
		if err != nil {
			// If the member is already added (race), log and continue.
			if strings.Contains(err.Error(), "already added") ||
				strings.Contains(err.Error(), "Peer URLs already exists") {
				log.Printf("etcd member-add: %s already registered, skipping", peerURL)
				continue
			}
			return added, fmt.Errorf("etcd member add %s: %w", peerURL, err)
		}
		log.Printf("etcd member-add: registered new member %s (node %s/%s)", peerURL, node.NodeID, node.Hostname)
		added = append(added, ip)
	}

	return added, nil
}

// removeStaleMembers removes etcd members whose peer URL doesn't match any
// desired etcd node. This handles node removal from the cluster.
func (m *etcdMemberManager) removeStaleMembers(ctx context.Context, desiredEtcdNodes []memberNode) error {
	if m == nil || m.client == nil {
		return nil
	}

	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := m.client.MemberList(tctx)
	if err != nil {
		return fmt.Errorf("etcd member list: %w", err)
	}

	// Build set of desired peer URLs.
	desiredPeerURLs := make(map[string]bool, len(desiredEtcdNodes))
	for _, node := range desiredEtcdNodes {
		if node.IP != "" {
			desiredPeerURLs[fmt.Sprintf("https://%s:2380", node.IP)] = true
		}
	}

	for _, member := range resp.Members {
		if member.Name == "" {
			continue // unstarted member, don't remove (might be joining)
		}
		isDesired := false
		for _, purl := range member.PeerURLs {
			if desiredPeerURLs[purl] {
				isDesired = true
				break
			}
		}
		if isDesired {
			continue
		}

		rmCtx, rmCancel := context.WithTimeout(ctx, 10*time.Second)
		_, err := m.client.MemberRemove(rmCtx, member.ID)
		rmCancel()
		if err != nil {
			log.Printf("etcd member-remove: failed to remove stale member %s (id=%d): %v", member.Name, member.ID, err)
			continue
		}
		log.Printf("etcd member-remove: removed stale member %s (id=%d, peer=%v)", member.Name, member.ID, member.PeerURLs)
	}

	return nil
}
