package main

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/globulario/services/golang/storage/storage_store"
	"github.com/globulario/services/golang/storage/storagepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const BufferSize = 1024 * 5 // chunk size used for streaming values

//////////////////////// Storage-specific RPCs ////////////////////////////

// CreateConnection creates or replaces a connection definition and persists it.
func (srv *server) CreateConnection(ctx context.Context, rqst *storagepb.CreateConnectionRqst) (*storagepb.CreateConnectionRsp, error) {
	if rqst.Connection == nil {
		return nil, errors.New("create connection: request missing connection object")
	}

	if srv.stores == nil {
		srv.stores = make(map[string]storage_store.Store)
	}
	if srv.Connections == nil {
		srv.Connections = make(map[string]connection)
	}

	// Close any existing store for this connection id
	if prev, ok := srv.Connections[rqst.Connection.Id]; ok {
		if st := srv.stores[prev.Id]; st != nil {
			_ = st.Close()
		}
	}

	conn := connection{
		Id:   rqst.Connection.Id,
		Name: rqst.Connection.Name,
		Type: rqst.Connection.Type,
	}
	srv.Connections[conn.Id] = conn

	if err := srv.Save(); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	logger.Info("connection created/updated", "id", conn.Id, "name", conn.Name, "type", conn.Type.String())

	return &storagepb.CreateConnectionRsp{Result: true}, nil
}

// DeleteConnection removes a connection and persists the config update.
func (srv *server) DeleteConnection(ctx context.Context, rqst *storagepb.DeleteConnectionRqst) (*storagepb.DeleteConnectionRsp, error) {
	id := rqst.GetId()
	if _, ok := srv.Connections[id]; !ok {
		return &storagepb.DeleteConnectionRsp{Result: true}, nil
	}

	// Close and remove
	if st := srv.stores[id]; st != nil {
		_ = st.Close()
		delete(srv.stores, id)
	}
	delete(srv.Connections, id)

	if err := srv.Save(); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	logger.Info("connection deleted", "id", id)
	return &storagepb.DeleteConnectionRsp{Result: true}, nil
}

// Open initializes the selected store with provided options.
func (srv *server) Open(ctx context.Context, rqst *storagepb.OpenRqst) (*storagepb.OpenRsp, error) {
	if _, ok := srv.Connections[rqst.GetId()]; !ok {
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("open: no connection found with id "+rqst.GetId())))
	}

	var store storage_store.Store
	conn := srv.Connections[rqst.GetId()]

	switch conn.Type {
	case storagepb.StoreType_LEVEL_DB:
		store = storage_store.NewLevelDB_store()
	case storagepb.StoreType_BIG_CACHE:
		store = storage_store.NewBigCache_store()
	case storagepb.StoreType_BADGER_DB:
		store = storage_store.NewBadger_store()
	case storagepb.StoreType_SCYLLA_DB:
		store = storage_store.NewScylla_store("127.0.0.1", "", 3)
	default:
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("open: unsupported store type for connection id "+rqst.GetId())))
	}

	if err := store.Open(rqst.GetOptions()); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.stores[rqst.GetId()] = store
	logger.Info("store opened", "id", rqst.GetId(), "type", conn.Type.String())
	return &storagepb.OpenRsp{Result: true}, nil
}

// Close shuts down the store connected to the given connection id.
func (srv *server) Close(ctx context.Context, rqst *storagepb.CloseRqst) (*storagepb.CloseRsp, error) {
	if _, ok := srv.Connections[rqst.GetId()]; !ok {
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("close: no connection found with id "+rqst.GetId())))
	}
	store := srv.stores[rqst.GetId()]
	if store == nil {
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("close: no store found for connection id "+rqst.GetId())))
	}

	if err := store.Close(); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	logger.Info("store closed", "id", rqst.GetId())
	return &storagepb.CloseRsp{Result: true}, nil
}

// SetItem writes a small value under the given key.
func (srv *server) SetItem(ctx context.Context, rqst *storagepb.SetItemRequest) (*storagepb.SetItemResponse, error) {
	if _, ok := srv.Connections[rqst.GetId()]; !ok {
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("setItem: no connection found with id "+rqst.GetId())))
	}
	store := srv.stores[rqst.GetId()]
	if store == nil {
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("setItem: no store found for connection id "+rqst.GetId())))
	}

	if err := store.SetItem(rqst.GetKey(), rqst.GetValue()); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &storagepb.SetItemResponse{Result: true}, nil
}

