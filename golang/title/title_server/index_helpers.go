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

	// If shared index is active, enqueue for cluster-wide indexing.
	if srv.sharedIndex != nil {
		return srv.sharedIndex.Enqueue("/search/titles", title.UUID, string(raw), string(raw), "UUID", nil)
	}

	// Fallback: local-only indexing.
	if err := index.Index(title.UUID, title); err != nil {
		return err
	}
	return index.SetInternal([]byte(title.UUID), raw)
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

	if srv.sharedIndex != nil {
		return srv.sharedIndex.Enqueue("/search/videos", video.UUID, string(raw), string(raw), "UUID", nil)
	}

	if err := index.Index(video.UUID, video); err != nil {
		return err
	}
	return index.SetInternal([]byte(video.UUID), raw)
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

	if srv.sharedIndex != nil {
		return srv.sharedIndex.Enqueue("/search/audios", audio.UUID, string(raw), string(raw), "UUID", nil)
	}

	if err := index.Index(audio.UUID, audio); err != nil {
		return err
	}
	return index.SetInternal([]byte(audio.UUID), raw)
}
