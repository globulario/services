package search_engine

import (
	"github.com/globulario/services/golang/search/searchpb"
)

/**
 * A key value data store.
 */
type SearchEngine interface {

	// Get the store version.
	GetVersion() string

	// JSON document search functionalities

	// Set a document from list of db from given paths...
	SearchDocuments(paths []string, language string, fields []string, query string, offset, pageSize, snippetLength int32) (*searchpb.SearchResults, error)

	// Delete a document with a given path and id.
	DeleteDocument(path string, id string) error

	// Index a given object.
	IndexJsonObject(path string, jsonStr string, language string, id string, indexs []string, data string) error

	// Count the number of document in a db.
	Count(path string) int32
}
