package search_engine

import (
	"strings"

	"encoding/json"
	//"fmt"
	"reflect"

	"log"

	"errors"
	"io/ioutil"

	xapian "github.com/davecourtois/GoXapian"
	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/search/searchpb"
)

type XapianEngine struct {
}

// Return the underlying engine version.
func (search_engine *XapianEngine) GetVersion() string {
	v := xapian.Version_string()
	return v
}

// That function is use to generate a snippet from a text file.
func (search_engine *XapianEngine) snippets(mset xapian.MSet, path string, mime string, length int64) (string, error) {

	// Here I will read the file and try to generate a snippet for it.
	var text string
	// TODO append other file parser here as needed.
	if strings.HasPrefix(mime, "text/") {
		_text, err := ioutil.ReadFile(path)
		text = string(_text)
		if err != nil {
			return "", err
		}
	}

	return mset.Snippet(text, length), nil
}

// Here I will append the sub-database.
func (search_engine *XapianEngine) addSubDBs(db xapian.Database, path string) []xapian.Database {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}

	// keep track of sub database.
	subDbs := make([]xapian.Database, 0)

	// Add the database.
	for i := 0; i < len(files); i++ {
		if files[i].IsDir() {
			_db := xapian.NewDatabase(path + "/" + files[i].Name())
			db.Add_database(_db)
			subDbs = append(subDbs, _db)
			subDbs = append(subDbs, search_engine.addSubDBs(db, path+"/"+files[i].Name())...)
		}
	}

	return subDbs
}

////////////////////////////////////////////////////////////////////////////////
// Json documents indexations.
////////////////////////////////////////////////////////////////////////////////

// Search documents...
func (search_engine *XapianEngine) searchDocuments(paths []string, language string, fields []string, queryStr string, offset int32, pageSize int32, snippetLength int32) ([]*searchpb.SearchResult, error) {

	if len(paths) == 0 {
		return nil, errors.New("no database was path given")
	}
	if !Utility.Exists(paths[0]) {
		return nil, errors.New("cannot open index, path does not exist " + paths[0])
	}

	db := xapian.NewDatabase(paths[0])
	defer xapian.DeleteDatabase(db)

	// Open the db for read...
	for i := 1; i < len(paths); i++ {
		path := paths[i]
		_db := xapian.NewDatabase(path)
		defer xapian.DeleteDatabase(_db)

		// Now I will recursively append data base is there is some subdirectory...
		subDbs := search_engine.addSubDBs(_db, path)
		db.Add_database(_db)

		// clear pointer memory...
		for j := 0; j < len(subDbs); j++ {
			defer xapian.DeleteDatabase(subDbs[j])
		}
	}

	queryParser := xapian.NewQueryParser()
	defer xapian.DeleteQueryParser(queryParser)

	stemmer := xapian.NewStem(language)
	defer xapian.DeleteStem(stemmer)

	queryParser.Set_stemmer(stemmer)
	queryParser.Set_stemming_strategy(xapian.XapianQueryParserStem_strategy(xapian.QueryParserSTEM_SOME))

	// Append the list of field to search for
	for i := 0; i < len(fields); i++ {
		field := strings.ToUpper(fields[i][0:1]) + strings.ToLower(fields[i][1:])
		queryParser.Add_prefix(field, "X")
	}

	// Generate the query from the given string.
	query := queryParser.Parse_query(queryStr)
	defer xapian.DeleteQuery(query)

	enquire := xapian.NewEnquire(db)
	defer xapian.DeleteEnquire(enquire)

	enquire.Set_query(query)

	// Here I will retreive the results.
	mset := enquire.Get_mset(uint(offset), uint(pageSize))
	defer xapian.DeleteMSet(mset)

	results := make([]*searchpb.SearchResult, 0)

	// Here I will
	for i := uint(0); i < mset.Size(); i++ {

		docId := mset.Get_docid(i)
		result := new(searchpb.SearchResult)
		result.DocId = Utility.ToString(int(docId))
		doc := mset.Get_document(uint(i))
		defer xapian.DeleteDocument(doc) // release memory
		result.Data = doc.Get_data()     // Set the necessery data to retreive document in it db
		result.Rank = int32(mset.Get_document_percentage(uint(i)))

		it := enquire.Get_matching_terms_begin(mset.Get_hit(uint(i)))

		if !it.Equals(enquire.Get_matching_terms_end(mset.Get_hit(uint(i)))) {

			infos := make(map[string]interface{})
			err := json.Unmarshal([]byte(doc.Get_data()), &infos)
			if err != nil {
				return nil, err
			}
			type_ := "object"
			if infos["__type__"] != nil {
				type_ = infos["__type__"].(string)
			}
			if type_ == "file" {
				snippet, err := search_engine.snippets(mset, infos["path"].(string), infos["mime"].(string), int64(snippetLength))
				if err != nil {
					return nil, err
				}
				result.Snippet = snippet
			}
		}

		results = append(results, result)

	}

	return results, nil

}

func (search_engine *XapianEngine) SearchDocuments(paths []string, language string, fields []string, query string, offset, pageSize, snippetLength int32) (*searchpb.SearchResults, error) {
	results := new(searchpb.SearchResults)
	var err error
	// Set as Hash key
	results.Results, err = search_engine.searchDocuments(paths, language, fields, query, offset, pageSize, snippetLength)
	if err != nil {
		return nil, err
	}

	return results, nil
}

