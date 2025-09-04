package main

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"github.com/globulario/services/golang/security"
)

// errf wraps an error with a clear, consistent message.
// Example: return errf("failed opening index %q", path, err)
func errf(msg string, err error, args ...any) error {
	if err == nil {
		return nil
	}
	prefix := fmt.Sprintf(msg, args...)
	return fmt.Errorf("%s: %w", prefix, err)
}

// here returns the caller function and file:line to aid debugging.
func here(skip int) (fn, file string, line int) {
	pc, f, l, _ := runtime.Caller(skip + 1)
	d := runtime.FuncForPC(pc)
	name := ""
	if d != nil {
		name = d.Name()
	}
	return name, f, l
}

// trace logs a debug line with the caller location.
func trace(msg string, kv ...any) {
	fn, file, line := here(1)
	logger.Debug(msg, append([]any{"fn", fn, "file", fmt.Sprintf("%s:%d", file, line)}, kv...)...)
}

// checkArg is a tiny helper to validate required string arguments.
func checkArg(name, val string) error {
	if len(val) == 0 {
		return fmt.Errorf("missing required %s", name)
	}
	return nil
}

// checkNotNil validates a required pointer argument.
func checkNotNil[T any](name string, ptr *T) error {
	if ptr == nil {
		return fmt.Errorf("missing required %s", name)
	}
	return nil
}

// clientIDFromCtx extracts the client ID from context using security.GetClientId.
func clientIDFromCtx(ctx context.Context) (string, error) {
	id, _, err := security.GetClientId(ctx)
	if err != nil {
		return "", errf("resolve client id", err)
	}
	if id == "" {
		return "", errors.New("empty client id")
	}
	return id, nil
}
