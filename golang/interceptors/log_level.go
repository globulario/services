package interceptors

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"google.golang.org/grpc/status"
)

// Global interceptor log ring buffer — shared across all services in a process.
// Capacity: 10,000 entries (~5-10 MB depending on field sizes).
var globalRing = NewLogRing(10000)

// GetLogRing returns the global interceptor log ring buffer.
func GetLogRing() *LogRing {
	return globalRing
}

// interceptorLogLevel controls the verbosity of interceptor logging.
// 0=TRACE, 1=DEBUG, 2=INFO (default), 3=WARN, 4=ERROR
var interceptorLogLevel atomic.Int32

func init() {
	interceptorLogLevel.Store(2) // default: INFO
}

// SetInterceptorLogLevel changes the interceptor verbosity at runtime.
// Valid levels: "TRACE", "DEBUG", "INFO", "WARN", "ERROR"
func SetInterceptorLogLevel(level string) error {
	rank := levelRank(strings.ToUpper(level))
	if rank == 0 && strings.ToUpper(level) != "TRACE" {
		return fmt.Errorf("invalid log level %q — use TRACE, DEBUG, INFO, WARN, ERROR", level)
	}
	interceptorLogLevel.Store(int32(rank))
	return nil
}

// GetInterceptorLogLevel returns the current interceptor log level as a string.
func GetInterceptorLogLevel() string {
	switch interceptorLogLevel.Load() {
	case 0:
		return "TRACE"
	case 1:
		return "DEBUG"
	case 2:
		return "INFO"
	case 3:
		return "WARN"
	case 4:
		return "ERROR"
	default:
		return "INFO"
	}
}

// shouldLog returns true if the given level should be logged at the current verbosity.
func shouldLog(level string) bool {
	return levelRank(level) >= int(interceptorLogLevel.Load())
}

// EmitLog writes a structured log entry to the ring buffer if the level passes the filter.
// This is the primary entry point for interceptor instrumentation.
func EmitLog(level, service, method, subject, remoteAddr, statusCode, message string, durationMs int64, fields map[string]string) {
	if !shouldLog(level) {
		return
	}
	globalRing.Push(LogEntry{
		Timestamp:  time.Now().UTC(),
		Level:      level,
		Service:    service,
		Method:     method,
		Subject:    subject,
		RemoteAddr: remoteAddr,
		DurationMs: durationMs,
		StatusCode: statusCode,
		Message:    message,
		Fields:     fields,
	})
}

// EmitRequestLog is a convenience wrapper for logging a completed gRPC request.
func EmitRequestLog(method, subject, remoteAddr string, duration time.Duration, err error) {
	code := "OK"
	level := "TRACE"
	msg := "request completed"

	if err != nil {
		if st, ok := status.FromError(err); ok {
			code = st.Code().String()
		} else {
			code = "Unknown"
		}
		// Elevate level based on error type
		switch code {
		case "PermissionDenied", "Unauthenticated":
			level = "WARN"
			msg = "request denied: " + code
		case "Internal", "Unknown":
			level = "ERROR"
			msg = "request failed: " + err.Error()
		default:
			level = "INFO"
			msg = "request error: " + code
		}
	}

	svc := applicationFromMethod(method)
	EmitLog(level, svc, method, subject, remoteAddr, code, msg, duration.Milliseconds(), nil)
}