// SetLargeItem streams chunks and stores the concatenated value under the given key.
func (srv *server) SetLargeItem(stream storagepb.StorageService_SetLargeItemServer) error {
	var rqst *storagepb.SetLargeItemRequest
	var buffer bytes.Buffer

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			rqst = msg // keep last non-nil for id/key
			if err := stream.SendAndClose(&storagepb.SetLargeItemResponse{}); err != nil {
				logger.Error("setLargeItem: send-and-close failed", "err", err)
				return err
			}
			break
		}
		if err != nil {
			return err
		}
		rqst = msg
		buffer.Write(msg.Value)
	}

	if _, ok := srv.Connections[rqst.GetId()]; !ok {
		return status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("setLargeItem: no connection found with id "+rqst.GetId())))
	}
	store := srv.stores[rqst.GetId()]
	if store == nil {
		return status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("setLargeItem: no store found for connection id "+rqst.GetId())))
	}

	if err := store.SetItem(rqst.GetKey(), buffer.Bytes()); err != nil {
		return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return nil
}

// GetItem streams back the stored value in fixed-size chunks.
func (srv *server) GetItem(rqst *storagepb.GetItemRequest, stream storagepb.StorageService_GetItemServer) error {
	if _, ok := srv.Connections[rqst.GetId()]; !ok {
		return status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("getItem: no connection found with id "+rqst.GetId())))
	}
	store := srv.stores[rqst.GetId()]
	if store == nil {
		return status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("getItem: no store found for connection id "+rqst.GetId())))
	}

	value, err := store.GetItem(rqst.GetKey())
	if err != nil {
		return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	reader := bytes.NewReader(value)
	for {
		var data [BufferSize]byte
		n, rerr := reader.Read(data[:])
		if n > 0 {
			if err := stream.Send(&storagepb.GetItemResponse{Result: data[:n]}); err != nil {
				return err
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	return nil
}

// RemoveItem deletes a specific key.
func (srv *server) RemoveItem(ctx context.Context, rqst *storagepb.RemoveItemRequest) (*storagepb.RemoveItemResponse, error) {
	if _, ok := srv.Connections[rqst.GetId()]; !ok {
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("removeItem: no connection found with id "+rqst.GetId())))
	}
	store := srv.stores[rqst.GetId()]
	if store == nil {
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("removeItem: no store found for connection id "+rqst.GetId())))
	}

	if err := store.RemoveItem(rqst.GetKey()); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &storagepb.RemoveItemResponse{Result: true}, nil
}

// Clear removes all keys/values from the store.
func (srv *server) Clear(ctx context.Context, rqst *storagepb.ClearRequest) (*storagepb.ClearResponse, error) {
	if _, ok := srv.Connections[rqst.GetId()]; !ok {
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("clear: no connection found with id "+rqst.GetId())))
	}
	store := srv.stores[rqst.GetId()]
	if store == nil {
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("clear: no store found for connection id "+rqst.GetId())))
	}

	if err := store.Clear(); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &storagepb.ClearResponse{Result: true}, nil
}

// Drop destroys the underlying storage (if supported) and closes it.
func (srv *server) Drop(ctx context.Context, rqst *storagepb.DropRequest) (*storagepb.DropResponse, error) {
	if _, ok := srv.Connections[rqst.GetId()]; !ok {
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("drop: no connection found with id "+rqst.GetId())))
	}
	store := srv.stores[rqst.GetId()]
	if store == nil {
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("drop: no store found for connection id "+rqst.GetId())))
	}

	if err := store.Drop(); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	delete(srv.stores, rqst.GetId())

	logger.Info("store dropped", "id", rqst.GetId())
	return &storagepb.DropResponse{Result: true}, nil
}

// GetAllKeys streams back all keys in the store.
func (srv *server) GetAllKeys(rqst *storagepb.GetAllKeysRequest, stream storagepb.StorageService_GetAllKeysServer) error {
	if _, ok := srv.Connections[rqst.GetId()]; !ok {
		return status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("getAllKeys: no connection found with id "+rqst.GetId())))
	}
	store := srv.stores[rqst.GetId()]
	if store == nil {
		return status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("getAllKeys: no store found for connection id "+rqst.GetId())))
	}

	keys, err := store.GetAllKeys()
	if err != nil {
		return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	const chunkSize = 100	
	for i := 0; i < len(keys); i += chunkSize {
		end := i + chunkSize
		if end > len(keys) {
			end = len(keys)
		}
		if err := stream.Send(&storagepb.GetAllKeysResponse{Keys: keys[i:end]}); err != nil {
			return err
		}
	}

	return nil
}
