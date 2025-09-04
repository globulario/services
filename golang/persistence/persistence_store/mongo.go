package persistence_store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
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
 * Supports:
 *   - optionsStr as a full MongoDB URI (e.g., mongodb://user:pass@host:port/?authSource=admin)
 *   - or default credential-based connection using host, port, user, password.
 */
func (store *MongoStore) Connect(connectionId string, host string, port int32, user string, password string, database string, timeout int32, optionsStr string) error {
	if timeout == 0 {
		timeout = 500 // Default timeout value (ms).
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Millisecond)
	defer cancel()

	if store.clients == nil {
		store.clients = make(map[string]*mongo.Client)
	} else if cl := store.clients[connectionId]; cl != nil {
		// If already connected and reachable, no-op.
		if err := cl.Ping(ctx, nil); err == nil {
			return nil
		}
	}

	var (
		client *mongo.Client
		err    error
	)

	// Prefer URI if supplied in optionsStr.
	if strings.HasPrefix(optionsStr, "mongodb://") || strings.HasPrefix(optionsStr, "mongodb+srv://") {
		clientOpts := options.Client().ApplyURI(optionsStr)
		client, err = mongo.Connect(ctx, clientOpts)
		if err != nil {
			slog.Error("mongo connect via URI failed", "id", connectionId, "err", err)
			return err
		}
	} else {
		// Build credentialed client options.
		// NOTE: Credentials expect raw strings, no need to URL-escape.
		userEsc := user
		passEsc := password

		clientOpts := options.Client().
			SetHosts([]string{fmt.Sprintf("%s:%d", host, port)}).
			SetAuth(options.Credential{
				AuthSource:    "admin",
				AuthMechanism: "SCRAM-SHA-256",
				Username:      userEsc,
				Password:      passEsc,
			})

		client, err = mongo.Connect(ctx, clientOpts)
		if err != nil {
			slog.Error("mongo connect failed", "id", connectionId, "host", host, "port", port, "err", err)
			return err
		}
	}

	// Verify connectivity now.
	if pingErr := client.Ping(ctx, nil); pingErr != nil {
		_ = client.Disconnect(context.Background())
		slog.Error("mongo ping failed after connect", "id", connectionId, "err", pingErr)
		return pingErr
	}

	store.clients[connectionId] = client
	slog.Info("mongo connected", "id", connectionId, "host", host, "port", port)
	return nil
}

func (store *MongoStore) Disconnect(connectionId string) error {
	if store.clients[connectionId] == nil {
		return errors.New("No connection found with name " + connectionId)
	}
	err := store.clients[connectionId].Disconnect(context.Background())
	delete(store.clients, connectionId)
	if err != nil {
		slog.Error("mongo disconnect failed", "id", connectionId, "err", err)
	} else {
		slog.Info("mongo disconnected", "id", connectionId)
	}
	return err
}

func (store *MongoStore) GetStoreType() string { return "MONGO" }

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
		if err := json.Unmarshal([]byte(optionsStr), &opts); err != nil {
			return 0, err
		}
	}

	q := make(map[string]interface{})
	if err := json.Unmarshal([]byte(query), &q); err != nil {
		return 0, err
	}

	count, err := store.clients[connectionId].Database(database).Collection(collection).CountDocuments(ctx, q, opts...)
	return count, err
}

