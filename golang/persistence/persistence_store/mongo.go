package persistence_store

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"

	Utility "github.com/globulario/utility"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

/**
 * Implementation of the Store interface with MongoDB.
 */
type MongoStore struct {
	// Keep track of connections with MongoDB.
	clients  map[string]*mongo.Client
	DataPath string
	Port     int
	Password string
	User     string // Must be the admin...
	Host     string
}

/**
 * Connect to the remote/local MongoDB store.
 * TODO: Add more connection options via the optionsStr and options package.
 */
func (store *MongoStore) Connect(connectionId string, host string, port int32, user string, password string, database string, timeout int32, optionsStr string) error {
	if timeout == 0 {
		timeout = 500 // Default timeout value.
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Millisecond)
	defer cancel()

	if store.clients == nil {
		store.clients = make(map[string]*mongo.Client)
	} else {
		if store.clients[connectionId] != nil {
			fmt.Println("Trying to ping connection:", connectionId)
			err := store.clients[connectionId].Ping(ctx, nil)
			if err == nil {
				return nil // Already connected.
			}
		}
	}

	var opts []*options.ClientOptions
	var client *mongo.Client
	if len(optionsStr) > 0 {
		err := json.Unmarshal([]byte(optionsStr), &opts)
		if err != nil {
			return err
		}
		client, err = mongo.NewClient(opts...)
		if err != nil {
			return err
		}
	} else {
		password_ := url.QueryEscape(password)
		user_ := url.QueryEscape(user)

		clientOpts := options.Client().SetHosts([]string{host}).SetAuth(
			options.Credential{
				AuthSource:    "admin",
				AuthMechanism: "SCRAM-SHA-256",
				Username:      user_,
				Password:      password_,
			},
		)

		var err error
		client, err = mongo.NewClient(clientOpts)
		if err != nil {
			fmt.Println("Failed to create client connection with error:", err)
			return err
		}
	}

	err := client.Connect(ctx)
	if err != nil {
		fmt.Println("Failed to connect with error:", err)
		return err
	}

	store.clients[connectionId] = client

	return nil
}

func (store *MongoStore) Disconnect(connectionId string) error {
	if store.clients[connectionId] == nil {
		return errors.New("No connection found with name " + connectionId)
	}

	// Close the connection
	err := store.clients[connectionId].Disconnect(context.Background())
	// Remove it from the map.
	delete(store.clients, connectionId)

	return err
}

func (store *MongoStore) GetStoreType() string {
	return "MONGO"
}

/**
 * Return nil on success.
 */
func (store *MongoStore) Ping(ctx context.Context, connectionId string) error {
	if store.clients[connectionId] == nil {
		return errors.New("No connection found with name " + connectionId)
	}

	return store.clients[connectionId].Ping(ctx, nil)
}

/**
 * Return the number of entries in a table.
 */
func (store *MongoStore) Count(ctx context.Context, connectionId string, database string, collection string, query string, optionsStr string) (int64, error) {
	if store.clients[connectionId] == nil {
		return -1, errors.New("No connection found with name " + connectionId)
	}

	var opts []*options.CountOptions
	if len(optionsStr) > 0 {
		err := json.Unmarshal([]byte(optionsStr), &opts)
		if err != nil {
			return int64(0), err
		}
	}

	q := make(map[string]interface{})
	err := json.Unmarshal([]byte(query), &q)
	if err != nil {
		return int64(0), err
	}

	count, err := store.clients[connectionId].Database(database).Collection(collection).CountDocuments(ctx, q, opts...)
	return count, err
}

func (store *MongoStore) CreateDatabase(ctx context.Context, connectionId string, name string) error {
	return errors.New("MongoDB will create your database at the first insert.")
}

/**
 * Delete a database.
 */
func (store *MongoStore) DeleteDatabase(ctx context.Context, connectionId string, name string) error {
	if store.clients[connectionId] == nil {
		return errors.New("No connection found with name " + connectionId)
	}

	return store.clients[connectionId].Database(name).Drop(ctx)
}

/**
 * Create a Collection.
 */
