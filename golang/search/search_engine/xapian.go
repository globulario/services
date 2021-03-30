package search_engine

import (
	"strings"

	"encoding/json"
	//"fmt"
	"reflect"

	"log"
	"os"

	"errors"
	"io/ioutil"

	"os/exec"

	"github.com/davecourtois/GoXapian"
	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/search/searchpb"
)

type XapianEngine struct {
}

// Return the underlying engine version.
func (self *XapianEngine) GetVersion() string {
	v := xapian.Version_string()
	return v
}

// That function is use to generate a snippet from a text file.
func (self *XapianEngine) snippets(mset xapian.MSet, path string, mime string, length int64) (string, error) {

	// Here I will read the file and try to generate a snippet for it.
	var text string
	var err error

	// TODO append other file parser here as needed.
	if mime == "application/pdf" {
		text, err = self.pdfToText(path)
		if err != nil {
			return "", err
		}
	} else if strings.HasPrefix(mime, "text/") {
		_text, err := ioutil.ReadFile(path)
		text = string(_text)
		if err != nil {
			return "", err
		}
	}

	return mset.Snippet(text, length), nil
}

// Here I will append the sub-database.
func (self *XapianEngine) addSubDBs(db xapian.Database, path string) []xapian.Database {
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
			subDbs = append(subDbs, self.addSubDBs(db, path+"/"+files[i].Name())...)
		}
	}

	return subDbs
}

////////////////////////////////////////////////////////////////////////////////
// Json documents indexations.
////////////////////////////////////////////////////////////////////////////////

