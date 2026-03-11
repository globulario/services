package search_engine

import (
	"github.com/globulario/services/golang/search/searchpb"
)

// -----------------------------------------------------------------------------
// engine.go (interface) — public prototype left unchanged, just light docs
// -----------------------------------------------------------------------------

// SearchEngine defines the JSON search/index operations used by the service.
//
// NOTE: This section mirrors the original engine.go content but with
// comments for clarity. The signatures are identical to preserve ABI.
type SearchEngine interface {
	// GetVersion returns a human-readable version label of the engine.
	GetVersion() string

	// SearchDocuments searches across one or more index paths and returns
	// a SearchResults message with highlighted fragments.
	SearchDocuments(paths []string, language string, fields []string, query string, offset, pageSize, snippetLength int32) (*searchpb.SearchResults, error)

	// DeleteDocument deletes a document id from a specific index path.
	DeleteDocument(path string, id string) error

	// IndexJsonObject indexes an object (or array of objects) provided as JSON.
	IndexJsonObject(path string, jsonStr string, language string, id string, indexs []string, data string) error

	// Count returns the number of documents in an index, or -1 on error.
	Count(path string) int32

	// GetIndexPaths returns the paths of all currently open indices.
	GetIndexPaths() []string

	// CloseAll closes all open indices, flushing pending writes to disk.
	// After CloseAll the engine will not serve queries until ReopenAll is called.
	CloseAll() error

	// ReopenAll reopens the given index paths after a backup or restore.
	ReopenAll(paths []string) error
}
