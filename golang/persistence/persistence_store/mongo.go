package persistence_store

import (
	"bytes"
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	//"time"

	"encoding/json"
	"errors"
	"fmt"
	"os"

	"net/url"

	"go.mongodb.org/mongo-driver/bson"

	//"github.com/iancoleman/orderedmap"

	//"github.com/davecourtois/Utility"
	"github.com/davecourtois/Utility"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	// execute...
	"os/exec"
)

/**
 * Implementation of the the Store interface with mongo db.
 */
type MongoStore struct {
	// keep track of connection with mongo db.
	clients  map[string]*mongo.Client
	DataPath string
	Port     int
	Password string
	User     string // Must be the admin...
	Host     string
}

/**
 * Connect to the remote/local mongo store
 * TODO add more connection options via the option_str and options package.
 */
func (store *MongoStore) Connect(connectionId string, host string, port int32, user string, password string, database string, timeout int32, optionsStr string) error {
	if timeout == 0 {
		timeout = 500 // default timeout value.
	}

	//ctx := context.Background()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Millisecond)

	// in case the connection fail.
	defer cancel()

	if store.clients == nil {
		store.clients = make(map[string]*mongo.Client)
	} else {
		if store.clients[connectionId] != nil {
			fmt.Println("try to ping connection: ", connectionId)

			// Ping seem's to be buggy...
			err := store.clients[connectionId].Ping(ctx, nil)
			if err == nil {
				return nil // already connected.
			}
		}
	}

	var opts []*options.ClientOptions
	var client *mongo.Client
	if len(optionsStr) > 0 {
		opts = make([]*options.ClientOptions, 0)
		err := json.Unmarshal([]byte(optionsStr), &opts)
		if err != nil {
			return err
		}
		client, err = mongo.NewClient(opts...)
		if err != nil {
			return err
		}
	} else {

		// basic connection string to begin with.
		password_ := url.QueryEscape(password)
		user_ := url.QueryEscape(user)

		var err error
		clientOpts := options.Client().SetHosts(
			[]string{host + ":" + strconv.Itoa(int(port))},
		).SetAuth(
			options.Credential{
				AuthSource:    "admin",
				AuthMechanism: "SCRAM-SHA-256",
				Username:      user_,
				Password:      password_,
			},
		)

		client, err = mongo.NewClient(clientOpts)
		if err != nil {
			fmt.Println("fail to create client connection with error: ", err)
			return err
		}
	}

	err := client.Connect(ctx)
	if err != nil {
		fmt.Println("fail to connect with error: ", err)
		return err
	}

	store.clients[connectionId] = client

	return nil
}

func (store *MongoStore) Disconnect(connectionId string) error {
	if store.clients[connectionId] == nil {
		return errors.New("no connection found with name " + connectionId)
	}

	// Close the conncetion
	err := store.clients[connectionId].Disconnect(context.Background())
	// remove it from the map.
	delete(store.clients, connectionId)

	return err
}

func (store *MongoStore) GetStoreType() string {
	return "MONGO"
}

/**
 * Return the nil on success.
 */
func (store *MongoStore) Ping(ctx context.Context, connectionId string) error {
	if store.clients[connectionId] == nil {
		return errors.New("no connection found with name " + connectionId)
	}

	return store.clients[connectionId].Ping(ctx, nil)
}

/**
 * return the number of entry in a table.
 */
