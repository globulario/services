package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/globulario/services/golang/monitoring/monitoring_store"
	"github.com/globulario/services/golang/monitoring/monitoringpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// createConnection creates or updates a connection in memory, dials the backing
// monitoring store (e.g., Prometheus), keeps the live store handle, and persists
// the service configuration to disk.
//
// This function is private; public RPC wiring is done in CreateConnection.
func (srv *server) createConnection(id, host string, port int32, storeType monitoringpb.StoreType) error {
	// Guard maps.
	if srv.Connections == nil {
		srv.Connections = make(map[string]connection)
	}
	if srv.stores == nil {
		srv.stores = make(map[string]monitoring_store.Store)
	}

	c := connection{
		Id:   id,
		Host: host,
		Port: port,
		Type: storeType,
	}

	// Build address and instantiate the proper store implementation.
	address := "http://" + c.Host + ":" + Utility.ToString(c.Port)

	var (
		store monitoring_store.Store
		err   error
	)
	switch c.Type {
	case monitoringpb.StoreType_PROMETHEUS:
		store, err = monitoring_store.NewPrometheusStore(address)
	default:
		err = errors.New("unsupported store type")
	}

	if err != nil {
		slog.Error("failed to create monitoring store",
			"conn_id", c.Id, "type", c.Type.String(), "address", address, "err", err)
		return err
	}
	if store == nil {
		err = errors.New("failed to connect to store")
		slog.Error("nil monitoring store returned",
			"conn_id", c.Id, "type", c.Type.String(), "address", address)
		return err
	}

	// Save connection and live store handle.
	srv.Connections[c.Id] = c
	srv.stores[c.Id] = store

	// Persist service configuration.
	if err := srv.Save(); err != nil {
		slog.Error("failed to persist configuration after creating connection",
			"conn_id", c.Id, "err", err)
		return err
	}

	slog.Info("connection created",
		"conn_id", c.Id, "type", c.Type.String(), "address", address)
	return nil
}

// storeFor resolves a monitoring store by connection id.
// It returns a user-facing gRPC error when missing to keep call-sites simple.
func (srv *server) storeFor(id string) (monitoring_store.Store, error) {
	store := srv.stores[id]
	if store == nil {
		err := errors.New("no store connection exists for id " + id)
		slog.Warn("store not found", "conn_id", id)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}
	return store, nil
}

// grpcErr wraps errors into a gRPC status with consistent payload.
func grpcErr(err error) error {
	return status.Errorf(
		codes.Internal,
		Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
	)
}

//------------------------------------------------------------------------------
// Public RPCs (prototypes unchanged)
//------------------------------------------------------------------------------

// CreateConnection creates a new monitoring connection (e.g., to Prometheus) and
// persists it in the service configuration.
func (srv *server) CreateConnection(ctx context.Context, rqst *monitoringpb.CreateConnectionRqst) (*monitoringpb.CreateConnectionRsp, error) {
	if rqst == nil || rqst.Connection == nil {
		err := errors.New("invalid request: missing connection")
		slog.Error("CreateConnection: bad request", "err", err)
		return nil, grpcErr(err)
	}

	err := srv.createConnection(
		rqst.Connection.Id,
		rqst.Connection.Host,
		rqst.Connection.Port,
		rqst.Connection.Store,
	)
	if err != nil {
		return nil, grpcErr(err)
	}

	return &monitoringpb.CreateConnectionRsp{Result: true}, nil
}

// DeleteConnection removes a connection by id and persists the change.
func (srv *server) DeleteConnection(ctx context.Context, rqst *monitoringpb.DeleteConnectionRqst) (*monitoringpb.DeleteConnectionRsp, error) {
	id := rqst.GetId()
	if _, ok := srv.Connections[id]; !ok {
		slog.Info("DeleteConnection: no-op, id not found", "conn_id", id)
		return &monitoringpb.DeleteConnectionRsp{Result: true}, nil
	}

	// Remove from memory.
	delete(srv.Connections, id)
	delete(srv.stores, id)

	// Persist service configuration.
	if err := srv.Save(); err != nil {
		slog.Error("DeleteConnection: failed to persist configuration", "conn_id", id, "err", err)
		return nil, grpcErr(err)
	}

	slog.Info("connection deleted", "conn_id", id)
	return &monitoringpb.DeleteConnectionRsp{Result: true}, nil
}

// Alerts returns all active alerts from the backing store.
func (srv *server) Alerts(ctx context.Context, rqst *monitoringpb.AlertsRequest) (*monitoringpb.AlertsResponse, error) {
	store, err := srv.storeFor(rqst.ConnectionId)
	if err != nil {
		return nil, err
	}
	out, err := store.Alerts(ctx)
	if err != nil {
		slog.Error("Alerts failed", "conn_id", rqst.ConnectionId, "err", err)
		return nil, grpcErr(err)
	}
	return &monitoringpb.AlertsResponse{Results: out}, nil
}