func (store *MongoStore) CreateDatabase(ctx context.Context, connectionId string, name string) error {
	// MongoDB creates databases lazily on first write.
	return errors.New("database will be created at the first insert")
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

	var opts []*options.CreateCollectionOptions
	if len(optionsStr) > 0 {
		if err := json.Unmarshal([]byte(optionsStr), &opts); err != nil {
			return err
		}
	}

	if err := store.clients[connectionId].Database(database).CreateCollection(ctx, name, opts...); err != nil {
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
	return store.clients[connectionId].Database(database).Collection(name).Drop(ctx)
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
		if err := json.Unmarshal([]byte(optionsStr), &opts); err != nil {
			return nil, err
		}
	}

	coll := store.clients[connectionId].Database(database).Collection(collection)
	res, err := coll.InsertOne(ctx, entity, opts...)
	if err != nil {
		return nil, err
	}
	return res.InsertedID, nil
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
		if err := json.Unmarshal([]byte(optionsStr), &opts); err != nil {
			return nil, err
		}
	}

	coll := store.clients[connectionId].Database(database).Collection(collection)
	res, err := coll.InsertMany(ctx, entities, opts...)
	if err != nil {
		return nil, err
	}
	return res.InsertedIDs, nil
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

	coll := store.clients[connectionId].Database(database).Collection(collection)

	q := make(map[string]interface{})
	if err := json.Unmarshal([]byte(query), &q); err != nil {
		return nil, err
	}

	var opts []*options.FindOptions
	if len(optionsStr) > 0 {
		if err := json.Unmarshal([]byte(optionsStr), &opts); err != nil {
			return nil, err
		}
	}

	cur, err := coll.Find(ctx, q, opts...)
	if err != nil {
		return nil, err
	}
	defer cur.Close(context.Background())

	results := make([]interface{}, 0)
	for cur.Next(ctx) {
		entity := make(map[string]interface{})
		if err := cur.Decode(&entity); err != nil {
			return nil, err
		}
		results = append(results, entity)
	}
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

	coll := store.clients[connectionId].Database(database).Collection(collection)

	p := make([]interface{}, 0)
	if err := json.Unmarshal([]byte(pipeline), &p); err != nil {
		return nil, err
	}

	var opts []*options.AggregateOptions
	if len(optionsStr) > 0 {
		if err := json.Unmarshal([]byte(optionsStr), &opts); err != nil {
			return nil, err
		}
	}

	cur, err := coll.Aggregate(ctx, p, opts...)
	if err != nil {
		return nil, err
	}
	defer cur.Close(context.Background())

	results := make([]interface{}, 0)
	for cur.Next(ctx) {
		entity := make(map[string]interface{})
		if err := cur.Decode(&entity); err != nil {
			return nil, err
		}
		results = append(results, entity)
	}
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

	coll := store.clients[connectionId].Database(database).Collection(collection)

	q := make(map[string]interface{})
	if err := json.Unmarshal([]byte(query), &q); err != nil {
		return nil, err
	}

	var opts []*options.FindOneOptions
	if len(optionsStr) > 0 {
		if err := json.Unmarshal([]byte(optionsStr), &opts); err != nil {
			return nil, err
		}
	}

	entity := make(map[string]interface{})
	if err := coll.FindOne(ctx, q, opts...).Decode(&entity); err != nil {
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

	coll := store.clients[connectionId].Database(database).Collection(collection)

	q := make(map[string]interface{})
	if err := json.Unmarshal([]byte(query), &q); err != nil {
		return err
	}

	// Update document (must contain update operators like $set).
	var v bson.D
	if err := bson.UnmarshalExtJSON([]byte(value), true, &v); err != nil {
		return err
	}

	var opts []*options.UpdateOptions
	if len(optionsStr) > 0 {
		if err := json.Unmarshal([]byte(optionsStr), &opts); err != nil {
			return err
		}
	}

	_, err := coll.UpdateMany(ctx, q, v, opts...)
	return err
}

/**
 * Update one document at a time.
 */
func (store *MongoStore) UpdateOne(ctx context.Context, connectionId string, database string, collection string, query string, value string, optionsStr string) error {
	if store.clients[connectionId] == nil {
		return errors.New("No connection found with name " + connectionId)
	}

	coll := store.clients[connectionId].Database(database).Collection(collection)

	q := make(map[string]interface{})
	if err := json.Unmarshal([]byte(query), &q); err != nil {
		return err
	}

	var v bson.D
	if err := bson.UnmarshalExtJSON([]byte(value), true, &v); err != nil {
		return err
	}

	var opts []*options.UpdateOptions
	if len(optionsStr) > 0 {
		if err := json.Unmarshal([]byte(optionsStr), &opts); err != nil {
			return err
		}
	}

	_, err := coll.UpdateOne(ctx, q, v, opts...)
	return err
}

/**
 * Replace a document by another.
 */
