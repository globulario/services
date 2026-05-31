package main

import (
	"context"
	"log/slog"

	"github.com/globulario/services/golang/echo/echopb"
)

// Echo implements echopb.EchoService.Echo.
// It returns the message back to the caller as pure business logic.
// NOTE: Config persistence removed in Phase 1 Step 2 - this is now a pure handler
// with no side effects. Config saving, if needed, should be done at the lifecycle
// level, not per-request.
//
// @awareness namespace=globular.examples.echo_service
// @awareness component=server.handler
// @awareness implements=globular.examples.echo_service:intent.echo_rpc_returns_input_unchanged
// @awareness enforces=globular.examples.echo_service:invariant.echo_rpc_is_stateless
// @awareness enforces=globular.examples.echo_service:invariant.echo_rpc_preserves_message_bytes
// @awareness tested_by=golang/echo/echo_server/server_test.go:TestEchoHandler
// @awareness tested_by=golang/echo/echo_server/server_test.go:TestBehaviorInvariant
// @awareness risk=low
func (srv *server) Echo(ctx context.Context, rqst *echopb.EchoRequest) (*echopb.EchoResponse, error) {
	// Log the incoming request (message length to avoid leaking very large payloads).
	slog.With(
		"service", srv.Name,
		"id", srv.Id,
		"msg_len", len(rqst.GetMessage()),
	).Info("echo request")

	// Pure echo - no side effects
	resp := &echopb.EchoResponse{Message: rqst.GetMessage()}

	slog.With(
		"service", srv.Name,
		"id", srv.Id,
	).Info("echo response sent")

	return resp, nil
}
