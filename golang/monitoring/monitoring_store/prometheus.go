package monitoring_store

import (
	"context"
	"log/slog"
	"time"

	Utility "github.com/globulario/utility"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

// PrometheusStore implements the Store interface on top of the Prometheus HTTP API.
type PrometheusStore struct {
	c v1.API
}

// NewPrometheusStore creates a new Prometheus-backed Store using the given HTTP endpoint.
// The address should include a scheme (e.g., "http://host:9090").
func NewPrometheusStore(address string) (Store, error) {
	client, err := api.NewClient(api.Config{
		Address: address,
	})
	if err != nil {
		slog.Error("prometheus: failed to create API client", "address", address, "err", err)
		return nil, err
	}
	store := &PrometheusStore{c: v1.NewAPI(client)}
	slog.Info("prometheus: store initialized", "address", address)
	return store, nil
}

// Alerts returns a JSON string for all active alerts.
func (store *PrometheusStore) Alerts(ctx context.Context) (string, error) {
	result, err := store.c.Alerts(ctx)
	if err != nil {
		slog.Error("prometheus: Alerts failed", "err", err)
		return "", err
	}
	str, err := Utility.ToJson(result)
	if err != nil {
		slog.Error("prometheus: Alerts JSON marshal failed", "err", err)
		return "", err
	}
	return str, nil
}

// AlertManagers returns a JSON overview of the current state of Alertmanager discovery.
func (store *PrometheusStore) AlertManagers(ctx context.Context) (string, error) {
	result, err := store.c.AlertManagers(ctx)
	if err != nil {
		slog.Error("prometheus: AlertManagers failed", "err", err)
		return "", err
	}
	str, err := Utility.ToJson(result)
	if err != nil {
		slog.Error("prometheus: AlertManagers JSON marshal failed", "err", err)
		return "", err
	}
	return str, nil
}

// CleanTombstones removes deleted data from disk and cleans up tombstones.
func (store *PrometheusStore) CleanTombstones(ctx context.Context) error {
	if err := store.c.CleanTombstones(ctx); err != nil {
		slog.Error("prometheus: CleanTombstones failed", "err", err)
		return err
	}
	return nil
}

// Config returns the current Prometheus configuration as JSON.
func (store *PrometheusStore) Config(ctx context.Context) (string, error) {
	result, err := store.c.Config(ctx)
	if err != nil {
		slog.Error("prometheus: Config failed", "err", err)
		return "", err
	}
	str, err := Utility.ToJson(result)
	if err != nil {
		slog.Error("prometheus: Config JSON marshal failed", "err", err)
		return "", err
	}
	return str, nil
}

// DeleteSeries deletes data for the given series matchers between startTime and endTime.
func (store *PrometheusStore) DeleteSeries(ctx context.Context, matches []string, startTime, endTime time.Time) error {
	if err := store.c.DeleteSeries(ctx, matches, startTime, endTime); err != nil {
		slog.Error("prometheus: DeleteSeries failed", "err", err, "matches", matches, "start", startTime, "end", endTime)
		return err
	}
	return nil
}

// Flags returns the launch flag values of the Prometheus server in JSON.
func (store *PrometheusStore) Flags(ctx context.Context) (string, error) {
	result, err := store.c.Flags(ctx)
	if err != nil {
		slog.Error("prometheus: Flags failed", "err", err)
		return "", err
	}
	str, err := Utility.ToJson(result)
	if err != nil {
		slog.Error("prometheus: Flags JSON marshal failed", "err", err)
		return "", err
	}
	return str, nil
}

// LabelNames returns all unique label names (as a slice) and any warnings (as JSON).
// NOTE: This uses empty matchers and an unbounded time range to mirror the original
// (no-arg) behavior of your Store method signature.
func (store *PrometheusStore) LabelNames(ctx context.Context) ([]string, string, error) {
	names, warnings, err := store.c.LabelNames(ctx, nil, time.Time{}, time.Time{})
	if err != nil {
		slog.Error("prometheus: LabelNames failed", "err", err)
		return nil, "", err
	}
	warningsStr, err := Utility.ToJson(warnings)
	if err != nil {
		slog.Error("prometheus: LabelNames warnings JSON marshal failed", "err", err)
		return nil, "", err
	}
	return names, warningsStr, nil
}

// LabelValues performs a label values query for the given label between startTime and endTime.
// Returns the values and warnings as JSON strings.
func (store *PrometheusStore) LabelValues(ctx context.Context, label string, values []string, startTime, endTime int64) (string, string, error) {
	startTime_ := time.Unix(startTime, 0)
	endTime_ := time.Unix(endTime, 0)

	results, warnings, err := store.c.LabelValues(ctx, label, values, startTime_, endTime_)
	if err != nil {
		slog.Error("prometheus: LabelValues failed", "label", label, "err", err)
		return "", "", err
	}

	warningsStr, err := Utility.ToJson(warnings)
	if err != nil {
		slog.Error("prometheus: LabelValues warnings JSON marshal failed", "err", err)
		return "", "", err
	}

	resultsStr, err := Utility.ToJson(results)
	if err != nil {
		slog.Error("prometheus: LabelValues results JSON marshal failed", "err", err)
		return "", "", err
	}

	return resultsStr, warningsStr, nil
}

// Query performs an instant query at the given timestamp and returns results/warnings as JSON.
func (store *PrometheusStore) Query(ctx context.Context, query string, ts time.Time) (string, string, error) {
	results, warnings, err := store.c.Query(ctx, query, ts)
	if err != nil {
		slog.Error("prometheus: Query failed", "query", query, "err", err)
		return "", "", err
	}

	warningsStr, err := Utility.ToJson(warnings)
	if err != nil {
		slog.Error("prometheus: Query warnings JSON marshal failed", "err", err)
		return "", "", err
	}

	resultsStr, err := Utility.ToJson(results)
	if err != nil {
		slog.Error("prometheus: Query results JSON marshal failed", "err", err)
		return "", "", err
	}

	return resultsStr, warningsStr, nil
}

// QueryRange performs a range query and returns results/warnings as JSON.
// step is expressed in milliseconds in the current API surface.
func (store *PrometheusStore) QueryRange(ctx context.Context, query string, startTime, endTime time.Time, step float64) (string, string, error) {
	r := v1.Range{
		Start: startTime,
		End:   endTime,
		Step:  time.Duration(step) * time.Millisecond,
	}

	results, warnings, err := store.c.QueryRange(ctx, query, r)
	if err != nil {
		slog.Error("prometheus: QueryRange failed", "query", query, "err", err, "start", startTime, "end", endTime, "step_ms", step)
		return "", "", err
	}

	warningsStr, err := Utility.ToJson(warnings)
	if err != nil {
		slog.Error("prometheus: QueryRange warnings JSON marshal failed", "err", err)
		return "", "", err
	}

	resultsStr, err := Utility.ToJson(results)
	if err != nil {
		slog.Error("prometheus: QueryRange results JSON marshal failed", "err", err)
		return "", "", err
	}

	return resultsStr, warningsStr, nil
}

// Series finds series by label matchers in the given time range and returns results/warnings as JSON.
func (store *PrometheusStore) Series(ctx context.Context, matches []string, startTime, endTime time.Time) (string, string, error) {
	results, warnings, err := store.c.Series(ctx, matches, startTime, endTime)
	if err != nil {
		slog.Error("prometheus: Series failed", "err", err, "matches", matches, "start", startTime, "end", endTime)
		return "", "", err
	}

	warningsStr, err := Utility.ToJson(warnings)
	if err != nil {
		slog.Error("prometheus: Series warnings JSON marshal failed", "err", err)
		return "", "", err
	}

	resultsStr, err := Utility.ToJson(results)
	if err != nil {
		slog.Error("prometheus: Series results JSON marshal failed", "err", err)
		return "", "", err
	}

	return resultsStr, warningsStr, nil
}

// Snapshot creates a TSDB snapshot and returns the resulting directory path as JSON.
func (store *PrometheusStore) Snapshot(ctx context.Context, skipHead bool) (string, error) {
	result, err := store.c.Snapshot(ctx, skipHead)
	if err != nil {
		slog.Error("prometheus: Snapshot failed", "err", err, "skipHead", skipHead)
		return "", err
	}
	str, err := Utility.ToJson(result)
	if err != nil {
		slog.Error("prometheus: Snapshot JSON marshal failed", "err", err)
		return "", err
	}
	return str, nil
}

// Rules returns the currently loaded alerting/recording rules as JSON.
func (store *PrometheusStore) Rules(ctx context.Context) (string, error) {
	result, err := store.c.Rules(ctx)
	if err != nil {
		slog.Error("prometheus: Rules failed", "err", err)
		return "", err
	}
	str, err := Utility.ToJson(result)
	if err != nil {
		slog.Error("prometheus: Rules JSON marshal failed", "err", err)
		return "", err
	}
	return str, nil
}

// Targets returns a JSON overview of the current target discovery state.
func (store *PrometheusStore) Targets(ctx context.Context) (string, error) {
	result, err := store.c.Targets(ctx)
	if err != nil {
		slog.Error("prometheus: Targets failed", "err", err)
		return "", err
	}
	str, err := Utility.ToJson(result)
	if err != nil {
		slog.Error("prometheus: Targets JSON marshal failed", "err", err)
		return "", err
	}
	return str, nil
}

// TargetsMetadata returns metadata about metrics currently scraped by the target as JSON.
func (store *PrometheusStore) TargetsMetadata(ctx context.Context, matchTarget, metric, limit string) (string, error) {
	results, err := store.c.TargetsMetadata(ctx, matchTarget, metric, limit)
	if err != nil {
		slog.Error("prometheus: TargetsMetadata failed", "err", err, "matchTarget", matchTarget, "metric", metric, "limit", limit)
		return "", err
	}
	str, err := Utility.ToJson(results)
	if err != nil {
		slog.Error("prometheus: TargetsMetadata JSON marshal failed", "err", err)
		return "", err
	}
	return str, nil
}
