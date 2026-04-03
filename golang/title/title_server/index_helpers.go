package main

import (
	"fmt"

	"github.com/blevesearch/bleve/v2"
	"github.com/globulario/services/golang/title/titlepb"
		"google.golang.org/protobuf/encoding/protojson"
	Utility "github.com/globulario/utility"
)

func (srv *server) indexTitleDoc(index bleve.Index, title *titlepb.Title) error {
	if title == nil {
		return fmt.Errorf("indexTitleDoc: nil title")
	}
	if title.UUID == "" {
		title.UUID = Utility.GenerateUUID(title.GetID())
	}

	raw, err := protojson.Marshal(title)
	if err != nil {
		return err
	}

	// Always write to the local Bleve index so reads on this node see
	// the data immediately (the shared index writer opens a separate
	// Bleve handle on the same directory which causes read/write split).
	if err := index.Index(title.UUID, title); err != nil {
		return err
	}
	if err := index.SetInternal([]byte(title.UUID), raw); err != nil {
		return err
	}

	// If shared index is active, also enqueue for cluster-wide distribution.
	if srv.sharedIndex != nil {
		if err := srv.sharedIndex.Enqueue("/search/titles", title.UUID, string(raw), string(raw), "UUID", nil); err != nil {
			logger.Warn("shared index enqueue failed (local index updated)", "titleID", title.ID, "err", err)
		}
	}
	return nil
}

func (srv *server) indexVideoDoc(index bleve.Index, video *titlepb.Video) error {
	if video == nil {
		return fmt.Errorf("indexVideoDoc: nil video")
	}
	if video.UUID == "" {
		video.UUID = Utility.GenerateUUID(video.GetID())
	}

	raw, err := protojson.Marshal(video)
	if err != nil {
		return err
	}

	if err := index.Index(video.UUID, video); err != nil {
		return err
	}
	if err := index.SetInternal([]byte(video.UUID), raw); err != nil {
		return err
	}

	if srv.sharedIndex != nil {
		if err := srv.sharedIndex.Enqueue("/search/videos", video.UUID, string(raw), string(raw), "UUID", nil); err != nil {
			logger.Warn("shared index enqueue failed (local index updated)", "videoID", video.ID, "err", err)
		}
	}
	return nil
}

func (srv *server) indexAudioDoc(index bleve.Index, audio *titlepb.Audio) error {
	if audio == nil {
		return fmt.Errorf("indexAudioDoc: nil audio")
	}
	if audio.UUID == "" {
		audio.UUID = Utility.GenerateUUID(audio.GetID())
	}

	raw, err := protojson.Marshal(audio)
	if err != nil {
		return err
	}

	if err := index.Index(audio.UUID, audio); err != nil {
		return err
	}
	if err := index.SetInternal([]byte(audio.UUID), raw); err != nil {
		return err
	}

	if srv.sharedIndex != nil {
		if err := srv.sharedIndex.Enqueue("/search/audios", audio.UUID, string(raw), string(raw), "UUID", nil); err != nil {
			logger.Warn("shared index enqueue failed (local index updated)", "audioID", audio.ID, "err", err)
		}
	}
	return nil
}
