package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/persistence/persistence_store"
	"github.com/globulario/services/golang/persistence/persistencepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

// ----------------------------------------------------------------------------
// Internal helpers
// ----------------------------------------------------------------------------

// norm normalizes identifiers used as map keys by replacing '@' and '.' with '_'.
func norm(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "@", "_"), ".", "_")
}

// grpcErr wraps a regular error into a standardized gRPC status using your Utility helper.
func grpcErr(err error) error {
	return status.Errorf(
		codes.Internal,
		Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
	)
}

// storeFor resolves a live store by (possibly raw) connection id.
func (srv *server) storeFor(id string) (persistence_store.Store, string, error) {
	nid := norm(id)
	store := srv.stores[nid]
	if store == nil {
		err := errors.New("no store connection exists for id " + nid)
		return nil, nid, err
	}
	return store, nid, nil
}

// ----------------------------------------------------------------------------
// Connection management (private)
// ----------------------------------------------------------------------------

func (srv *server) createConnection(
	ctx context.Context,
	user, password, id, name, host string,
	port int32,
	storeType persistencepb.StoreType,
	save bool,
	options string,
) error {
	var c connection
	var err error

	// Expand local aliases to configured domain.
	if host == "0.0.0.0" || host == "localhost" {
		if h, _ := config.GetDomain(); h != "" {
			host = h
		}
	}

	// If a connection already exists with same id and password, it's a no-op.
	if existing, ok := srv.connections[id]; ok {
		if existing.Password != password {
			return errors.New("a connection with id " + id + " already exists")
		}
		return nil
	}

	// Build a new connection descriptor.
	c = connection{
		Id:       id,
		Name:     name,
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		Store:    storeType,
		Options:  options,
	}

	// Persist in server configuration if requested; otherwise keep in-memory only.
	if save {
		if srv.Connections == nil {
			srv.Connections = make(map[string]connection)
		}
		srv.Connections[c.Id] = c
		if err = srv.Save(); err != nil {
			slog.Error("createConnection: save failed", "id", c.Id, "err", err)
			return err
		}
	} else {
		if srv.connections == nil {
			srv.connections = make(map[string]connection)
		}
		srv.connections[c.Id] = c
	}

	// Create a concrete store implementation.
	switch c.Store {
	case persistencepb.StoreType_MONGO:
		s := new(persistence_store.MongoStore)
		if err = s.Connect(c.Id, c.Host, c.Port, c.User, c.Password, c.Name, c.Timeout, c.Options); err != nil {
			slog.Error("mongo connect failed", "id", c.Id, "err", err)
			return err
		}
		srv.stores[c.Id] = s

	case persistencepb.StoreType_SQL:
		s := new(persistence_store.SqlStore)
		if err = s.Connect(c.Id, c.Host, c.Port, c.User, c.Password, c.Name, c.Timeout, c.Options); err != nil {
			slog.Error("sql connect failed", "id", c.Id, "err", err)
			return err
		}
		srv.stores[c.Id] = s

	case persistencepb.StoreType_SCYLLA:
		s := new(persistence_store.ScyllaStore)
		if err = s.Connect(c.Id, c.Host, c.Port, c.User, c.Password, c.Name, c.Timeout, c.Options); err != nil {
			slog.Error("scylla connect failed", "id", c.Id, "err", err)
			return err
		}
		srv.stores[c.Id] = s

	default:
		return errors.New("store type not supported")
	}

	// Validate connectivity.
	if err = srv.stores[c.Id].Ping(ctx, c.Id); err != nil {
		_ = srv.stores[c.Id].Disconnect(c.Id)
		slog.Error("store ping failed", "id", c.Id, "err", err)
		return err
	}

	slog.Info("connection created", "id", c.Id, "store", c.Store.String(), "host", c.Host, "port", c.Port)
	return nil
}

// ----------------------------------------------------------------------------
// Public RPCs (signatures unchanged) — now documented and logged with slog
// ----------------------------------------------------------------------------

