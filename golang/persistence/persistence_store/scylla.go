package persistence_store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/davecourtois/Utility"
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
	lock        sync.Mutex
}

func (store *ScyllaStore) GetStoreType() string {
	return "SCYLLADB"
}

/**
 * Create a new ScyllaDB store.
 */
func (store *ScyllaStore) createKeyspace(connectionId, keyspace string) (*gocql.ClusterConfig, error) {

	if len(keyspace) == 0 {
		return nil, errors.New("the database is required")
	}

	// Get the connection.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()
	if connection == nil {
		return nil, errors.New("the connection does not exist")

	}

	createKeyspaceQuery := `
	CREATE KEYSPACE IF NOT EXISTS ` + keyspace + `
	WITH replication = {
		'class': 'SimpleStrategy',
		'replication_factor': 1
	}`

	// Create the admin session.
	adminCluster := gocql.NewCluster()                          // Replace with your ScyllaDB cluster IP address
	adminCluster.Hosts = []string{connection.Host, "127.0.0.1"} // add local host as well.
	adminCluster.Keyspace = "system"                            // Use the 'system' keyspace for administrative tasks
	adminSession, err := adminCluster.CreateSession()
	if err != nil {
		return nil, err
	}

	defer adminSession.Close()

	// Create the keyspace.
	if err := adminSession.Query(createKeyspaceQuery).Exec(); err != nil {
		return nil, err
	}

	// The cluster address...
	cluster := gocql.NewCluster() // Set your ScyllaDB cluster address here
	cluster.Keyspace = keyspace   // Set your keyspace name here
	cluster.Consistency = gocql.Quorum
	cluster.Hosts = []string{connection.Host, "127.0.0.1"}
	cluster.Port = 9042

	return cluster, nil
}

/**
 * Connect to the database.
 */
func (store *ScyllaStore) Connect(id string, host string, port int32, user string, password string, keyspace string, timeout int32, options_str string) error {

	if len(id) == 0 {
		return errors.New("the connection id is required")
	}

	if store.connections != nil {
		store.lock.Lock()
		if _, ok := store.connections[id]; ok {
			if store.connections[id].sessions != nil {
				if _, ok := store.connections[id].sessions[keyspace]; ok {
					store.lock.Unlock()
					return nil
				}
			}
		}
		store.lock.Unlock()
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

	// Save the connection.
	store.lock.Lock()
	store.connections[id] = connection
	store.lock.Unlock()

	// Create the cluster.
	cluster, err := store.createKeyspace(id, keyspace)
	if err != nil {
		return err
	}

	session, err := cluster.CreateSession()
	if err != nil {
		return err
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
			store.lock.Lock()
			if store.connections[connectionId].sessions != nil {
				for _, session := range store.connections[connectionId].sessions {
					session.Close()
				}
			}
			store.lock.Unlock()
		}
	}
	return nil
}

/**
 * Ping the database.
 */
func (store *ScyllaStore) Ping(ctx context.Context, connectionId string) error {

	// Get the connection.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()
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

	return nil
}

/**
 * Create a new database (keyspace)
 */
func (store *ScyllaStore) CreateDatabase(ctx context.Context, connectionId string, keyspace string) error {

	if len(keyspace) == 0 {
		return errors.New("the database is required")
	}

	_, err := store.createKeyspace(connectionId, keyspace)
	if err != nil {
		return err
	}

	return nil
}

func camelToSnake(input string) string {
	var result bytes.Buffer
	for i, runeValue := range input {
		if i > 0 && unicode.IsUpper(runeValue) {
			result.WriteRune('_')
		}
		result.WriteRune(unicode.ToLower(runeValue))
	}
	return result.String()
}

func (store *ScyllaStore) isTableExist(connectionId string, keyspace string, table string) bool {
	// I will get the session for that keyspace.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()
	if connection == nil {
		return false
	}

	// Get the first found session.
	session, ok := connection.sessions[keyspace]
	if !ok {
		return false
	}

	// Check if the table exist.
	query := session.Query(`SELECT columnfamily_name FROM system.schema_columnfamilies WHERE keyspace_name = ? AND columnfamily_name = ?`, keyspace, table)
	iter := query.Iter()
	defer iter.Close()

	// Test if the table exist.
	return iter.NumRows() > 0
}

func deduceColumnType(value interface{}) string {

	goType := reflect.TypeOf(value)

	switch goType.Kind() {
	case reflect.Bool:
		return "boolean"
	case reflect.Int64:
		return "bigint"
	case reflect.Int, reflect.Int32:
		return "int"
	case reflect.Float64:
		return "double"
	case reflect.String:
		return "text"
	case reflect.Slice:
		return "array"
	case reflect.Map:
		return "map"
	default:
		//fmt.Println("unsupported data type: %s", goType.String())
		return ""
	}
}

