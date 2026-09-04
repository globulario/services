package globular_client

// Option (c) for the cluster_uid membership gap: when a request is refused for
// want of a membership badge this client could not obtain, say WHY.
//
// Background. After cluster initialization the server-side interceptor requires
// cluster_uid on any request that is not bootstrap, mTLS, JWT, loopback, or an
// allowlisted method. The client attaches it from security.GetLocalClusterUID,
// which reads the UUID through the etcd client, which needs the cluster service
// keypair — material an unprivileged CLI may not be able to read. So a caller
// that is not already privileged cannot obtain the badge that proves it is a
// member, and its requests are refused the moment discovery routes them
// off-node (loopback being exempt is what makes this look intermittent rather
// than deterministic).
//
// Why this does NOT refuse pre-emptively. The obvious form of (c) — "if we have
// no cluster_uid and the cluster is initialized and the target is not loopback,
// fail before dialling" — is wrong here, and dangerously so. Services talk to
// each other over mTLS, which the server exempts, and the client interceptor
// cannot cheaply determine whether a given ClientConn negotiated mTLS. A
// pre-emptive refusal would therefore break inter-service calls that currently
// succeed through that exemption. Refusing a request we only SUSPECT will fail
// trades a legible error for an outage.
//
// So the refusal stays where it belongs — with the server, which knows the
// exemptions — and this file makes the answer diagnostic instead of opaque. The
// operator gets the root cause and where to look, rather than a bare
// Unauthenticated they cannot act on.
//
// This does not fix the circularity. See defect-5-cluster-uid.md and the
// contract_unknown entry of the same name for the two repairs that would, both
// of which move an authorization boundary.

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	clusterUIDMu sync.RWMutex
	// clusterUIDReason is why the last attempt to obtain the membership badge
	// failed. Empty means the badge was obtained, or has not been attempted.
	clusterUIDReason string
	clusterUIDLogged time.Time
)

// clusterUIDLogInterval bounds how often the unavailability is logged. The
// interceptor runs on every RPC; without this a broken client would drown its
// own logs in the one message an operator needs to read.
const clusterUIDLogInterval = 5 * time.Minute

// noteClusterUIDUnavailable records why the membership badge could not be
// attached. The error used to be discarded entirely, which is why this
// condition reached production as an unexplained intermittent refusal.
func noteClusterUIDUnavailable(err error) {
	reason := "unknown"
	if err != nil {
		reason = err.Error()
	}
	clusterUIDMu.Lock()
	clusterUIDReason = reason
	shouldLog := time.Since(clusterUIDLogged) > clusterUIDLogInterval
	if shouldLog {
		clusterUIDLogged = time.Now()
	}
	clusterUIDMu.Unlock()

	if !shouldLog {
		return
	}
	// Only worth saying once the cluster is past bootstrap: before that, an
	// absent cluster_uid is expected and every request is exempt anyway.
	if initialized, _ := security.IsClusterInitialized(context.Background()); !initialized {
		return
	}
	slog.Warn("cluster_uid unavailable — requests that leave this node will be refused",
		"reason", reason,
		"effect", "server enforces cluster_uid after initialization for non-loopback, non-mTLS, non-JWT calls")
}

// noteClusterUIDAvailable clears a previously recorded failure so a recovered
// client stops attributing later errors to a condition it no longer has.
func noteClusterUIDAvailable() {
	clusterUIDMu.Lock()
	clusterUIDReason = ""
	clusterUIDMu.Unlock()
}

// annotateClusterUIDRefusal turns the server's opaque membership refusal into
// one that names the cause. Any other error is returned untouched — this must
// never reshape an error it does not understand.
func annotateClusterUIDRefusal(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unauthenticated {
		return err
	}
	if !strings.Contains(st.Message(), "cluster_uid") {
		return err
	}

	clusterUIDMu.RLock()
	reason := clusterUIDReason
	clusterUIDMu.RUnlock()
	if reason == "" {
		// The badge was available, so the refusal is about something else —
		// a mismatched membership, say. Do not invent a diagnosis.
		return err
	}

	return status.Error(codes.Unauthenticated, st.Message()+
		" — this client could not attach cluster_uid: "+reason+
		". The membership UUID is read through the etcd client, which requires the"+
		" cluster service keypair under /var/lib/globular/pki/issued/services/;"+
		" a caller that cannot read it can only reach services on this node,"+
		" because loopback is the one path the server exempts. Run as a user"+
		" that can read the service keypair, or use a JWT-authenticated client")
}
