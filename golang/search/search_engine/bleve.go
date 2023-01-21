package search_engine

import (
	"encoding/json"
	"errors"
	"github.com/blevesearch/bleve"
	"github.com/globulario/services/golang/search/searchpb"
	"log"
)

/**
 * A key value data store.
 */
type BleveSearchEngine struct {
	indexs map[string]bleve.Index
}

/**
 * Return indexation for a given path...
 */
func (engine *BleveSearchEngine) getIndex(path string) (bleve.Index, error) {
	if engine.indexs[path] == nil {
		index, err := bleve.Open(path) // try to open existing index.
		if err != nil {
			mapping := bleve.NewIndexMapping()
			var err error
			index, err = bleve.New(path, mapping)
			if err != nil {
				return nil, err
			}
		}

		if engine.indexs == nil {
			engine.indexs = make(map[string]bleve.Index, 0)
		}

		engine.indexs[path] = index
	}

	return engine.indexs[path], nil
}

// Get the store version.
func (engine *BleveSearchEngine) GetVersion() string {
	return "2.0"
}

// Set a document from list of db from given paths...
func (engine *BleveSearchEngine) SearchDocuments(paths []string, language string, fields []string, q string, offset, pageSize, snippetLength int32) (*searchpb.SearchResults, error) {
	results := new(searchpb.SearchResults)
	results.Results = make([]*searchpb.SearchResult, 0)
	for i := 0; i < len(paths); i++ {
		index, err := engine.getIndex(paths[i])
		if err != nil {
			return nil, err
		}

		query := bleve.NewQueryStringQuery(q)
		searchRequest := bleve.NewSearchRequest(query)
		searchRequest.Fields = fields
		searchRequest.From = int(offset)
		searchRequest.Size = int(pageSize)
		searchRequest.Highlight = bleve.NewHighlightWithStyle("html")
		searchResult, err := index.Search(searchRequest)
		if err != nil {
			return nil, err
		}

		// Now from the result I will
		if searchResult.Total == 0 {
			return nil, errors.New("No matches") // return as error...
		}

		// Now I will return the data
		for _, val := range searchResult.Hits {
			id := val.ID
			raw, err := index.GetInternal([]byte(id))
			if err != nil {
				log.Fatal("Trouble getting internal doc:", err)
			}
			result := new(searchpb.SearchResult)
			result.Data = string(raw)

			result.DocId = id
			result.Rank = int32(val.Score * 100)

			// serialyse the fragment and set it as snippet.
			data, err := json.Marshal(val.Fragments)
			if err == nil {
				result.Snippet = string(data)
			}

			results.Results = append(results.Results, result)
		}

	}

	return results, nil
}

// Delete a document with a given path and id.
func (engine *BleveSearchEngine) DeleteDocument(path string, id string) error {

	index, err := engine.getIndex(path)
	if err != nil {
		return err
	}

	err = index.Delete(id)
	if err != nil {
		return err
	}

	return nil
}

func (search_engine *BleveSearchEngine) indexJsonObject(index bleve.Index, obj map[string]interface{}, language string, id string, indexs []string, data string) error {
	id_ := obj[id].(string)
	err := index.Index(id_, obj)
	if err != nil {
		return err
	}

	// Associated original object here...
	if len(data) > 0 {
		err = index.SetInternal([]byte(id_), []byte(data))
	} else {
		var data_ []byte
		data_, err = json.Marshal(obj)
		if err == nil {
			err = index.SetInternal([]byte(id_), data_)
		}
	}

	return err
}

// Index a given object.
func (engine *BleveSearchEngine) IndexJsonObject(path string, jsonStr string, language string, id string, indexs []string, data string) error {

	index, err := engine.getIndex(path)
	if err != nil {
		return err
	}

	var obj interface{}
	err = json.Unmarshal([]byte(jsonStr), &obj)
	if err != nil {
		return err
	}

	// Now I will append the object into the database.
	switch v := obj.(type) {
	case map[string]interface{}:
		err = engine.indexJsonObject(index, v, language, id, indexs, data)

	case []interface{}:
		for i := 0; i < len(v); i++ {
			err := engine.indexJsonObject(index, v[i].(map[string]interface{}), language, id, indexs, data)
			if err != nil {
				break
			}
		}
	}
	return nil
}

// Count the number of document in a db.
func (engine *BleveSearchEngine) Count(path string) int32 {
	index, err := engine.getIndex(path)
	if err != nil {
		return -1
	}

	total, err := index.DocCount()

	// convert to int 32
	return int32(total)
}
