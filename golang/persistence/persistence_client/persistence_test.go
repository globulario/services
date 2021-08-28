package persistence_client

import (
	//"fmt"
	//"io/ioutil"
	"log"
	"testing"

	"github.com/globulario/services/golang/authentication/authentication_client"
	//"github.com/davecourtois/Utility"
)

// start: mongod --dbpath E:\Project\src\github.com\davecourtois\Globular\data\mongodb-data
// Set the correct addresse here as needed.
var (

	// Connect to the plc client.
	domain                    = "localhost"
	client, _                 = NewPersistenceService_Client(domain, "persistence.PersistenceService")
	authentication_client_, _ = authentication_client.NewAuthenticationService_Client(domain, "authentication.AuthenticationService")
	token, _                  = authentication_client_.Authenticate("sa", "adminadmin")
)
/*
func TestCreateSaConnection(t *testing.T) {

	//log.Println(token)
	log.Println("Connection creation test.")
	user := "sa"
	pwd := "adminadmin"
	err := client.CreateConnection("local_resource", "local_resource", domain, 27017, 0, user, pwd, 500, "", true)
	if err != nil {
		log.Println("fail to create connection! ", err)
	}
}
*/
// First test create a fresh new connection...

func TestCreateConnection(t *testing.T) {

	//log.Println(token)
	log.Println("Connection creation test.")
	user := "sa"
	pwd := "adminadmin"
	err := client.CreateConnection("test_connection", "test_connection", domain, 27017, 0, user, pwd, 500, "", true)
	if err != nil {
		log.Println("fail to create connection! ", err)
	}
}

/* In case of mongoDB the Collection and Database is create at first insert.*/
func TestCreateDatabase(t *testing.T) {
	Id := "test_connection"
	Database := "TestDB"
	err := client.CreateDatabase(Id, Database)
	if err != nil {
		log.Println("fail to create database ", Database, err)
	}
}

func TestConnect(t *testing.T) {

	err := client.Connect("test_connection", "adminadmin")
	if err != nil {
		log.Println("fail to connect to the backend with error ", err)
	}

}

func TestPingConnection(t *testing.T) {

	err := client.Ping("test_connection")
	if err != nil {
		log.Fatalln("fail to ping the backend with error ", err)
	}

	log.Println("Ping test_connection successed!")
}

func TestPersistOne(t *testing.T) {

	Id := "test_connection"
	Database := "TestDB"
	Collection := "Employees"
	employe := map[string]interface{}{
		"hire_date":  "2007-07-01",
		"last_name":  "Courtois",
		"first_name": "Dave",
		"birth_date": "1976-01-28",
		"emp_no":     200000,
		"gender":     "M"}

	id, err := client.InsertOne(Id, Database, Collection, employe, "")

	if err != nil {
		log.Fatalf("fail to pesist entity with error %v", err)
	}

	log.Println("Entity persist with id ", id)
}

func TestPersistMany(t *testing.T) {

	entities :=
		[]interface{}{
			map[string]interface{}{
				"_id":               "rirani",
				"jobTitleName":      "Developer",
				"firstName":         "Romin",
				"lastName":          "Irani",
				"preferredFullName": "Romin Irani",
				"employeeCode":      "E1",
				"region":            "CA",
				"phoneNumber":       "408-1234567",
				"emailAddress":      "romin.k.irani@gmail.com",
			},
			map[string]interface{}{
				"_id":               "nirani",
				"jobTitleName":      "Developer",
				"firstName":         "Neil",
				"lastName":          "Irani",
				"preferredFullName": "Neil Irani",
				"employeeCode":      "E2",
				"region":            "CA",
				"phoneNumber":       "408-1111111",
				"emailAddress":      "neilrirani@gmail.com",
			},
			map[string]interface{}{
				"_id":               "thanks",
				"jobTitleName":      "Program Directory",
				"firstName":         "Tom",
				"lastName":          "Hanks",
				"preferredFullName": "Tom Hanks",
				"employeeCode":      "E3",
				"region":            "CA",
				"phoneNumber":       "408-2222222",
				"emailAddress":      "tomhanks@gmail.com",
			},
		}

	Id := "test_connection"
	Database := "TestDB"
	Collection := "Employees"

	err := client.InsertMany(Id, Database, Collection, entities, "")
	if err != nil {
		log.Fatalf("Fail to insert many entities whit error %v", err)
	}
}

/** Test Replace One **/