// CreateConnection creates a new persistence store connection (Mongo, SQL, Scylla)
// and, if requested, persists it in the server configuration.
func (srv *server) CreateConnection(ctx context.Context, rqst *persistencepb.CreateConnectionRqst) (*persistencepb.CreateConnectionRsp, error) {
	if rqst.Connection == nil {
		return nil, grpcErr(errors.New("no connection provided"))
	}
	if rqst.Connection.Id == "" {
		return nil, grpcErr(errors.New("no connection id provided"))
	}

	err := srv.createConnection(
		ctx,
		rqst.Connection.User,
		rqst.Connection.Password,
		rqst.Connection.Id,
		rqst.Connection.Name,
		rqst.Connection.Host,
		rqst.Connection.Port,
		rqst.Connection.Store,
		rqst.Save,
		rqst.Connection.Options,
	)
	if err != nil {
		return nil, grpcErr(err)
	}
	return &persistencepb.CreateConnectionRsp{Result: true}, nil
}

// Connect opens a persisted connection by id using the provided (possibly updated) password.
func (srv *server) Connect(ctx context.Context, rqst *persistencepb.ConnectRqst) (*persistencepb.ConnectRsp, error) {
	if rqst.GetConnectionId() == "" {
		return nil, grpcErr(errors.New("no connection id provided"))
	}

	// The connection must exist in persisted config.
	c, ok := srv.Connections[rqst.ConnectionId]
	if !ok {
		return nil, grpcErr(errors.New("no connection found with id " + rqst.ConnectionId))
	}
	// Override password at connect time if provided.
	if rqst.Password != "" {
		c.Password = rqst.Password
	}

	switch c.Store {
	case persistencepb.StoreType_MONGO:
		s := new(persistence_store.MongoStore)
		if err := s.Connect(c.Id, c.Host, c.Port, c.User, c.Password, c.Name, c.Timeout, c.Options); err != nil {
			return nil, grpcErr(err)
		}
		srv.stores[c.Id] = s

	case persistencepb.StoreType_SQL:
		s := new(persistence_store.SqlStore)
		if err := s.Connect(c.Id, c.Host, c.Port, c.User, c.Password, c.Name, c.Timeout, c.Options); err != nil {
			return nil, grpcErr(err)
		}
		srv.stores[c.Id] = s

	case persistencepb.StoreType_SCYLLA:
		s := new(persistence_store.ScyllaStore)
		if err := s.Connect(c.Id, c.Host, c.Port, c.User, c.Password, c.Name, c.Timeout, c.Options); err != nil {
			return nil, grpcErr(err)
		}
		srv.stores[c.Id] = s

	default:
		return nil, grpcErr(errors.New("store type not supported"))
	}

	// Save updated connection (e.g., password change).
	srv.Connections[c.Id] = c
	slog.Info("connection opened", "id", c.Id, "store", c.Store.String())
	return &persistencepb.ConnectRsp{Result: true}, nil
}

// Disconnect closes an open connection by id.
func (srv *server) Disconnect(ctx context.Context, rqst *persistencepb.DisconnectRqst) (*persistencepb.DisconnectRsp, error) {
	if rqst.GetConnectionId() == "" {
		return nil, grpcErr(errors.New("no connection id provided"))
	}
	store, nid, err := srv.storeFor(rqst.GetConnectionId())
	if err != nil {
		return nil, grpcErr(err)
	}
	if err := store.Disconnect(nid); err != nil {
		return nil, grpcErr(err)
	}
	slog.Info("connection disconnected", "id", nid)
	return &persistencepb.DisconnectRsp{Result: true}, nil
}

// CreateDatabase creates a database in the target store.
func (srv *server) CreateDatabase(ctx context.Context, rqst *persistencepb.CreateDatabaseRqst) (*persistencepb.CreateDatabaseRsp, error) {
	if rqst.Id == "" {
		return nil, grpcErr(errors.New("no connection id provided"))
	}
	if rqst.Database == "" {
		return nil, grpcErr(errors.New("no database provided"))
	}
	store, nid, err := srv.storeFor(rqst.Id)
	if err != nil {
		return nil, grpcErr(err)
	}
	if err := store.CreateDatabase(ctx, nid, norm(rqst.Database)); err != nil {
		return nil, grpcErr(err)
	}
	return &persistencepb.CreateDatabaseRsp{Result: true}, nil
}

