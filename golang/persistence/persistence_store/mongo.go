package persistence_store

import (
	"context"
	"strconv"

	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"net/url"

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
}

/**
 * Connect to the remote/local mongo store
 * TODO add more connection options via the option_str and options package.
 */
func (store *MongoStore) Connect(connectionId string, host string, port int32, user string, password string, database string, timeout int32, optionsStr string) error {

	ctx := context.Background()
	//ctx, _ := context.WithTimeout(api.GetClientContext(store), time.Duration(timeout)*time.Second)

	if store.clients == nil {
		store.clients = make(map[string]*mongo.Client)
	} else {
		if store.clients[connectionId] != nil {
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
		connectionStr := "mongodb://" + user_ + ":" + password_ + "@" + host + ":" + strconv.Itoa(int(port)) + "/" + database + "?authSource=admin&compressors=disabled&gssapiServiceName=mongodb&ssl=false"
		var err error
		client, err = mongo.NewClient(options.Client().ApplyURI(connectionStr))
		if err != nil {
			return err
		}
	}

	err := client.Connect(ctx)
	if err != nil {
		return err
	}

	store.clients[connectionId] = client
	fmt.Println("connection is made ", connectionId)
	return nil
}

func (store *MongoStore) Disconnect(connectionId string) error {
	// Close the conncetion
	err := store.clients[connectionId].Disconnect(context.Background())
	// remove it from the map.
	delete(store.clients, connectionId)

	return err
}

func (store *MongoStore) GetStoreType() string {
	return "MONGODB"
}

/**
 * Return the nil on success.
 */
func (store *MongoStore) Ping(ctx context.Context, connectionId string) error {
	return store.clients[connectionId].Ping(ctx, nil)
}

/**
 * return the number of entry in a table.
 */
func (store *MongoStore) Count(ctx context.Context, connectionId string, database string, collection string, query string, optionsStr string) (int64, error) {
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
	return store.clients[connectionId].Database(name).Drop(ctx)
}

/**
 * Create a Collection
 */
func (store *MongoStore) CreateCollection(ctx context.Context, connectionId string, database string, name string, optionsStr string) error {
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
	cmd := "mongo"
	args := make([]string, 0)

	// if the command need authentication.
	if len(user) > 0 {
		args = append(args, "-u")
		args = append(args, user)
		args = append(args, "-p")
		args = append(args, password)
		args = append(args, "--authenticationDatabase")
		args = append(args, "admin")
	}

	args = append(args, "--eval")
	args = append(args, script)

	cmd_ := exec.Command(cmd, args...)
	err := cmd_.Run()

	return err
}

/**
 * Start the datastore.
 */
func (store *MongoStore) Start(user, password string, port int, dataPath string) error {

	// Set store attributes.
	store.User = user
	store.Password = password
	store.Port = port
	store.DataPath = dataPath

	return store.registerSa()
}

/**
 * Stop the datastore.
 */
func (store *MongoStore) Stop() error {
	return store.stopMongod()
}

/**
 * Create a role, privilege is a json sring describing the privilege.
 * privileges ex. [{ resource: { db: "mytest", collection: "col2"}, actions: ["find"]}], roles: []}
 */
func (store *MongoStore) CreateRole(ctx context.Context, connectionId string, role string, privileges string, options string) error {
	return nil
}

/** Stop mongod process **/
func (store *MongoStore) stopMongod() error {
	pids, err := Utility.GetProcessIdsByName("mongod")
	if err == nil {
		if len(pids) == 0 {
			return nil
		}
	}else{
		return nil
	}

	closeCmd := exec.Command("mongo", "--eval", "db=db.getSiblingDB('admin');db.adminCommand( { shutdown: 1 } );")
	return closeCmd.Run()
}

/** Create the super administrator in the db. not the sa globular account!!! **/
func (store *MongoStore) registerSa() error {

	// Here I will test if mongo db exist on the store.
	existMongo := exec.Command("mongod", "--version")
	err := existMongo.Run()
	if err != nil {
		return err
	}

	// Here I will create super admin if it not already exist.
	dataPath := store.DataPath + "/mongodb-data"

	if !Utility.Exists(dataPath) {

		// Kill mongo db store if the process already run...
		err := store.stopMongod()
		if err != nil {
			return err
		}

		// Here I will create the directory
		err = os.MkdirAll(dataPath, os.ModeDir)
		if err != nil {
			return err
		}

		// Now I will start the command
		mongod := exec.Command("mongod", "--port", "27017", "--dbpath", dataPath)
		err = mongod.Start()
		if err != nil {
			return err
		}

		store.waitForMongo(60, false)

		// Now I will create a new user name sa and give it all admin write.
		createSaScript := fmt.Sprintf(
			`db=db.getSiblingDB('admin');db.createUser({ user: '%s', pwd: '%s', roles: ['userAdminAnyDatabase','userAdmin','readWrite','dbAdmin','clusterAdmin','readWriteAnyDatabase','dbAdminAnyDatabase']});`, store.User, store.Password) // must be change...

		createSaCmd := exec.Command("mongo", "--eval", createSaScript)

		err = createSaCmd.Run()
		if err != nil {
			// remove the mongodb-data
			os.RemoveAll(dataPath)
			return err
		}


		store.stopMongod()
	}

	// Now I will start mongod with auth available.
	mongod := exec.Command("mongod", "--auth", "--port", Utility.ToString(store.Port), "--bind_ip", "0.0.0.0", "--dbpath", dataPath)

	err = mongod.Start()
	if err != nil {
		return err
	}

	// wait 15 seconds that the store restart.
	store.waitForMongo(60, true)

	// Get the list of all services method.
	return nil
}

func (store *MongoStore) waitForMongo(timeout int, withAuth bool) error {

	time.Sleep(1 * time.Second)

	if timeout == 0 {
		return errors.New("mongod is not responding")
	}

	args := make([]string, 0)
	if withAuth {
		args = append(args, "-u")
		args = append(args, store.User)
		args = append(args, "-p")
		args = append(args, store.Password)
		args = append(args, "--authenticationDatabase")
		args = append(args, "admin")
	}

	args = append(args, "--eval")
	args = append(args, "db=db.getSiblingDB('admin');db.getMongo().getDBNames()")
	script := exec.Command("mongo", args...)
	err := script.Run()
	fmt.Println("Try to start mongoDB with command: mongo -u " + store.User + " -p " + store.Password + "--authenticationDatabase admin --eval \"db=db.getSiblingDB('admin');db.getMongo().getDBNames()\"")
	if err != nil {
		if timeout == 0 {
			return errors.New("mongod is not responding")
		}
		// call again.
		timeout -= 1
		fmt.Println("wait for mongdb ", timeout)
		return store.waitForMongo(timeout, withAuth)
	}
	// call again.
	pids, err := Utility.GetProcessIdsByName("mongod")
	if pids != nil {
		if len(pids) == 0 {
			timeout -= 1
			fmt.Println("wait for mongdb ", timeout)
		}
	}

	return err
}
