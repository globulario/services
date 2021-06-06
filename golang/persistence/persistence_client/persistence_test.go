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
	domain                    = "globular.cloud"
	client, _                 = NewPersistenceService_Client(domain, "persistence.PersistenceService")
	authentication_client_, _ = authentication_client.NewAuthenticationService_Client(domain, "authentication.AuthenticationService")
	token, _                  = authentication_client_.Authenticate("sa", "adminadmin")
)

// First test create a fresh new connection...
/*
func TestCreateConnection(t *testing.T) {

	//log.Println(token)
	log.Println(client.port)
	fmt.Println("Connection creation test.")
	user := "sa"
	pwd := "adminadmin"
	err := client.CreateConnection("mongo_db_test_connection", "mongo_db_test_connection", domain, 27017, 0, user, pwd, 500, "", true)
	if err != nil {
		log.Println("fail to create connection! ", err)
	}
}*/

/* In case of mongoDB the Collection and Database is create at first insert.
func TestCreateDatabase(t *testing.T){
	Id := "mongo_db_test_connection"
	Database := "TestMongoDB"
	err := client.CreateDatabase(Id, Database)
	if err != nil {
		log.Println("fail to create database ", Database, err)
	}
}
*/
func TestConnect(t *testing.T) {

	err := client.Connect("mongo_db_test_connection", "adminadmin")
	if err != nil {
		log.Println("fail to connect to the backend with error ", err)
	}

}

func TestPingConnection(t *testing.T) {

	err := client.Ping("mongo_db_test_connection")
	if err != nil {
		log.Fatalln("fail to ping the backend with error ", err)
	}

	log.Println("Ping mongo_db_test_connection successed!")
}
/*
func TestPersistOne(t *testing.T) {

	Id := "mongo_db_test_connection"
	Database := "TestMongoDB"
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
*/
/*
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

	Id := "mongo_db_test_connection"
	Database := "TestCreateAndDelete_DB"
	Collection := "Employees"

	err := client.InsertMany(Id, Database, Collection, entities, "")
	if err != nil {
		log.Fatalf("Fail to insert many entities whit error %v", err)
	}
}
*/
/** Test Replace One **/
func TestReplaceOne(t *testing.T) {

	Id := "mongo_db_test_connection"
	Database := "TestCreateAndDelete_DB"
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

func TestUpdateOne(t *testing.T){
	Id := "mongo_db_test_connection"
	Database := "TestCreateAndDelete_DB"
	Collection := "Employees"

	err := client.UpdateOne(Id, Database, Collection, `{"_id":"nirani"}`, `{ "$set":{"employeeCode":"E2.2"},"$set":{"phoneNumber":"408-1231234"}}`, "")
	if err != nil {
		log.Fatalf("Fail to update entity %v", err)
	}
}

func TestUpdate(t *testing.T) {
	Id := "mongo_db_test_connection"
	Database := "TestCreateAndDelete_DB"
	Collection := "Employees"
	Query := `{"region": "CA"}`
	Value := `{"$set":{"state":"California"}}`

	err := client.Update(Id, Database, Collection, Query, Value, "")
	if err != nil {
		log.Fatalf("TestUpdate fail %v", err)
	}
	log.Println("---> update success!")
}

/*
func TestAggregate(t *testing.T) {
	//fmt.Println("Aggregate")
	/*user := "sa"
	pwd := "adminadmin"
	err := client.CreateConnection("mongo_db_test_connection", "local_resource", "localhost", 27017, 0, user, pwd, 500, "", true)
	if err != nil {
		log.Println("fail to create connection! ", err)
	}

	Id := "mongo_db_test_connection"
	Database := "local_resource"
	Collection := "Employees"

	results, err := client.Aggregate(Id, Database, Collection, `[{"$count":"toto"}]`, "")
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("---> ", results)

}
*/

/** Test find one **/

/** Test find many **/
/*func TestFind(t *testing.T) {
	fmt.Println("Find many test.")

	Id := "mongo_db_test_connection"
	Database := "TestMongoDB"
	Collection := "Employees"
	Query := `{"first_name": "Dave"}`

	values, err := client.Find(Id, Database, Collection, Query, `[{"Projection":{"first_name":1}}]`)
	if err != nil {
		log.Fatalf("TestFind fail %v", err)
	}

	log.Println(values)
	log.Println("--> end of find!")

}*/

/** Test find one **/
/*func TestFindOne(t *testing.T) {
	fmt.Println("Find one test.")

	Id := "mongo_db_test_connection"
	Database := "TestMongoDB"
	Collection := "Employees"
	Query := `{"first_name": "Dave"}`

	values, err := client.FindOne(Id, Database, Collection, Query, "")
	if err != nil {
		log.Fatalf("TestFind fail %v", err)
	}

	log.Println(values)
}*/

/** Test remove **/
/*
func TestRemove(t *testing.T) {
	fmt.Println("Test Remove")

	Id := "visualinspection_db"
	Database := "visualinspection_db"
	Collection := "Postits"
	Query := `{"date": 1618952053013}`

	err := client.DeleteOne(Id, Database, Collection, Query, "")
	if err != nil {
		log.Fatalf("DeleteOne fail %v", err)
	}

	log.Println("---> Delete success!")
}
*/

/*
func TestRemoveMany(t *testing.T) {
	fmt.Println("Test Remove")

	Id := "mongo_db_test_connection"
	Database := "TestMongoDB"
	Collection := "Employees"
	Query := `{"emp_no": 200000}`

	err := client.Delete(Id, Database, Collection, Query, "")
	if err != nil {
		log.Fatalf("DeleteOne fail %v", err)
	}

	log.Println("---> Delete success!")
}*/

// Test create a db, create a collection and remove it after...
/*
func TestCreateAndDelete(t *testing.T) {
	fmt.Println("Test Create And Delete")

	// Id := "mongo_db_test_connection"
	Id := "local_resource"
	Database := "local_resource"
	Collection := "Employees"
	JsonStr := `{"hire_date":"2007-07-01", "last_name":"Courtois", "first_name":"Dave", "birth_data":"1976-01-28", "emp_no":200000, "gender":"M"}`

	id, err := client.InsertOne(Id, Database, Collection, JsonStr, "")
	if err != nil {
		log.Println(err)
		return
	}

	var c int
	c, err = client.Count(Id, Database, Collection, "{}", "")

	if err != nil {
		log.Fatalln(err)
	}

	log.Println("---> count is ", c)

	// Test drop collection.
	err = client.DeleteCollection(Id, Database, Collection)
	if err != nil {
		log.Panicln(err)
	}

	err = client.DeleteDatabase(Id, Database)
	if err != nil {
		log.Panicln(err)
	}

	log.Println(id)

}
*/

/*
func TestDeleteConnection(t *testing.T) {
	fmt.Println("Connection creation test.")
	err := client.DeleteConnection("mongo_db_test_connection")
	if err != nil {
		log.Println("fail to delete connection! ", err)
	}
}
*/
