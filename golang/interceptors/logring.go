package interceptors

import (
	"sync"
	"time"
)

// LogEntry is a structured log entry captured by the interceptor ring buffer.
type LogEntry struct {
	Timestamp   time.Time         `json:"timestamp"`
	Level       string            `json:"level"`       // TRACE, DEBUG, INFO, WARN, ERROR
	Service     string            `json:"service"`      // gRPC service name (e.g. "dns.DnsService")
	Method      string            `json:"method"`       // full gRPC method (e.g. "/dns.DnsService/CreateZone")
	Subject     string            `json:"subject"`      // caller identity
	RemoteAddr  string            `json:"remote_addr"`  // source IP:port
	DurationMs  int64             `json:"duration_ms"`  // request duration
	StatusCode  string            `json:"status_code"`  // gRPC status code ("OK", "PermissionDenied", etc.)
	Message     string            `json:"message"`      // human-readable summary
	Fields      map[string]string `json:"fields,omitempty"` // additional structured fields
}

// LogRing is a thread-safe circular buffer of LogEntry.
type LogRing struct {
	mu      sync.RWMutex
	entries []LogEntry
	head    int  // next write position
	count   int  // current number of entries
	cap     int  // max capacity
}

// NewLogRing creates a ring buffer with the given capacity.
func NewLogRing(capacity int) *LogRing {
	if capacity <= 0 {
		capacity = 10000
	}
	return &LogRing{
		entries: make([]LogEntry, capacity),
		cap:     capacity,
	}
}

// Push adds an entry to the ring, overwriting the oldest if full.
func (r *LogRing) Push(entry LogEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[r.head] = entry
	r.head = (r.head + 1) % r.cap
	if r.count < r.cap {
		r.count++
	}
}

// Query returns entries matching the filter, newest first.
// All filter fields are optional — empty means "match all".
func (r *LogRing) Query(filter LogFilter) []LogEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}

	var results []LogEntry

	// Walk backwards from newest to oldest
	for i := 0; i < r.count && len(results) < limit; i++ {
		idx := (r.head - 1 - i + r.cap) % r.cap
		entry := r.entries[idx]

		if matchesFilter(entry, filter) {
			results = append(results, entry)
		}
	}

	return results
}

// Count returns the current number of entries in the ring.
func (r *LogRing) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.count
}

// LogFilter controls which log entries are returned by Query.
type LogFilter struct {
	Level     string    // minimum level: TRACE, DEBUG, INFO, WARN, ERROR
	Service   string    // substring match on service name
	Method    string    // substring match on method
	Pattern   string    // substring match on message
	Subject   string    // exact match on subject
	Since     time.Time // entries after this time
	Until     time.Time // entries before this time
	Limit     int       // max results (default 100)
}

func matchesFilter(e LogEntry, f LogFilter) bool {
	if f.Level != "" && levelRank(e.Level) < levelRank(f.Level) {
		return false
	}
	if f.Service != "" && !containsCI(e.Service, f.Service) {
		return false
	}
	if f.Method != "" && !containsCI(e.Method, f.Method) {
		return false
	}
	if f.Pattern != "" && !containsCI(e.Message, f.Pattern) {
		return false
	}
	if f.Subject != "" && e.Subject != f.Subject {
		return false
	}
	if !f.Since.IsZero() && e.Timestamp.Before(f.Since) {
		return false
	}
	if !f.Until.IsZero() && e.Timestamp.After(f.Until) {
		return false
	}
	return true
}

func levelRank(level string) int {
	switch level {
	case "TRACE":
		return 0
	case "DEBUG":
		return 1
	case "INFO":
		return 2
	case "WARN":
		return 3
	case "ERROR":
		return 4
	default:
		return 0
	}
}

func containsCI(haystack, needle string) bool {
	// Simple case-insensitive contains
	h := []byte(haystack)
	n := []byte(needle)
	if len(n) > len(h) {
		return false
	}
	for i := 0; i <= len(h)-len(n); i++ {
		match := true
		for j := 0; j < len(n); j++ {
			a, b := h[i+j], n[j]
			if a >= 'A' && a <= 'Z' {
				a += 32
			}
			if b >= 'A' && b <= 'Z' {
				b += 32
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
