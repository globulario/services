package storage_client

import (
	"fmt"
	"log"
	"testing"
)

// Set the correct addresse here as needed.
var (
	client, _ = NewStorageService_Client("localhost", "storage.StorageService")
)

// First test create a fresh new connection...
func TestCreateConnection(t *testing.T) {
	fmt.Println("Connection creation test.")
	err := client.CreateConnection("test_storage", "storage_test", 3.0)
	if err != nil {
		log.Fatalf("error while CreateConnection: %v", err)
	}

	log.Println("Create connection success!")
}

func TestOpenConnection(t *testing.T) {
	//err := client.OpenConnection("test_storage", `{"path":"/tmp/storage/test", "name":"storage_test"}`)
	fmt.Println("Open connection test.")
	err := client.OpenConnection("test_storage", ``)
	if err != nil {
		log.Fatalf("fail to open connection: %v", err)
	}
	log.Println("Open connection success!")
}

// Test set item.
func TestSetItem(t *testing.T) {

	fmt.Println("Set item test.")

	err := client.SetItem("test_storage", "1", []byte(`{"prop_1":"This is a test!", "prop_2":1212}`))
	if err != nil {
		log.Fatalf("error set item test with error: %v", err)
	}
	log.Println("Set item success!")
}

func TestGetItem(t *testing.T) {
	fmt.Println("Get item test.")
	values, err := client.GetItem("test_storage", "1")
	if err != nil {
		log.Fatalf("fail to get item with error: %v", err)
	}
	log.Println("Get item success with value", string(values))
}

func TestRemoveItem(t *testing.T) {
	fmt.Println("Remove item test.")
	err := client.RemoveItem("test_storage", "1")
	if err != nil {
		log.Fatalf("fail to remove item with error: %v", err)
	}
	log.Println("Remove item success!")
}

/*func TestClear(t *testing.T) {
	err := client.Clear("test_storage")
	if err != nil {
		log.Fatalf("error while deleting the connection: %v", err)
	}
	log.Println("Clear all items success!")
}

func TestDrop(t *testing.T) {

	err := client.Drop("test_storage")
	if err != nil {
		log.Fatalf("error while deleting the connection: %v", err)
	}
	log.Println("Drop store success!")
}*/

func TestCloseConnection(t *testing.T) {

	err := client.CloseConnection("test_storage")
	if err != nil {
		log.Fatalf("error while deleting the connection: %v", err)
	}
	log.Println("close connection success!")
}

// Test a ldap query.
func TestDeleteConnection(t *testing.T) {
	err := client.DeleteConnection("test_storage")
	if err != nil {
		log.Fatalf("error while deleting the connection: %v", err)
	}
	log.Println("Delete connection success!")
}
