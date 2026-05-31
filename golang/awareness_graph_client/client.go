// Package awareness_graph_client is a thin gRPC client for the awareness-graph
// service (github.com/globulario/awareness-graph).
//
// The service is a separate process (default port 10120) that compiles project
// intent, invariants, failure modes, and incident patterns into an RDF graph
// and exposes four typed RPCs: Resolve, Impact, Briefing, Query.
//
// This client does no caching, no retry, and no parsing — it forwards calls
// over mTLS to the cluster's awareness-graph instance(s) and returns the
// protobuf response verbatim. Callers (MCP tools, CLI, eventual doctor/AI
// enrichment) wrap the response for their own surface.
//
// Resource model:
//   - One *Client per consumer process. Reuse across calls.
//   - Service address is resolved from etcd (no hardcoded endpoints).
//   - Empty address + no etcd registration → New() returns an error; callers
//     should surface a DEGRADED status to the user.
package awareness_graph_client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"time"

	awarenesspb "github.com/globulario/awareness-graph/golang/pb"
	"github.com/globulario/services/golang/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Option configures a Client at construction time. Use via New(addr, ...Option).
type Option func(*options)

type options struct {
	insecure bool
}

// WithInsecure disables TLS for the gRPC connection. Use only for localhost
// dev/smoke testing against a non-TLS awareness-graph instance (the standalone
// server defaults to plaintext). Production deployments MUST use mTLS — never
// pass this option when talking to a cluster service.
func WithInsecure() Option {
	return func(o *options) { o.insecure = true }
}

// ServiceName is the canonical etcd service registration key for the
// awareness-graph gRPC service. The awareness-graph repo registers itself
// under this name; this client looks it up via config.ResolveServiceAddr.
const ServiceName = "awareness.AwarenessGraphService"

// DefaultTimeout bounds individual RPC calls. Briefing can hit the Oxigraph
// backend with multi-clause SPARQL; the longer cap covers that.
const (
	DefaultResolveTimeout  = 5 * time.Second
	DefaultImpactTimeout   = 10 * time.Second
	DefaultBriefingTimeout = 15 * time.Second
	DefaultQueryTimeout    = 5 * time.Second
)

// Client is a stateful holder for the underlying gRPC connection.
// Construct with New, dispose with Close. Safe to call from multiple goroutines.
type Client struct {
	cc  *grpc.ClientConn
	pb  awarenesspb.AwarenessGraphClient
	addr string
}

// New constructs a client by resolving the awareness-graph service address
// from etcd and dialing over mTLS with the cluster CA.
//
// If addr is non-empty, it is used directly (useful for tests and dev). When
// addr is empty, the address is resolved via etcd; if no registration exists
// the function returns ErrServiceUnregistered so callers can surface a clear
// DEGRADED status rather than a generic dial failure.
//
// Pass WithInsecure() to disable TLS — only appropriate for localhost dev or
// smoke testing the standalone awareness-graph (which defaults to plaintext).
func New(addr string, opts ...Option) (*Client, error) {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	if addr == "" {
		addr = config.ResolveServiceAddr(ServiceName, "")
		if addr == "" {
			return nil, ErrServiceUnregistered
		}
	}
	target := config.ResolveDialTarget(addr)
	if target.Address == "" {
		return nil, fmt.Errorf("awareness_graph_client: invalid address %q", addr)
	}

	var creds grpc.DialOption
	if o.insecure {
		creds = grpc.WithTransportCredentials(insecure.NewCredentials())
	} else {
		tlsCfg, err := buildTLSConfig(target.ServerName)
		if err != nil {
			return nil, err
		}
		creds = grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg))
	}

	cc, err := grpc.NewClient(target.Address, creds)
	if err != nil {
		return nil, fmt.Errorf("awareness_graph_client: dial %s: %w", target.Address, err)
	}
	return &Client{
		cc:   cc,
		pb:   awarenesspb.NewAwarenessGraphClient(cc),
		addr: target.Address,
	}, nil
}

// ErrServiceUnregistered means awareness-graph has no entry in etcd. Callers
// should treat this as a DEGRADED-equivalent state: the service simply isn't
// deployed on this cluster yet.
var ErrServiceUnregistered = errors.New("awareness_graph_client: service not registered in etcd")

// Addr returns the resolved dial target. Useful for logs and error messages.
func (c *Client) Addr() string { return c.addr }

// Close releases the underlying gRPC connection. Safe to call once.
func (c *Client) Close() error {
	if c == nil || c.cc == nil {
		return nil
	}
	return c.cc.Close()
}

// Briefing returns the prose narration for a file or task. Exactly one of
// file or task should be set. Depth is one of "compact" (default), "standard",
// or "deep" — see the awareness-graph proto for token budgets.
//
// The returned response carries a BriefingStatus (OK/EMPTY/DEGRADED); callers
// must check it before treating prose as authoritative.
func (c *Client) Briefing(ctx context.Context, file, task, depth string) (*awarenesspb.BriefingResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultBriefingTimeout)
	defer cancel()
	return c.pb.Briefing(ctx, &awarenesspb.BriefingRequest{
		File:  file,
		Task:  task,
		Depth: depth,
	})
}

// Impact returns the direct + inferred awareness anchors that touch the given
// repo-relative file path. Direct nodes name the file in their protects/
// enforces/etc.; inferred nodes are reached via package, symbol, or service
// walks.
func (c *Client) Impact(ctx context.Context, file string) (*awarenesspb.ImpactResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultImpactTimeout)
	defer cancel()
	return c.pb.Impact(ctx, &awarenesspb.ImpactRequest{File: file})
}

// Resolve fetches one awareness node by (class, id). class names the unqualified
// class from ontology/awareness.ttl (e.g. "Invariant", "FailureMode",
// "IncidentPattern"); id is the bare ID without class prefix.
//
// On miss, the response's Found field is false and Node is nil.
func (c *Client) Resolve(ctx context.Context, class, id string) (*awarenesspb.ResolveResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultResolveTimeout)
	defer cancel()
	return c.pb.Resolve(ctx, &awarenesspb.ResolveRequest{Class: class, Id: id})
}

// Query runs a typed-mode query against the graph (BY_FILE, BY_ID, BY_CLASS,
// RELATED). The caller assembles the QueryRequest directly; this client only
// forwards.
func (c *Client) Query(ctx context.Context, req *awarenesspb.QueryRequest) (*awarenesspb.QueryResponse, error) {
	if req == nil {
		return nil, errors.New("awareness_graph_client: nil QueryRequest")
	}
	ctx, cancel := context.WithTimeout(ctx, DefaultQueryTimeout)
	defer cancel()
	return c.pb.Query(ctx, req)
}

// buildTLSConfig returns a tls.Config that verifies the awareness-graph server
// cert against the cluster CA, with ServerName pinned to the dial target's
// cert-valid hostname. Mirrors the pattern used by node_agent ↔ controller.
func buildTLSConfig(serverName string) (*tls.Config, error) {
	cfg := &tls.Config{ServerName: serverName}

	caPath := config.GetTLSFile("", "", "ca.crt")
	if caPath == "" {
		// Cluster CA not on this host (e.g. dev machine with no Globular
		// install). Fall back to the system root pool so a developer can
		// still talk to an awareness-graph reachable via a public-CA cert.
		// Production clusters always have the cluster CA on every node.
		return cfg, nil
	}

	data, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("awareness_graph_client: read cluster CA %s: %w", caPath, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return nil, fmt.Errorf("awareness_graph_client: parse cluster CA %s", caPath)
	}
	cfg.RootCAs = pool
	return cfg, nil
}