// Search documents...
func (self *XapianEngine) searchDocuments(paths []string, language string, fields []string, queryStr string, offset int32, pageSize int32, snippetLength int32) ([]*searchpb.SearchResult, error) {
	log.Println("Search document ", paths, language, fields, queryStr)
	if len(paths) == 0 {
		return nil, errors.New("No database was path given!")
	}
	if !Utility.Exists(paths[0]) {
		return nil, errors.New("No database found at path " + paths[0])
	}

	db := xapian.NewDatabase(paths[0])
	defer xapian.DeleteDatabase(db)

	// Open the db for read...
	for i := 1; i < len(paths); i++ {
		path := paths[i]
		_db := xapian.NewDatabase(path)
		defer xapian.DeleteDatabase(_db)

		// Now I will recursively append data base is there is some subdirectory...
		subDbs := self.addSubDBs(_db, path)
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

			infos := make(map[string]interface{}, 0)
			err := json.Unmarshal([]byte(doc.Get_data()), &infos)
			if err != nil {
				return nil, err
			}
			type_ := "object"
			if infos["__type__"] != nil {
				type_ = infos["__type__"].(string)
			}
			if type_ == "file" {
				snippet, err := self.snippets(mset, infos["path"].(string), infos["mime"].(string), int64(snippetLength))
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

func (self *XapianEngine) SearchDocuments(paths []string, language string, fields []string, query string, offset, pageSize, snippetLength int32) (*searchpb.SearchResults, error) {
	results := new(searchpb.SearchResults)
	var err error
	// Set as Hash key
	results.Results, err = self.searchDocuments(paths, language, fields, query, offset, pageSize, snippetLength)
	if err != nil {
		return nil, err
	}

	return results, nil

}

// Delete a document.
func (self *XapianEngine) DeleteDocument(path string, id string) error {
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
func (self *XapianEngine) indexJsonObjectField(db xapian.WritableDatabase, termgenerator xapian.TermGenerator, k string, v interface{}, indexs []string) error {
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
func (self *XapianEngine) indexJsonObject(db xapian.WritableDatabase, obj map[string]interface{}, language string, id string, indexs []string, data string) error {
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
				self.indexJsonObject(db, v.(map[string]interface{}), language, id, indexs, data)
			} else if typeOf == reflect.Slice {
				s := reflect.ValueOf(v)
				for i := 0; i < s.Len(); i++ {
					_v := s.Index(i)

					typeOf := reflect.TypeOf(_v).Kind()
					if typeOf == reflect.Map {
						// Slice of object.
						self.indexJsonObject(db, _v.Interface().(map[string]interface{}), language, id, indexs, data)
					} else {
						// Slice of literal type.

						self.indexJsonObjectField(db, termgenerator, k, _v.Interface(), indexs)
					}
				}
			} else {
				self.indexJsonObjectField(db, termgenerator, k, v, indexs)
			}
		}

	}

	// Here I will set object metadata.
	var infos map[string]interface{}
	if len(data) > 0 {
		infos = make(map[string]interface{}, 0)
		json.Unmarshal([]byte(data), &infos)
	} else {
		infos = make(map[string]interface{}, 0)
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
func (self *XapianEngine) IndexJsonObject(path string, jsonStr string, language string, id string, indexs []string, data string) error {
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
		err = self.indexJsonObject(db, v, language, id, indexs, data)

	case []interface{}:
		for i := 0; i < len(v); i++ {
			err := self.indexJsonObject(db, v[i].(map[string]interface{}), language, id, indexs, data)
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

func (self *XapianEngine) Count(path string) int32 {
	db := xapian.NewDatabase(path)
	//defer db.Close()
	defer xapian.DeleteDatabase(db)
	count := int32(db.Get_doccount())
	return count
}

////////////////////////////////////////////////////////////////////////////////
// Files and directories indexations
////////////////////////////////////////////////////////////////////////////////

/**
 * Index the a dir and it content.
 */
func (self *XapianEngine) indexDir(dbPath string, dirPath string, language string) error {

	dirInfo, err := os.Stat(dirPath)
	if err != nil {
		return err
	}

	if !dirInfo.IsDir() {
		return errors.New("The file " + dirPath + " is not a directory ")
	}

	// So here I will create the directory entry in the dbPath
	err = Utility.CreateDirIfNotExist(dbPath)

	if err != nil {
		return err
	}

	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		log.Fatal(err)
	}

	// The database path.
	db := xapian.NewWritableDatabase(dbPath, xapian.DB_CREATE_OR_OPEN)
	defer xapian.DeleteWritableDatabase(db)
	db.Begin_transaction()

	if err != nil {
		db.Cancel_transaction()
		db.Close()
		return err
	}

	// Now I will index files and recursively index dir content.
	for _, file := range files {
		if file.IsDir() {
			err := self.indexDir(dbPath+"/"+file.Name(), dirPath+"/"+file.Name(), language)
			if err != nil {
				return err
			}
		} else {
			// Here I will index the file contain in the directory.
			path := dirPath + "/" + file.Name()
			err := self.indexFile(db, path, language)
			if err != nil {
				log.Println(file.Name(), err)
			}
		}
	}

	mime := "folder"

	// set document meta data.
	modified := "D" + dirInfo.ModTime().Format("YYYYMMDD")
	doc := xapian.NewDocument()
	defer xapian.DeleteDocument(doc)
	doc.Add_term(modified)
	doc.Add_term("P" + dirPath)
	doc.Add_term("T" + mime)

	id := "Q" + Utility.GenerateUUID(dirPath)
	doc.Add_boolean_term(id)

	infos := make(map[string]interface{}, 0)

	infos["path"] = dirPath
	infos["__type__"] = "file"

	infos["mime"] = mime

	jsonStr, _ := Utility.ToJson(infos)
	doc.Set_data(jsonStr)

	// Create the directory information.
	db.Replace_document(id, doc)

	db.Commit_transaction()
	db.Close()

	return err
}

func (self *XapianEngine) IndexDir(dbPath string, dirPath string, language string) error {

	err := self.indexDir(dbPath, dirPath, language)
	if err != nil {
		return err
	}

	return nil
}

// pdftotext bin must be install on the server to be able to generate text
// file from pdf file.
// On linux type...
// sudo apt-get install poppler-utils
func (self *XapianEngine) pdfToText(path string) (string, error) {
	// First of all I will test if pdftotext is install.
	cmd := exec.Command("pdftotext", path)
	_, err := cmd.Output()
	if err != nil {
		return "", err
	}

	_path := path[0:strings.LastIndex(path, ".")] + ".txt"
	defer os.Remove(_path)

	// Here I will index the text file
	text, err := ioutil.ReadFile(_path)
	if err != nil {
		return "", err
	}

	return string(text), err

}

// Indexation of pdf file.
func (self *XapianEngine) indexPdfFile(db xapian.WritableDatabase, path string, doc xapian.Document, termgenerator xapian.TermGenerator) error {
	text, err := self.pdfToText(path)
	if err != nil {
		return err
	}
	termgenerator.Index_text(strings.ToLower(string(text)))
	termgenerator.Increase_termpos()
	return nil
}

func (self *XapianEngine) indexFile(db xapian.WritableDatabase, path string, language string) error {
	path = strings.ReplaceAll(strings.ReplaceAll(path, "\\", string(os.PathSeparator)), "/", string(os.PathSeparator))

	f, err := os.Open(path)
	if err != nil {
		return err
	}

	mime, err := Utility.GetFileContentType(f)
	if err != nil {
		return err
	}

	// create the document.
	doc := xapian.NewDocument()
	defer xapian.DeleteDocument(doc)

	// create the term generator for the file.
	termgenerator := xapian.NewTermGenerator()
	stemmer := xapian.NewStem(language)
	defer xapian.DeleteStem(stemmer)
	termgenerator.Set_stemmer(xapian.NewStem(language))
	defer xapian.DeleteTermGenerator(termgenerator)
	termgenerator.Set_document(doc)

	// Now I will index file metat
	fileStat, err := os.Stat(path)
	if err != nil {
		return err
	}

	// set document meta data.
	modified := "D" + fileStat.ModTime().Format("YYYYMMDD")
	doc.Add_term(modified)
	doc.Add_term("P" + path)
	doc.Add_term("T" + mime)

	if mime == "application/pdf" {
		err = self.indexPdfFile(db, path, doc, termgenerator)
		if err != nil {
			return err
		}
	} else if strings.HasPrefix(mime, "text") {
		text, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		termgenerator.Index_text(strings.ToLower(string(text)))
		termgenerator.Increase_termpos()

	} else {
		return errors.New("Unsuported file type! " + mime)
	}

	id := "Q" + Utility.GenerateUUID(path)
	doc.Add_boolean_term(id)

	infos := make(map[string]interface{}, 0)
	infos["path"] = path
	infos["__type__"] = "file"
	infos["mime"] = mime

	jsonStr, _ := Utility.ToJson(infos)
	doc.Set_data(jsonStr)

	db.Replace_document(id, doc)

	return nil
}

// Indexation of a text (docx, pdf,xlsx...) file.
func (self *XapianEngine) IndexFile(filePath string, dbPath string, language string) error {

	// The file must be accessible on the server side.
	if !Utility.Exists(filePath) {
		return errors.New("File " + filePath + " was not found!")
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return err
	}

	if fileInfo.IsDir() {
		return errors.New("The file " + filePath + " is a directory ")
	}

	// The database path.
	db := xapian.NewWritableDatabase(dbPath, xapian.DB_CREATE_OR_OPEN)
	defer xapian.DeleteWritableDatabase(db)
	db.Begin_transaction()

	err = self.indexFile(db, filePath, language)
	if err != nil {
		db.Cancel_transaction()
		db.Close()
		return err
	}

	db.Commit_transaction()
	db.Close()

	return nil
}
