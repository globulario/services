package persistence_store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/gocql/gocql"
)

// Connection represent a connection to a SQL database.
type ScyllaConnection struct {
	Id       string
	Host     string
	Token    string
	sessions map[string]*gocql.Session
}

/**
 * The ScyllaDB store.
 */
type ScyllaStore struct {

	/** the connections... */
	connections map[string]*ScyllaConnection
}

func (store *ScyllaStore) GetStoreType() string {
	return "SCYLLADB"
}

/**
 * Connect to the database.
 */
func (store *ScyllaStore) Connect(id string, host string, port int32, user string, password string, keyspace string, timeout int32, options_str string) error {

	if len(id) == 0 {
		return errors.New("the connection id is required")
	}

	if store.connections != nil {
		if _, ok := store.connections[id]; ok {
			if store.connections[id].sessions != nil {
				if _, ok := store.connections[id].sessions[keyspace]; ok {
					return nil
				}
			}
		}
	} else {
		store.connections = make(map[string]*ScyllaConnection)
	}

	if len(host) == 0 {
		return errors.New("the host is required")
	}

	if len(user) == 0 {
		return errors.New("the user is required")

	}

	if len(password) == 0 {
		return errors.New("the password is required")
	}

	if len(keyspace) == 0 {
		return errors.New("the database is required")
	}

	// So here I will authenticate the user and password.
	authentication_client, err := authentication_client.NewAuthenticationService_Client(host, "authentication.AuthenticationService")
	if err != nil {
		return err
	}

	// Authenticate the user, I will try 5 times.
	nbTry := 5
	var token string
	for nbTry > 0 {

		var err error
		// Authenticate the user.
		token, err = authentication_client.Authenticate(user, password)
		if err != nil && nbTry == 0 {
			fmt.Println("Fail to authenticate user ", user, err)
			return err
		} else if err == nil {
			break
		}

		time.Sleep(1 * time.Second)
	}

	// Create the cluster
	cluster := gocql.NewCluster(host)
	cluster.Port = int(port)
	cluster.Keyspace = keyspace
	cluster.Timeout = time.Duration(timeout) * time.Second
	
	// Create the session
	session, err := cluster.CreateSession()
	if err != nil {
		return err
	}

	var connection *ScyllaConnection
	if _, ok := store.connections[id]; ok {
		connection = store.connections[id]
	} else {
		// Now I will save the connection.
		connection = &ScyllaConnection{
			Id:       id,
			Host:     host,
			Token:    token,
			sessions: make(map[string]*gocql.Session),
		}
	}

	// Save the session for that keyspace.
	connection.sessions[keyspace] = session

	return nil
}

/**
 * Disconnect from the database.
 */
func (store *ScyllaStore) Disconnect(connectionId string) error {
	// close all sessions for that connection.
	if store.connections != nil {
		if _, ok := store.connections[connectionId]; ok {
			if store.connections[connectionId].sessions != nil {
				for _, session := range store.connections[connectionId].sessions {
					session.Close()
				}
			}
		}
	}
	return nil
}

/**
 * Ping the database.
 */
func (store *ScyllaStore) Ping(ctx context.Context, connectionId string) error {
	
	// Get the connection.
	connection := store.connections[connectionId]
	if connection == nil {
		return errors.New("the connection does not exist")
	}

	// Get the first found session.
	var session *gocql.Session
	for _, s := range connection.sessions {
		session = s
		break
	}

	if session == nil {
		return errors.New("the session does not exist")
	}

	 // Execute a simple query to check connectivity
	 if err := session.Query("SELECT release_version FROM system.local").Exec(); err != nil {
        fmt.Printf("Failed to execute query: %v\n", err)
        return err
    }

    fmt.Println("ScyllaDB cluster is up and running!")
	return nil
}

func (store *ScyllaStore) CreateDatabase(ctx context.Context, connectionId string, keyspace string) error {
	return errors.New("not implemented")
}

func (store *ScyllaStore) DeleteDatabase(ctx context.Context, connectionId string, keyspace string) error {
	return errors.New("not implemented")
}

func (store *ScyllaStore) Count(ctx context.Context, connectionId string, keyspace string, table string, query string, options string) (int64, error) {
	return 0, errors.New("not implemented")
}

func (store *ScyllaStore) InsertOne(ctx context.Context, connectionId string, keyspace string, table string, entity interface{}, options string) (interface{}, error) {
	return nil, errors.New("not implemented")
}

func (store *ScyllaStore) FindOne(ctx context.Context, connectionId string, keyspace string, table string, query string, options string) (interface{}, error) {
	return nil, errors.New("not implemented")
}

func (store *ScyllaStore) Find(ctx context.Context, connectionId string, keyspace string, table string, query string, options string) ([]interface{}, error) {
	return nil, errors.New("not implemented")
}

func (store *ScyllaStore) ReplaceOne(ctx context.Context, connectionId string, keyspace string, table string, query string, value string, options string) error {
	return errors.New("not implemented")
}

func (store *ScyllaStore) Update(ctx context.Context, connectionId string, keyspace string, table string, query string, value string, options string) error {
	return errors.New("not implemented")
}

func (store *ScyllaStore) UpdateOne(ctx context.Context, connectionId string, keyspace string, table string, query string, value string, options string) error {
	return errors.New("not implemented")
}

func (store *ScyllaStore) Delete(ctx context.Context, connectionId string, keyspace string, table string, query string, options string) error {
	return errors.New("not implemented")
}

func (store *ScyllaStore) DeleteOne(ctx context.Context, connectionId string, keyspace string, table string, query string, options string) error {
	return errors.New("not implemented")
}

func (store *ScyllaStore) InsertMany(ctx context.Context, connectionId string, keyspace string, table string, entities []interface{}, options string) ([]interface{}, error) {
	return nil, errors.New("not implemented")
}

func (store *ScyllaStore) Aggregate(ctx context.Context, connectionId string, keyspace string, table string, pipeline string, optionsStr string) ([]interface{}, error) {
	return nil, errors.New("not implemented")
}

func (store *ScyllaStore) CreateCollection(ctx context.Context, connectionId string, keyspace string, collection string, options string) error {
	return errors.New("not implemented")
}

func (store *ScyllaStore) DeleteCollection(ctx context.Context, connectionId string, keyspace string, collection string) error {
	return errors.New("not implemented")
}

func (store *ScyllaStore) RunAdminCmd(ctx context.Context, connectionId string, user string, password string, script string) error {
	return errors.New("not implemented")
}