func (store *MongoStore) CreateCollection(ctx context.Context, connectionId string, database string, name string, optionsStr string) error {
	if store.clients[connectionId] == nil {
		return errors.New("No connection found with name " + connectionId)
	}

	db := store.clients[connectionId].Database(database)
	if db == nil {
		return errors.New("Database " + database + " does not exist!")
	}

	var opts []*options.CreateCollectionOptions
	if len(optionsStr) > 0 {
		err := json.Unmarshal([]byte(optionsStr), &opts)
		if err != nil {
			return err
		}
	}

	err := db.CreateCollection(ctx, name, opts...)
	if err != nil {
		return errors.New("Failed to create collection " + name + " with error: " + err.Error())
	}

	return nil
}

/**
 * Delete a collection.
 */
func (store *MongoStore) DeleteCollection(ctx context.Context, connectionId string, database string, name string) error {
	if store.clients[connectionId] == nil {
		return errors.New("No connection found with name " + connectionId)
	}

	err := store.clients[connectionId].Database(database).Collection(name).Drop(ctx)
	return err
}

////////////////////////////////////////////////////////////////////////////////
// Insert
////////////////////////////////////////////////////////////////////////////////

/**
 * Insert one value in the store.
 */
func (store *MongoStore) InsertOne(ctx context.Context, connectionId string, database string, collection string, entity interface{}, optionsStr string) (interface{}, error) {
	if store.clients[connectionId] == nil {
		return nil, errors.New("No connection found with name " + connectionId)
	}

	var opts []*options.InsertOneOptions
	if len(optionsStr) > 0 {
		err := json.Unmarshal([]byte(optionsStr), &opts)
		if err != nil {
			return int64(0), err
		}
	}

	// Get the collection object.
	collection_ := store.clients[connectionId].Database(database).Collection(collection)

	result, err := collection_.InsertOne(ctx, entity, opts...)

	if err != nil {
		return nil, err
	}

	return result.InsertedID, nil
}

/**
 * Insert many results at a time.
 */
func (store *MongoStore) InsertMany(ctx context.Context, connectionId string, database string, collection string, entities []interface{}, optionsStr string) ([]interface{}, error) {
	if store.clients[connectionId] == nil {
		return nil, errors.New("No connection found with name " + connectionId)
	}

	var opts []*options.InsertManyOptions
	if len(optionsStr) > 0 {
		err := json.Unmarshal([]byte(optionsStr), &opts)
		if err != nil {
			return nil, err
		}
	}

	// Get the collection object.
	collection_ := store.clients[connectionId].Database(database).Collection(collection)

	// Return store.clients[connectionId].Ping(ctx, nil)
	insertManyResult, err := collection_.InsertMany(ctx, entities, opts...)
	if err != nil {
		return nil, err
	}

	return insertManyResult.InsertedIDs, nil
}

////////////////////////////////////////////////////////////////////////////////
// Read
////////////////////////////////////////////////////////////////////////////////

/**
 * Find many values from a query.
 */
func (store *MongoStore) Find(ctx context.Context, connectionId string, database string, collection string, query string, optionsStr string) ([]interface{}, error) {
	if store.clients[connectionId] == nil {
		return nil, errors.New("No connection found with name " + connectionId)
	}

	if store.clients[connectionId].Database(database) == nil {
		return nil, errors.New("No database found with name " + database)
	}

	if store.clients[connectionId].Database(database).Collection(collection) == nil {
		return nil, errors.New("No collection found with name " + collection)
	}

	collection_ := store.clients[connectionId].Database(database).Collection(collection)
	q := make(map[string]interface{})
	err := json.Unmarshal([]byte(query), &q)

	if err != nil {
		return nil, err
	}

	var opts []*options.FindOptions
	if len(optionsStr) > 0 {
		err := json.Unmarshal([]byte(optionsStr), &opts)
		if err != nil {
			return nil, err
		}
	}

	cur, err := collection_.Find(ctx, q, opts...)
	if err != nil {
		return nil, err
	}

	defer cur.Close(context.Background())
	results := make([]interface{}, 0)

	for cur.Next(ctx) {
		entity := make(map[string]interface{})
		err := cur.Decode(&entity)
		if err != nil {
			return nil, err
		}
		// In that case, I will return the whole entity
		results = append(results, entity)
	}

	// In case of error
	if err := cur.Err(); err != nil {
		return results, err
	}

	return results, nil
}