// AlertManagers returns the state of alertmanager discovery.
func (srv *server) AlertManagers(ctx context.Context, rqst *monitoringpb.AlertManagersRequest) (*monitoringpb.AlertManagersResponse, error) {
	store, err := srv.storeFor(rqst.ConnectionId)
	if err != nil {
		return nil, err
	}
	out, err := store.AlertManagers(ctx)
	if err != nil {
		slog.Error("AlertManagers failed", "conn_id", rqst.ConnectionId, "err", err)
		return nil, grpcErr(err)
	}
	return &monitoringpb.AlertManagersResponse{Results: out}, nil
}

// CleanTombstones compacts tombstones and deletes associated data on disk.
func (srv *server) CleanTombstones(ctx context.Context, rqst *monitoringpb.CleanTombstonesRequest) (*monitoringpb.CleanTombstonesResponse, error) {
	store, err := srv.storeFor(rqst.ConnectionId)
	if err != nil {
		return nil, err
	}
	if err := store.CleanTombstones(ctx); err != nil {
		slog.Error("CleanTombstones failed", "conn_id", rqst.ConnectionId, "err", err)
		return nil, grpcErr(err)
	}
	return &monitoringpb.CleanTombstonesResponse{Result: true}, nil
}

// Config returns the current store configuration (e.g., Prometheus config).
func (srv *server) Config(ctx context.Context, rqst *monitoringpb.ConfigRequest) (*monitoringpb.ConfigResponse, error) {
	store, err := srv.storeFor(rqst.ConnectionId)
	if err != nil {
		return nil, err
	}
	out, err := store.Config(ctx)
	if err != nil {
		slog.Error("Config failed", "conn_id", rqst.ConnectionId, "err", err)
		return nil, grpcErr(err)
	}
	return &monitoringpb.ConfigResponse{Results: out}, nil
}

// DeleteSeries deletes data for selected series in a time range.
func (srv *server) DeleteSeries(ctx context.Context, rqst *monitoringpb.DeleteSeriesRequest) (*monitoringpb.DeleteSeriesResponse, error) {
	store, err := srv.storeFor(rqst.ConnectionId)
	if err != nil {
		return nil, err
	}

	start := time.Unix(int64(rqst.GetStartTime()), 0)
	end := time.Unix(int64(rqst.GetEndTime()), 0)

	if err := store.DeleteSeries(ctx, rqst.GetMatches(), start, end); err != nil {
		slog.Error("DeleteSeries failed", "conn_id", rqst.ConnectionId, "err", err)
		return nil, grpcErr(err)
	}
	return &monitoringpb.DeleteSeriesResponse{Result: true}, nil
}

// Flags returns launch flag values of the backing store process.
func (srv *server) Flags(ctx context.Context, rqst *monitoringpb.FlagsRequest) (*monitoringpb.FlagsResponse, error) {
	store, err := srv.storeFor(rqst.ConnectionId)
	if err != nil {
		return nil, err
	}
	out, err := store.Flags(ctx)
	if err != nil {
		slog.Error("Flags failed", "conn_id", rqst.ConnectionId, "err", err)
		return nil, grpcErr(err)
	}
	return &monitoringpb.FlagsResponse{Results: out}, nil
}

// LabelNames returns all unique label names in sorted order.
func (srv *server) LabelNames(ctx context.Context, rqst *monitoringpb.LabelNamesRequest) (*monitoringpb.LabelNamesResponse, error) {
	store, err := srv.storeFor(rqst.ConnectionId)
	if err != nil {
		return nil, err
	}
	labels, warnings, err := store.LabelNames(ctx)
	if err != nil {
		slog.Error("LabelNames failed", "conn_id", rqst.ConnectionId, "err", err)
		return nil, grpcErr(err)
	}
	return &monitoringpb.LabelNamesResponse{
		Labels:   labels,
		Warnings: warnings,
	}, nil
}

// LabelValues queries the values for a given label (with optional matchers/time range).
func (srv *server) LabelValues(ctx context.Context, rqst *monitoringpb.LabelValuesRequest) (*monitoringpb.LabelValuesResponse, error) {
	store, err := srv.storeFor(rqst.ConnectionId)
	if err != nil {
		return nil, err
	}
	valuesJSON, warnings, err := store.LabelValues(ctx, rqst.Label, rqst.Values, rqst.StartTime, rqst.EndTime)
	if err != nil {
		slog.Error("LabelValues failed", "conn_id", rqst.ConnectionId, "label", rqst.Label, "err", err)
		return nil, grpcErr(err)
	}
	return &monitoringpb.LabelValuesResponse{
		LabelValues: valuesJSON,
		Warnings:    warnings,
	}, nil
}

// Query executes an instant query at the given timestamp.
func (srv *server) Query(ctx context.Context, rqst *monitoringpb.QueryRequest) (*monitoringpb.QueryResponse, error) {
	store, err := srv.storeFor(rqst.ConnectionId)
	if err != nil {
		return nil, err
	}
	ts := time.Unix(int64(rqst.GetTs()), 0)
	out, warnings, err := store.Query(ctx, rqst.Query, ts)
	if err != nil {
		slog.Error("Query failed", "conn_id", rqst.ConnectionId, "query", rqst.Query, "err", err)
		return nil, grpcErr(err)
	}
	return &monitoringpb.QueryResponse{Value: out, Warnings: warnings}, nil
}

