package persistence_client

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/config"
)

/*
This suite covers (Scylla backend):
- Connection lifecycle: CreateConnection → CreateDatabase → Connect → Ping → Disconnect → DeleteConnection
- CRUD on a collection, including scalar fields and array side-table handling:
  InsertOne, InsertMany, FindOne (by _id), Find (all), Count ({}), UpdateOne (scalar + array),
  ReplaceOne (upsert semantics), DeleteOne, Delete (bulk by "{}"), DeleteCollection, DeleteDatabase.

Important Scylla note:
- We avoid non–primary-key filters (e.g., {"firstName":"Dave"}) because Scylla requires ALLOW FILTERING
  or secondary indexes for those. We use _id queries or "{}" (full scan) instead.
*/

func must[T any](t *testing.T, v T, err error) T {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return v
}

func uniq(prefix string) string { return fmt.Sprintf("%s_%d", prefix, time.Now().Unix()) }

func getServiceAddress(t *testing.T) string {
	t.Helper()
	addr, err := config.GetAddress()
	if err != nil || addr == "" {
		t.Fatalf("cannot resolve service address from config: %v", err)
	}
	return addr
}

// Resolve Scylla host/port for CreateConnection.
// If SCYLLA_HOST/SCYLLA_PORT env vars are set, prefer those.
// Otherwise, derive host from the service address; port defaults to 9042.
func resolveScyllaHostPort(t *testing.T) (string, int) {
	t.Helper()
	host := strings.TrimSpace(os.Getenv("SCYLLA_HOST"))
	if host == "" {
		addr := getServiceAddress(t)
		host = strings.Split(addr, ":")[0]
	}
	port := 9042
	if p := strings.TrimSpace(os.Getenv("SCYLLA_PORT")); p != "" {
		if pi, err := strconv.Atoi(p); err == nil && pi > 0 {
			port = pi
		}
	}
	return host, port
}

