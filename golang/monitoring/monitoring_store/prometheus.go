package monitoring_store

import (
	"context"
	"time"

	Utility "github.com/davecourtois/!utility"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

// Implementation of prometheus store.
type PrometheusStore struct {
	c v1.API
}

func NewPrometheusStore(address string) (Store, error) {

	client, err := api.NewClient(api.Config{
		Address: address,
	})
	if err != nil {
		return nil, err
	}

	store := new(PrometheusStore)

	store.c = v1.NewAPI(client)

	return store, nil
}

// Alerts returns a list of all active alerts.
func (store *PrometheusStore) Alerts(ctx context.Context) (string, error) {
	result, err := store.c.Alerts(ctx)
	if err != nil {
		return "", err
	}

	str, err := Utility.ToJson(result)
	if err != nil {
		return "", err
	}

	return str, nil
}

// AlertManagers returns an overview of the current state of the Prometheus alert manager discovery.
func (store *PrometheusStore) AlertManagers(ctx context.Context) (string, error) {
	result, err := store.c.AlertManagers(ctx)
	if err != nil {
		return "", err
	}

	str, err := Utility.ToJson(result)
	if err != nil {
		return "", err
	}

	return str, nil
}

// CleanTombstones removes the deleted data from disk and cleans up the existing tombstones.
func (store *PrometheusStore) CleanTombstones(ctx context.Context) error {
	return store.c.CleanTombstones(ctx)
}

// Config returns the current Prometheus configuration.
func (store *PrometheusStore) Config(ctx context.Context) (string, error) {
	result, err := store.c.Config(ctx)
	if err != nil {
		return "", err
	}

	str, err := Utility.ToJson(result)
	if err != nil {
		return "", err
	}

	return str, nil
}

// DeleteSeries deletes data for a selection of series in a time range.
func (store *PrometheusStore) DeleteSeries(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) error {
	return store.c.DeleteSeries(ctx, matches, startTime, endTime)
}

// Flags returns the flag values that Prometheus was launched with.
func (store *PrometheusStore) Flags(ctx context.Context) (string, error) {
	result, err := store.c.Flags(ctx)
	if err != nil {
		return "", err
	}

	str, err := Utility.ToJson(result)
	if err != nil {
		return "", err
	}

	return str, nil
}

// LabelNames returns all the unique label names present in the block in sorted order.
func (store *PrometheusStore) LabelNames(ctx context.Context) ([]string, string, error) {
	var results []string
	/*results , warnings, err := store.c.LabelNames(ctx)
	if err != nil {
		return nil, "", err
	}*/

	var warningsStr string
	/*warningsStr, err := Utility.ToJson(warnings)
	if err != nil {
		return nil, "", err
	}*/

	return results, warningsStr, nil
}

// LabelValues performs a query for the values of the given label.
func (store *PrometheusStore) LabelValues(ctx context.Context, label string, values []string, startTime int64, endTime int64) (string, string, error) {
	startTime_ := time.Unix(startTime, 0)
	endTime_ := time.Unix(endTime, 0)

	results, warnings, err := store.c.LabelValues(ctx, label, values, startTime_, endTime_)
	if err != nil {
		return "", "", err
	}

	var warningsStr string
	warningsStr, err = Utility.ToJson(warnings)
	if err != nil {
		return "", "", err
	}

	resultsStr, err := Utility.ToJson(results)
	if err != nil {
		return "", "", err
	}

	return resultsStr, warningsStr, nil
}

// Query performs a query for the given time.
func (store *PrometheusStore) Query(ctx context.Context, query string, ts time.Time) (string, string, error) {
	results, warnings, err := store.c.Query(ctx, query, ts)
	if err != nil {
		return "", "", err
	}

	var warningsStr string
	warningsStr, err = Utility.ToJson(warnings)
	if err != nil {
		return "", "", err
	}

	resultsStr, err := Utility.ToJson(results)
	if err != nil {
		return "", "", err
	}

	return resultsStr, warningsStr, nil
}

// QueryRange performs a query for the given range.
func (store *PrometheusStore) QueryRange(ctx context.Context, query string, startTime time.Time, endTime time.Time, step float64) (string, string, error) {
	// Initialyse the parameter.
	var r v1.Range
	r.End = endTime
	r.Start = startTime
	r.Step = time.Duration(step) * time.Millisecond

	results, warnings, err := store.c.QueryRange(ctx, query, r)
	if err != nil {
		return "", "", err
	}
	var warningsStr string
	warningsStr, err = Utility.ToJson(warnings)
	if err != nil {
		return "", "", err
	}

	resultsStr, err := Utility.ToJson(results)
	if err != nil {
		return "", "", err
	}

	return resultsStr, warningsStr, nil
}

// Series finds series by label matchers.
func (store *PrometheusStore) Series(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) (string, string, error) {
	results, warnings, err := store.c.Series(ctx, matches, startTime, endTime)
	if err != nil {
		return "", "", err
	}

	var warningsStr string
	warningsStr, err = Utility.ToJson(warnings)
	if err != nil {
		return "", "", err
	}

	resultsStr, err := Utility.ToJson(results)
	if err != nil {
		return "", "", err
	}

	return resultsStr, warningsStr, nil
}

// Snapshot creates a snapshot of all current data into snapshots/<datetime>-<rand>
// under the TSDB's data directory and returns the directory as response.
func (store *PrometheusStore) Snapshot(ctx context.Context, skipHead bool) (string, error) {
	result, err := store.c.Snapshot(ctx, skipHead)
	if err != nil {
		return "", err
	}

	str, err := Utility.ToJson(result)
	if err != nil {
		return "", err
	}

	return str, nil
}

// Rules returns a list of alerting and recording rules that are currently loaded.
func (store *PrometheusStore) Rules(ctx context.Context) (string, error) {
	result, err := store.c.Rules(ctx)
	if err != nil {
		return "", err
	}

	str, err := Utility.ToJson(result)
	if err != nil {
		return "", err
	}

	return str, nil
}

// Targets returns an overview of the current state of the Prometheus target discovery.
func (store *PrometheusStore) Targets(ctx context.Context) (string, error) {
	result, err := store.c.Targets(ctx)
	if err != nil {
		return "", err
	}

	str, err := Utility.ToJson(result)
	if err != nil {
		return "", err
	}

	return str, nil
}

// TargetsMetadata returns metadata about metrics currently scraped by the target.
func (store *PrometheusStore) TargetsMetadata(ctx context.Context, matchTarget string, metric string, limit string) (string, error) {
	var results []string
	/*results, err := store.c.TargetsMetadata(ctx, matchTarget, metric, limit)
	if err != nil {
		return "", err
	}*/

	str, err := Utility.ToJson(results)
	if err != nil {
		return "", err
	}

	return str, nil
}
