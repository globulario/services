package globular_service

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	Utility "github.com/globulario/utility"
)

// CLI Helper Functions - Phase 2 Step 1
//
// These functions are extracted from Echo, Discovery, and Repository services.
// They handle common CLI operations like --describe, --health, port allocation, etc.
//
// Services use the existing Service interface defined in services.go.

// HandleInformationalFlags processes --describe, --health, --help, --version.
// Returns true if a flag was handled and the program should exit.
//
// The printUsage function must be provided by the caller (service-specific).
//
// Note: --debug flag should be handled by the service before calling this function,
// as it typically modifies a global logger variable.
func HandleInformationalFlags(srv Service, args []string, logger *slog.Logger, printUsage func()) bool {
	for _, a := range args {
		switch strings.ToLower(a) {
		case "--describe":
			HandleDescribeFlag(srv, logger)
			return true
		case "--health":
			HandleHealthFlag(srv, logger)
			return true
		case "--help", "-h", "/?":
			printUsage()
			return true
		case "--version", "-v":
			fmt.Println(srv.GetVersion())
			return true
		}
	}
	return false
}

// HandleDescribeFlag handles the --describe flag.
// Outputs service metadata as JSON to stdout and exits.
func HandleDescribeFlag(srv Service, logger *slog.Logger) {
	srv.SetProcess(os.Getpid())
	srv.SetState("starting")

	// Set harmless defaults for Domain/Address without hitting etcd
	if v, ok := os.LookupEnv("GLOBULAR_DOMAIN"); ok && v != "" {
		srv.SetDomain(strings.ToLower(v))
	} else {
		srv.SetDomain("localhost")
	}
	if v, ok := os.LookupEnv("GLOBULAR_ADDRESS"); ok && v != "" {
		srv.SetAddress(strings.ToLower(v))
	} else {
		srv.SetAddress("localhost:" + Utility.ToString(srv.GetPort()))
	}

	b, err := DescribeJSON(srv)
	if err != nil {
		logger.Error("describe error", "service", srv.GetName(), "id", srv.GetId(), "err", err)
		os.Exit(2)
	}
	_, _ = os.Stdout.Write(b)
	_, _ = os.Stdout.Write([]byte("\n"))
}

// HandleHealthFlag handles the --health flag.
// Performs health check and outputs JSON to stdout, then exits.
func HandleHealthFlag(srv Service, logger *slog.Logger) {
	if srv.GetPort() == 0 || srv.GetName() == "" {
		logger.Error("health error: uninitialized", "service", srv.GetName(), "port", srv.GetPort())
		os.Exit(2)
	}

	b, err := HealthJSON(srv, &HealthOptions{
		Timeout:     1500 * time.Millisecond,
		ServiceName: "",
	})
	if err != nil {
		logger.Error("health error", "service", srv.GetName(), "id", srv.GetId(), "err", err)
		os.Exit(2)
	}
	_, _ = os.Stdout.Write(b)
	_, _ = os.Stdout.Write([]byte("\n"))
}

// ParsePositionalArgs extracts service ID and config path from positional arguments.
// Ignores flag arguments (those starting with "-").
func ParsePositionalArgs(srv Service, args []string) {
	nonFlagArgs := []string{}
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			nonFlagArgs = append(nonFlagArgs, arg)
		}
	}

	if len(nonFlagArgs) >= 1 {
		srv.SetId(nonFlagArgs[0])
	}
	if len(nonFlagArgs) >= 2 {
		srv.SetConfigurationPath(nonFlagArgs[1])
	}
}

// AllocatePortIfNeeded allocates a port for the service if no arguments were provided.
// Generates a UUID for the service ID and allocates the next available port.
func AllocatePortIfNeeded(srv Service, args []string) error {
	if len(args) == 0 {
		srv.SetId(Utility.GenerateUUID(srv.GetName() + ":" + srv.GetAddress()))
		allocator, err := config.NewDefaultPortAllocator()
		if err != nil {
			return fmt.Errorf("port allocator creation failed: %w", err)
		}

		p, err := allocator.Next(srv.GetId())
		if err != nil {
			return fmt.Errorf("port allocation failed: %w", err)
		}
		srv.SetPort(p)
	}
	return nil
}

// LoadRuntimeConfig loads domain and address from config backend (etcd or file).
// Falls back to "localhost" if domain is not configured.
func LoadRuntimeConfig(srv Service) {
	if d, err := config.GetDomain(); err == nil {
		srv.SetDomain(d)
	} else {
		srv.SetDomain("localhost")
	}
	if a, err := config.GetAddress(); err == nil && strings.TrimSpace(a) != "" {
		srv.SetAddress(a)
	}
}
