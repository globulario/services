package actions

import (
	"context"
	"errors"
	"fmt"
	"net"
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

	serviceId := resolveServiceId(svc)
	if serviceId == "" {
		return "", fmt.Errorf("unknown service %s (not in identity registry)", svc)
	}

	cfgPath := filepath.Join(stateRoot(), "services", serviceId+".json")
	cfg, err := readServiceConfig(cfgPath)
	if err != nil {
		// Config file missing or unreadable — skip probe with warning.
		fmt.Printf("WARN probe.service_config_tcp: config %s unreadable for %s: %v\n", cfgPath, svc, err)
		return fmt.Sprintf("skipped: config unreadable for %s (%v)", svc, err), nil
	}

	port := firstPort(cfg.Port, portFromAddress(cfg.Address))
	if port <= 0 {
		fmt.Printf("WARN probe.service_config_tcp: no port in config %s for %s (incomplete config?)\n", cfgPath, svc)
		return fmt.Sprintf("skipped: no port in config for %s", svc), nil
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

// ActionBinDir and ActionStateDir are package-level defaults.
// Tests may override before calling actions.
var (
	ActionBinDir   = "/usr/lib/globular/bin"
	ActionStateDir = "/var/lib/globular"

	// ActionStagingRoot overrides the staging root used by service.install_payload.
	// When non-empty, staging goes to filepath.Join(ActionStagingRoot, service)
	// instead of filepath.Join(ActionStateDir, "staging", service).
	ActionStagingRoot string

	// ActionSkipSystemd skips systemd unit installation when true.
	ActionSkipSystemd bool

	// ActionConfigDir is the base directory for service configuration files.
	ActionConfigDir = "/etc/globular"

	// ActionSystemdDir is the directory for systemd unit files.
	ActionSystemdDir = "/etc/systemd/system"

	// AllowMissingSHA256 controls whether artifact verification accepts
	// missing SHA256 digests. Default false (production).
	AllowMissingSHA256 bool

	// ActionArtifactRepoRoot is the root directory for local artifact resolution.
	ActionArtifactRepoRoot = "/var/lib/globular/repository/artifacts"
)

func installBinDir() string { return ActionBinDir }
func stateRoot() string     { return ActionStateDir }

func firstPort(values ...int) int {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}
