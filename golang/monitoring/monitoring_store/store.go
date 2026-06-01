package monitoring_store

import (
	"context"
	"time"
)

// Store defines the contract for a monitoring data store (e.g., Prometheus).
// Each implementation is expected to map these calls to the underlying system's
// API, typically returning JSON-encoded results or raising errors.
//
// All methods are synchronous and respect the provided context for cancellation.
type Store interface {
	// Alerts returns all currently active alerts in JSON format.
	Alerts(ctx context.Context) (string, error)

	// AlertManagers returns the state of the Prometheus Alertmanager discovery in JSON format.
	AlertManagers(ctx context.Context) (string, error)

	// CleanTombstones removes deleted data from disk and cleans up existing tombstones.
	CleanTombstones(ctx context.Context) error

	// Config returns the current Prometheus configuration in JSON format.
	Config(ctx context.Context) (string, error)

	// DeleteSeries deletes data for the given matchers within the provided time range.
	DeleteSeries(ctx context.Context, matches []string, startTime, endTime time.Time) error

	// Flags returns the startup flag values Prometheus was launched with, as JSON.
	Flags(ctx context.Context) (string, error)

	// LabelNames returns all unique label names and any warnings.
	// The first return value is the label names slice,
	// the second is the warnings as JSON.
	LabelNames(ctx context.Context) ([]string, string, error)

	// LabelValues returns all values for the given label within a time range.
	// The first return value is the values JSON,
	// the second is warnings JSON.
	LabelValues(ctx context.Context, label string, values []string, startTime, endTime int64) (string, string, error)

	// Query performs an instant query at the given timestamp.
	// Returns results and warnings as JSON.
	Query(ctx context.Context, query string, ts time.Time) (string, string, error)

	// QueryRange performs a query over a time range with the given step (milliseconds).
	// Returns results and warnings as JSON.
	QueryRange(ctx context.Context, query string, startTime, endTime time.Time, step float64) (string, string, error)

	// Series finds series by label matchers in a time range.
	// Returns results and warnings as JSON.
	Series(ctx context.Context, matches []string, startTime, endTime time.Time) (string, string, error)

	// Snapshot creates a snapshot of all current data under snapshots/<datetime>-<rand>
	// in the TSDB's data directory. Returns the directory path as JSON.
	Snapshot(ctx context.Context, skipHead bool) (string, error)

	// Rules returns all alerting and recording rules currently loaded, as JSON.
	Rules(ctx context.Context) (string, error)

	// Targets returns the current state of target discovery in JSON format.
	Targets(ctx context.Context) (string, error)

	// TargetsMetadata returns metadata about metrics currently scraped by the target.
	// Returns results in JSON format.
	TargetsMetadata(ctx context.Context, matchTarget, metric, limit string) (string, error)
}
