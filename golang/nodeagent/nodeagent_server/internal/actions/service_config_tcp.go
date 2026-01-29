package actions

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
)

type serviceConfigTCPProbe struct{}

func (serviceConfigTCPProbe) Name() string { return "probe.service_config_tcp" }

func (serviceConfigTCPProbe) Validate(args *structpb.Struct) error {
	if args == nil {
		return errors.New("args required")
	}
	if strings.TrimSpace(args.GetFields()["service"].GetStringValue()) == "" {
		return errors.New("service is required")
	}
	return nil
}

func (serviceConfigTCPProbe) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	svc := strings.TrimSpace(args.GetFields()["service"].GetStringValue())
	if svc == "" {
		return "", errors.New("service is required")
	}
	exe := executableForService(svc)
	if exe == "" {
		return "", fmt.Errorf("unknown service %s", svc)
	}

	binDir := installBinDir()
	desc, err := runDescribe(ctx, filepath.Join(binDir, exe))
	if err != nil {
		return "", err
	}
	if desc == nil || desc.Id == "" {
		return "", fmt.Errorf("describe missing Id for %s", svc)
	}

	state := stateRoot()
	cfgPath := filepath.Join(state, "services", desc.Id+".json")
	cfg, err := readServiceConfig(cfgPath)
	if err != nil {
		return "", fmt.Errorf("read config: %w", err)
	}

	port := firstPort(cfg.Port, portFromAddress(cfg.Address), desc.Port)
	if port <= 0 {
		return "", fmt.Errorf("missing port in config %s", cfgPath)
	}

	timeout := 5 * time.Second
	if to := args.GetFields()["timeout_ms"].GetNumberValue(); to > 0 {
		timeout = time.Duration(int64(to)) * time.Millisecond
	}
	dialer := &net.Dialer{Timeout: timeout}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		// try IPv6 loopback as fallback
		conn6, err6 := dialer.DialContext(ctx, "tcp", fmt.Sprintf("[::1]:%d", port))
		if err6 != nil {
			return "", err
		}
		conn6.Close()
		return fmt.Sprintf("tcp %s ok", addr), nil
	}
	conn.Close()
	return fmt.Sprintf("tcp %s ok", addr), nil
}

func init() {
	Register(serviceConfigTCPProbe{})
}

func installBinDir() string {
	if v := strings.TrimSpace(os.Getenv("GLOBULAR_INSTALL_BIN_DIR")); v != "" {
		return v
	}
	return "/usr/local/bin"
}

func stateRoot() string {
	if v := strings.TrimSpace(os.Getenv("GLOBULAR_STATE_DIR")); v != "" {
		return v
	}
	return "/var/lib/globular"
}

func firstPort(values ...int) int {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}
