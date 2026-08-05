package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/globulario/services/golang/oci"
	ocidocker "github.com/globulario/services/golang/oci/docker"
)

const usageText = `globular-oci-runner manages one persistent OCI-backed Globular service.

Usage:
  globular-oci-runner validate      --spec FILE [--policy FILE]
  globular-oci-runner capabilities  [--socket ENDPOINT]
  globular-oci-runner run           --spec FILE [common options]
  globular-oci-runner status        --spec FILE [--state-root DIR]

Common options:
  --policy FILE       node-owned OCI admission policy (default: built-in safe policy)
  --socket ENDPOINT   Docker Engine endpoint (default: unix:///var/run/docker.sock)
  --state-root DIR    observed-state root (default: /var/lib/globular/oci)
`

type options struct {
	specPath  string
	policy    string
	socket    string
	stateRoot string
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "globular-oci-runner:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(stdout, usageText)
		return nil
	}
	command := args[0]
	if command == "capabilities" {
		fs := flag.NewFlagSet(command, flag.ContinueOnError)
		fs.SetOutput(stderr)
		socket := fs.String("socket", ocidocker.DefaultSocket, "Docker Engine endpoint")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		runtime, err := ocidocker.NewRuntime(*socket)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		capabilities, err := runtime.Capabilities(ctx)
		if err != nil {
			return err
		}
		return writeJSON(stdout, capabilities)
	}

	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts options
	fs.StringVar(&opts.specPath, "spec", "", "OCI service specification")
	fs.StringVar(&opts.policy, "policy", "", "node-owned OCI policy")
	fs.StringVar(&opts.socket, "socket", ocidocker.DefaultSocket, "Docker Engine endpoint")
	fs.StringVar(&opts.stateRoot, "state-root", "/var/lib/globular/oci", "observed-state root")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if opts.specPath == "" {
		return errors.New("--spec is required")
	}
	spec, err := oci.LoadSpec(opts.specPath)
	if err != nil {
		return err
	}
	policy, err := oci.LoadPolicy(opts.policy)
	if err != nil {
		return err
	}
	if command == "validate" {
		if err := oci.Validate(spec, policy); err != nil {
			return err
		}
		digest, err := spec.SpecDigest()
		if err != nil {
			return err
		}
		return writeJSON(stdout, map[string]any{
			"valid":          true,
			"service":        spec.Metadata.Name,
			"instance":       spec.Metadata.Instance,
			"container_name": spec.ContainerName(),
			"image":          spec.CanonicalImageReference(),
			"spec_digest":    digest,
		})
	}
	if command == "status" {
		state, err := (oci.FileStateStore{Root: opts.stateRoot}).Read(spec.Metadata.Name, spec.Metadata.Instance)
		if err != nil {
			return err
		}
		return writeJSON(stdout, state)
	}
	if command != "run" {
		return fmt.Errorf("unknown command %q\n%s", command, usageText)
	}

	runtime, err := ocidocker.NewRuntime(opts.socket)
	if err != nil {
		return err
	}
	reconciler := &oci.Reconciler{
		Runtime: runtime,
		Policy:  policy,
		State:   oci.FileStateStore{Root: filepath.Clean(opts.stateRoot)},
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	return reconciler.Run(ctx, spec, stdout, stderr)
}

func writeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
