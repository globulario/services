package persistence_client

import (
	//"fmt"
	//"io/ioutil"
	"fmt"
	"log"
	"testing"

	"github.com/globulario/services/golang/authentication/authentication_client"
	//"github.com/davecourtois/Utility"
)

// start: mongod --dbpath E:\Project\src\github.com\davecourtois\Globular\data\mongodb-data
// Set the correct addresse here as needed.
var (

	// Connect to the plc client.
	database                  = "test_db"
	domain                    = "globule-ryzen.globular.cloud"
	client, _                 = NewPersistenceService_Client(domain, "persistence.PersistenceService")
	authentication_client_, _ = authentication_client.NewAuthenticationService_Client(domain, "authentication.AuthenticationService")
	token, _                  = authentication_client_.Authenticate("sa", "adminadmin")
)

// First test create a fresh new connection...
func TestCreateConnection(t *testing.T) {

	//fmt.Println(token)
	fmt.Println("Connection creation test.")
	user := "sa"
	pwd := "adminadmin"
	err := client.CreateConnection("test_connection", database, domain, 9042, 2, user, pwd, 500, "", true)
	if err != nil {
		fmt.Println("fail to create connection! ", err)
	}
}

/* In case of mongoDB the Collection and Database is create at first insert.*/
func TestCreateDatabase(t *testing.T) {
	Id := "test_connection"

	err := client.CreateDatabase(Id, database)
	if err != nil {
		fmt.Println("fail to create database ", database, err)
	}
}

func TestConnect(t *testing.T) {

	err := client.Connect("test_connection", "adminadmin")
	if err != nil {
		fmt.Println("fail to connect to the backend with error ", err)
	}

}

func TestPingConnection(t *testing.T) {

	err := client.Ping("test_connection")
	if err != nil {
		log.Fatalln("fail to ping the backend with error ", err)
	}

	fmt.Println("Ping test_connection successed!")
}

func TestPersistOne(t *testing.T) {

	Id := "test_connection"
	Collection := "Employees"
	employe := map[string]interface{}{
		"_id":                  "1",
		"employeeNumber":       1.0,
		"jobTitleName":         "Developer",
		"firstName":            "Dave",
		"lastName":             "Courtois",
		"preferredFullName":    "Dave Courtois",
		"employeeCode":         "E1",
		"region":               "Or",
		"state":                "Oregon",
		"phoneNumber":          "408-123-4567",
		"emailAddress":         "dave.courtois60@gmail.com",
		"programmingLanguages": []string{"JavaScript", "C++", "C", "Python", "Scala", "Java", "Go"},
	}

	id, err := client.InsertOne(Id, database, Collection, employe, "")

	if err != nil {
		fmt.Println("fail to pesist entity with error", err)
	}

	fmt.Println("Entity persist with id ", id)
}

func TestPersistMany(t *testing.T) {

	entities :=
		[]interface{}{
			map[string]interface{}{
				"_id":               "2",
				"employeeNumber":    2,
				"jobTitleName":      "Developer",
				"firstName":         "Romin",
				"lastName":          "Irani",
				"preferredFullName": "Romin Irani",
				"employeeCode":      "E2",
				"region":            "CA",
				"state":			 "California",
				"phoneNumber":       "408-123-4567",
				"emailAddress":      "romin.k.irani@gmail.com",
				"programmingLanguages": []string{"JavaScript", "C++", "C", "Python", "Scala", "Java", "Go"},
			},
			map[string]interface{}{
				"_id":               "3",
				"employeeNumber":    3,
				"jobTitleName":      "Developer",
				"firstName":         "Neil",
				"lastName":          "Irani",
				"preferredFullName": "Neil Irani",
				"employeeCode":      "E3",
				"region":            "CA",
				"state":			 "California",
				"phoneNumber":       "408-111-1111",
				"emailAddress":      "neilrirani@gmail.com",
				"programmingLanguages": []string{"JavaScript", "C++", "Java", "Python"},
			},
			map[string]interface{}{
				"_id":               "4",
				"employeeNumber":    4,
				"jobTitleName":      "Program Directory",
				"firstName":         "Tom",
				"lastName":          "Hanks",
				"preferredFullName": "Tom Hanks",
				"employeeCode":      "E4",
				"region":            "CA",
				"state":			 "California",
				"phoneNumber":       "408-222-2222",
				"emailAddress":      "tomhanks@gmail.com",
				"programmingLanguages": []string{"Java", "C++", "Scala"},
			},
		}

	Id := "test_connection"
	Collection := "Employees"

	err := client.InsertMany(Id, database, Collection, entities, "")
	if err != nil {
		fmt.Println("Fail to insert many entities with error ", err)
	}
}