// DeleteDatabase deletes a database in the target store.
func (srv *server) DeleteDatabase(ctx context.Context, rqst *persistencepb.DeleteDatabaseRqst) (*persistencepb.DeleteDatabaseRsp, error) {
	if rqst.Id == "" {
		return nil, grpcErr(errors.New("no connection id provided"))
	}
	if rqst.Database == "" {
		return nil, grpcErr(errors.New("no database provided"))
	}
	store, nid, err := srv.storeFor(rqst.Id)
	if err != nil {
		return nil, grpcErr(err)
	}
	if err := store.DeleteDatabase(ctx, nid, norm(rqst.Database)); err != nil {
		return nil, grpcErr(err)
	}
	return &persistencepb.DeleteDatabaseRsp{Result: true}, nil
}

// CreateCollection creates a collection with optional options.
func (srv *server) CreateCollection(ctx context.Context, rqst *persistencepb.CreateCollectionRqst) (*persistencepb.CreateCollectionRsp, error) {
	if rqst.Id == "" {
		return nil, grpcErr(errors.New("no connection id provided"))
	}
	if rqst.Database == "" {
		return nil, grpcErr(errors.New("no database provided"))
	}
	store, nid, err := srv.storeFor(rqst.Id)
	if err != nil {
		return nil, grpcErr(err)
	}
	if err := store.CreateCollection(ctx, nid, norm(rqst.Database), rqst.Collection, rqst.OptionsStr); err != nil {
		return nil, grpcErr(err)
	}
	return &persistencepb.CreateCollectionRsp{Result: true}, nil
}

// DeleteCollection deletes a collection in the target database.
func (srv *server) DeleteCollection(ctx context.Context, rqst *persistencepb.DeleteCollectionRqst) (*persistencepb.DeleteCollectionRsp, error) {
	if rqst.Id == "" {
		return nil, grpcErr(errors.New("no connection id provided"))
	}
	if rqst.Database == "" {
		return nil, grpcErr(errors.New("no database provided"))
	}
	store, nid, err := srv.storeFor(rqst.Id)
	if err != nil {
		return nil, grpcErr(err)
	}
	if err := store.DeleteCollection(ctx, nid, norm(rqst.Database), rqst.Collection); err != nil {
		return nil, grpcErr(err)
	}
	return &persistencepb.DeleteCollectionRsp{Result: true}, nil
}

// Ping validates connectivity to the backing store for the given connection id.
func (srv *server) Ping(ctx context.Context, rqst *persistencepb.PingConnectionRqst) (*persistencepb.PingConnectionRsp, error) {
	if rqst.Id == "" {
		return nil, grpcErr(errors.New("no connection id provided"))
	}
	store, nid, err := srv.storeFor(rqst.Id)
	if err != nil {
		return nil, grpcErr(err)
	}
	if err := store.Ping(ctx, nid); err != nil {
		return nil, grpcErr(err)
	}
	return &persistencepb.PingConnectionRsp{Result: "pong"}, nil
}

// Count returns the number of entries in a collection matching an optional query.
func (srv *server) Count(ctx context.Context, rqst *persistencepb.CountRqst) (*persistencepb.CountRsp, error) {
	if rqst.Id == "" {
		return nil, grpcErr(errors.New("no connection id provided"))
	}
	if rqst.Database == "" {
		return nil, grpcErr(errors.New("no database provided"))
	}
	store, nid, err := srv.storeFor(rqst.Id)
	if err != nil {
		return nil, grpcErr(err)
	}
	count, err := store.Count(ctx, nid, norm(rqst.Database), rqst.Collection, rqst.Query, rqst.Options)
	if err != nil {
		return nil, grpcErr(err)
	}
	return &persistencepb.CountRsp{Result: count}, nil
}

