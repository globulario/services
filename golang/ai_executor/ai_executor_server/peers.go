package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	ai_executorpb "github.com/globulario/services/golang/ai_executor/ai_executorpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"crypto/tls"
	"crypto/x509"
)

// peerManager discovers and communicates with ai-executor instances on other nodes.
type peerManager struct {
	localNodeID   string
	localHostname string
	localProfiles []string
	mu            sync.RWMutex
	peers         map[string]*peerConn // node_id -> connection
}

type peerConn struct {
	NodeID   string
	Hostname string
	Endpoint string // host:port
	Client   ai_executorpb.AiExecutorServiceClient
	Conn     *grpc.ClientConn
	LastSeen time.Time
}

func newPeerManager(nodeID, hostname string, profiles []string) *peerManager {
	return &peerManager{
		localNodeID:   nodeID,
		localHostname: hostname,
		localProfiles: profiles,
		peers:         make(map[string]*peerConn),
	}
}

// discoverPeers finds other ai-executor instances from etcd service registry.
func (pm *peerManager) discoverPeers() {
	services, err := config.GetServicesConfigurations()
	if err != nil {
		return
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Resolve local IPs so we can skip our own instance in the service list.
	localIPs := make(map[string]bool)
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if ipNet, ok := a.(*net.IPNet); ok {
				localIPs[ipNet.IP.String()] = true
			}
		}
	}

	found := make(map[string]bool)
	for _, svc := range services {
		name, _ := svc["Name"].(string)
		if !strings.Contains(name, "ai_executor.AiExecutorService") {
			continue
		}

		address, _ := svc["Address"].(string)
		port, _ := svc["Port"].(float64)
		if address == "" || port == 0 {
			continue
		}

		// Strip any existing port from address — service configs sometimes
		// store "host:port" in the Address field, which would produce
		// "host:port:port" when combined with Port below.
		if host, _, err := net.SplitHostPort(address); err == nil && host != "" {
			address = host
		}

		// Skip our own instance by checking if the address is local.
		if localIPs[address] || address == "localhost" || address == "127.0.0.1" {
			continue
		}

		// Use address:port as the unique peer identifier since the service
		// Id/Mac fields are shared across all instances.
		endpoint := fmt.Sprintf("%s:%d", address, int(port))
		found[endpoint] = true

		// Already connected?
		if p, ok := pm.peers[endpoint]; ok {
			p.LastSeen = time.Now()
			continue
		}

		// New peer — connect.
		conn, client, err := dialPeer(endpoint)
		if err != nil {
			logger.Debug("peer: connect failed", "endpoint", endpoint, "err", err)
			continue
		}

		hostname := ""
		// Try to get hostname via Ping. Inject auth metadata so the peer's
		// server interceptor accepts the call post-cluster-init.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		ctx = peerAuthContext(ctx)
		resp, err := client.Ping(ctx, &ai_executorpb.PeerPingRequest{
			SenderNodeId:   pm.localNodeID,
			SenderHostname: pm.localHostname,
		})
		cancel()
		if err == nil {
			hostname = resp.Hostname
		} else {
			logger.Warn("peer: ping failed", "endpoint", endpoint, "err", err)
		}

		// Fall back to /etc/hosts if Ping didn't return a hostname.
		// We avoid net.LookupAddr because PTR queries to our internal DNS
		// time out, blocking discovery for 15+ seconds.
		if hostname == "" {
			hostname = resolveHostnameFromHosts(address)
		}
		if hostname == "" {
			hostname = address // last resort: use IP as identifier
		}

		pm.peers[endpoint] = &peerConn{
			NodeID:   endpoint,
			Hostname: hostname,
			Endpoint: endpoint,
			Client:   client,
			Conn:     conn,
			LastSeen: time.Now(),
		}
		logger.Info("peer: connected", "node", hostname, "endpoint", endpoint)
	}

	// Remove stale peers.
	for id, p := range pm.peers {
		if !found[id] {
			p.Conn.Close()
			delete(pm.peers, id)
			logger.Info("peer: removed stale", "node", p.Hostname)
		}
	}
}

// dialPeer creates a gRPC connection to a peer executor.
func dialPeer(endpoint string) (*grpc.ClientConn, ai_executorpb.AiExecutorServiceClient, error) {
	dt := config.ResolveDialTarget(endpoint)
	creds := buildPeerTLS(dt.ServerName)
	conn, err := grpc.NewClient(dt.Address, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, nil, err
	}
	return conn, ai_executorpb.NewAiExecutorServiceClient(conn), nil
}

// peerAuthContext attaches outgoing metadata required by the target server's
// interceptor chain: cluster_id (post-init enforcement) and a local service
// token so the peer accepts the call without flagging it anonymous.
func peerAuthContext(ctx context.Context) context.Context {
	md := metadata.MD{}
	if clusterID, err := security.GetLocalClusterID(); err == nil && clusterID != "" {
		md.Set("cluster_id", clusterID)
	}
	// Best-effort: include the local service token if available. Peers accept
	// loopback/mesh calls without tokens, but real mesh calls need one.
	if mac, err := config.GetMacAddress(); err == nil && mac != "" {
		if tok, err := security.GetLocalToken(mac); err == nil && tok != "" {
			md.Set("token", tok)
		}
	}
	return metadata.NewOutgoingContext(ctx, md)
}