func (store *ScyllaStore) createScyllaTable(session *gocql.Session, keyspace, tableName string, data map[string]interface{}) error {
	// Prepare the CREATE TABLE query
	createTableQuery := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s (", keyspace, tableName)

	// Iterate through the map's fields and deduce the data types
	for fieldName, value := range data {
		fieldType := deduceColumnType(value)
		if fieldType != "unknow" {
			if fieldType != "array" {
				fieldName = camelToSnake(fieldName)
				createTableQuery += fieldName + " " + fieldType + ", "
			}
		}
	}

	// scylla does not support _id, so I will replace it with id.
	createTableQuery = strings.ReplaceAll(createTableQuery, "_id", "id")

	if !strings.Contains(createTableQuery, "id") {
		createTableQuery += "id text, "
	}

	if !strings.Contains(createTableQuery, "typename") {
		createTableQuery += "typename text, "
	}

	// Add the primary key
	createTableQuery += "PRIMARY KEY (id));"

	// Execute the CREATE TABLE query
	err := session.Query(createTableQuery).Exec()
	if err != nil {
		fmt.Println(createTableQuery)
		fmt.Println("Failed to create table ", err)
	}

	return err
}

func deleteKeyspace(host, keyspace string) error {

	// Create the admin session.
	adminCluster := gocql.NewCluster()               // Replace with your ScyllaDB cluster IP address
	adminCluster.Hosts = []string{host, "127.0.0.1"} // add local host as well.
	adminCluster.Keyspace = "system"                 // Use the 'system' keyspace for administrative tasks
	adminSession, err := adminCluster.CreateSession()
	if err != nil {
		return err
	}

	defer adminSession.Close()

	query := fmt.Sprintf("DROP KEYSPACE IF EXISTS %s;", keyspace)
	return adminSession.Query(query).Exec()
}

func (store *ScyllaStore) DeleteDatabase(ctx context.Context, connectionId string, keyspace string) error {
	if len(keyspace) == 0 {
		return errors.New("the database is required")
	}

	// I will get the session for that keyspace.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()

	if connection == nil {
		return errors.New("the connection does not exist")
	}

	// Drop the keyspace.
	if err := deleteKeyspace(connection.Host, keyspace); err != nil {
		fmt.Println("Fail to drop keyspace ", err)
		return err
	}

	return nil
}

func (store *ScyllaStore) Count(ctx context.Context, connectionId string, keyspace string, table string, query string, options string) (int64, error) {
	return 0, errors.New("not implemented")
}

func (store *ScyllaStore) insertData(connectionId, keyspace, tableName string, data map[string]interface{}) (map[string]interface{}, error) {

	// I will get the session for that keyspace.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()

	if connection == nil {
		return nil, errors.New("the connection does not exist")
	}

	// Get the first found session.
	session, ok := connection.sessions[keyspace]
	if !ok {
		return nil, errors.New("the session does not exist")
	}

	var id string
	if data["id"] != nil {
		id = data["id"].(string)
	} else if data["_id"] != nil {
		id = data["_id"].(string)
	}

	if len(id) == 0 {
		return nil, errors.New("the id is required")
	}

	// test if the data already exist.
	if store.isTableExist(connectionId, keyspace, tableName) {
		// I will check if the data already exist.
		query := fmt.Sprintf("SELECT * FROM %s WHERE id='%s'", tableName, id)
		values, err := store.FindOne(context.Background(), connectionId, keyspace, tableName, query, "")
		if err == nil {
			return values.(map[string]interface{}), nil
		}
	}

	columns := make([]string, 0)
	values := make([]interface{}, 0)

	for column, value := range data {
		goType := reflect.TypeOf(value)
		if goType.Kind() == reflect.Slice {
			// This is an array column, insert the values into the array table
			sliceValue := reflect.ValueOf(data[column])
			length := sliceValue.Len()

			for i := 0; i < length; i++ {
				element := sliceValue.Index(i)
				valueType := reflect.TypeOf(element.Interface())

				if valueType.Kind() == reflect.Map {
					entity := element.Interface().(map[string]interface{})
					if entity["typeName"] != nil {
						typeName := entity["typeName"].(string)

						// I will save the entity itself.
						var err error
						entity, err = store.insertData(connectionId, keyspace, typeName+"s", entity)
						if err != nil {
							fmt.Println("Error inserting data into array table: ", err)
						}

						// I will get the entity id.
						_id := Utility.ToInt(entity["_id"])
						sourceCollection := tableName
						field := column

						// He I will create the reference table.
						// I will create the table if not already exist.
						createTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS ` + sourceCollection + `_` + field + ` (source_id TEXT, target_id TEXT, PRIMARY KEY (source_id, target_id))`)
						err = session.Query(createTable).Exec()
						if err == nil {
							fmt.Println("Table created: ", sourceCollection+"_"+field)
						}

						// I will insert the reference into the table.
						insertSQL := fmt.Sprintf("INSERT INTO " + sourceCollection + "_" + field + " (source_id, target_id) VALUES (?, ?);")
						parameters := make([]interface{}, 0)
						parameters = append(parameters, id)
						parameters = append(parameters, _id)

						err = session.Query(insertSQL, parameters...).Exec()

						if err != nil {
							fmt.Println("Error inserting data into array table: ", err)
						}
					}
				} else if valueType.Kind() == reflect.String {

					arrayTableName := tableName + "_" + column
					str := element.Interface().(string)

					createTable := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (value TEXT, %s_id TEXT, PRIMARY KEY ((value, %s_id)));", arrayTableName, tableName,  tableName)

					// Create the table.
					err := session.Query(createTable).Exec()
					if err != nil {
						fmt.Println("Error creating array table: ", err)
					}

					// Insert the value into the table.
					insertSQL := fmt.Sprintf("INSERT INTO %s (value, %s_id) VALUES (?, ?);", arrayTableName, tableName)
					parameters := make([]interface{}, 0)
					parameters = append(parameters, str)
					parameters = append(parameters, id)

					err = session.Query(insertSQL, parameters...).Exec()
					if err != nil {
						fmt.Println("Error inserting data into array table: ", err)
					}
				}
			}

		} else if goType.Kind() == reflect.Map {
			continue
		} else {
			column = camelToSnake(column)
			columns = append(columns, column)
			values = append(values, value)
		}
	}

	query := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES (", keyspace, tableName, joinStrings(columns, ", "))
	placeholders := make([]string, len(columns))

	for i := range columns {
		placeholders[i] = "?"
	}

	query += joinStrings(placeholders, ", ")
	query += ");"

	// scylla does not support _id, so I will replace it with id.
	query = strings.ReplaceAll(query, "_id", "id")

	// Execute the CREATE TABLE query
	err := session.Query(query, values...).Exec()
	if err != nil {
		fmt.Println(query)
		fmt.Println("Failed to insert entity ", err)
		return nil, err
	}

	return data, nil
}

func joinStrings(slice []string, separator string) string {
	if len(slice) == 0 {
		return ""
	}
	result := slice[0]
	for i := 1; i < len(slice); i++ {
		result += separator + slice[i]
	}
	return result
}

func (store *ScyllaStore) InsertOne(ctx context.Context, connectionId string, keyspace string, table string, entity interface{}, options string) (interface{}, error) {

	// I will get the session for that keyspace.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()

	if connection == nil {
		return nil, errors.New("the connection does not exist")
	}

	// Get the first found session.
	session, ok := connection.sessions[keyspace]
	if !ok {
		return nil, errors.New("the session does not exist")
	}

	// Check if the table exist.
	if !store.isTableExist(connectionId, keyspace, table) {
		// return nil, errors.New("the table does not exist")
		// Create the table.
		if err := store.createScyllaTable(session, keyspace, table, entity.(map[string]interface{})); err != nil {
			fmt.Println("Fail to create table ", err)
			return nil, err
		}

		// Wait for the table to be created.
		time.Sleep(1 * time.Second)
	}

	// Generate the INSERT statement
	var err error
	entity, err = store.insertData(connectionId, keyspace, table, entity.(map[string]interface{}))
	if err != nil {
		return nil, err
	}

	return entity, nil
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

// The collection will be created when the first entity is inserted...
func (store *ScyllaStore) CreateCollection(ctx context.Context, connectionId string, keyspace string, collection string, options string) error {
	return errors.New("not implemented")
}

func dropTable(session *gocql.Session, keyspace, tableName string) error {
	query := fmt.Sprintf("DROP TABLE IF EXISTS %s.%s;", keyspace, tableName)
	return session.Query(query).Exec()
}

func (store *ScyllaStore) DeleteCollection(ctx context.Context, connectionId string, keyspace string, collection string) error {
	if len(keyspace) == 0 {
		return errors.New("the database is required")
	}

	if len(collection) == 0 {
		return errors.New("the collection is required")
	}

	// I will get the session for that keyspace.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()
	if connection == nil {
		return errors.New("the connection does not exist")
	}

	// Get the first found session.
	session, ok := connection.sessions[keyspace]
	if !ok {
		return errors.New("the session does not exist")
	}

	// Drop the table.
	if err := dropTable(session, keyspace, collection); err != nil {
		fmt.Println("Fail to drop table ", err)
		return err
	}

	return nil
}

func (store *ScyllaStore) RunAdminCmd(ctx context.Context, connectionId string, user string, password string, script string) error {
	return errors.New("not implemented")
}
