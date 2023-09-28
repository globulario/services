package persistence_store

import (
	"context"
	"errors"
	"time"
	"github.com/gocql/gocql"
)

/**
 * The ScyllaDB store.
 */
 type ScyllaStore struct {
	/** The cluster */
	cluster *gocql.ClusterConfig

	/** The session */
	session *gocql.Session
}

func (store *ScyllaStore) GetStoreType() string {
	return "SCYLLADB"
}

func (store *ScyllaStore) Connect(id string, host string, port int32, user string, password string, keyspace string, timeout int32, options_str string) error {
	// Create the cluster
	cluster := gocql.NewCluster(host)
	cluster.Port = int(port)
	cluster.Keyspace = keyspace
	cluster.Authenticator = gocql.PasswordAuthenticator{
		Username: user,
		Password: password,
	}
	cluster.Timeout = time.Duration(timeout) * time.Second

	// Create the session
	session, err := cluster.CreateSession()
	if err != nil {
		return err
	}

	// Store the session
	store.cluster = cluster
	store.session = session

	return nil
}

func (store *ScyllaStore) Disconnect(connectionId string) error {
	return errors.New("not implemented")
}

func (store *ScyllaStore) Ping(ctx context.Context, connectionId string) error {
	return errors.New("not implemented")
}

func (store *ScyllaStore) CreateDatabase(ctx context.Context, connectionId string, name string) error {
	return errors.New("not implemented")
}

func (store *ScyllaStore) DeleteDatabase(ctx context.Context, connectionId string, name string) error {
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

func (store *ScyllaStore) CreateCollection(ctx context.Context, connectionId string, database string, collection string, options string) error {
	return errors.New("not implemented")
}

func (store *ScyllaStore) DeleteCollection(ctx context.Context, connectionId string, database string, collection string) error {
	return errors.New("not implemented")
}

func (store *ScyllaStore) RunAdminCmd(ctx context.Context, connectionId string, user string, password string, script string) error {
	return errors.New("not implemented")
}

