package search_client

import (
	//"encoding/json"
	"log"
	"testing"
)

var (
	client    *Search_Client
	tmpDir    = "/tmp"
	ebookPath = "/home/dave/Documents/ebook"
)

func getClient() *Search_Client {
	if client != nil {
		return client
	}
	client, _ = NewSearchService_Client("globule-ryzen.globular.cloud:10002", "search.SearchService")
	return client
}
/*
func TestIndexJsonObject(t *testing.T) {
	var str = `
	[
	    {
		  "id": 1,
	      "name": "Tom Cruise",
	      "age": 56,
	      "BornAt": "Syracuse, NY",
	      "Birthdate": "July 3, 1962",
	      "photo": "https://jsonformatter.org/img/tom-cruise.jpg",
	      "wife": null,
	      "weight": 67.5,
	      "hasChildren": true,
	      "hasGreyHair": false,
	      "children": [
	        "Suri",
	        "Isabella Jane",
	        "Connor"
	      ]
	    },
	    {
	      "id": 2,
	      "name": "Robert Downey Jr.",
	      "age": 53,
	      "BornAt": "New York City, NY",
	      "Birthdate": "April 4, 1965",
	      "photo": "https://jsonformatter.org/img/Robert-Downey-Jr.jpg",
	      "wife": "Susan Downey",
	      "weight": 77.1,
	      "hasChildren": true,
	      "hasGreyHair": false,
	      "children": [
	        "Indio Falconer",
	        "Avri Roel",
	        "Exton Elias"
	      ]
	    }
	]
	`

	err := getClient().IndexJsonObject(tmpDir+"/search_test_db", str, "english", "id", []string{"name", "BornAt"}, "")
	if err != nil {
		log.Println(err)
	}

	// Count the number of document in the db
	count, _ := getClient().Count(tmpDir + "/search_test_db")

	log.Println(count)
}

// Test various function here.
func TestVersion(t *testing.T) {

	// Connect to the plc client.
	val, err := getClient().GetVersion()
	if err != nil {
		log.Println(err)
	} else {
		log.Println("found version ", val)
	}

}

func TestSearchDocument(t *testing.T) {
	paths := []string{tmpDir + "/search_test_db"}
	query := `name:"Tom Cruise"`
	language := "english"
	fields := []string{"Name"}
	offset := int32(0)
	pageSize := int32(10)
	snippetLength := int32(500)

	results, err := getClient().SearchDocuments(paths, query, language, fields, offset, pageSize, snippetLength)
	if err != nil {
		log.Println(err)
		return
	}

	for i := 0; i < len(results); i++ {
		log.Println(results[i])
	}
}

func TestDeleteDocument(t *testing.T) {
	err := getClient().DeleteDocument(tmpDir+"/search_test_db", "2")
	if err != nil {
		log.Println(err)
	}

	// Count the number of document in the db
	count, _ := getClient().Count(tmpDir + "/search_test_db")
	log.Println(count)
}

/*
func TestIndexDir(t *testing.T) {
	log.Println("test index dir...")
	err := getClient().IndexDir("C:/temp/ebooks", "E:/ebooks", "english")
	if err != nil {
		log.Print(err)
	}
}

func TestSearchTextFiles(t *testing.T) {
	paths := []string{"C:/temp/ebooks"}
	query := `traversal`
	language := "english"
	fields := []string{}
	offset := int32(0)
	pageSize := int32(100)
	snippetLength := int32(500)
	log.Println("test search words...")
	results, err := getClient().SearchDocuments(paths, query, language, fields, offset, pageSize, snippetLength)
	if err != nil {
		log.Println("---> ", err)
		log.Println(err)
	}

	for i := 0; i < len(results); i++ {
		result := results[i]
		log.Println(result)
	}
}
*/

func TestIndexPdfFile(t *testing.T) {
	path := "E:/ebooks/javascript/Advanced_JavaScript.pdf"
	err := getClient().IndexFile("C:/temp/search_test_db", path, "english")
	if err != nil {
		log.Print(err)
	}
}


//  Search text in a given file. I made use the snippet's to display search results.
func TestSearchTextFile(t *testing.T) {
	paths := []string{"C:/temp/search_test_db"}
	query := `test`
	language := "english"
	fields := []string{}
	offset := int32(0)
	pageSize := int32(10)
	snippetLength := int32(70)

	results, err := getClient().SearchDocuments(paths, query, language, fields, offset, pageSize, snippetLength)
	if err != nil {
		log.Println(err)
	}

	for i := 0; i < len(results); i++ {
		//result := results[i]
		/*
		for j := 0; j < len(result.); j++ {
			log.Println("---------> ", j+1, result.Snippets[j])
		}*/
	}
}