func (store *MongoStore) Count(ctx context.Context, connectionId string, database string, collection string, query string, optionsStr string) (int64, error) {
	if store.clients[connectionId] == nil {
		return -1, errors.New("no connection found with name " + connectionId)
	}

	var opts []*options.CountOptions
	if len(optionsStr) > 0 {
		opts = make([]*options.CountOptions, 0)
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
	return errors.New("MongoDb will create your database at first insert.")
}

/**
 * Delete a database
 */
func (store *MongoStore) DeleteDatabase(ctx context.Context, connectionId string, name string) error {
	if store.clients[connectionId] == nil {
		return errors.New("no connection found with name " + connectionId)
	}

	return store.clients[connectionId].Database(name).Drop(ctx)
}

/**
 * Create a Collection
 */
func (store *MongoStore) CreateCollection(ctx context.Context, connectionId string, database string, name string, optionsStr string) error {
	if store.clients[connectionId] == nil {
		return errors.New("no connection found with name " + connectionId)
	}

	db := store.clients[connectionId].Database(database)
	if db == nil {
		return errors.New("Database " + database + " dosen't exist!")
	}

	var opts []*options.CollectionOptions
	if len(optionsStr) > 0 {
		opts = make([]*options.CollectionOptions, 0)
		err := json.Unmarshal([]byte(optionsStr), &opts)
		if err != nil {
			return err
		}
	}

	collection := db.Collection(name, opts...)
	if collection == nil {
		return errors.New("fail to create collection " + name)
	}

	return nil
}

/**
 * Delete collection
 */
func (store *MongoStore) DeleteCollection(ctx context.Context, connectionId string, database string, name string) error {
	if store.clients[connectionId] == nil {
		return errors.New("no connection found with name " + connectionId)
	}

	err := store.clients[connectionId].Database(database).Collection(name).Drop(ctx)
	return err
}

//////////////////////////////////////////////////////////////////////////////////
// Insert
//////////////////////////////////////////////////////////////////////////////////
/**
 * Insert one value in the store.
 */
func (store *MongoStore) InsertOne(ctx context.Context, connectionId string, database string, collection string, entity interface{}, optionsStr string) (interface{}, error) {
	if store.clients[connectionId] == nil {
		return nil, errors.New("no connection found with name " + connectionId)
	}

	var opts []*options.InsertOneOptions
	if len(optionsStr) > 0 {
		opts = make([]*options.InsertOneOptions, 0)
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
 * Insert many results at time.
 */
func (store *MongoStore) InsertMany(ctx context.Context, connectionId string, database string, collection string, entities []interface{}, optionsStr string) ([]interface{}, error) {
	if store.clients[connectionId] == nil {
		return nil, errors.New("no connection found with name " + connectionId)
	}

	var opts []*options.InsertManyOptions
	if len(optionsStr) > 0 {
		opts = make([]*options.InsertManyOptions, 0)
		err := json.Unmarshal([]byte(optionsStr), &opts)
		if err != nil {
			return nil, err
		}
	}

	// Get the collection object.
	collection_ := store.clients[connectionId].Database(database).Collection(collection)

	// return store.clients[connectionId].Ping(ctx, nil)
	insertManyResult, err := collection_.InsertMany(ctx, entities, opts...)
	if err != nil {
		return nil, err
	}

	return insertManyResult.InsertedIDs, nil
}

//////////////////////////////////////////////////////////////////////////////////
// Read
//////////////////////////////////////////////////////////////////////////////////

/**
 * Find many values from a query
 */
func (store *MongoStore) Find(ctx context.Context, connectionId string, database string, collection string, query string, optionsStr string) ([]interface{}, error) {
	if store.clients[connectionId] == nil {
		return nil, errors.New("no connection found with name " + connectionId)
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
		opts = make([]*options.FindOptions, 0)
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
		// In that case I will return the whole entity
		results = append(results, entity)
	}

	// In case of error
	if err := cur.Err(); err != nil {
		return results, err
	}

	return results, nil
}

/**
 * Aggregate result from a collection.
 */
func (store *MongoStore) Aggregate(ctx context.Context, connectionId string, database string, collection string, pipeline string, optionsStr string) ([]interface{}, error) {
	if store.clients[connectionId] == nil {
		return nil, errors.New("no connection found with name " + connectionId)
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
		opts = make([]*options.AggregateOptions, 0)
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
		// In that case I will return the whole entity
		results = append(results, entity)
	}

	// In case of error
	if err := cur.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

/**
 * Find one result at time.
 */
func (store *MongoStore) FindOne(ctx context.Context, connectionId string, database string, collection string, query string, optionsStr string) (interface{}, error) {
	if store.clients[connectionId] == nil {
		return nil, errors.New("no connection found with name " + connectionId)
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
		opts = make([]*options.FindOneOptions, 0)
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

//////////////////////////////////////////////////////////////////////////////////
// Update
//////////////////////////////////////////////////////////////////////////////////

/**
 * Update one or more value that match the query.
 */
func (store *MongoStore) Update(ctx context.Context, connectionId string, database string, collection string, query string, value string, optionsStr string) error {
	if store.clients[connectionId] == nil {
		return errors.New("no connection found with name " + connectionId)
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
		opts = make([]*options.UpdateOptions, 0)
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
 * Update one document at time
 */
func (store *MongoStore) UpdateOne(ctx context.Context, connectionId string, database string, collection string, query string, value string, optionsStr string) error {

	if store.clients[connectionId] == nil {
		return errors.New("no connection found with name " + connectionId)
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
		opts = make([]*options.UpdateOptions, 0)
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
		return errors.New("no connection found with name " + connectionId)
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
		opts = make([]*options.ReplaceOptions, 0)
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

//////////////////////////////////////////////////////////////////////////////////
// Delete
//////////////////////////////////////////////////////////////////////////////////

/**
 * Remove one or more value depending of the query results.
 */
func (store *MongoStore) Delete(ctx context.Context, connectionId string, database string, collection string, query string, optionsStr string) error {
	if store.clients[connectionId] == nil {
		return errors.New("no connection found with name " + connectionId)
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
		opts = make([]*options.DeleteOptions, 0)
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
 * Remove one document at time
 */
func (store *MongoStore) DeleteOne(ctx context.Context, connectionId string, database string, collection string, query string, optionsStr string) error {

	if store.clients[connectionId] == nil {
		return errors.New("no connection found with name " + connectionId)
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
		opts = make([]*options.DeleteOptions, 0)
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
 * Create a user. optionaly assing it a role.
 * roles ex. [{ role: "myReadOnlyRole", db: "mytest"}]
 */
func (store *MongoStore) RunAdminCmd(ctx context.Context, connectionId string, user string, password string, script string) error {
	// Here I will retreive the path of the mondod and use it to find the mongo command.

	if store.clients[connectionId] == nil {
		return errors.New("no connection found with name " + connectionId)
	}

	cmd := "mongosh" // now mongos since version 6
	args := make([]string, 0)

	// if the command need authentication.
	if len(user) > 0 {
		args = append(args, "-u")
		args = append(args, user)
		args = append(args, "-p")
		args = append(args, password)
		args = append(args, "--port")
		args = append(args, Utility.ToString(store.Port))
		args = append(args, "--host")
		args = append(args, "0.0.0.0")
		args = append(args, "--authenticationDatabase")
		args = append(args, "admin")
	}

	args = append(args, "--eval")
	args = append(args, script)

	cmd_ := exec.Command(cmd, args...)
	cmd_.Dir = os.TempDir()

	err := cmd_.Run()

	return err
}


/**
 * Create a role, privilege is a json sring describing the privilege.
 * privileges ex. [{ resource: { db: "mytest", collection: "col2"}, actions: ["find"]}], roles: []}
 */
func (store *MongoStore) CreateRole(ctx context.Context, connectionId string, role string, privileges string, options string) error {
	if store.clients[connectionId] == nil {
		return errors.New("no connection found with name " + connectionId)
	}

	return nil
}

/** Stop mongod process **/
func (store *MongoStore) stopMongod() error {
	fmt.Println("try to stop mongoDB instance")
	pids, err := Utility.GetProcessIdsByName("mongod")
	if err == nil {
		if len(pids) == 0 {
			fmt.Println("no mongoDB instance found")
			return nil
		}
	} else {
		fmt.Println("fail to retreive mongoDB instance with error ", err)
		return nil
	}

	cmd := "mongosh"
	closeCmd := exec.Command(cmd, "--host", "0.0.0.0", "--port", Utility.ToString(store.Port), "--eval", "db=db.getSiblingDB('admin');db.adminCommand( { shutdown: 1 } );")
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

/** Create the super administrator in the db. not the sa globular account!!! **/
func (store *MongoStore) registerSa() error {

	// Here I will test if mongo db exist on the store.
	wait := make(chan error)
	Utility.RunCmd("mongod", os.TempDir(), []string{"--version"}, wait)
	err := <-wait
	if err != nil {
		return err
	}

	// Here I will create super admin if it not already exist.
	dataPath := store.DataPath + "/mongodb-data"

	fmt.Println("test if path ", dataPath, "exist")
	if !Utility.Exists(dataPath) {

		// Kill mongo db store if the process already run...
		fmt.Println("No mongoDB database exist, I will create one")

		fmt.Println("stop existing mongoDB instance")
		err := store.stopMongod()
		if err != nil {
			fmt.Println("fail to stop current mongodb server instance with error: ", err)
			return err
		}

		// Here I will create the directory
		err = os.MkdirAll(dataPath, os.ModeDir)
		if err != nil {
			fmt.Println("fail to create data `"+dataPath+"` dir with error: ", err)
			return err
		}

		// wait until mongodb stop.
		fmt.Println("start mongoDB without authentication")
		go startMongoDB(store.Port, dataPath, false, wait)

		err = <-wait
		fmt.Println("830")

		if err != nil {
			fmt.Println("fail to start mongoDB with error: ", err)
			return err
		}

		fmt.Println("create sa user and set authentication parameters")

		// Now I will create a new user name sa and give it all admin write.
		createSaScript := fmt.Sprintf(
			`db=db.getSiblingDB('admin');db.createUser({ user: '%s', pwd: '%s', roles: ['userAdminAnyDatabase','userAdmin','readWrite','dbAdmin','clusterAdmin','readWriteAnyDatabase','dbAdminAnyDatabase']});`, store.User, store.Password) // must be change...

		Utility.RunCmd("mongosh", os.TempDir(), []string{"--host", "0.0.0.0", "--port", Utility.ToString(store.Port), "--eval", createSaScript}, wait)
		err = <-wait

		if err != nil {
			// remove the mongodb-data
			fmt.Println("839 fail to run command mogosh with error: ", err)
			os.RemoveAll(dataPath)
			return err
		}

		fmt.Println("stop mongoDB instance instance and restart it with authentication")
		err = store.stopMongod()
		if err != nil {
			fmt.Println("847 fail to stop current mongodb server instance with error: ", err)
			return err
		}
	}

	fmt.Println("start mongoDB server with authentication")

	// wait until mongo was started...
	go startMongoDB(store.Port, dataPath, true, wait)
	err = <-wait
	if err != nil {
		fmt.Println("fail start mongoDB with error: ", err)
		return err
	}

	// Get the list of all services method.
	return nil
}

func startMongoDB(port int, dataPath string, withAuth bool, wait chan error) error {

	fmt.Println("try to start mongoDB at port:", port, "data path:", dataPath)
	pids, _ := Utility.GetProcessIdsByName("mongod")
	if pids != nil {
		if len(pids) > 0 {
			wait <- nil
			fmt.Println("mondoDB already running with pid ", pids)
			return nil // mongod already running
		}
	}

	baseCmd := "mongod"
	cmdArgs := []string{"--port", Utility.ToString(port), "--bind_ip", "0.0.0.0", "--dbpath", dataPath}

	if withAuth {
		cmdArgs = append([]string{"--auth"}, cmdArgs...)
	}

	cmd := exec.Command(baseCmd, cmdArgs...)
	cmd.Dir = filepath.Dir(dataPath)

	pid := -1

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("fail to connect stdout with error: ", err)
		return err
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output := make(chan string)
	done := make(chan bool)

	// Process message util the command is done.
	go func() {
		for {
			select {
			case <-done:
				fmt.Println("mongod process terminate...")
				wait <- nil // unblock it...
				break

			case result := <-output:
				if cmd.Process != nil {
					pid = cmd.Process.Pid
				}

				if withAuth {
					if strings.Contains(result, "Waiting for connections") {
						fmt.Println("wait for mongo to start...")
						time.Sleep(time.Second * 1)
						fmt.Println("mongoDB is running with pid: ", pid)
						wait <- nil // unblock it...
					}
				} else {
					if strings.Contains(result, "Operating System") {
						fmt.Println("wait for mongo to start...")
						time.Sleep(time.Second * 5)

						fmt.Println("mongoDB is running with pid: ", pid)

						wait <- nil // unblock it...
					}
				}
				//fmt.Println("mongod:",pid, result)
			}
		}
	}()

	// Start reading the output
	go Utility.ReadOutput(output, stdout)
	err = cmd.Run()
	if err != nil {
		fmt.Println("fail to run mongod with error ", fmt.Sprint(err)+": "+stderr.String())
		fmt.Println("mongod", "--port", Utility.ToString(port), "--bind_ip", "0.0.0.0", "--dbpath", dataPath)
		return errors.New(fmt.Sprint(err) + ": " + stderr.String())
	}

	cmd.Wait()

	// Close the output.
	stdout.Close()
	done <- true

	return nil
}
