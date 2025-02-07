package search_client

import (
	//"encoding/json"
	"fmt"
	"log"
	"testing"
)

var (
	client    *Search_Client
	tmpDir    = "/tmp"
	ebookPath = "E:/ebooks"
)

func getClient() *Search_Client {
	if client != nil {
		return client
	}
	client, _ = NewSearchService_Client("globule-ryzen.globular.cloud:443", "search.SearchService")
	return client
}

func TestIndexDocument(t *testing.T){
	fmt.Println("test index document")

}

func TestIndexJsonObject(t *testing.T) {
	fmt.Println("test index json object")

	var str = `
	[
	    {
		  "id": "1",
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
	      "id": "2",
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
	fields := []string{"name"}
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

func TestSearchPdf(t *testing.T) {
	fmt.Println("-----------> TestSearchDocument")
	paths := []string{`/var/globular/data/files/users/sa@globular.io/.hidden/img1/__index_db__`, `/var/globular/data/files/users/sa@globular.io/.hidden/95062B1 Mandat/__index_db__`}
	query := `Text:Certificat`
	language := "english"
	fields := []string{"DocId", "Path"}
	offset := int32(0)
	pageSize := int32(10)
	snippetLength := int32(500)

	results, err := getClient().SearchDocuments(paths, query, language, fields, offset, pageSize, snippetLength)
	if err != nil {
		log.Println("------------> ", err)
		return
	}

	for i := 0; i < len(results); i++ {
		log.Println("------------>", results[i])
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