// QueryRange performs a range query and streams the (potentially large) JSON
// result in chunks to the client.
func (srv *server) QueryRange(rqst *monitoringpb.QueryRangeRequest, stream monitoringpb.MonitoringService_QueryRangeServer) error {
	store, err := srv.storeFor(rqst.ConnectionId)
	if err != nil {
		return err
	}
	ctx := stream.Context()

	start := time.Unix(int64(rqst.GetStartTime()), 0)
	end := time.Unix(int64(rqst.GetEndTime()), 0)
	step := rqst.Step

	out, warnings, err := store.QueryRange(ctx, rqst.GetQuery(), start, end, step)
	if err != nil {
		slog.Error("QueryRange failed", "conn_id", rqst.ConnectionId, "query", rqst.GetQuery(), "err", err)
		return grpcErr(err)
	}

	// Stream in fixed-size chunks to avoid hitting gRPC message limits.
	const chunk = 2000
	for i := 0; i < len(out); i += chunk {
		rsp := &monitoringpb.QueryRangeResponse{Warnings: warnings}
		if i+chunk < len(out) {
			rsp.Value = out[i : i+chunk]
		} else {
			rsp.Value = out[i:]
		}
		if err := stream.Send(rsp); err != nil {
			slog.Error("QueryRange stream send failed", "conn_id", rqst.ConnectionId, "err", err)
			return grpcErr(err)
		}
	}
	return nil
}

// Series finds series by label matchers over a time range.
func (srv *server) Series(ctx context.Context, rqst *monitoringpb.SeriesRequest) (*monitoringpb.SeriesResponse, error) {
	store, err := srv.storeFor(rqst.ConnectionId)
	if err != nil {
		return nil, err
	}
	start := time.Unix(int64(rqst.GetStartTime()), 0)
	end := time.Unix(int64(rqst.GetEndTime()), 0)

	out, warnings, err := store.Series(ctx, rqst.GetMatches(), start, end)
	if err != nil {
		slog.Error("Series failed", "conn_id", rqst.ConnectionId, "err", err)
		return nil, grpcErr(err)
	}
	return &monitoringpb.SeriesResponse{LabelSet: out, Warnings: warnings}, nil
}

// Snapshot creates a TSDB snapshot and returns the created directory path.
func (srv *server) Snapshot(ctx context.Context, rqst *monitoringpb.SnapshotRequest) (*monitoringpb.SnapshotResponse, error) {
	store, err := srv.storeFor(rqst.ConnectionId)
	if err != nil {
		return nil, err
	}
	dir, err := store.Snapshot(ctx, rqst.GetSkipHead())
	if err != nil {
		slog.Error("Snapshot failed", "conn_id", rqst.ConnectionId, "err", err)
		return nil, grpcErr(err)
	}
	return &monitoringpb.SnapshotResponse{Result: dir}, nil
}

// Rules returns the currently loaded alerting/recording rules.
func (srv *server) Rules(ctx context.Context, rqst *monitoringpb.RulesRequest) (*monitoringpb.RulesResponse, error) {
	store, err := srv.storeFor(rqst.ConnectionId)
	if err != nil {
		return nil, err
	}
	out, err := store.Rules(ctx)
	if err != nil {
		slog.Error("Rules failed", "conn_id", rqst.ConnectionId, "err", err)
		return nil, grpcErr(err)
	}
	return &monitoringpb.RulesResponse{Result: out}, nil
}

// Targets returns the current state of target discovery.
func (srv *server) Targets(ctx context.Context, rqst *monitoringpb.TargetsRequest) (*monitoringpb.TargetsResponse, error) {
	store, err := srv.storeFor(rqst.ConnectionId)
	if err != nil {
		return nil, err
	}
	out, err := store.Targets(ctx)
	if err != nil {
		slog.Error("Targets failed", "conn_id", rqst.ConnectionId, "err", err)
		return nil, grpcErr(err)
	}
	return &monitoringpb.TargetsResponse{Result: out}, nil
}

// TargetsMetadata returns metadata about metrics scraped by a target.
func (srv *server) TargetsMetadata(ctx context.Context, rqst *monitoringpb.TargetsMetadataRequest) (*monitoringpb.TargetsMetadataResponse, error) {
	store, err := srv.storeFor(rqst.ConnectionId)
	if err != nil {
		return nil, err
	}
	out, err := store.TargetsMetadata(ctx, rqst.GetMatchTarget(), rqst.GetMetric(), rqst.GetLimit())
	if err != nil {
		slog.Error("TargetsMetadata failed", "conn_id", rqst.ConnectionId, "err", err)
		return nil, grpcErr(err)
	}
	return &monitoringpb.TargetsMetadataResponse{Result: out}, nil
}
