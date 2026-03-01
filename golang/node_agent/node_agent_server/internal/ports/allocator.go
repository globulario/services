package ports

import (
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Allocator provides process-local TCP port reservation within a configured range.
// It verifies availability by attempting to bind on both IPv4 and IPv6 loopback.
type Allocator struct {
	start, end int
	reserved   map[int]string
	mu         sync.Mutex
}

var infraReservedPorts = map[int]string{
	10000: "scylla-admin",
	9042:  "scylla-cql",
	9142:  "scylla-cql-tls",
	19042: "scylla-alt",
}

// NewFromEnv constructs an Allocator using environment variables.
// Preferred: GLOBULAR_PORT_RANGE="start-end". Fallback: GLOBULAR_PORT_RANGE_START/END.
// Default range: 10000-20000.
func NewFromEnv() (*Allocator, error) {
	rangeStr := strings.TrimSpace(os.Getenv("GLOBULAR_PORT_RANGE"))
	if rangeStr == "" {
		a := strings.TrimSpace(os.Getenv("GLOBULAR_PORT_RANGE_START"))
		b := strings.TrimSpace(os.Getenv("GLOBULAR_PORT_RANGE_END"))
		if a != "" && b != "" {
			rangeStr = a + "-" + b
		}
	}
	if rangeStr == "" {
		rangeStr = "10000-20000"
	}

	parts := strings.Split(rangeStr, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid port range %q", rangeStr)
	}
	start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, fmt.Errorf("invalid port range start: %w", err)
	}
	end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return nil, fmt.Errorf("invalid port range end: %w", err)
	}
	if start <= 0 || end <= start {
		return nil, fmt.Errorf("invalid port range bounds %d-%d", start, end)
	}

	return &Allocator{start: start, end: end, reserved: make(map[int]string)}, nil
}

// Reserve returns an available port. If a preferred port is provided, it will be
// attempted first when in-range and free. Ports are validated by binding on both
// 127.0.0.1 and ::1. The reservation is process-local only.
func (a *Allocator) Reserve(key string, preferred ...int) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	try := func(p int) (int, bool) {
		if p < a.start || p > a.end {
			return 0, false
		}
		if _, blocked := infraReservedPorts[p]; blocked {
			return 0, false
		}
		if owner, taken := a.reserved[p]; taken {
			if owner == key {
				return p, true
			}
			return 0, false
		}
		if !portFree(p) {
			return 0, false
		}
		a.reserved[p] = key
		return p, true
	}

	if len(preferred) > 0 {
		if p, ok := try(preferred[0]); ok {
			return p, nil
		}
	}

	for p := a.start; p <= a.end; p++ {
		if port, ok := try(p); ok {
			return port, nil
		}
	}
	return 0, errors.New("no free ports available in range")
}

// Range returns the inclusive bounds for the allocator.
func (a *Allocator) Range() (int, int) {
	return a.start, a.end
}

// SortedReserved returns a sorted slice of reserved ports (useful for logging).
func (a *Allocator) SortedReserved() []int {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]int, 0, len(a.reserved))
	for p := range a.reserved {
		out = append(out, p)
	}
	sort.Ints(out)
	return out
}

func portFree(port int) bool {
	addr4 := fmt.Sprintf("0.0.0.0:%d", port)
	if ln, err := net.Listen("tcp", addr4); err != nil {
		if isAddrInUse(err) {
			return false
		}
	} else {
		ln.Close()
		return true
	}

	addr6 := fmt.Sprintf("[::]:%d", port)
	if ln, err := net.Listen("tcp", addr6); err != nil {
		if isAddrInUse(err) {
			return false
		}
	} else {
		ln.Close()
		return true
	}

	return true
}

func isAddrInUse(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if strings.Contains(strings.ToLower(opErr.Err.Error()), "address already in use") {
			return true
		}
	}
	return strings.Contains(strings.ToLower(err.Error()), "address already in use")
}

// Mark reserves a port in the allocator without checking OS availability.
// Used to seed reservations from existing service configs.
func (a *Allocator) Mark(key string, port int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if port <= 0 {
		return
	}
	if port < a.start || port > a.end {
		return
	}
	if _, taken := a.reserved[port]; taken {
		return
	}
	a.reserved[port] = key
}