func TestPersistenceServiceLifecycle(t *testing.T) {
	address := getServiceAddress(t)

	// Authenticate (validates we can talk to the platform, not used further here)
	authClient, err := authentication_client.NewAuthenticationService_Client(address, "authentication.AuthenticationService")
	auth := must(t, authClient, err)
	if _, err := auth.Authenticate("sa", "adminadmin"); err != nil {
		t.Fatalf("Authenticate(sa) failed: %v", err)
	}

	client, err := NewPersistenceService_Client(address, "persistence.PersistenceService")
	if err != nil {
		t.Fatalf("NewPersistenceService_Client failed: %v", err)
	}

	// Unique IDs per run
	connID := uniq("test_connection")
	db := uniq("test_db")
	coll := "Employees" // keep constant so side-table name stays predictable

	// Scylla host/port
	scyllaHost, scyllaPort := resolveScyllaHostPort(t)

	t.Run("CreateConnection", func(t *testing.T) {
		options := `{"consistency":"ONE"}`
		if err := client.CreateConnection(connID, db, scyllaHost, float64(scyllaPort), 2 /*SCYLLA*/, "sa", "adminadmin", 500, options, true); err != nil {
			t.Fatalf("CreateConnection failed: %v", err)
		}
	})

	t.Run("CreateDatabase", func(t *testing.T) {
		if err := client.CreateDatabase(connID, db); err != nil {
			t.Fatalf("CreateDatabase(%s) failed: %v", db, err)
		}
	})

	t.Run("ConnectAndPing", func(t *testing.T) {
		if err := client.Connect(connID, "adminadmin"); err != nil {
			t.Fatalf("Connect failed: %v", err)
		}
		if err := client.Ping(connID); err != nil {
			t.Fatalf("Ping failed: %v", err)
		}
	})

	// ---- CRUD ----

	t.Run("InsertOne_withArray", func(t *testing.T) {
		emp := map[string]interface{}{
			"_id":                  "1",
			"employeeNumber":       1,
			"jobTitleName":         "Developer",
			"firstName":            "Dave",
			"lastName":             "Courtois",
			"preferredFullName":    "Dave Courtois",
			"employeeCode":         "E1",
			"region":               "OR",
			"state":                "Oregon",
			"phoneNumber":          "408-123-4567",
			"emailAddress":         "dave.courtois60@gmail.com",
			"programmingLanguages": []string{"JavaScript", "C++", "C", "Python", "Scala", "Java", "Go"},
		}
		if _, err := client.InsertOne(connID, db, coll, emp, ""); err != nil {
			t.Fatalf("InsertOne failed: %v", err)
		}
	})

	t.Run("FindOne_byId_and_CountAll", func(t *testing.T) {
		obj, err := client.FindOne(connID, db, coll, `{"_id":"1"}`, "")
		if err != nil {
			t.Fatalf("FindOne by _id failed: %v", err)
		}
		if got := fmt.Sprint(obj["_id"]); got != "1" {
			t.Fatalf("FindOne _id want 1, got %v", got)
		}
		total, err := client.Count(connID, db, coll, `{}`, "")
		if err != nil {
			t.Fatalf("Count({}) failed: %v", err)
		}
		if total != 1 {
			t.Fatalf("Count({}) want 1, got %d", total)
		}
	})

	t.Run("InsertMany", func(t *testing.T) {
		entities := []interface{}{
			map[string]interface{}{
				"_id":                  "2",
				"employeeNumber":       2,
				"jobTitleName":         "Developer",
				"firstName":            "Romin",
				"lastName":             "Irani",
				"preferredFullName":    "Romin Irani",
				"employeeCode":         "E2",
				"region":               "CA",
				"state":                "California",
				"phoneNumber":          "408-123-4567",
				"emailAddress":         "romin.k.irani@gmail.com",
				"programmingLanguages": []string{"JavaScript", "C++", "C", "Python", "Scala", "Java", "Go"},
			},
			map[string]interface{}{
				"_id":                  "3",
				"employeeNumber":       3,
				"jobTitleName":         "Developer",
				"firstName":            "Neil",
				"lastName":             "Irani",
				"preferredFullName":    "Neil Irani",
				"employeeCode":         "E3",
				"region":               "CA",
				"state":                "California",
				"phoneNumber":          "408-111-1111",
				"emailAddress":         "neilrirani@gmail.com",
				"programmingLanguages": []string{"JavaScript", "C++", "Java", "Python"},
			},
			map[string]interface{}{
				"_id":                  "4",
				"employeeNumber":       4,
				"jobTitleName":         "Program Director",
				"firstName":            "Tom",
				"lastName":             "Hanks",
				"preferredFullName":    "Tom Hanks",
				"employeeCode":         "E4",
				"region":               "CA",
				"state":                "California",
				"phoneNumber":          "408-222-2222",
				"emailAddress":         "tomhanks@gmail.com",
				"programmingLanguages": []string{"Java", "C++", "Scala"},
			},
		}
		if err := client.InsertMany(connID, db, coll, entities, ""); err != nil {
			t.Fatalf("InsertMany failed: %v", err)
		}
	})

	t.Run("FindAll_and_CountAll", func(t *testing.T) {
		results, err := client.Find(connID, db, coll, `{}`, "")
		if err != nil {
			t.Fatalf("Find({}) failed: %v", err)
		}
		if len(results) != 4 {
			t.Fatalf("Find({}) want 4 docs, got %d", len(results))
		}
		total, err := client.Count(connID, db, coll, `{}`, "")
		if err != nil {
			t.Fatalf("Count({}) failed: %v", err)
		}
		if total != 4 {
			t.Fatalf("Count({}) want 4, got %d", total)
		}
	})

	t.Run("UpdateOne_scalar_fields_byId", func(t *testing.T) {
		if err := client.UpdateOne(connID, db, coll, `{"_id":"3"}`, `{"$set":{"employeeCode":"E2.2","phoneNumber":"408-123-1234"}}`, ""); err != nil {
			t.Fatalf("UpdateOne failed: %v", err)
		}
		obj, err := client.FindOne(connID, db, coll, `{"_id":"3"}`, "")
		if err != nil {
			t.Fatalf("FindOne after UpdateOne failed: %v", err)
		}
		if got := fmt.Sprint(obj["employeeCode"]); got != "E2.2" {
			t.Fatalf("employeeCode want E2.2, got %v", got)
		}
		if got := fmt.Sprint(obj["phoneNumber"]); got != "408-123-1234" {
			t.Fatalf("phoneNumber want 408-123-1234, got %v", got)
		}
	})

	t.Run("ReplaceOne_upsert_and_array_full_overwrite", func(t *testing.T) {
		entity := map[string]interface{}{
			"_id":                  "3",
			"employeeNumber":       3,
			"jobTitleName":         "Full Stack Developer",
			"firstName":            "Neil",
			"lastName":             "Irani",
			"preferredFullName":    "Neil Irani",
			"employeeCode":         "E2.2",
			"region":               "CA",
			"state":                "California",
			"phoneNumber":          "408-123-1234",
			"emailAddress":         "neilrirani@gmail.com",
			"programmingLanguages": []string{"JavaScript", "C++", "Java", "Python", "TypeScript", "React", "Angular", "Vue", "React Native"},
		}
		if err := client.ReplaceOne(connID, db, coll, `{"_id":"3"}`, entity, `[{"upsert": true}]`); err != nil {
			t.Fatalf("ReplaceOne(upsert) failed: %v", err)
		}
		obj, err := client.FindOne(connID, db, coll, `{"_id":"3"}`, "")
		if err != nil {
			t.Fatalf("FindOne after ReplaceOne failed: %v", err)
		}
		if title := fmt.Sprint(obj["jobTitleName"]); title != "Full Stack Developer" {
			t.Fatalf("jobTitleName want Full Stack Developer, got %v", title)
		}
		if langs, ok := obj["programmingLanguages"].([]interface{}); !ok || len(langs) < 5 {
			t.Fatalf("programmingLanguages not replaced as expected: %#v", obj["programmingLanguages"])
		}
	})

	t.Run("UpdateOne_array_field_byId", func(t *testing.T) {
		if err := client.UpdateOne(connID, db, coll, `{"_id":"2"}`, `{"$set":{"programmingLanguages":["Go","Rust"]}}`, ""); err != nil {
			t.Fatalf("UpdateOne(array) failed: %v", err)
		}
		obj, err := client.FindOne(connID, db, coll, `{"_id":"2"}`, "")
		if err != nil {
			t.Fatalf("FindOne after UpdateOne(array) failed: %v", err)
		}
		langs, ok := obj["programmingLanguages"].([]interface{})
		if !ok || len(langs) != 2 || fmt.Sprint(langs[0]) != "Go" || fmt.Sprint(langs[1]) != "Rust" {
			t.Fatalf("programmingLanguages want [Go Rust], got %#v", langs)
		}
	})

	// ---- Delete paths & cleanup ----

	t.Run("DeleteOne_byId", func(t *testing.T) {
		if err := client.DeleteOne(connID, db, coll, `{"_id":"3"}`, ""); err != nil {
			t.Fatalf("DeleteOne failed: %v", err)
		}
		if _, err := client.FindOne(connID, db, coll, `{"_id":"3"}`, ""); err == nil {
			t.Fatalf("FindOne should fail after DeleteOne")
		}
	})

	t.Run("Delete_all_with_empty_query", func(t *testing.T) {
		// This exercises the bulk Delete path without non-PK filters.
		if err := client.Delete(connID, db, coll, `{}`, ""); err != nil {
			t.Fatalf("Delete({}) failed: %v", err)
		}
		total, err := client.Count(connID, db, coll, `{}`, "")
		if err != nil {
			t.Fatalf("Count({}) after Delete failed: %v", err)
		}
		if total != 0 {
			t.Fatalf("Count({}) want 0 after Delete, got %d", total)
		}
	})

	t.Run("DeleteCollection", func(t *testing.T) {
		if err := client.DeleteCollection(connID, db, coll); err != nil {
			t.Fatalf("DeleteCollection failed: %v", err)
		}
	})

	t.Run("DeleteDatabase", func(t *testing.T) {
		if err := client.DeleteDatabase(connID, db); err != nil {
			t.Fatalf("DeleteDatabase failed: %v", err)
		}
	})

	t.Run("Disconnect_and_DeleteConnection", func(t *testing.T) {
		if err := client.Disconnect(connID); err != nil {
			t.Fatalf("Disconnect failed: %v", err)
		}
		if err := client.DeleteConnection(connID); err != nil {
			t.Fatalf("DeleteConnection failed: %v", err)
		}
	})
}