func TestReplaceOne(t *testing.T) {

	Id := "test_connection"
	Database := "TestDB"
	Collection := "Employees"

	entity := map[string]interface{}{
		"_id":               "nirani",
		"jobTitleName":      "Full Stack Developper",
		"firstName":         "Neil",
		"lastName":          "Irani",
		"preferredFullName": "Neil Irani",
		"employeeCode":      "E2",
		"region":            "CA",
		"phoneNumber":       "408-1111111",
		"emailAddress":      "neilrirani@gmail.com"}

	err := client.ReplaceOne(Id, Database, Collection, `{"_id":"nirani"}`, entity, "")
	if err != nil {
		log.Fatalf("Fail to replace entity %v", err)
	}
}

func TestUpdateOne(t *testing.T) {
	Id := "test_connection"
	Database := "TestDB"
	Collection := "Employees"

	err := client.UpdateOne(Id, Database, Collection, `{"_id":"nirani"}`, `{ "$set":{"employeeCode":"E2.2"},"$set":{"phoneNumber":"408-1231234"}}`, "")
	if err != nil {
		log.Fatalf("Fail to update entity %v", err)
	}
}

func TestUpdate(t *testing.T) {
	Id := "test_connection"
	Database := "TestDB"
	Collection := "Employees"
	Query := `{"region": "CA"}`
	Value := `{"$set":{"state":"California"}}`

	err := client.Update(Id, Database, Collection, Query, Value, "")
	if err != nil {
		log.Fatalf("TestUpdate fail %v", err)
	}
	log.Println("---> update success!")
}

/** Test find one **/
func TestFindOne(t *testing.T) {
	log.Println("Find one test.")

	Id := "test_connection"
	Database := "TestDB"
	Collection := "Employees"
	Query := `{"first_name": "Dave"}`

	values, err := client.FindOne(Id, Database, Collection, Query, "")
	if err != nil {
		log.Fatalf("TestFind fail %v", err)
	}

	log.Println(values)
}

/** Test find many **/
func TestFind(t *testing.T) {
	log.Println("Find many test.")

	Id := "test_connection"
	Database := "TestDB"
	Collection := "Employees"
	Query := `{"region": "CA"}`

	values, err := client.Find(Id, Database, Collection, Query, `[{"Projection":{"firstName":1}}]`)
	if err != nil {
		log.Fatalf("fail to find entities with error %v", err)
	}

	log.Println(values)

}

func TestAggregate(t *testing.T) {
	//fmt.Println("Aggregate")
	Id := "test_connection"
	Database := "TestDB"
	Collection := "Employees"

	results, err := client.Aggregate(Id, Database, Collection, `[{"$count":"region"}]`, "")
	if err != nil {
		log.Fatalf("fail to create aggregation with error %v", err)
	}
	log.Println("---> ", results)

}

/** Test remove **/

func TestRemove(t *testing.T) {
	log.Println("Test Remove")

	Id := "test_connection"
	Database := "TestDB"
	Collection := "Employees"
	Query := `{"_id":"nirani"}`

	err := client.DeleteOne(Id, Database, Collection, Query, "")
	if err != nil {
		log.Fatalf("Fail to delete one entity with error %v", err)
	}

	log.Println("---> Delete success!")
}

func TestRemoveMany(t *testing.T) {
	log.Println("Test Remove")

	Id := "test_connection"
	Database := "TestDB"
	Collection := "Employees"
	Query := `{"region": "CA"}`

	err := client.Delete(Id, Database, Collection, Query, "")
	if err != nil {
		log.Fatalf("Fail to remove entities %v", err)
	}
	log.Println("---> Delete success!")
}

func TestDeleteCollection(t *testing.T) {
	log.Println("Delete collection test.")
	Id := "test_connection"
	Database := "TestDB"
	Collection := "Employees"
	err := client.DeleteCollection(Id, Database, Collection)
	if err != nil {
		log.Println("fail to delete collection! ", err)
	}
}

func TestDeleteDatabase(t *testing.T) {
	log.Println("Delete database test.")
	Id := "test_connection"
	Database := "TestDB"
	err := client.DeleteDatabase(Id, Database)
	if err != nil {
		log.Println("fail to delete database! ", err)
	}
}

func TestDisconnect(t *testing.T) {
	log.Println("Disconnect test.")
	err := client.Disconnect("test_connection")
	if err != nil {
		log.Println("fail to delete connection! ", err)
	}
}

func TestDeleteConnection(t *testing.T) {
	log.Println("Delete connection test.")
	err := client.DeleteConnection("test_connection")
	if err != nil {
		log.Println("fail to delete connection! ", err)
	}
}