/** Test Replace One **/
func TestReplaceOne(t *testing.T) {

	Id := "test_connection"
	Collection := "Employees"

	entity := map[string]interface{}{
		"_id":               "3",
		"employeeNumber":    3,
		"jobTitleName":      "Full Stack Developper",
		"firstName":         "Neil",
		"lastName":          "Irani",
		"preferredFullName": "Neil Irani",
		"employeeCode":      "E2",
		"region":            "CA",
		"phoneNumber":       "408-111-1111",
		"emailAddress":      "neilrirani@gmail.com",
		"programmingLanguages": []string{"JavaScript", "C++", "Java", "Python", "TypeScript", "React", "Angular", "Vue", "React Native"},
	}

	err := client.ReplaceOne(Id, database, Collection, `{"_id":"3"}`, entity, `[{"upsert": true}]`)
	if err != nil {
		fmt.Println("Fail to replace entity", err)
	}
}

func TestUpdateOne(t *testing.T) {
	Id := "test_connection"
	Collection := "Employees"

	err := client.UpdateOne(Id, database, Collection, `{"_id":"3"}`, `{"$set":{"employeeCode":"E2.2", "phoneNumber":"408-123-1234"} }`, "")
	if err != nil {
		fmt.Println("Fail to update one entity", err)
	}
}

func TestUpdate(t *testing.T) {
	Id := "test_connection"
	Collection := "Employees"
	Query := `{"region": "CA"}`
	Value := `{"$set":{"state":"Californication"}}`

	err := client.Update(Id, database, Collection, Query, Value, "")
	if err != nil {
		fmt.Println("TestUpdate fail", err)
	}
	fmt.Println("---> update success!")
}

/** Test find one **/
func TestFindOne(t *testing.T) {
	fmt.Println("Find one test.")

	Id := "test_connection"
	Collection := "Employees"
	Query := `{"firstName": "Dave"}`

	values, err := client.FindOne(Id, database, Collection, Query, "")
	if err != nil {
		fmt.Println("Test Find One fail", err)
	}

	fmt.Println(values)
}

/** Test find many **/
/*func TestFind(t *testing.T) {
	fmt.Println("Find many test.")

	Id := "test_connection"
	Collection := "Employees"
	Query := `{"region": "CA"}`

	values, err := client.Find(Id, database, Collection, Query, `[{"Projection":{"firstName":1}}]`)
	if err != nil {
		fmt.Println("fail to find entities with error", err)
	}

	fmt.Println(values)

}*/

/*func TestAggregate(t *testing.T) {
	//fmt.Println("Aggregate")
	Id := "test_connection"
	Collection := "Employees"

	results, err := client.Aggregate(Id, database, Collection, `[{"$count":"region"}]`, "")
	if err != nil {
		fmt.Println("fail to create aggregation with error", err)
	}
	fmt.Println("---> ", results)

}*/

/** Test remove **/

/*func TestRemove(t *testing.T) {
	fmt.Println("Test Remove")

	Id := "test_connection"
	Collection := "Employees"
	Query := `{"_id":"3"}`

	err := client.DeleteOne(Id, database, Collection, Query, "")
	if err != nil {
		fmt.Println("Fail to delete one entity with error", err)
	}
}*/

/*func TestRemoveMany(t *testing.T) {
	fmt.Println("Test Remove")

	Id := "test_connection"
	Collection := "Employees"
	Query := `{"region": "CA"}`

	err := client.Delete(Id, database, Collection, Query, "")
	if err != nil {
		fmt.Println("Fail to remove entities", err)
	}
	fmt.Println("---> Delete success!")
}*/

/*func TestDeleteCollection(t *testing.T) {
	fmt.Println("Delete collection test.")

	Id := "test_connection"
	Collection := "Employees"

	err := client.DeleteCollection(Id, database, Collection)
	if err != nil {
		fmt.Println("fail to delete collection! ", err)
	}
}*/

/*func TestDeleteDatabase(t *testing.T) {
	fmt.Println("Delete database test.")

	Id := "test_connection"
	err := client.DeleteDatabase(Id, database)
	if err != nil {
		fmt.Println("fail to delete database! ", err)
	}
}

func TestDisconnect(t *testing.T) {
	fmt.Println("Disconnect test.")
	err := client.Disconnect("test_connection")
	if err != nil {
		fmt.Println("fail to delete connection! ", err)
	}
}*/

/*func TestDeleteConnection(t *testing.T) {
	fmt.Println("Delete connection test.")
	err := client.DeleteConnection("test_connection")
	if err != nil {
		fmt.Println("fail to delete connection! ", err)
	}
}*/
