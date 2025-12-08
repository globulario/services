package main

import (
	"context"
	"errors"
	"os"

	"github.com/blevesearch/bleve/v2"
	"github.com/globulario/services/golang/title/titlepb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/emptypb"
)

var defaultCollectionIndexPaths = map[string]string{
	"titles": "/search/titles",
	"videos": "/search/videos",
	"audios": "/search/audios",
}

func (srv *server) RebuildIndexFromStore(ctx context.Context, rqst *titlepb.RebuildIndexRequest) (*emptypb.Empty, error) {
	collections := rqst.GetCollections()
	if len(collections) == 0 {
		collections = []string{"titles", "videos", "audios"}
	}

	for _, collection := range collections {
		if err := srv.rebuildCollection(ctx, collection, rqst.GetIncremental()); err != nil {
			return nil, status.Errorf(codes.Internal, "rebuild %s index: %v", collection, err)
		}
	}
	return &emptypb.Empty{}, nil
}

func (srv *server) rebuildCollection(ctx context.Context, collection string, incremental bool) error {
	indexPath, ok := defaultCollectionIndexPaths[collection]
	if !ok {
		return errors.New("unknown collection " + collection)
	}

	resolved, err := srv.resolveIndexPath(indexPath)
	if err != nil {
		return err
	}

	if !incremental {
		if idx, ok := srv.indexs[resolved]; ok && idx != nil {
			_ = idx.Close()
			delete(srv.indexs, resolved)
		}
		if err := os.RemoveAll(resolved); err != nil {
			logger.Warn("rebuild: remove index dir failed", "path", resolved, "err", err)
		}
	}

	index, err := srv.getIndex(indexPath)
	if err != nil {
		return err
	}

	switch collection {
	case "titles":
		return srv.reindexTitles(ctx, indexPath, index)
	case "videos":
		return srv.reindexVideos(ctx, indexPath, index)
	case "audios":
		return srv.reindexAudios(ctx, indexPath, index)
	default:
		return errors.New("unsupported collection " + collection)
	}
}

func (srv *server) reindexTitles(ctx context.Context, indexPath string, index bleve.Index) error {
	store, err := srv.getMetadataStore(indexPath, "titles")
	if err != nil {
		return err
	}
	keys, err := store.GetAllKeys()
	if err != nil {
		return err
	}
	for _, key := range keys {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		data, err := store.GetItem(key)
		if err != nil || len(data) == 0 {
			continue
		}
		title := new(titlepb.Title)
		if err := protojson.Unmarshal(data, title); err != nil {
			logger.Warn("rebuild: decode title failed", "key", key, "err", err)
			continue
		}
		if err := srv.indexTitleDoc(index, title); err != nil {
			logger.Warn("rebuild: index title failed", "titleID", title.GetID(), "err", err)
		}
	}
	return nil
}

func (srv *server) reindexVideos(ctx context.Context, indexPath string, index bleve.Index) error {
	store, err := srv.getMetadataStore(indexPath, "videos")
	if err != nil {
		return err
	}
	keys, err := store.GetAllKeys()
	if err != nil {
		return err
	}
	for _, key := range keys {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		data, err := store.GetItem(key)
		if err != nil || len(data) == 0 {
			continue
		}
		video := new(titlepb.Video)
		if err := protojson.Unmarshal(data, video); err != nil {
			logger.Warn("rebuild: decode video failed", "key", key, "err", err)
			continue
		}
		if err := srv.indexVideoDoc(index, video); err != nil {
			logger.Warn("rebuild: index video failed", "videoID", video.GetID(), "err", err)
		}
	}
	return nil
}

func (srv *server) reindexAudios(ctx context.Context, indexPath string, index bleve.Index) error {
	store, err := srv.getMetadataStore(indexPath, "audios")
	if err != nil {
		return err
	}
	keys, err := store.GetAllKeys()
	if err != nil {
		return err
	}
	for _, key := range keys {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		data, err := store.GetItem(key)
		if err != nil || len(data) == 0 {
			continue
		}
		audio := new(titlepb.Audio)
		if err := protojson.Unmarshal(data, audio); err != nil {
			logger.Warn("rebuild: decode audio failed", "key", key, "err", err)
			continue
		}
		if err := srv.indexAudioDoc(index, audio); err != nil {
			logger.Warn("rebuild: index audio failed", "audioID", audio.GetID(), "err", err)
		}
	}
	return nil
}