func buildPeerTLS(serverName string) credentials.TransportCredentials {
	tlsCfg := &tls.Config{ServerName: serverName}
	caFile := config.GetTLSFile("", "", "ca.crt")
	if caFile != "" {
		if caData, err := os.ReadFile(caFile); err == nil {
			pool := x509.NewCertPool()
			if pool.AppendCertsFromPEM(caData) {
				tlsCfg.RootCAs = pool
			}
		}
	}
	// Include client certificate for mTLS — peers require mutual TLS
	// to authenticate inter-node calls.
	certFile := "/var/lib/globular/pki/issued/services/service.crt"
	keyFile := "/var/lib/globular/pki/issued/services/service.key"
	if cert, err := tls.LoadX509KeyPair(certFile, keyFile); err == nil {
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	return credentials.NewTLS(tlsCfg)
}

// getPeers returns a snapshot of connected peers.
func (pm *peerManager) getPeers() []*peerConn {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	peers := make([]*peerConn, 0, len(pm.peers))
	for _, p := range pm.peers {
		peers = append(peers, p)
	}
	return peers
}

// peerCount returns the number of connected peers.
func (pm *peerManager) peerCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return len(pm.peers)
}

// --- Consensus ---

// ConsensusResult holds the outcome of a peer vote.
type ConsensusResult struct {
	Approved  int
	Rejected  int
	Escalated int
	Abstained int
	Total     int
	Passed    bool
	Reasons   []string
}

// seekConsensus asks all peers to vote on a proposed action.
// Returns when all peers respond or timeout (5s per peer).
func (pm *peerManager) seekConsensus(ctx context.Context, proposal *ai_executorpb.PeerProposalRequest) *ConsensusResult {
	peers := pm.getPeers()
	result := &ConsensusResult{Total: len(peers) + 1} // +1 for self (always approves own proposal)
	result.Approved = 1 // self-vote

	if len(peers) == 0 {
		// Solo node — self-approve.
		result.Passed = true
		return result
	}

	type vote struct {
		nodeID string
		resp   *ai_executorpb.PeerProposalResponse
		err    error
	}

	ch := make(chan vote, len(peers))

	for _, p := range peers {
		go func(peer *peerConn) {
			pCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			resp, err := peer.Client.ProposeAction(pCtx, proposal)
			ch <- vote{nodeID: peer.NodeID, resp: resp, err: err}
		}(p)
	}

	// Collect votes.
	for i := 0; i < len(peers); i++ {
		select {
		case v := <-ch:
			if v.err != nil {
				result.Abstained++
				result.Reasons = append(result.Reasons, fmt.Sprintf("%s: unreachable", v.nodeID))
				continue
			}
			switch v.resp.Vote {
			case ai_executorpb.PeerVote_VOTE_APPROVE:
				result.Approved++
			case ai_executorpb.PeerVote_VOTE_REJECT:
				result.Rejected++
				result.Reasons = append(result.Reasons, fmt.Sprintf("%s rejects: %s", v.resp.NodeId, v.resp.Reason))
			case ai_executorpb.PeerVote_VOTE_ESCALATE:
				result.Escalated++
				result.Reasons = append(result.Reasons, fmt.Sprintf("%s wants escalation: %s", v.resp.NodeId, v.resp.Reason))
			default:
				result.Abstained++
			}
		case <-ctx.Done():
			result.Abstained += len(peers) - i
			break
		}
	}

	// Majority rule: approved > (rejected + escalated)
	result.Passed = result.Approved > (result.Rejected + result.Escalated)

	logger.Info("consensus",
		"proposal", proposal.ProposalId,
		"approved", result.Approved,
		"rejected", result.Rejected,
		"escalated", result.Escalated,
		"abstained", result.Abstained,
		"passed", result.Passed)

	return result
}

// broadcastObservation shares a local observation with all peers.
func (pm *peerManager) broadcastObservation(ctx context.Context, obs *ai_executorpb.PeerObservationRequest) map[string]*ai_executorpb.PeerObservationResponse {
	peers := pm.getPeers()
	results := make(map[string]*ai_executorpb.PeerObservationResponse)
	var mu sync.Mutex

	var wg sync.WaitGroup
	for _, p := range peers {
		wg.Add(1)
		go func(peer *peerConn) {
			defer wg.Done()
			pCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			resp, err := peer.Client.ShareObservation(pCtx, obs)
			if err == nil {
				mu.Lock()
				results[peer.NodeID] = resp
				mu.Unlock()
			}
		}(p)
	}
	wg.Wait()
	return results
}

// notifyPeers tells all peers about an action that was taken.
func (pm *peerManager) notifyPeers(ctx context.Context, notification *ai_executorpb.PeerActionNotification) {
	peers := pm.getPeers()
	for _, p := range peers {
		go func(peer *peerConn) {
			pCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			_, _ = peer.Client.NotifyActionTaken(pCtx, notification)
		}(p)
	}
}

// startDiscoveryLoop periodically discovers and pings peers.
func (pm *peerManager) startDiscoveryLoop(ctx context.Context) {
	// Initial discovery after a short delay (let services register).
	time.Sleep(30 * time.Second)
	pm.discoverPeers()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pm.discoverPeers()
		}
	}
}

// close disconnects all peers.
func (pm *peerManager) close() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for _, p := range pm.peers {
		p.Conn.Close()
	}
	pm.peers = make(map[string]*peerConn)
}

// resolveHostnameFromHosts parses /etc/hosts and returns the short hostname
// for the given IP address. Returns empty string if not found.
func resolveHostnameFromHosts(ip string) string {
	f, err := os.Open("/etc/hosts")
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != ip {
			continue
		}
		// Return the short hostname (first name, stripped of domain).
		name := fields[1]
		if idx := strings.Index(name, "."); idx > 0 {
			name = name[:idx]
		}
		return name
	}
	return ""
}

// Suppress unused import warning for slog.
var _ = slog.Info