// InsertOne inserts a single document into a collection.
func (srv *server) InsertOne(ctx context.Context, rqst *persistencepb.InsertOneRqst) (*persistencepb.InsertOneRsp, error) {
	if rqst.Id == "" {
		return nil, grpcErr(errors.New("no connection id provided"))
	}
	if rqst.Database == "" {
		return nil, grpcErr(errors.New("no database provided"))
	}
	store, nid, err := srv.storeFor(rqst.Id)
	if err != nil {
		return nil, grpcErr(err)
	}

	entity := make(map[string]interface{})
	if err := json.Unmarshal([]byte(rqst.Data), &entity); err != nil {
		return nil, grpcErr(err)
	}

	id, err := store.InsertOne(ctx, nid, norm(rqst.Database), rqst.Collection, entity, rqst.Options)
	if err != nil {
		return nil, grpcErr(err)
	}

	jsonStr, err := Utility.ToJson(id)
	if err != nil {
		return nil, grpcErr(err)
	}
	return &persistencepb.InsertOneRsp{Id: string(jsonStr)}, nil
}

// InsertMany streams a JSON array of documents and inserts them into the target collection.
func (srv *server) InsertMany(stream persistencepb.PersistenceService_InsertManyServer) error {
    var (
        lastID, lastDB, lastColl, lastOptions string
        buf                                   bytes.Buffer
        gotChunk                              bool
    )

    // 1) Read all chunks
    for {
        rqst, err := stream.Recv()
        if err == io.EOF {
            break
        }
        if err != nil {
            return grpcErr(err)
        }

        // Basic validation per chunk (keeps helpful error messages)
        if rqst.GetId() == "" || rqst.GetDatabase() == "" || rqst.GetCollection() == "" {
            return status.Errorf(codes.InvalidArgument, "missing id/database/collection")
        }

        // Remember connection info from the latest chunk
        lastID = rqst.GetId()
        lastDB = rqst.GetDatabase()
        lastColl = rqst.GetCollection()
        if opt := rqst.GetOptions(); opt != "" {
            lastOptions = opt
        }

        if len(rqst.Data) > 0 {
            if _, err := buf.Write(rqst.Data); err != nil {
                return grpcErr(err)
            }
            gotChunk = true
        }
    }

    if !gotChunk {
        return status.Errorf(codes.InvalidArgument, "no data received")
    }

    // 2) Decode JSON array once
    var entities []interface{}
    if err := json.Unmarshal(buf.Bytes(), &entities); err != nil {
        return grpcErr(fmt.Errorf("invalid JSON array: %w", err))
    }

    // 3) Execute write(s)
    store := srv.stores[lastID]
    if store == nil {
        return status.Errorf(codes.NotFound, "connection %q not found", lastID)
    }

    if _, err := store.InsertMany(stream.Context(), lastID, lastDB, lastColl, entities, lastOptions); err != nil {
        return grpcErr(err)
    }

	// 4) Send response AFTER success
	return stream.SendAndClose(&persistencepb.InsertManyRsp{/* nothing here*/})
}

// Find streams matching documents as JSON chunks.
func (srv *server) Find(rqst *persistencepb.FindRqst, stream persistencepb.PersistenceService_FindServer) error {
	if rqst.Id == "" {
		return grpcErr(errors.New("no connection id provided"))
	}
	if rqst.Database == "" {
		return grpcErr(errors.New("no database provided"))
	}

	nid := norm(rqst.Id)
	store := srv.stores[nid]
	if store == nil {
		return grpcErr(errors.New("Find no store connection exists for id " + nid))
	}

	results, err := store.Find(stream.Context(), nid, norm(rqst.Database), rqst.Collection, rqst.Query, rqst.Options)
	if err != nil {
		return grpcErr(errors.New(nid + " " + rqst.Collection + " " + rqst.Query + " " + err.Error()))
	}

	var buffer bytes.Buffer
	enc := json.NewEncoder(&buffer)
	if err := enc.Encode(results); err != nil {
		return grpcErr(err)
	}

	for {
		var data [BufferSize]byte
		n, rerr := buffer.Read(data[0:BufferSize])
		if n > 0 {
			resp := &persistencepb.FindResp{Data: data[0:n]}
			if err := stream.Send(resp); err != nil {
				return grpcErr(err)
			}
		}
		if rerr == io.EOF {
			break
		} else if rerr != nil {
			return grpcErr(rerr)
		}
	}
	return nil
}

