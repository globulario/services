package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/djimenez/iconv-go"
	"github.com/globulario/services/golang/sql/sqlpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// maxChunkBytes is the maximum encoded JSON size (in bytes) of a row batch
// sent over the stream. It balances fragmentation vs. serialization overhead.
const maxChunkBytes = 16_000

// pingTimeout controls how long we wait for a DB connection ping.
const pingTimeout = 1 * time.Second


// CreateConnection creates (or replaces) a connection entry and persists it.
// It validates basic parameters, attempts to open the DB using the provided
// driver/DSN, saves the connection, and finally pings it to confirm reachability.
func (srv *server) CreateConnection(ctx context.Context, rqst *sqlpb.CreateConnectionRqst) (*sqlpb.CreateConnectionRsp, error) {
	if rqst == nil || rqst.Connection == nil {
		return nil, status.Error(codes.InvalidArgument, "CreateConnection: request or connection is nil")
	}

	var c connection
	c.Id = rqst.Connection.Id
	c.Name = rqst.Connection.Name
	c.Host = rqst.Connection.Host
	c.Port = rqst.Connection.Port
	c.User = rqst.Connection.User
	c.Password = rqst.Connection.Password
	c.Driver = rqst.Connection.Driver
	c.Charset = rqst.Connection.Charset
	c.Path = rqst.Connection.Path

	if c.Driver == "sqlite3" && len(c.Path) == 0 {
		return nil, status.Errorf(codes.InvalidArgument,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("path is required for sqlite3 connection")))
	}

	db, err := sql.Open(c.Driver, c.getConnectionString())
	if err != nil {
		logger.Error("CreateConnection: sql.Open failed", "driver", c.Driver, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	defer func() {
		if cerr := db.Close(); cerr != nil {
					logger.Warn("CreateConnection: closing DB failed", "err", cerr)
		}
	}()

	// Save/replace connection definition then persist.
	srv.Connections[c.Id] = c
	if err := srv.Save(); err != nil {
		logger.Error("CreateConnection: saving connections failed", "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Validate reachability.
	if _, err := srv.ping(ctx, c.Id); err != nil {
		logger.Error("CreateConnection: ping failed", "id", c.Id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	logger.Info("CreateConnection: success", "id", c.Id, "name", c.Name, "driver", c.Driver)
	return &sqlpb.CreateConnectionRsp{Result: true}, nil
}

// DeleteConnection removes a connection entry and persists the change.
// It is idempotent: deleting a non-existent ID returns success.
func (srv *server) DeleteConnection(ctx context.Context, rqst *sqlpb.DeleteConnectionRqst) (*sqlpb.DeleteConnectionRsp, error) {
	if rqst == nil {
		return nil, status.Error(codes.InvalidArgument, "DeleteConnection: request is nil")
	}
	id := rqst.GetId()
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "DeleteConnection: id is empty")
	}

	if _, ok := srv.Connections[id]; ok {
		delete(srv.Connections, id)
		if err := srv.Save(); err != nil {
			logger.Error("DeleteConnection: saving connections failed", "id", id, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		logger.Info("DeleteConnection: deleted", "id", id)
	} else {
		// Idempotent success if not present.
		logger.Info("DeleteConnection: no-op (not found)", "id", id)
	}

	return &sqlpb.DeleteConnectionRsp{Result: true}, nil
}

// ping performs a direct DB open+ping for the connection id.
// Returns "pong" on success.
func (srv *server) ping(ctx context.Context, id string) (string, error) {
	conn, ok := srv.Connections[id]
	if !ok {
		return "", status.Errorf(codes.NotFound, "connection with id %q doesn't exist", id)
	}

	db, err := sql.Open(conn.Driver, conn.getConnectionString())
	if err != nil {
		return "", status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	defer func() {
		if cerr := db.Close(); cerr != nil {
			logger.Warn("ping: closing DB failed", "id", id, "err", cerr)
		}
	}()

	cctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := db.PingContext(cctx); err != nil {
		return "", status.Errorf(codes.Unavailable, "database ping failed: %v", err)
	}
	return "pong", nil
}

// Ping validates that the stored connection can be reached and responds.
func (srv *server) Ping(ctx context.Context, rqst *sqlpb.PingConnectionRqst) (*sqlpb.PingConnectionRsp, error) {
	if rqst == nil {
		return nil, status.Error(codes.InvalidArgument, "Ping: request is nil")
	}
	pong, err := srv.ping(ctx, rqst.GetId())
	if err != nil {
		logger.Error("Ping: failed", "id", rqst.GetId(), "err", err)
		return nil, err
	}
	return &sqlpb.PingConnectionRsp{Result: pong}, nil
}

// QueryContext executes a read-only SQL query and streams results back in chunks.
// The first streamed message is a header describing columns and types.
// Subsequent messages contain JSON-encoded row batches, sized to stay under maxChunkBytes.
func (srv *server) QueryContext(rqst *sqlpb.QueryContextRqst, stream sqlpb.SqlService_QueryContextServer) error {
	if rqst == nil || rqst.Query == nil {
		return status.Error(codes.InvalidArgument, "QueryContext: request or query is nil")
	}
	connID := rqst.Query.ConnectionId
	if connID == "" {
		return status.Error(codes.InvalidArgument, "QueryContext: connection id is empty")
	}

	conn, ok := srv.Connections[connID]
	if !ok {
		return status.Errorf(codes.NotFound, "connection with id %q doesn't exist", connID)
	}

	db, err := sql.Open(conn.Driver, conn.getConnectionString())
	if err != nil {
		logger.Error("QueryContext: sql.Open failed", "id", connID, "err", err)
		return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	defer func() {
		if cerr := db.Close(); cerr != nil {
			logger.Warn("QueryContext: closing DB failed", "id", connID, "err", cerr)
		}
	}()

	// Decode parameters if present.
	var params []interface{}
	if p := rqst.Query.Parameters; len(p) > 0 {
		if err := json.Unmarshal([]byte(p), &params); err != nil {
			return status.Errorf(codes.InvalidArgument, "QueryContext: invalid parameters JSON: %v", err)
		}
	}

	rows, err := db.QueryContext(stream.Context(), rqst.Query.Query, params...)
	if err != nil {
		logger.Error("QueryContext: db.QueryContext failed", "id", connID, "err", err)
		return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			logger.Warn("QueryContext: closing rows failed", "id", connID, "err", cerr)
		}
	}()

	columns, err := rows.Columns()
	if err != nil {
		return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Build and send header.
	header := make([]interface{}, len(columns))
	for i := range columns {
		ti := map[string]interface{}{
			"DatabaseTypeName": colTypes[i].DatabaseTypeName(),
			"Name":             colTypes[i].Name(),
		}
		if p, s, ok := colTypes[i].DecimalSize(); ok {
			ti["Precision"] = p
			ti["Scale"] = s
		}
		if l, ok := colTypes[i].Length(); ok {
			ti["Precision"] = l
		}
		if n, ok := colTypes[i].Nullable(); ok {
			ti["IsNullable"] = true
			ti["IsNull"] = n
		} else {
			ti["IsNullable"] = false
		}
		header[i] = map[string]interface{}{"name": columns[i], "typeInfo": ti}
	}
	if headerJSON, _ := Utility.ToJson(header); len(headerJSON) > 0 {
		if err := stream.Send(&sqlpb.QueryContextRsp{
			Result: &sqlpb.QueryContextRsp_Header{Header: headerJSON},
		}); err != nil {
			return status.Errorf(codes.Unavailable, "QueryContext: sending header failed: %v", err)
		}
	}

	// Prepare scanning buffers.
	count := len(columns)
	values := make([]interface{}, count)
	scanArgs := make([]interface{}, count)
	for i := range values {
		scanArgs[i] = &values[i]
	}

	var (
		batch     = make([]interface{}, 0, 64)
		targetCS  = rqst.Query.Charset
		sourceCS  = conn.Charset
		flushBatch = func() error {
			if len(batch) == 0 {
				return nil
			}
			buf, err := json.Marshal(batch)
			if err != nil {
				return status.Errorf(codes.Internal, "QueryContext: encoding batch failed: %v", err)
			}
			if err := stream.Send(&sqlpb.QueryContextRsp{
				Result: &sqlpb.QueryContextRsp_Rows{Rows: string(buf)},
			}); err != nil {
				return status.Errorf(codes.Unavailable, "QueryContext: sending rows failed: %v", err)
			}
			batch = batch[:0]
			return nil
		}
	)

	for rows.Next() {
		if err := rows.Scan(scanArgs...); err != nil {
			return status.Errorf(codes.Internal, "QueryContext: scanning row failed: %v", err)
		}

		row := make([]interface{}, count)
		for i, v := range values {
			switch {
			case v == nil:
				row[i] = nil
			case Utility.IsNumeric(v):
				row[i] = Utility.ToNumeric(v)
			case Utility.IsBool(v):
				row[i] = Utility.ToBool(v)
			default:
				s := Utility.ToString(v)
				// Optional charset conversion (e.g., windows-1252 -> utf-8).
				if sourceCS != "" && targetCS != "" && sourceCS != targetCS {
					if s2, err := iconv.ConvertString(s, sourceCS, targetCS); err == nil {
						s = s2
					} else {
						logger.Warn("QueryContext: charset conversion failed", "from", sourceCS, "to", targetCS, "err", err)
					}
				}
				row[i] = s
			}
		}

		// Add to batch and flush if encoded size exceeds threshold.
		batch = append(batch, row)
		if encodedSize(batch) > maxChunkBytes {
			// Remove last row, flush, then start new batch with it.
			last := batch[len(batch)-1]
			batch = batch[:len(batch)-1]
			if err := flushBatch(); err != nil {
				return err
			}
			batch = append(batch, last)
		}
	}

	if err := rows.Err(); err != nil {
		return status.Errorf(codes.Internal, "QueryContext: row iteration error: %v", err)
	}

	// Flush remaining rows.
	if err := flushBatch(); err != nil {
		return err
	}

	logger.Info("QueryContext: completed", "id", connID)
	return nil
}

// ExecContext executes a write operation (CREATE/INSERT/UPDATE/DELETE, etc.).
// It supports optional transaction semantics and returns last insert ID and rows affected.
func (srv *server) ExecContext(ctx context.Context, rqst *sqlpb.ExecContextRqst) (*sqlpb.ExecContextRsp, error) {
	if rqst == nil || rqst.Query == nil {
		return nil, status.Error(codes.InvalidArgument, "ExecContext: request or query is nil")
	}
	connID := rqst.Query.ConnectionId
	if connID == "" {
		return nil, status.Error(codes.InvalidArgument, "ExecContext: connection id is empty")
	}

	conn, ok := srv.Connections[connID]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "connection with id %q doesn't exist", connID)
	}

	db, err := sql.Open(conn.Driver, conn.getConnectionString())
	if err != nil {
		logger.Error("ExecContext: sql.Open failed", "id", connID, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	defer func() {
		if cerr := db.Close(); cerr != nil {
			logger.Warn("ExecContext: closing DB failed", "id", connID, "err", cerr)
		}
	}()

	// Decode parameters if present.
	var params []interface{}
	if p := rqst.Query.Parameters; len(p) > 0 {
		if err := json.Unmarshal([]byte(p), &params); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "ExecContext: invalid parameters JSON: %v", err)
		}
	}

	query := rqst.Query.Query
	var res sql.Result

	if rqst.Tx {
		tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		res, err = tx.ExecContext(ctx, query, params...)
		if err != nil {
			// Prefer rollback error details if rollback also fails.
			if rbErr := tx.Rollback(); rbErr != nil {
				logger.Error("ExecContext: exec+rollback failed", "execErr", err, "rollbackErr", rbErr)
				return nil, status.Errorf(codes.Internal, "ExecContext: exec failed (%v) and rollback failed (%v)", err, rbErr)
			}
			return nil, status.Errorf(codes.Internal, "ExecContext: exec failed: %v", err)
		}
		if err := tx.Commit(); err != nil {
			return nil, status.Errorf(codes.Internal, "ExecContext: commit failed: %v", err)
		}
	} else {
		res, err = db.ExecContext(ctx, query, params...)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "ExecContext: exec failed: %v", err)
		}
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ExecContext: getting rows affected failed: %v", err)
	}
	lastID, _ := res.LastInsertId()

	logger.Info("ExecContext: completed", "id", connID, "affected", affected, "lastID", lastID)
	return &sqlpb.ExecContextRsp{LastId: lastID, AffectedRows: affected}, nil
}

// encodedSize returns the approximate JSON-encoded size of v (slice of rows).
func encodedSize(v interface{}) int {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	_ = enc.Encode(v)
	return buf.Len()
}

// NOTE:
// - This refactor relies on existing types: `server`, `connection`, and `connection.getConnectionString()`.
// - Logging uses slog; wire your project-wide logger into `logger` if needed.
// - Public method signatures are unchanged.
