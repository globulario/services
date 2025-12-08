package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/storage/storage_store"
	"github.com/globulario/services/golang/title/titlepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/emptypb"
)

const watchingStoreBaseName = "watching"

func (srv *server) getWatchingStore() (storage_store.Store, error) {
	name := watchingStoreBaseName
	if srv.Domain != "" {
		name = fmt.Sprintf("%s_%s", watchingStoreBaseName, srv.Domain)
	}
	root := filepath.Join(config.GetDataDir(), watchingStoreBaseName)
	if srv.Domain != "" {
		root = filepath.Join(root, srv.Domain)
	}
	if err := Utility.CreateIfNotExists(root, 0o755); err != nil {
		return nil, fmt.Errorf("ensure watching store directory %s: %w", root, err)
	}
	return srv.getStore(name, root)
}

func (srv *server) ListWatching(ctx context.Context, rqst *titlepb.ListWatchingRequest) (*titlepb.ListWatchingResponse, error) {
	store, err := srv.getWatchingStore()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "open watching store: %v", err)
	}
	keys, err := store.GetAllKeys()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list watching keys: %v", err)
	}
	resp := &titlepb.ListWatchingResponse{Items: make([]*titlepb.WatchingEntry, 0, len(keys))}
	for _, key := range keys {
		data, err := store.GetItem(key)
		if err != nil || len(data) == 0 {
			continue
		}
		entry := new(titlepb.WatchingEntry)
		if err := protojson.Unmarshal(data, entry); err != nil {
			logger.Warn("watching: decode entry failed", "key", key, "err", err)
			continue
		}
		resp.Items = append(resp.Items, entry)
	}
	return resp, nil
}

func (srv *server) GetWatching(ctx context.Context, rqst *titlepb.GetWatchingRequest) (*titlepb.WatchingEntry, error) {
	titleID := strings.TrimSpace(rqst.GetTitleId())
	if titleID == "" {
		return nil, status.Error(codes.InvalidArgument, "title id is required")
	}
	store, err := srv.getWatchingStore()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "open watching store: %v", err)
	}
	data, err := store.GetItem(titleID)
	if err != nil || len(data) == 0 {
		return nil, status.Errorf(codes.NotFound, "watching entry %s not found", titleID)
	}
	entry := new(titlepb.WatchingEntry)
	if err := protojson.Unmarshal(data, entry); err != nil {
		return nil, status.Errorf(codes.Internal, "decode watching entry: %v", err)
	}
	return entry, nil
}

func (srv *server) SaveWatching(ctx context.Context, rqst *titlepb.SaveWatchingRequest) (*emptypb.Empty, error) {
	entry := rqst.GetEntry()
	if entry == nil {
		return nil, status.Error(codes.InvalidArgument, "entry is required")
	}
	titleID := strings.TrimSpace(entry.GetTitleId())
	if titleID == "" {
		return nil, status.Error(codes.InvalidArgument, "title id is required")
	}
	if entry.Id == "" {
		entry.Id = titleID
	}
	if entry.Domain == "" {
		entry.Domain = srv.Domain
	}
	if entry.UpdatedAt == "" {
		entry.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}

	store, err := srv.getWatchingStore()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "open watching store: %v", err)
	}
	data, err := protojson.Marshal(entry)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "encode watching entry: %v", err)
	}
	if err := store.SetItem(titleID, data); err != nil {
		return nil, status.Errorf(codes.Internal, "persist watching entry: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (srv *server) RemoveWatching(ctx context.Context, rqst *titlepb.RemoveWatchingRequest) (*emptypb.Empty, error) {
	titleID := strings.TrimSpace(rqst.GetTitleId())
	if titleID == "" {
		return nil, status.Error(codes.InvalidArgument, "title id is required")
	}
	store, err := srv.getWatchingStore()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "open watching store: %v", err)
	}
	if err := store.RemoveItem(titleID); err != nil {
		return nil, status.Errorf(codes.Internal, "remove watching entry: %v", err)
	}
	return &emptypb.Empty{}, nil
}
