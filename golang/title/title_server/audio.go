package main

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/title/titlepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// CreateAudio inserts or updates an Audio in the search index and sets RBAC ownership.
// It also ensures the album exists and publishes an update event.
func (srv *server) CreateAudio(ctx context.Context, rqst *titlepb.CreateAudioRequest) (*titlepb.CreateAudioResponse, error) {
	if err := checkNotNil("audio", rqst.Audio); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if err := checkArg("audio id", rqst.Audio.GetID()); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	clientId, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolve client id: %v", err)
	}

	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "open index: %v", err)
	}

	rqst.Audio.UUID = Utility.GenerateUUID(rqst.Audio.ID)

	if err := srv.indexAudioDoc(index, rqst.Audio); err != nil {
		return nil, status.Errorf(codes.Internal, "index audio %q: %v", rqst.Audio.ID, err)
	}
	jsonStr, err := protojson.Marshal(rqst.Audio)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal audio: %v", err)
	}
	if err := srv.persistMetadata(rqst.IndexPath, "audios", rqst.Audio.ID, rqst.Audio); err != nil {
		logger.Warn("persistMetadata audio failed", "audioID", rqst.Audio.ID, "err", err)
	}

	// RBAC: ensure owner
	rbacClient, err := srv.getRbacClient()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "rbac client: %v", err)
	}
	if perms, _ := rbacClient.GetResourcePermissions(rqst.Audio.ID); perms == nil {
		if err := rbacClient.AddResourceOwner(token, rqst.Audio.ID, clientId, "audio_infos", rbacpb.SubjectType_ACCOUNT); err != nil {
			return nil, status.Errorf(codes.Internal, "set audio owner: %v", err)
		}
	}

	// Ensure album exists alongside audio
	if _, err := srv.getAlbum(rqst.IndexPath, rqst.Audio.Album); err != nil {
		album := &titlepb.Album{
			ID:     rqst.Audio.Album,
			Artist: rqst.Audio.AlbumArtist,
			Year:   rqst.Audio.Year,
			Genres: rqst.Audio.Genres,
			Poster: rqst.Audio.Poster,
		}
		if raw, err := protojson.Marshal(album); err == nil {
			if err := index.SetInternal([]byte(Utility.GenerateUUID(album.ID)), raw); err != nil {
				return nil, status.Errorf(codes.Internal, "store raw album %q: %v", album.ID, err)
			}
		}
	}

	evt, err := srv.getEventClient()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "event client: %v", err)
	}
	evt.Publish("update_audio_infos_evt", jsonStr)

	logger.Info("audio created", "audioID", rqst.Audio.ID)
	return &titlepb.CreateAudioResponse{}, nil
}

// getAudioById returns an Audio stored in the index's internal store by its id.
func (srv *server) getAudioById(indexPath, id string) (*titlepb.Audio, error) {
	resolved, err := srv.resolveIndexPath(indexPath)
	if err != nil {
		return nil, err
	}
	if !Utility.Exists(resolved) {
		return nil, errors.New("no database found at path " + resolved)
	}
	index, err := srv.getIndex(resolved)
	if err != nil {
		return nil, errf("open index %q", err, indexPath)
	}
	uuid := Utility.GenerateUUID(id)
	raw, err := index.GetInternal([]byte(uuid))
	if err != nil {
		return nil, errf("load audio %q", err, id)
	}
	if len(raw) == 0 {
		return nil, errors.New("no audio found with id " + id)
	}
	audio := new(titlepb.Audio)
	if err := protojson.Unmarshal(raw, audio); err != nil {
		return nil, errf("decode audio %q", err, id)
	}
	return audio, nil
}