/**
 * Aggregate results from a collection.
 */
func (store *MongoStore) Aggregate(ctx context.Context, connectionId string, database string, collection string, pipeline string, optionsStr string) ([]interface{}, error) {
	if store.clients[connectionId] == nil {
		return nil, errors.New("No connection found with name " + connectionId)
	}

	if store.clients[connectionId].Database(database) == nil {
		return nil, errors.New("No database found with name " + database)
	}

	if store.clients[connectionId].Database(database).Collection(collection) == nil {
		return nil, errors.New("No collection found with name " + collection)
	}

	collection_ := store.clients[connectionId].Database(database).Collection(collection)

	p := make([]interface{}, 0)
	err := json.Unmarshal([]byte(pipeline), &p)
	if err != nil {
		return nil, err
	}

	var opts []*options.AggregateOptions
	if len(optionsStr) > 0 {
		err := json.Unmarshal([]byte(optionsStr), &opts)
		if err != nil {
			return nil, err
		}
	}

	cur, err := collection_.Aggregate(ctx, p, opts...)
	if err != nil {
		return nil, err
	}
	defer cur.Close(context.Background())

	results := make([]interface{}, 0)
	for cur.Next(ctx) {
		entity := make(map[string]interface{})
		err := cur.Decode(&entity)
		if err != nil {
			return nil, err
		}
		// In that case, I will return the whole entity
		results = append(results, entity)
	}

	// In case of error
	if err := cur.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

/**
 * Find one result at a time.
 */
func (store *MongoStore) FindOne(ctx context.Context, connectionId string, database string, collection string, query string, optionsStr string) (interface{}, error) {
	if store.clients[connectionId] == nil {
		return nil, errors.New("No connection found with name " + connectionId)
	}

	if store.clients[connectionId].Database(database) == nil {
		return nil, errors.New("No database found with name " + database)
	}

	if store.clients[connectionId].Database(database).Collection(collection) == nil {
		return nil, errors.New("No collection found with name " + collection)
	}

	collection_ := store.clients[connectionId].Database(database).Collection(collection)
	q := make(map[string]interface{})
	err := json.Unmarshal([]byte(query), &q)
	if err != nil {
		return nil, err
	}

	var opts []*options.FindOneOptions
	if len(optionsStr) > 0 {
		err := json.Unmarshal([]byte(optionsStr), &opts)
		if err != nil {
			return nil, err
		}
	}

	entity := make(map[string]interface{})
	err = collection_.FindOne(ctx, q, opts...).Decode(&entity)
	if err != nil {
		return nil, err
	}

	return entity, nil
}

////////////////////////////////////////////////////////////////////////////////
// Update
////////////////////////////////////////////////////////////////////////////////

/**
 * Update one or more values that match the query.
 */
func (store *MongoStore) Update(ctx context.Context, connectionId string, database string, collection string, query string, value string, optionsStr string) error {
	if store.clients[connectionId] == nil {
		return errors.New("No connection found with name " + connectionId)
	}

	if store.clients[connectionId].Database(database) == nil {
		return errors.New("No database found with name " + database)
	}

	if store.clients[connectionId].Database(database).Collection(collection) == nil {
		return errors.New("No collection found with name " + collection)
	}

	collection_ := store.clients[connectionId].Database(database).Collection(collection)
	q := make(map[string]interface{})
	err := json.Unmarshal([]byte(query), &q)
	if err != nil {
		return err
	}

	v := new(bson.D)
	err = bson.UnmarshalExtJSON([]byte(value), true, &v)
	if err != nil {
		return err
	}

	var opts []*options.UpdateOptions
	if len(optionsStr) > 0 {
		err := json.Unmarshal([]byte(optionsStr), &opts)
		if err != nil {
			return err
		}
	}

	_, err = collection_.UpdateMany(ctx, q, v, opts...)
	if err != nil {
		return err
	}

	return nil
}

/**
 * Update one document at a time.
 */
func (store *MongoStore) UpdateOne(ctx context.Context, connectionId string, database string, collection string, query string, value string, optionsStr string) error {
	if store.clients[connectionId] == nil {
		return errors.New("No connection found with name " + connectionId)
	}

	if store.clients[connectionId].Database(database) == nil {
		return errors.New("No database found with name " + database)
	}

	if store.clients[connectionId].Database(database).Collection(collection) == nil {
		return errors.New("No collection found with name " + collection)
	}

	collection_ := store.clients[connectionId].Database(database).Collection(collection)
	q := make(map[string]interface{})
	err := json.Unmarshal([]byte(query), &q)
	if err != nil {
		return err
	}

	v := new(bson.D)
	err = bson.UnmarshalExtJSON([]byte(value), true, &v)
	if err != nil {
		return err
	}

	var opts []*options.UpdateOptions
	if len(optionsStr) > 0 {
		err := json.Unmarshal([]byte(optionsStr), &opts)
		if err != nil {
			return err
		}
	}

	_, err = collection_.UpdateOne(ctx, q, v, opts...)
	if err != nil {
		return err
	}

	return nil
}

/**
 * Replace a document by another.
 */
func (store *MongoStore) ReplaceOne(ctx context.Context, connectionId string, database string, collection string, query string, value string, optionsStr string) error {
	if store.clients[connectionId] == nil {
		return errors.New("No connection found with name " + connectionId)
	}

	if store.clients[connectionId].Database(database) == nil {
		return errors.New("No database found with name " + database)
	}

	if store.clients[connectionId].Database(database).Collection(collection) == nil {
		return errors.New("No collection found with name " + collection)
	}

	collection_ := store.clients[connectionId].Database(database).Collection(collection)
	q := make(map[string]interface{})
	err := json.Unmarshal([]byte(query), &q)
	if err != nil {
		return err
	}

	v := new(bson.D)
	err = bson.UnmarshalExtJSON([]byte(value), true, &v)
	if err != nil {
		return err
	}

	var opts []*options.ReplaceOptions
	if len(optionsStr) > 0 {
		err := json.Unmarshal([]byte(optionsStr), &opts)
		if err != nil {
			return err
		}
	}

	_, err = collection_.ReplaceOne(ctx, q, v, opts...)

	if err != nil {
		return err
	}

	return nil
}

////////////////////////////////////////////////////////////////////////////////
// Delete
////////////////////////////////////////////////////////////////////////////////

/**
 * Remove one or more values depending on the query results.
 */
func (store *MongoStore) Delete(ctx context.Context, connectionId string, database string, collection string, query string, optionsStr string) error {
	if store.clients[connectionId] == nil {
		return errors.New("No connection found with name " + connectionId)
	}

	if store.clients[connectionId].Database(database) == nil {
		return errors.New("No database found with name " + database)
	}

	if store.clients[connectionId].Database(database).Collection(collection) == nil {
		return errors.New("No collection found with name " + collection)
	}

	collection_ := store.clients[connectionId].Database(database).Collection(collection)
	q := make(map[string]interface{})
	err := json.Unmarshal([]byte(query), &q)
	if err != nil {
		return err
	}

	var opts []*options.DeleteOptions
	if len(optionsStr) > 0 {
		err := json.Unmarshal([]byte(optionsStr), &opts)
		if err != nil {
			return err
		}
	}

	_, err = collection_.DeleteMany(ctx, q, opts...)
	if err != nil {
		return err
	}

	return nil
}

/**
 * Remove one document at a time.
 */
func (store *MongoStore) DeleteOne(ctx context.Context, connectionId string, database string, collection string, query string, optionsStr string) error {
	if store.clients[connectionId] == nil {
		return errors.New("No connection found with name " + connectionId)
	}

	if store.clients[connectionId].Database(database) == nil {
		return errors.New("No database found with name " + database)
	}

	if store.clients[connectionId].Database(database).Collection(collection) == nil {
		return errors.New("No collection found with name " + collection)
	}

	collection_ := store.clients[connectionId].Database(database).Collection(collection)
	q := make(map[string]interface{})
	err := json.Unmarshal([]byte(query), &q)
	if err != nil {
		return err
	}

	var opts []*options.DeleteOptions
	if len(optionsStr) > 0 {
		err := json.Unmarshal([]byte(optionsStr), &opts)
		if err != nil {
			return err
		}
	}

	_, err = collection_.DeleteOne(ctx, q, opts...)
	if err != nil {
		return err
	}

	return nil
}

/**
 * Create a user. Optionally assign it a role.
 * Roles example: [{ role: "myReadOnlyRole", db: "mytest"}]
 */
func (store *MongoStore) RunAdminCmd(ctx context.Context, connectionId string, user string, password string, script string) error {
	// Here I will retrieve the path of the MongoDB and use it to find the MongoDB command.

	if store.clients[connectionId] == nil {
		return errors.New("No connection found with name " + connectionId)
	}

	cmd := "mongosh" // Now mongosh since version 6
	args := []string{"--host", "0.0.0.0", "--port", strconv.Itoa(store.Port)}

	// If the command needs authentication.
	if len(user) > 0 {
		args = append(args, "-u", user, "-p", password, "--authenticationDatabase", "admin")
	}

	args = append(args, "--eval", script)

	cmd_ := exec.Command(cmd, args...)
	cmd_.Dir = os.TempDir()

	err := cmd_.Run()

	return err
}

/**
 * Create a role. Privilege is a JSON string describing the privilege.
 * Privileges example: [{ resource: { db: "mytest", collection: "col2" }, actions: ["find"] }, roles: []}
 */
func (store *MongoStore) CreateRole(ctx context.Context, connectionId string, role string, privileges string, options string) error {
	if store.clients[connectionId] == nil {
		return errors.New("No connection found with name " + connectionId)
	}

	return nil
}

/**
 * Stop MongoDB instance.
 */
func (store *MongoStore) stopMongod() error {
	fmt.Println("Trying to stop MongoDB instance")
	pids, err := Utility.GetProcessIdsByName("mongod")
	if err == nil {
		if len(pids) == 0 {
			fmt.Println("No MongoDB instance found")
			return nil
		}
	} else {
		fmt.Println("Failed to retrieve MongoDB instance with error", err)
		return nil
	}

	cmd := "mongosh"
	closeCmd := exec.Command(cmd, "--host", "0.0.0.0", "--port", strconv.Itoa(store.Port), "--eval", "db=db.getSiblingDB('admin');db.adminCommand({ shutdown: 1 });")
	closeCmd.Dir = os.TempDir()

	err = closeCmd.Run()
	if err != nil {
		pids, _ := Utility.GetProcessIdsByName("mongod")
		if len(pids) == 0 {
			return nil
		}
	}
	return err
}

/**
 * Create the super administrator in the DB. Not the SA global account!!!
 */
func (store *MongoStore) registerSa() error {
	// Here I will test if MongoDB exists on the store.
	wait := make(chan error)
	Utility.RunCmd("mongod", os.TempDir(), []string{"--version"}, wait)
	err := <-wait
	if err != nil {
		return err
	}

	// Here I will create the super admin if it does not already exist.
	dataPath := store.DataPath + "/mongodb-data"

	fmt.Println("Testing if path", dataPath, "exists")
	if !Utility.Exists(dataPath) {
		// Kill MongoDB store if the process is already running...
		fmt.Println("No MongoDB database exists, I will create one")

		fmt.Println("Stopping existing MongoDB instance")
		err := store.stopMongod()
		if err != nil {
			fmt.Println("Failed to stop current MongoDB server instance with error:", err)
			return err
		}

		// Here I will create the directory
		err = os.MkdirAll(dataPath, os.ModeDir)
		if err != nil {
			fmt.Println("Failed to create data `"+dataPath+"` dir with error:", err)
			return err
		}

		// Wait until MongoDB stops.
		fmt.Println("Starting MongoDB without authentication")
		go startMongoDB(store.Port, dataPath, false, wait)

		err = <-wait
		fmt.Println("830")

		if err != nil {
			fmt.Println("Failed to start MongoDB with error:", err)
			return err
		}

		fmt.Println("Creating SA user and setting authentication parameters")

		// Now I will create a new user named SA and give it all admin write.
		createSaScript := fmt.Sprintf(
			`db=db.getSiblingDB('admin');db.createUser({ user: '%s', pwd: '%s', roles: ['userAdminAnyDatabase','userAdmin','readWrite','dbAdmin','clusterAdmin','readWriteAnyDatabase','dbAdminAnyDatabase']});`, store.User, store.Password) // Must be changed...

		Utility.RunCmd("mongosh", os.TempDir(), []string{"--host", "0.0.0.0", "--port", strconv.Itoa(store.Port), "--eval", createSaScript}, wait)
		err = <-wait

		if err != nil {
			// Remove the mongodb-data
			fmt.Println("839 Failed to run command mongosh with error:", err)
			os.RemoveAll(dataPath)
			return err
		}

		fmt.Println("Stopping MongoDB instance instance and restarting it with authentication")
		err = store.stopMongod()
		if err != nil {
			fmt.Println("847 Failed to stop current MongoDB server instance with error:", err)
			return err
		}
	}

	fmt.Println("Starting MongoDB server with authentication")

	// Wait until MongoDB was started...
	go startMongoDB(store.Port, dataPath, true, wait)
	err = <-wait
	if err != nil {
		fmt.Println("Failed to start MongoDB with error:", err)
		return err
	}

	// Get the list of all services method.
	return nil
}

func startMongoDB(port int, dataPath string, withAuth bool, wait chan error) error {
	fmt.Println("Trying to start MongoDB at port:", port, "data path:", dataPath)
	pids, _ := Utility.GetProcessIdsByName("mongod")
	if pids != nil {
		if len(pids) > 0 {
			wait <- nil
			fmt.Println("MongoDB already running with pid", pids)
			return nil // MongoDB already running
		}
	}

	baseCmd := "mongod"
	cmdArgs := []string{"--port", strconv.Itoa(port), "--bind_ip", "0.0.0.0", "--dbpath", dataPath}

	if withAuth {
		cmdArgs = append([]string{"--auth"}, cmdArgs...)
	}

	cmd := exec.Command(baseCmd, cmdArgs...)
	cmd.Dir = filepath.Dir(dataPath)

	pid := -1

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("Failed to connect stdout with error:", err)
		return err
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output := make(chan string)
	done := make(chan bool)

	// Process message until the command is done.
	go func() {
		for {
			select {
			case <-done:
				fmt.Println("MongoDB process terminated...")
				wait <- nil // Unblock it...
				break

			case result := <-output:
				if cmd.Process != nil {
					pid = cmd.Process.Pid
				}

				if withAuth {
					if strings.Contains(result, "Waiting for connections") {
						fmt.Println("Waiting for MongoDB to start...")
						time.Sleep(time.Second * 1)
						fmt.Println("MongoDB is running with pid:", pid)
						wait <- nil // Unblock it...
					}
				} else {
					if strings.Contains(result, "Operating System") {
						fmt.Println("Waiting for MongoDB to start...")
						time.Sleep(time.Second * 5)

						fmt.Println("MongoDB is running with pid:", pid)

						wait <- nil // Unblock it...
					}
				}
				//fmt.Println("mongod:", pid, result)
			}
		}
	}()

	// Start reading the output
	go Utility.ReadOutput(output, stdout)
	err = cmd.Run()
	if err != nil {
		fmt.Println("Failed to run MongoDB with error", fmt.Sprint(err)+": "+stderr.String())
		fmt.Println("mongod", "--port", strconv.Itoa(port), "--bind_ip", "0.0.0.0", "--dbpath", dataPath)
		return errors.New(fmt.Sprint(err) + ": " + stderr.String())
	}

	cmd.Wait()

	// Close the output.
	stdout.Close()
	done <- true

	return nil
}