// Delete a document.
func (search_engine *XapianEngine) DeleteDocument(path string, id string) error {
	db := xapian.NewWritableDatabase(path, xapian.DB_CREATE_OR_OPEN)
	defer xapian.DeleteWritableDatabase(db)

	// Begin the transaction.
	db.Begin_transaction(true)

	id_ := "Q" + Utility.GenerateUUID(id) // strings.ToUpper(id[0:1]) + strings.ToLower(id[1:])

	// Delete a document from the database.
	db.Delete_document(id_)

	db.Commit_transaction()

	db.Close()

	return nil
}

/**
 * Set base type indexation.
 */
func (search_engine *XapianEngine) indexJsonObjectField(db xapian.WritableDatabase, termgenerator xapian.TermGenerator, k string, v interface{}, indexs []string) error {
	typeOf := reflect.TypeOf(v).Kind()
	field := strings.ToLower(k)
	field = strings.ToUpper(field[0:1]) + field[1:]
	if typeOf == reflect.String {
		// Index each field with a suitable prefix.
		termgenerator.Index_text(v, uint(1), "X"+field)
		// # Index fields without prefixes for general search.
		if Utility.Contains(indexs, k) {
			termgenerator.Index_text(v)
			termgenerator.Increase_termpos()
		}
	} else if typeOf == reflect.Bool {

	} else if typeOf == reflect.Int || typeOf == reflect.Int8 || typeOf == reflect.Int16 || typeOf == reflect.Int32 || typeOf == reflect.Int64 {

	} else if typeOf == reflect.Float32 || typeOf == reflect.Float64 {

	} else if typeOf == reflect.Struct {
		//v := reflect.ValueOf(v)
	}
	return nil
}

/**
 * Index a json object.
 */
func (search_engine *XapianEngine) indexJsonObject(db xapian.WritableDatabase, obj map[string]interface{}, language string, id string, indexs []string, data string) error {
	if obj[id] == nil {
		return errors.New("Objet has no field named " + id + " required to index...")
	}

	doc := xapian.NewDocument()
	defer xapian.DeleteDocument(doc)

	termgenerator := xapian.NewTermGenerator()
	stemmer := xapian.NewStem(language)
	defer xapian.DeleteStem(stemmer)
	termgenerator.Set_stemmer(xapian.NewStem(language))
	defer xapian.DeleteTermGenerator(termgenerator)
	termgenerator.Set_document(doc)

	// Here I will iterate over the object and append fields to the document.
	// Here I will index each field's
	for k, v := range obj {

		if v != nil {
			typeOf := reflect.TypeOf(v).Kind()
			field := strings.ToLower(k)
			field = strings.ToUpper(field[0:1]) + field[1:]
			if typeOf == reflect.Map {
				// In case of recursive structure.
				search_engine.indexJsonObject(db, v.(map[string]interface{}), language, id, indexs, data)
			} else if typeOf == reflect.Slice {
				s := reflect.ValueOf(v)
				for i := 0; i < s.Len(); i++ {
					_v := s.Index(i)

					typeOf := reflect.TypeOf(_v).Kind()
					if typeOf == reflect.Map {
						// Slice of object.
						search_engine.indexJsonObject(db, _v.Interface().(map[string]interface{}), language, id, indexs, data)
					} else {
						// Slice of literal type.

						search_engine.indexJsonObjectField(db, termgenerator, k, _v.Interface(), indexs)
					}
				}
			} else {
				search_engine.indexJsonObjectField(db, termgenerator, k, v, indexs)
			}
		}

	}

	// Here I will set object metadata.
	var infos map[string]interface{}
	if len(data) > 0 {
		infos = make(map[string]interface{})
		json.Unmarshal([]byte(data), &infos)
	} else {
		infos = make(map[string]interface{})
		// keep meta data inside the object...
		if len(id) > 0 {
			infos["__id__"] = id
			infos["__type__"] = "object"
		}
	}

	jsonStr, _ := Utility.ToJson(infos)
	doc.Set_data(jsonStr)

	// Here If the object contain an id I will add it as boolean term and
	// replace existing document or create it.
	if len(id) > 0 {
		_id := "Q" + Utility.GenerateUUID(Utility.ToString(obj[id]))
		doc.Add_boolean_term(_id)
		db.Replace_document(_id, doc)
	} else {
		db.Add_document(doc)
	}

	return nil
}

/**
 * Index a json object.
 */
func (search_engine *XapianEngine) IndexJsonObject(path string, jsonStr string, language string, id string, indexs []string, data string) error {
	db := xapian.NewWritableDatabase(path, xapian.DB_CREATE_OR_OPEN)
	defer xapian.DeleteWritableDatabase(db)

	// Begin the transaction.
	db.Begin_transaction(true)

	var obj interface{}
	var err error
	err = json.Unmarshal([]byte(jsonStr), &obj)
	if err != nil {
		return err
	}

	// Now I will append the object into the database.
	switch v := obj.(type) {
	case map[string]interface{}:
		err = search_engine.indexJsonObject(db, v, language, id, indexs, data)

	case []interface{}:
		for i := 0; i < len(v); i++ {
			err := search_engine.indexJsonObject(db, v[i].(map[string]interface{}), language, id, indexs, data)
			if err != nil {
				break
			}
		}
	}

	if err != nil {
		db.Cancel_transaction()
		db.Close()
		return err
	}

	// Write object int he database.
	db.Commit_transaction()
	db.Close()

	return nil
}

func (search_engine *XapianEngine) Count(path string) int32 {
	db := xapian.NewDatabase(path)
	//defer db.Close()
	defer xapian.DeleteDatabase(db)
	count := int32(db.Get_doccount())
	return count
}