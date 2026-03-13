package actions

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/protobuf/types/known/structpb"
)

// grpcHealthProbeAction implements the "probe.grpc_health" action.
// It checks a gRPC service's health status via the standard grpc.health.v1.Health/Check RPC.
//
// Args:
//   - address (string, required): host:port of the gRPC server
//   - timeout_ms (number, optional): timeout in milliseconds (default 5000)
//   - service (string, optional): service name for the health check (default "" = overall server health)
type grpcHealthProbeAction struct{}

func (grpcHealthProbeAction) Name() string { return "probe.grpc_health" }

func (grpcHealthProbeAction) Validate(args *structpb.Struct) error {
	if args == nil {
		return errors.New("args required")
	}
	if args.GetFields()["address"].GetStringValue() == "" {
		return errors.New("address is required")
	}
	return nil
}

func (grpcHealthProbeAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	address := strings.TrimSpace(args.GetFields()["address"].GetStringValue())
	service := strings.TrimSpace(args.GetFields()["service"].GetStringValue())

	timeout := 5 * time.Second
	if to := args.GetFields()["timeout_ms"].GetNumberValue(); to > 0 {
		timeout = time.Duration(int64(to)) * time.Millisecond
	}

	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return "", fmt.Errorf("grpc dial %s: %w", address, err)
	}
	defer conn.Close()

	client := healthpb.NewHealthClient(conn)
	resp, err := client.Check(dialCtx, &healthpb.HealthCheckRequest{
		Service: service,
	})
	if err != nil {
		return "", fmt.Errorf("grpc health check %s: %w", address, err)
	}

	if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		return "", fmt.Errorf("grpc health %s: status %s (expected SERVING)", address, resp.GetStatus())
	}

	return fmt.Sprintf("grpc_health %s SERVING", address), nil
}

func init() {
	Register(grpcHealthProbeAction{})
}