// Aggregate runs an aggregation pipeline and streams the result as JSON chunks.
func (srv *server) Aggregate(rqst *persistencepb.AggregateRqst, stream persistencepb.PersistenceService_AggregateServer) error {
	if rqst.Id == "" {
		return grpcErr(errors.New("no connection id provided"))
	}
	if rqst.Database == "" {
		return grpcErr(errors.New("no database provided"))
	}

	store := srv.stores[norm(rqst.Id)]
	if store == nil {
		return grpcErr(errors.New("Aggregate no store connection exists for id " + norm(rqst.Id)))
	}

	results, err := store.Aggregate(stream.Context(), norm(rqst.Id), norm(rqst.Database), rqst.Collection, rqst.Pipeline, rqst.Options)
	if err != nil {
		return grpcErr(err)
	}

	var buffer bytes.Buffer
	enc := json.NewEncoder(&buffer)
	if err := enc.Encode(results); err != nil {
		return grpcErr(err)
	}

	for {
		var data [BufferSize]byte
		n, rerr := buffer.Read(data[0:BufferSize])
		if n > 0 {
			resp := &persistencepb.AggregateResp{Data: data[0:n]}
			if err := stream.Send(resp); err != nil {
				return grpcErr(err)
			}
		}
		if rerr == io.EOF {
			break
		} else if rerr != nil {
			return grpcErr(rerr)
		}
	}
	return nil
}

// FindOne returns a single matching document.
func (srv *server) FindOne(ctx context.Context, rqst *persistencepb.FindOneRqst) (*persistencepb.FindOneResp, error) {
	if rqst.Id == "" {
		return nil, grpcErr(errors.New("no connection id provided"))
	}
	if rqst.Database == "" {
		return nil, grpcErr(errors.New("no database provided"))
	}

	nid := norm(rqst.Id)
	store := srv.stores[nid]
	if store == nil {
		return nil, grpcErr(errors.New("FindOne no store connection exists for id " + nid))
	}

	result, err := store.FindOne(ctx, nid, norm(rqst.Database), rqst.Collection, rqst.Query, rqst.Options)
	if err != nil {
		return nil, grpcErr(errors.New(rqst.Collection + " " + rqst.Query + " " + err.Error()))
	}

	objMap, err := Utility.ToMap(result)
	if err != nil {
		return nil, grpcErr(err)
	}
	obj, err := structpb.NewStruct(objMap)
	if err != nil {
		return nil, grpcErr(err)
	}
	return &persistencepb.FindOneResp{Result: obj}, nil
}

// Update updates multiple documents matching a query.
func (srv *server) Update(ctx context.Context, rqst *persistencepb.UpdateRqst) (*persistencepb.UpdateRsp, error) {
	if rqst.Id == "" {
		return nil, grpcErr(errors.New("no connection id provided"))
	}
	if rqst.Database == "" {
		return nil, grpcErr(errors.New("no database provided"))
	}

	store := srv.stores[norm(rqst.Id)]
	if store == nil {
		return nil, grpcErr(errors.New("Update no store connection exists for id " + norm(rqst.Id)))
	}

	if err := store.Update(ctx, norm(rqst.Id), norm(rqst.Database), rqst.Collection, rqst.Query, rqst.Value, rqst.Options); err != nil {
		return nil, grpcErr(err)
	}
	return &persistencepb.UpdateRsp{Result: true}, nil
}