func (store *MongoStore) ReplaceOne(ctx context.Context, connectionId string, database string, collection string, query string, value string, optionsStr string) error {
	if store.clients[connectionId] == nil {
		return errors.New("No connection found with name " + connectionId)
	}

	coll := store.clients[connectionId].Database(database).Collection(collection)

	q := make(map[string]interface{})
	if err := json.Unmarshal([]byte(query), &q); err != nil {
		return err
	}

	// Replacement document (no operators).
	var v bson.D
	if err := bson.UnmarshalExtJSON([]byte(value), true, &v); err != nil {
		return err
	}

	var opts []*options.ReplaceOptions
	if len(optionsStr) > 0 {
		if err := json.Unmarshal([]byte(optionsStr), &opts); err != nil {
			return err
		}
	}

	_, err := coll.ReplaceOne(ctx, q, v, opts...)
	return err
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

	coll := store.clients[connectionId].Database(database).Collection(collection)

	q := make(map[string]interface{})
	if err := json.Unmarshal([]byte(query), &q); err != nil {
		return err
	}

	var opts []*options.DeleteOptions
	if len(optionsStr) > 0 {
		if err := json.Unmarshal([]byte(optionsStr), &opts); err != nil {
			return err
		}
	}

	_, err := coll.DeleteMany(ctx, q, opts...)
	return err
}

/**
 * Remove one document at a time.
 */
func (store *MongoStore) DeleteOne(ctx context.Context, connectionId string, database string, collection string, query string, optionsStr string) error {
	if store.clients[connectionId] == nil {
		return errors.New("No connection found with name " + connectionId)
	}

	coll := store.clients[connectionId].Database(database).Collection(collection)

	q := make(map[string]interface{})
	if err := json.Unmarshal([]byte(query), &q); err != nil {
		return err
	}

	var opts []*options.DeleteOptions
	if len(optionsStr) > 0 {
		if err := json.Unmarshal([]byte(optionsStr), &opts); err != nil {
			return err
		}
	}

	_, err := coll.DeleteOne(ctx, q, opts...)
	return err
}

/**
 * Create a user. Optionally assign it a role.
 * Roles example: [{ role: "myReadOnlyRole", db: "mytest"}]
 */
func (store *MongoStore) RunAdminCmd(ctx context.Context, connectionId string, user string, password string, script string) error {
	if store.clients[connectionId] == nil {
		return errors.New("No connection found with name " + connectionId)
	}

	cmd := "mongosh" // mongosh since MongoDB 6+
	args := []string{"--host", "0.0.0.0", "--port", strconv.Itoa(store.Port)}

	// If the command needs authentication.
	if len(user) > 0 {
		args = append(args, "-u", user, "-p", password, "--authenticationDatabase", "admin")
	}

	args = append(args, "--eval", script)

	cmd_ := exec.Command(cmd, args...)
	cmd_.Dir = os.TempDir()

	err := cmd_.Run()
	if err != nil {
		slog.Error("mongosh admin cmd failed", "script", script, "err", err)
	}
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
	// Not implemented; keep signature.
	return nil
}

/////////////////
// MongoDB management (start/stop, SA creation).
// /**
//  * Stop MongoDB instance.
//  */
// func (store *MongoStore) stopMongod() error {
// 	pids, err := Utility.GetProcessIdsByName("mongod")
// 	if err != nil {
// 		slog.Error("failed to get mongod pids", "err", err)
// 		return nil
// 	}
// 	if len(pids) == 0 {
// 		slog.Info("no mongod instance found")
// 		return nil
// 	}

// 	closeCmd := exec.Command("mongosh", "--host", "0.0.0.0", "--port", strconv.Itoa(store.Port), "--eval", "db=db.getSiblingDB('admin');db.adminCommand({ shutdown: 1 });")
// 	closeCmd.Dir = os.TempDir()

// 	if err := closeCmd.Run(); err != nil {
// 		pids, _ := Utility.GetProcessIdsByName("mongod")
// 		if len(pids) == 0 {
// 			return nil
// 		}
// 		return err
// 	}
// 	return nil
// }

// /**
//  * Create the super administrator in the DB. Not the SA global account!!!
//  */
// func (store *MongoStore) registerSa() error {
// 	// Validate mongod availability
// 	wait := make(chan error)
// 	Utility.RunCmd("mongod", os.TempDir(), []string{"--version"}, wait)
// 	if err := <-wait; err != nil {
// 		return err
// 	}

