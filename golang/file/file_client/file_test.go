package file_client

import (
	//"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"testing"
)

// Set the correct addresse here as needed.
var (
	client, _ = NewFileService_Client("localhost", "file.FileService")
)

// First test create a fresh new connection...
func _TestReadDir(t *testing.T) {
	fmt.Println("Read dir test")

	_, err := client.ReadDir("c:/temp", true, 80, 80)
	if err != nil {
		log.Println(err)
		t.Fail()
		return
	}

	log.Println("TestReadDir successed!")
}

func TestGetThumbnails(t *testing.T) {
	fmt.Println("Get Thumbnails")

	_, err := client.GetThumbnails("C:/temp", true, 80, 80)
	if err != nil {
		log.Println(err)
		t.Fail()
		return
	}

	log.Println("TestReadDir successed!")
}

/**
 * Create a new directory.
 */
func TestCreateDir(t *testing.T) {
	fmt.Println("Create dir test")

	err := client.CreateDir("C:/temp", "testDir")
	if err != nil {
		log.Println(err)
		t.Fail()
		return
	}

	log.Println("success TestCreateDir")
}

/**
 * Rename a directory
 */
func TestRenameDir(t *testing.T) {
	fmt.Println("Rename dir test")

	// Create a new client service...
	err := client.RenameDir("C:/temp", "TestTestTestDir", "")
	if err != nil {
		log.Println(err)
		t.Fail()
		return
	}

	log.Println("succed TestRenameDir")
}

/**
 * Rename a directory
 */
func TestDeleteDir(t *testing.T) {
	fmt.Println("Delete dir test")

	err := client.DeleteDir("C:\\Temp\\TestTestTestDir")
	if err != nil {
		log.Println(err)
		t.Fail()
		return
	}

	log.Println("succed TestDeleteDir")

}

////////////////////////////////////////////////////////////////////////////////
// File test
////////////////////////////////////////////////////////////////////////////////
func TestGetFileInfo(t *testing.T) {
	fmt.Println("Get File info test")

	_, err := client.GetFileInfo("C:/Users/mm006819/Pictures/bob.jpg", false, 56, 56)
	if err != nil {
		log.Println(err)
		t.Fail()
		return
	}

	log.Println("succed TestGetFileInfo")
}

// Read file test.
func TestReadFile(t *testing.T) {
	fmt.Println("Read file test")

	_, err := client.ReadFile("C:/Users/mm006819/Pictures/bob.jpg")
	if err != nil {
		log.Println(err)
		t.Fail()
		return
	}

	log.Println("succed TestReadFile")

}

// Test delete file on the server and HtmlToPdf
func TestHtmToPdfFile(t *testing.T) {
	htmlStr := `<html><body><img src="file:///C:/Users/mm006819/Pictures/images.jpg"></img><h1 style="color:red;">This is an html from pdf to test color</h1></body></html>`

	data, err := client.HtmlToPdf(htmlStr)
	if err != nil {
		log.Println(err)
		t.Fail()
		return
	}

	ioutil.WriteFile("C:/temp/pdfTest.pdf", data, 0644)

	log.Println("succed TestHtmToPdfFile")
}
