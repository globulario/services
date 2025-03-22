package search_engine

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/blevesearch/bleve/v2"
	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/search/searchpb"
)

/**
 * A key value data store.
 */
type BleveSearchEngine struct {
	indexs map[string]bleve.Index
}

// Create a new search engine.
func NewBleveSearchEngine() *BleveSearchEngine {
	// init the indexs map.
	return &BleveSearchEngine{indexs: make(map[string]bleve.Index, 0)}
}

/**
 * Return indexation for a given path...
 */
func (engine *BleveSearchEngine) getIndex(path string) (bleve.Index, error) {

	if len(path) == 0 {
		return nil, errors.New("path is empty")
	}

	if engine.indexs[path] != nil {
		if !Utility.Exists(path) {
			if engine.indexs[path] != nil {
				engine.indexs[path].Close()
				delete(engine.indexs, path)
			}
			return nil, errors.New("path: '" + path + "' does not exist")
		}
	}

	if engine.indexs[path] == nil {

		index, err := bleve.Open(path) // try to open existing index.
		if err != nil {
			fmt.Println("failed to open index with error: ", err)
			mapping := bleve.NewIndexMapping()
			var err error
			index, err = bleve.New(path, mapping)
			if err != nil {
				return nil, err
			}
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
		index, _ := engine.getIndex(paths[i])
		if index != nil {

			query := bleve.NewQueryStringQuery(q)
			searchRequest := bleve.NewSearchRequest(query)
			searchRequest.Fields = fields
			searchRequest.From = int(offset)
			searchRequest.Size = int(pageSize)
			searchRequest.Highlight = bleve.NewHighlightWithStyle("html")
			searchResult, _ := index.Search(searchRequest)

			// Now from the result I will
			if searchResult != nil {
				fmt.Println("found ", len(searchResult.Hits), " results")
				// Now I will return the data
				for _, val := range searchResult.Hits {
					id := val.ID
					raw, err := index.GetInternal([]byte(id))
					if err == nil {
						result := new(searchpb.SearchResult)
						result.Data = string(raw)

						result.DocId = id
						result.Rank = int32(val.Score * 100)

						// serialyse the fragment and set it as snippet.
						data, err := Utility.ToJson(val.Fragments)
						if err == nil {
							result.Snippet = string(data)
						}
						results.Results = append(results.Results, result)
					} else {
						fmt.Println("fait to retreiv raw data with error: ", err)
					}
				}
			}
		}
	}

	if len(results.Results) == 0 {
		return nil, errors.New("no results found")
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
	if len(id) == 0 {
		return errors.New("no id field found")
	}

	id_ := obj[id].(string)
	if len(id_) == 0 {
		return errors.New("id is empty field: " + id)
	}

	err := index.Index(id_, obj)
	if err != nil {
		return err
	}

	// Associated original object here...
	if len(data) > 0 {
		err = index.SetInternal([]byte(id_), []byte(data))
	} else {
		data_, err := Utility.ToJson(obj)
		if err == nil {
			err = index.SetInternal([]byte(id_), []byte(data_))
			if err != nil {
				return err
			}
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

	return err
}

// Count the number of document in a db.
func (engine *BleveSearchEngine) Count(path string) int32 {
	index, err := engine.getIndex(path)
	if err != nil {
		return -1
	}

	total, err := index.DocCount()
	if err != nil {
		return -1
	}

	// convert to int 32
	return int32(total)
}