// 	dataPath := store.DataPath + "/mongodb-data"

// 	if !Utility.Exists(dataPath) {
// 		// Ensure no running mongod.
// 		if err := store.stopMongod(); err != nil {
// 			slog.Error("failed to stop mongod", "err", err)
// 			return err
// 		}

// 		// Create data dir.
// 		if err := os.MkdirAll(dataPath, os.ModeDir); err != nil {
// 			slog.Error("failed to create data dir", "path", dataPath, "err", err)
// 			return err
// 		}

// 		// Start without auth to create SA.
// 		go startMongoDB(store.Port, dataPath, false, wait)
// 		if err := <-wait; err != nil {
// 			slog.Error("failed to start mongod (no auth)", "err", err)
// 			return err
// 		}

// 		createSaScript := fmt.Sprintf(
// 			`db=db.getSiblingDB('admin');db.createUser({ user: '%s', pwd: '%s', roles: ['userAdminAnyDatabase','userAdmin','readWrite','dbAdmin','clusterAdmin','readWriteAnyDatabase','dbAdminAnyDatabase']});`,
// 			store.User, store.Password,
// 		)

// 		Utility.RunCmd("mongosh", os.TempDir(), []string{"--host", "0.0.0.0", "--port", strconv.Itoa(store.Port), "--eval", createSaScript}, wait)
// 		if err := <-wait; err != nil {
// 			slog.Error("failed to create SA user", "err", err)
// 			_ = os.RemoveAll(dataPath)
// 			return err
// 		}

// 		if err := store.stopMongod(); err != nil {
// 			slog.Error("failed to stop mongod after SA creation", "err", err)
// 			return err
// 		}
// 	}

// 	// Start with auth.
// 	go startMongoDB(store.Port, dataPath, true, wait)
// 	if err := <-wait; err != nil {
// 		slog.Error("failed to start mongod (auth)", "err", err)
// 		return err
// 	}

// 	return nil
// }

// func startMongoDB(port int, dataPath string, withAuth bool, wait chan error) error {
// 	pids, _ := Utility.GetProcessIdsByName("mongod")
// 	if len(pids) > 0 {
// 		wait <- nil
// 		return nil // MongoDB already running
// 	}

// 	cmdArgs := []string{"--port", strconv.Itoa(port), "--bind_ip", "0.0.0.0", "--dbpath", dataPath}
// 	if withAuth {
// 		cmdArgs = append([]string{"--auth"}, cmdArgs...)
// 	}

// 	cmd := exec.Command("mongod", cmdArgs...)
// 	cmd.Dir = filepath.Dir(dataPath)

// 	pid := -1

// 	stdout, err := cmd.StdoutPipe()
// 	if err != nil {
// 		slog.Error("mongod stdout pipe failed", "err", err)
// 		return err
// 	}

// 	var stderr bytes.Buffer
// 	cmd.Stderr = &stderr

// 	output := make(chan string)
// 	done := make(chan bool)

// 	// Process message until the command is done.
// 	go func() {
// 		for {
// 			select {
// 			case <-done:
// 				wait <- nil
// 				return
// 			case result := <-output:
// 				if cmd.Process != nil {
// 					pid = cmd.Process.Pid
// 				}
// 				if withAuth {
// 					if strings.Contains(result, "Waiting for connections") {
// 						slog.Info("mongod started (auth)", "pid", pid)
// 						time.Sleep(time.Second)
// 						wait <- nil
// 					}
// 				} else {
// 					if strings.Contains(result, "Operating System") {
// 						slog.Info("mongod starting (no auth)", "pid", pid)
// 						time.Sleep(5 * time.Second)
// 						wait <- nil
// 					}
// 				}
// 			}
// 		}
// 	}()

// 	// Start reading the output
// 	go Utility.ReadOutput(output, stdout)
// 	if err := cmd.Run(); err != nil {
// 		slog.Error("mongod run failed", "err", fmt.Sprint(err)+": "+stderr.String(),
// 			"args", strings.Join(cmdArgs, " "))
// 		return errors.New(fmt.Sprint(err) + ": " + stderr.String())
// 	}

// 	_ = cmd.Wait()
// 	_ = stdout.Close()
// 	done <- true
// 	return nil
// }
