package main

import (
	"context"
	"log"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// autoInitiateJoin submits a join request to the controller when a join token
// is present in state but no request has been submitted yet. It retries until
// the controller is reachable. Once the request is accepted it hands off to
// startJoinApprovalWatcher to poll for admission.
//
// Called once at startup (as a goroutine) if joinToken != "" && joinRequestID == "".
// Stops when ctx is cancelled or the request is accepted/permanently rejected.
func (srv *NodeAgentServer) autoInitiateJoin(ctx context.Context) {
	token := srv.joinToken
	if token == "" {
		return
	}
	log.Printf("auto-join: starting (controller=%s)", srv.controllerEndpoint)

	backoff := 5 * time.Second
	const maxBackoff = 60 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Another code path may have submitted the request already.
		srv.stateMu.Lock()
		alreadyRequested := srv.joinRequestID != "" || srv.joinToken == ""
		srv.stateMu.Unlock()
		if alreadyRequested {
			log.Printf("auto-join: request already in flight or token cleared — stopping")
			return
		}

		if err := srv.ensureControllerClient(ctx); err != nil {
			log.Printf("auto-join: controller unreachable: %v — retry in %s", err, backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
				if backoff < maxBackoff {
					backoff *= 2
				}
			}
			continue
		}

		resp, err := srv.controllerClient.RequestJoin(ctx, &cluster_controllerpb.RequestJoinRequest{
			JoinToken:    token,
			Identity:     srv.buildNodeIdentity(),
			Labels:       srv.joinRequestLabels(),
			Capabilities: buildNodeCapabilities(),
		})
		if err != nil {
			if st, ok := status.FromError(err); ok {
				switch st.Code() {
				case codes.PermissionDenied, codes.NotFound, codes.InvalidArgument:
					log.Printf("auto-join: non-retriable rejection from controller: %v", err)
					return
				}
			}
			log.Printf("auto-join: RequestJoin failed: %v — retry in %s", err, backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
				if backoff < maxBackoff {
					backoff *= 2
				}
			}
			continue
		}

		requestID := resp.GetRequestId()
		if requestID == "" {
			log.Printf("auto-join: controller returned empty request_id — retry in %s", backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			continue
		}

		srv.stateMu.Lock()
		srv.joinRequestID = requestID
		srv.state.RequestID = requestID
		srv.stateMu.Unlock()
		if err := srv.saveState(); err != nil {
			log.Printf("auto-join: warn: persist join request state: %v", err)
		}
		log.Printf("auto-join: join request submitted (id=%s status=%s) — polling for approval", requestID, resp.GetStatus())
		srv.startJoinApprovalWatcher(ctx, requestID)
		return
	}
}
