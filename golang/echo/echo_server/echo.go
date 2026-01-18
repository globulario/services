package main

import (
	"context"
	"log/slog"

	"github.com/globulario/services/golang/echo/echopb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Echo implements echopb.EchoService.Echo.
// It persists the service configuration (mirroring the existing behavior) and
// returns the message back to the caller. On error, a structured gRPC status is returned.
func (srv *server) Echo(ctx context.Context, rqst *echopb.EchoRequest) (*echopb.EchoResponse, error) {
	// Log the incoming request (message length to avoid leaking very large payloads).
	slog.With(
		"service", srv.Name,
		"id", srv.Id,
		"msg_len", len(rqst.GetMessage()),
	).Info("echo request")

	// Persist config as in the original implementation.
	if err := srv.Save(); err != nil {
		slog.With(
			"service", srv.Name,
			"id", srv.Id,
			"err", err,
		).Error("config save failed on echo")
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}

	// Successful echo.
	resp := &echopb.EchoResponse{Message: rqst.GetMessage()}
	slog.With(
		"service", srv.Name,
		"id", srv.Id,
	).Info("echo response sent")

	return resp, nil
}