// UpdateOne updates a single document matching a query.
func (srv *server) UpdateOne(ctx context.Context, rqst *persistencepb.UpdateOneRqst) (*persistencepb.UpdateOneRsp, error) {
	if rqst.Id == "" {
		return nil, grpcErr(errors.New("no connection id provided"))
	}
	if rqst.Database == "" {
		return nil, grpcErr(errors.New("no database provided"))
	}

	store := srv.stores[norm(rqst.Id)]
	if store == nil {
		return nil, grpcErr(errors.New("UpdateOne no store connection exists for id " + norm(rqst.Id)))
	}

	if err := store.UpdateOne(ctx, norm(rqst.Id), norm(rqst.Database), rqst.Collection, rqst.Query, rqst.Value, rqst.Options); err != nil {
		return nil, grpcErr(err)
	}
	return &persistencepb.UpdateOneRsp{Result: true}, nil
}

// ReplaceOne replaces a single document matching a query.
func (srv *server) ReplaceOne(ctx context.Context, rqst *persistencepb.ReplaceOneRqst) (*persistencepb.ReplaceOneRsp, error) {
	store := srv.stores[norm(rqst.Id)]
	if store == nil {
		return nil, grpcErr(errors.New("ReplaceOne no store connection exists for id " + norm(rqst.Id) + " collection: " + rqst.Collection + " query: " + rqst.Query))
	}

	if err := store.ReplaceOne(ctx, norm(rqst.Id), norm(rqst.Database), rqst.Collection, rqst.Query, rqst.Value, rqst.Options); err != nil {
		return nil, grpcErr(err)
	}
	return &persistencepb.ReplaceOneRsp{Result: true}, nil
}

// Delete deletes documents matching a query (one or many based on options).
func (srv *server) Delete(ctx context.Context, rqst *persistencepb.DeleteRqst) (*persistencepb.DeleteRsp, error) {
	store := srv.stores[norm(rqst.Id)]
	if store == nil {
		return nil, grpcErr(errors.New("Delete no store connection exists for id " + norm(rqst.Id)))
	}
	if err := store.Delete(ctx, norm(rqst.Id), norm(rqst.Database), rqst.Collection, rqst.Query, rqst.Options); err != nil {
		return nil, grpcErr(err)
	}
	return &persistencepb.DeleteRsp{Result: true}, nil
}

// DeleteOne deletes a single document matching a query.
func (srv *server) DeleteOne(ctx context.Context, rqst *persistencepb.DeleteOneRqst) (*persistencepb.DeleteOneRsp, error) {
	store := srv.stores[norm(rqst.Id)]
	if store == nil {
		return nil, grpcErr(errors.New("DeleteOne no store connection exists for id " + norm(rqst.Id)))
	}
	if err := store.DeleteOne(ctx, norm(rqst.Id), norm(rqst.Database), rqst.Collection, rqst.Query, rqst.Options); err != nil {
		return nil, grpcErr(err)
	}
	return &persistencepb.DeleteOneRsp{Result: true}, nil
}

// DeleteConnection removes a persisted connection from configuration.
func (srv *server) DeleteConnection(ctx context.Context, rqst *persistencepb.DeleteConnectionRqst) (*persistencepb.DeleteConnectionRsp, error) {
	id := norm(rqst.Id)
	if _, ok := srv.Connections[id]; !ok {
		return &persistencepb.DeleteConnectionRsp{Result: true}, nil
	}

	delete(srv.Connections, id)
	if err := srv.Save(); err != nil {
		return nil, grpcErr(err)
	}
	slog.Info("connection removed from config", "id", id)
	return &persistencepb.DeleteConnectionRsp{Result: true}, nil
}

// RunAdminCmd runs an admin script against the target store with the provided credentials.
func (srv *server) RunAdminCmd(ctx context.Context, rqst *persistencepb.RunAdminCmdRqst) (*persistencepb.RunAdminCmdRsp, error) {
	store, _, err := srv.storeFor(rqst.GetConnectionId())
	if err != nil {
		return nil, grpcErr(errors.New("RunAdminCmd " + err.Error()))
	}
	if err := store.RunAdminCmd(ctx, rqst.GetConnectionId(), rqst.User, rqst.Password, rqst.Script); err != nil {
		return nil, grpcErr(err)
	}
	return &persistencepb.RunAdminCmdRsp{Result: ""}, nil
}