// GetAudioById returns an Audio with associated file paths, if any.
func (srv *server) GetAudioById(ctx context.Context, rqst *titlepb.GetAudioByIdRequest) (*titlepb.GetAudioByIdResponse, error) {
	resolved, err := srv.resolveIndexPath(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	audio, err := srv.getAudioById(resolved, rqst.AudioId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	paths := []string{}
	if store := srv.getAssociations(resolved); store != nil {
		if data, err := store.GetItem(rqst.AudioId); err == nil {
			a := new(fileTileAssociation)
			if err := json.Unmarshal(data, a); err == nil {
				paths = a.Paths
			}
		}
	}

	return &titlepb.GetAudioByIdResponse{Audio: audio, FilesPaths: paths}, nil
}

// getAlbum loads an Album from the index by ID (stored in internal KV).
func (srv *server) getAlbum(indexPath, id string) (*titlepb.Album, error) {
	if !Utility.Exists(indexPath) {
		return nil, errors.New("no database found at path " + indexPath)
	}
	index, err := srv.getIndex(indexPath)
	if err != nil {
		return nil, err
	}
	q := bleve.NewQueryStringQuery(id)
	req := bleve.NewSearchRequest(q)
	res, err := index.Search(req)
	if err != nil {
		return nil, err
	}
	if res.Total == 0 {
		return nil, errors.New("no album found with id " + id)
	}
	for _, h := range res.Hits {
		if h.ID == id {
			uuid := Utility.GenerateUUID(h.ID)
			raw, err := index.GetInternal([]byte(uuid))
			if err != nil {
				return nil, err
			}
			album := new(titlepb.Album)
			if err := protojson.Unmarshal(raw, album); err != nil {
				return nil, err
			}
			return album, nil
		}
	}
	return nil, errors.New("no album found with id " + id)
}

// GetAlbum returns Album info by ID.
func (srv *server) GetAlbum(ctx context.Context, rqst *titlepb.GetAlbumRequest) (*titlepb.GetAlbumResponse, error) {
	album, err := srv.getAlbum(rqst.IndexPath, rqst.AlbumId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &titlepb.GetAlbumResponse{Album: album}, nil
}

// DeleteAlbum removes an album from the Bleve index and its internal store.
// NOTE: This mirrors the original behavior; it does not cascade-delete
// related audio tracks yet (left as a TODO).
func (srv *server) DeleteAlbum(ctx context.Context, rqst *titlepb.DeleteAlbumRequest) (*titlepb.DeleteAlbumResponse, error) {
	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		logger.Error("open index failed", "indexPath", rqst.IndexPath, "err", err)
		return nil, status.Errorf(codes.Internal, "open index: %v", err)
	}

	uuid := Utility.GenerateUUID(rqst.AlbumId)

	if err := index.Delete(uuid); err != nil {
		logger.Error("delete album doc failed", "albumID", rqst.AlbumId, "uuid", uuid, "err", err)
		return nil, status.Errorf(codes.Internal, "delete album: %v", err)
	}

	if err := index.DeleteInternal([]byte(uuid)); err != nil {
		logger.Error("delete album internal failed", "albumID", rqst.AlbumId, "uuid", uuid, "err", err)
		return nil, status.Errorf(codes.Internal, "delete album (internal): %v", err)
	}

	logger.Info("album deleted", "albumID", rqst.AlbumId, "indexPath", rqst.IndexPath)
	// TODO: remove associated files / tracks if you want cascade behavior.
	return &titlepb.DeleteAlbumResponse{}, nil
}

// deleteAudio removes an Audio and all associations, RBAC permissions and publishes events.
func (srv *server) deleteAudio(token, indexPath string, audioId string) error {
	index, err := srv.getIndex(indexPath)
	if err != nil {
		return err
	}

	dirs := make([]string, 0)
	if paths, err := srv.getTitleFiles(indexPath, audioId); err == nil {
		for _, p := range paths {
			_ = srv.dissociateFileWithTitle(token, indexPath, audioId, p)
			dirs = append(dirs, filepath.Dir(strings.ReplaceAll(p, config.GetDataDir()+"/files", "")))
		}
	}

	uuid := Utility.GenerateUUID(audioId)
	if err := index.Delete(uuid); err != nil {
		return err
	}
	if err := index.DeleteInternal([]byte(uuid)); err != nil {
		return err
	}
	srv.removeMetadata(indexPath, "audios", audioId)

	rbacClient, err := srv.getRbacClient()
	if err != nil {
		return err
	}
	if err := rbacClient.DeleteResourcePermissions(token, audioId); err != nil {
		return err
	}

	for _, d := range dirs {
		_ = srv.publish("reload_dir_event", []byte(d))
	}
	return srv.publish("delete_audio_event", []byte(audioId))
}

// DeleteAudio gRPC removes an audio by ID.
func (srv *server) DeleteAudio(ctx context.Context, rqst *titlepb.DeleteAudioRequest) (*titlepb.DeleteAudioResponse, error) {
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolve client id: %v", err)
	}

	if err := srv.deleteAudio(token, rqst.IndexPath, rqst.AudioId); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	logger.Info("audio deleted", "audioID", rqst.AudioId)
	return &titlepb.DeleteAudioResponse{}, nil
}
