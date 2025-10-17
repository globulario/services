package persistence_store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"time"
	"unicode"

	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/config"
	Utility "github.com/globulario/utility"
	_ "github.com/mattn/go-sqlite3" // Import the sqlite3 driver
)

// SqlConnection represents a connection to a SQL database.
type SqlConnection struct {
	Id        string             // Unique identifier for the connection
	Host      string             // Host address of the SQL server
	Token     string             // Authentication token
	Path      string             // Path to the database file
	databases map[string]*sql.DB // Map of database connections
}

// SqlStore represents the SQL store backed by SQLite databases.
//
// Notes:
//   - This implementation authenticates users through the Authentication service,
//     but the actual data store is file-based SQLite.
//   - Each logical "database" is a .db file in the configured path.
//   - Arrays and references are materialized in auxiliary tables like <table>_<field>.
type SqlStore struct {
	connections map[string]SqlConnection // Map of SQL connections
}

// GetStoreType returns the store type identifier ("SQL").
func (store *SqlStore) GetStoreType() string {
	return "SQL"
}

// --------- Canonical link-table helpers (NEW) ----------

func ensurePlural(s string) string {
	s = strings.ToLower(s)
	if !strings.HasSuffix(s, "s") {
		return s + "s"
	}
	return s
}

// Connect establishes a new logical connection to a SQL database (SQLite file).
// If the database file does not exist, it will be created.
// It authenticates the user with the Authentication service before opening the DB.
func (store *SqlStore) Connect(id string, host string, port int32, user string, password string, database string, timeout int32, options_str string) error {
	log := slog.With(
		"component", "SqlStore",
		"method", "Connect",
		"id", id,
		"host", host,
		"database", database,
	)

	if len(id) == 0 {
		return errors.New("the connection id is required")
	}

	if store.connections != nil {
		if _, ok := store.connections[id]; ok {
			if store.connections[id].databases != nil {
				if _, ok := store.connections[id].databases[database]; ok {
					log.Debug("connection already established")
					return nil
				}
			}
		}
	} else {
		store.connections = make(map[string]SqlConnection)
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

	if len(database) == 0 {
		return errors.New("the database is required")
	}

	// Authenticate the user.
	authCli, err := authentication_client.NewAuthenticationService_Client(host, "authentication.AuthenticationService")
	if err != nil {
		log.Error("failed to create authentication client", "err", err)
		return err
	}

	nbTry := 5
	var token string
	for nbTry > 0 {
		var aerr error
		token, aerr = authCli.Authenticate(user, password)
		if aerr == nil {
			break
		}
		nbTry--
		if nbTry == 0 {
			log.Error("authentication failed", "err", aerr)
			return aerr
		}
		time.Sleep(1 * time.Second)
	}

	// Parse options.
	options := make(map[string]interface{})
	if len(options_str) > 0 {
		if err := json.Unmarshal([]byte(options_str), &options); err != nil {
			log.Error("failed to parse options", "err", err)
			return err
		}
	}

	// Determine data path.
	path := config.GetDataDir() + "/sql-data"
	if v, ok := options["path"]; ok {
		if s, ok2 := v.(string); ok2 && s != "" {
			path = s
		}
	}

	// Ensure directory exists.
	if err := Utility.CreateDirIfNotExist(path); err != nil {
		log.Error("failed to ensure data path", "path", path, "err", err)
		return err
	}

	// Ensure connection struct is present.
	var connection SqlConnection
	if existing, ok := store.connections[id]; ok {
		connection = existing
	} else {
		connection = SqlConnection{
			Id:        id,
			Host:      host,
			Token:     token,
			Path:      path,
			databases: make(map[string]*sql.DB, 0),
		}
		store.connections[id] = connection
	}

	// Open (or create) the database file.
	databasePath := connection.Path + "/" + database + ".db"
	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		log.Error("failed to open sqlite database", "path", databasePath, "err", err)
		return err
	}
	if connection.databases == nil {
		connection.databases = make(map[string]*sql.DB)
	}
	connection.databases[database] = db
	store.connections[id] = connection

	// Initialize user_data if needed (compat behavior).
	count, _ := store.Count(context.Background(), id, "", "user_data", `SELECT * FROM user_data WHERE _id='`+user+`'`, "")
	if count == 0 && id != "local_resource" {
		_, _ = store.InsertOne(context.Background(), id, database, "user_data",
			map[string]interface{}{"_id": user, "first_name": "", "last_name": "", "middle_name": "", "profile_picture": "", "email": ""}, "")
	}

	log.Info("connected")
	return nil
}

// ExecContext executes a statement with optional parameters and optional transaction.
// It returns a JSON string containing "lastId" and "rowsAffected".
func (store *SqlStore) ExecContext(connectionId string, database string, query string, parameters []interface{}, tx_ int) (string, error) {
	log := slog.With(
		"component", "SqlStore",
		"method", "ExecContext",
		"id", connectionId,
		"database", database,
	)

	if len(connectionId) == 0 {
		return "", errors.New("the connection id is required")
	}
	if len(database) == 0 {
		return "", errors.New("the database is required")
	}

	conn, exists := store.connections[connectionId]
	if !exists {
		return "", fmt.Errorf("connection with ID %s does not exist", connectionId)
	}

	if conn.databases == nil {
		conn.databases = make(map[string]*sql.DB, 0)
	}

	if conn.databases[database] == nil {
		databasePath := conn.Path + "/" + database + ".db"
		db, err := sql.Open("sqlite3", databasePath)
		if err != nil {
			log.Error("failed to open sqlite database", "path", databasePath, "err", err)
			return "", err
		}
		conn.databases[database] = db
		// persist mutation
		store.connections[connectionId] = conn
	}

	hasTx := tx_ == 1
	var result sql.Result
	if hasTx {
		tx, err := conn.databases[database].BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
		if err != nil {
			log.Error("failed to begin transaction", "err", err)
			return "", err
		}
		res, execErr := tx.ExecContext(context.Background(), query, parameters...)
		if execErr != nil {
			_ = tx.Rollback()
			log.Error("exec failed (tx)", "err", execErr, "query", query)
			return "", fmt.Errorf("update failed: %v", execErr)
		}
		if err := tx.Commit(); err != nil {
			log.Error("commit failed", "err", err)
			return "", err
		}
		result = res
	} else {
		var err error
		result, err = conn.databases[database].ExecContext(context.Background(), query, parameters...)
		if err != nil {
			log.Error("exec failed", "err", err, "query", query)
			return "", err
		}
	}

	numRowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Error("failed to read rows affected", "err", err)
		return "", err
	}
	lastId, _ := result.LastInsertId()

	out := fmt.Sprintf("{\"lastId\": %d, \"rowsAffected\": %d}", lastId, numRowsAffected)
	log.Debug("exec ok", "rowsAffected", numRowsAffected, "lastId", lastId)
	return out, nil
}

// QueryContext runs a SELECT query with parameters and returns a JSON object:
// {
//   "header": [{ "name": "<col>", "typeInfo": { ... } }, ...],
//   "data":   [[v1, v2, ...], ...]
// }
func (store *SqlStore) QueryContext(connectionId string, database string, query string, parameters_ string) (string, error) {
	log := slog.With("component", "SqlStore", "method", "QueryContext", "id", connectionId, "database", database)

	if len(connectionId) == 0 {
		return "", errors.New("the connection id is required")
	}
	if len(database) == 0 {
		return "", errors.New("the database is required")
	}

	conn, exists := store.connections[connectionId]
	if !exists {
		return "", fmt.Errorf("connection with ID %s does not exist", connectionId)
	}

	if conn.databases == nil {
		conn.databases = make(map[string]*sql.DB, 0)
	}

	if conn.databases[database] == nil {
		databasePath := conn.Path + "/" + database + ".db"
		db, err := sql.Open("sqlite3", databasePath)
		if err != nil {
			log.Error("failed to open sqlite database", "path", databasePath, "err", err)
			return "", err
		}
		conn.databases[database] = db
		store.connections[connectionId] = conn
	}

	// Bind parameters.
	parameters := make([]interface{}, 0)
	if len(parameters_) > 0 {
		if err := json.Unmarshal([]byte(parameters_), &parameters); err != nil {
			log.Error("failed to parse parameters", "err", err)
			return "", err
		}
	}

	rows, err := conn.databases[database].QueryContext(context.Background(), query, parameters...)
	if err != nil {
		log.Error("query failed", "err", err, "query", query)
		return "", err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		log.Error("failed to read columns", "err", err)
		return "", err
	}

	columnsType, err := rows.ColumnTypes()
	if err != nil {
		log.Error("failed to read column types", "err", err)
		return "", err
	}

	header := make([]interface{}, len(columns))
	for i := 0; i < len(columnsType); i++ {
		column := columns[i]

		typeInfo := map[string]interface{}{
			"DatabaseTypeName": columnsType[i].DatabaseTypeName(),
			"Name":             columnsType[i].DatabaseTypeName(),
		}

		if precision, scale, ok := columnsType[i].DecimalSize(); ok {
			typeInfo["Scale"] = scale
			typeInfo["Precision"] = precision
		}
		if length, ok := columnsType[i].Length(); ok {
			typeInfo["Precision"] = length
		}
		if nullable, ok := columnsType[i].Nullable(); ok {
			typeInfo["IsNullable"] = ok
			typeInfo["IsNull"] = nullable
		}

		header[i] = map[string]interface{}{"name": column, "typeInfo": typeInfo}
	}

	count := len(columns)
	values := make([]interface{}, count)
	scanArgs := make([]interface{}, count)
	for i := range values {
		scanArgs[i] = &values[i]
	}

	rowsOut := make([]interface{}, 0)
	for rows.Next() {
		row := make([]interface{}, count)
		if err := rows.Scan(scanArgs...); err != nil {
			log.Error("row scan failed", "err", err)
			return "", err
		}

		for i, v := range values {
			switch {
			case v == nil:
				row[i] = nil
			case Utility.IsNumeric(v):
				row[i] = Utility.ToNumeric(v)
			case Utility.IsBool(v):
				row[i] = Utility.ToBool(v)
			default:
				row[i] = Utility.ToString(v)
			}
		}
		rowsOut = append(rowsOut, row)
	}

	result := map[string]interface{}{
		"header": header,
		"data":   rowsOut,
	}
	resultStr, _ := Utility.ToJson(result)
	return resultStr, nil
}

// Disconnect closes all databases for the given connection and removes it from the store.
func (store *SqlStore) Disconnect(connectionId string) error {
	log := slog.With("component", "SqlStore", "method", "Disconnect", "id", connectionId)

	conn, exists := store.connections[connectionId]
	if !exists {
		return fmt.Errorf("connection with ID %s does not exist", connectionId)
	}

	for name, db := range conn.databases {
		if err := db.Close(); err != nil {
			log.Error("failed closing database", "database", name, "err", err)
			return err
		}
	}
	delete(store.connections, connectionId)
	log.Info("disconnected")
	return nil
}

// Ping checks the store liveness for the given connection.
// (No-op for the SQLite implementation.)
func (store *SqlStore) Ping(ctx context.Context, connectionId string) error {
	return nil
}

// CreateDatabase creates a new database (not implemented for SQLite variant).
func (store *SqlStore) CreateDatabase(ctx context.Context, connectionId string, name string) error {
	return errors.New("not implemented")
}

// DeleteDatabase removes a database file and its contents.
func (store *SqlStore) DeleteDatabase(ctx context.Context, connectionId string, db string) error {
	if len(db) == 0 {
		return errors.New("the database name is required")
	}
	databasePath := store.connections[connectionId].Path + "/" + db + ".db"
	return os.RemoveAll(databasePath)
}

// Count returns the number of records for a given query.
// If query is "{}", a SELECT * FROM <table> is used.
func (store *SqlStore) Count(ctx context.Context, connectionId string, db string, table string, query string, options string) (int64, error) {
	if len(query) == 0 || query == "{}" {
		query = fmt.Sprintf("SELECT * FROM %s", table)
	} else if strings.HasPrefix(query, "{") && strings.HasSuffix(query, "}") {
		var err error
		query, err = store.formatQuery(table, query)
		if err != nil {
			return 0, err
		}
	}

	str, err := store.QueryContext(connectionId, db, query, "[]")
	if err != nil {
		return 0, err
	}

	data := make(map[string]interface{})
	if err := json.Unmarshal([]byte(str), &data); err != nil {
		return 0, err
	}
	return int64(len(data["data"].([]interface{}))), nil
}

func (store *SqlStore) isTableExist(connectionId string, db string, table string) bool {
	query := fmt.Sprintf("SELECT name FROM sqlite_master WHERE type='table' AND name='%s'", table)
	str, err := store.QueryContext(connectionId, db, query, "[]")
	if err != nil {
		return false
	}
	result := make(map[string]interface{})
	if err := json.Unmarshal([]byte(str), &result); err != nil {
		return false
	}
	return len(result["data"].([]interface{})) > 0
}

// generateCreateTableSQL builds the CREATE TABLE statement for the main entity table.
func generateCreateTableSQL(tableName string, columns map[string]interface{}) (string, []string) {
	var columnsSQL []string
	var arrayTables []string

	for columnName, columnType := range columns {
		if columnType == nil {
			continue
		}
		sqlType := getSQLType(reflect.TypeOf(columnType))
		isArray := reflect.Slice == reflect.TypeOf(columnType).Kind()
		if !isArray && columnName != "typeName" {
			if columnName == "id" {
				columnName = "_id"
			}
			if columnName != "_id" && sqlType != "" {
				columnsSQL = append(columnsSQL, fmt.Sprintf("\"%s\" %s", columnName, sqlType))
			}
		}
	}

	createTableSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS \"%s\" (_id TEXT PRIMARY KEY, %s);", tableName, strings.Join(columnsSQL, ", "))
	return createTableSQL, arrayTables
}

func getSQLType(goType reflect.Type) string {
	if goType == nil {
		return ""
	}
	switch goType.Kind() {
	case reflect.Int, reflect.Int32, reflect.Int64:
		return "INTEGER"
	case reflect.Float32, reflect.Float64:
		return "REAL"
	case reflect.String:
		return "TEXT"
	default:
		return ""
	}
}

// generateMainInsertSQL creates the INSERT statement for the main table.
func generateMainInsertSQL(tableName string, data map[string]interface{}) (string, []interface{}) {
	var mainColumns []string
	var mainPlaceholders []string
	var mainValues []interface{}

	for columnName, columnValue := range data {
		if columnValue == nil || columnName == "typeName" {
			continue
		}
		isArray := reflect.Slice == reflect.TypeOf(columnValue).Kind()
		if isArray {
			continue
		}
		if columnName == "id" {
			columnName = "_id"
		}
		mainColumns = append(mainColumns, fmt.Sprintf("\"%s\"", columnName))
		mainPlaceholders = append(mainPlaceholders, "?")
		mainValues = append(mainValues, columnValue)
	}

	mainInsertSQL := fmt.Sprintf("INSERT INTO \"%s\" (%s) VALUES (%s);",
		tableName, strings.Join(mainColumns, ", "), strings.Join(mainPlaceholders, ", "))

	return mainInsertSQL, mainValues
}

// insertData writes one entity into the main table and its auxiliary array/reference tables.
func (store *SqlStore) insertData(connectionId string, db string, tableName string, data map[string]interface{}) (map[string]interface{}, error) {
	log := slog.With("component", "SqlStore", "method", "insertData", "table", tableName)

	var id string
	switch {
	case data["id"] != nil:
		id = Utility.ToString(data["id"])
	case data["_id"] != nil:
		id = Utility.ToString(data["_id"])
	case data["Uid"] != nil:
		id = Utility.ToString(data["Uid"])
	case data["uid"] != nil:
		id = Utility.ToString(data["uid"])
	case data["uuid"] != nil:
		id = Utility.ToString(data["uuid"])
	case data["UUID"] != nil:
		id = Utility.ToString(data["UUID"])
	}

	if len(id) == 0 {
		log.Error("missing id for insert")
		return nil, errors.New("the id is required to insert data into the database")
	}
	data["_id"] = id

	// If table exists and entity exists, return it (idempotent).
	if store.isTableExist(connectionId, db, tableName) {
		query := fmt.Sprintf("SELECT * FROM %s WHERE id='%s'", tableName, id)
		if values, err := store.FindOne(context.Background(), connectionId, db, tableName, query, ""); err == nil {
			return values.(map[string]interface{}), nil
		}
	}

	insertSQL, values := generateMainInsertSQL(tableName, data)
	if _, err := store.ExecContext(connectionId, db, insertSQL, values, 0); err != nil {
		log.Error("insert main table failed", "err", err)
		return nil, err
	}

	// Process array and reference fields.
	for columnName, columnValue := range data {
		if columnName == "typeName" || columnValue == nil {
			continue
		}

		// Arrays of primitives or references.
		if reflect.Slice == reflect.TypeOf(columnValue).Kind() {
			sliceValue := reflect.ValueOf(columnValue)
			length := sliceValue.Len()

			for i := 0; i < length; i++ {
				element := sliceValue.Index(i)

				switch el := element.Interface().(type) {
				case int, float64, string:
					// Primitive arrays keep the old shape <table>_<field>
					arrayTableName := tableName + "_" + columnName
					// Ensure primitive array table exists.
					if !store.isTableExist(connectionId, db, arrayTableName) {
						sqlType := getSQLType(reflect.TypeOf(el))
						createTableSQL := fmt.Sprintf(
							"CREATE TABLE IF NOT EXISTS %s (value %s, %s_id TEXT)",
							arrayTableName, sqlType, tableName,
						)
						if _, err := store.ExecContext(connectionId, db, createTableSQL, nil, 0); err != nil {
							return nil, err
						}
					}
					insert := fmt.Sprintf("INSERT INTO %s (value, %s_id) VALUES (?, ?);", arrayTableName, tableName)
					if _, err := store.ExecContext(connectionId, db, insert, []interface{}{el, id}, 0); err != nil {
						log.Error("insert array value failed", "arrayTable", arrayTableName, "err", err)
						return nil, err
					}

				case map[string]interface{}:
					entity := el
					// Embedded typed entity -> persist and link
					if entity["typeName"] != nil {
						typeName := ensurePlural(Utility.ToString(entity["typeName"]))
						// normalize domain
						localDomain, _ := config.GetDomain()
						if entity["domain"] == nil {
							entity["domain"] = localDomain
						} else if entity["domain"] == "localhost" {
							entity["domain"] = localDomain
						}
						var err error
						entity, err = store.insertData(connectionId, db, typeName, entity)
						if err != nil {
							log.Error("insert nested entity failed", "type", typeName, "err", err)
						}
						targetID := Utility.ToString(entity["_id"])
						linkTable, baseIsFirst := canonicalRefTable(tableName, typeName)

						createLink := fmt.Sprintf(
							"CREATE TABLE IF NOT EXISTS %s (source_id TEXT, target_id TEXT)",
							linkTable,
						)
						_, _ = store.ExecContext(connectionId, db, createLink, nil, 0)

						var src, dst interface{}
						if baseIsFirst {
							src, dst = id, targetID
						} else {
							src, dst = targetID, id
						}
						ins := fmt.Sprintf("INSERT INTO %s (source_id, target_id) VALUES (?, ?);", linkTable)
						_, _ = store.ExecContext(connectionId, db, ins, []interface{}{src, dst}, 0)

					} else if entity["$ref"] != nil {
						targetCollection := ensurePlural(Utility.ToString(entity["$ref"]))
						targetID := Utility.ToString(entity["$id"])
						linkTable, baseIsFirst := canonicalRefTable(tableName, targetCollection)

						createLink := fmt.Sprintf(
							"CREATE TABLE IF NOT EXISTS %s (source_id TEXT, target_id TEXT)",
							linkTable,
						)
						_, _ = store.ExecContext(connectionId, db, createLink, nil, 0)

						var src, dst interface{}
						if baseIsFirst {
							src, dst = id, targetID
						} else {
							src, dst = targetID, id
						}
						ins := fmt.Sprintf("INSERT INTO %s (source_id, target_id) VALUES (?, ?);", linkTable)
						_, _ = store.ExecContext(connectionId, db, ins, []interface{}{src, dst}, 0)
					}
				default:
					// Unknown element type: skip
				}
			}

		} else if reflect.Map == reflect.TypeOf(columnValue).Kind() {
			// Single reference/embedded object
			entity := columnValue.(map[string]interface{})
			if entity["typeName"] != nil {
				typeName := ensurePlural(Utility.ToString(entity["typeName"]))
				if entity["domain"] == nil {
					localDomain, _ := config.GetDomain()
					entity["domain"] = localDomain
				}
				var err error
				entity, err = store.insertData(connectionId, db, typeName, entity)
				if err != nil {
					log.Error("insert nested entity failed", "type", typeName, "err", err)
				}
				targetID := Utility.ToString(entity["_id"])
				linkTable, baseIsFirst := canonicalRefTable(tableName, typeName)

				createLink := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (source_id TEXT, target_id TEXT)", linkTable)
				_, _ = store.ExecContext(connectionId, db, createLink, nil, 0)

				var src, dst interface{}
				if baseIsFirst {
					src, dst = id, targetID
				} else {
					src, dst = targetID, id
				}
				ins := fmt.Sprintf("INSERT INTO %s (source_id, target_id) VALUES (?, ?);", linkTable)
				_, _ = store.ExecContext(connectionId, db, ins, []interface{}{src, dst}, 0)

			} else if entity["$ref"] != nil {
				targetCollection := ensurePlural(Utility.ToString(entity["$ref"]))
				targetID := Utility.ToString(entity["$id"])
				linkTable, baseIsFirst := canonicalRefTable(tableName, targetCollection)

				createLink := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (source_id TEXT, target_id TEXT)", linkTable)
				_, _ = store.ExecContext(connectionId, db, createLink, nil, 0)

				var src, dst interface{}
				if baseIsFirst {
					src, dst = id, targetID
				} else {
					src, dst = targetID, id
				}
				ins := fmt.Sprintf("INSERT INTO %s (source_id, target_id) VALUES (?, ?);", linkTable)
				_, _ = store.ExecContext(connectionId, db, ins, []interface{}{src, dst}, 0)
			}
		}
	}

	return data, nil
}

// InsertOne inserts a single entity, creating the table if needed.
func (store *SqlStore) InsertOne(ctx context.Context, connectionId string, db string, table string, entity interface{}, options string) (interface{}, error) {
	entity_, err := Utility.ToMap(entity)
	if err != nil {
		return nil, err
	}

	if !store.isTableExist(connectionId, db, table) {
		createTableSQL, arrayTableSQL := generateCreateTableSQL(table, entity_)
		if _, err := store.ExecContext(connectionId, db, createTableSQL, nil, 0); err != nil {
			return nil, err
		}
		for _, sql := range arrayTableSQL {
			if _, err := store.ExecContext(connectionId, db, sql, nil, 0); err != nil {
				return nil, err
			}
		}
	}

	result, err := store.insertData(connectionId, db, table, entity_)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// InsertMany inserts multiple entities, creating the table if needed.
func (store *SqlStore) InsertMany(ctx context.Context, connectionId string, db string, table string, entities []interface{}, options string) ([]interface{}, error) {
	if !store.isTableExist(connectionId, db, table) {
		entity_, err := Utility.ToMap(entities[0])
		if err != nil {
			return nil, err
		}
		createTableSQL, arrayTableSQL := generateCreateTableSQL(table, entity_)
		if _, err := store.ExecContext(connectionId, db, createTableSQL, nil, 0); err != nil {
			return nil, err
		}
		for _, sql := range arrayTableSQL {
			if _, err := store.ExecContext(connectionId, db, sql, nil, 0); err != nil {
				return nil, err
			}
		}
	}

	results := make([]interface{}, 0, len(entities))
	for _, e := range entities {
		entity_, err := Utility.ToMap(e)
		if err != nil {
			return nil, err
		}
		result, err := store.insertData(connectionId, db, table, entity_)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func isIntegerType(sqlType string) bool { return sqlType == "INTEGER" }

func getIntValue(value interface{}) (int32, error) {
	v := Utility.ToInt(value)
	return int32(v), nil
}

// recreateArrayOfObjects expands rows into structured objects, resolving array tables and reference tables.
func (store *SqlStore) recreateArrayOfObjects(connectionId, db, tableName string, dataHeader map[string]interface{}, options []map[string]interface{}) ([]interface{}, error) {
	data := dataHeader["data"]
	header := dataHeader["header"]

	var objects []interface{}
	var projection map[string]interface{}
	if len(options) > 0 {
		for _, option := range options {
			if option["Projection"] != nil {
				projection = option["Projection"].(map[string]interface{})
				projection["_id"] = 1 // ensure id presence
			}
		}
	}

	for _, dr := range data.([]interface{}) {
		dataRow := dr.([]interface{})
		object := make(map[string]interface{})
		object["typeName"] = tableName

		for index, fieldInfos := range header.([]interface{}) {
			fi := fieldInfos.(map[string]interface{})
			fieldName := fi["name"].(string)
			value := dataRow[index]
			typeInfos := fi["typeInfo"].(map[string]interface{})

			allow := projection == nil || projection[fieldName] != nil
			if !allow {
				continue
			}
			if isIntegerType(typeInfos["DatabaseTypeName"].(string)) {
				intValue, err := getIntValue(value)
				if err != nil {
					return nil, err
				}
				object[fieldName] = intValue
			} else {
				object[fieldName] = value
			}
		}

		objects = append(objects, object)

		// Discover auxiliary tables that belong to this database.
		query := "SELECT name FROM sqlite_master WHERE type='table'"
		str, err := store.QueryContext(connectionId, db, query, "[]")
		if err != nil {
			return nil, err
		}

		tables := make(map[string]interface{})
		if err := json.Unmarshal([]byte(str), &tables); err != nil {
			return nil, err
		}

		domain, _ := config.GetDomain()
		base := ensurePlural(tableName)
		id := Utility.ToString(object["_id"])

		for _, values := range tables["data"].([]interface{}) {
			for _, value := range values.([]interface{}) {
				t := value.(string)

				// 1) Primitive array tables: <tableName>_<field>
				if strings.HasPrefix(t, tableName+"_") {
					field := strings.TrimPrefix(t, tableName+"_")
					if object[field] != nil {
						continue
					}
					q := fmt.Sprintf("SELECT value FROM %s WHERE %s=?", t, tableName+"_id")
					paramsJSON, _ := Utility.ToJson([]interface{}{id})
					if s, e := store.QueryContext(connectionId, db, q, paramsJSON); e == nil {
						data := make(map[string]interface{})
						if err := json.Unmarshal([]byte(s), &data); err != nil {
							return nil, err
						}
						array := make([]interface{}, 0, len(data["data"].([]interface{})))
						for _, v := range data["data"].([]interface{}) {
							array = append(array, v.([]interface{})[0])
						}
						object[field] = array
						continue
					}
				}

				// 2) Canonical ref tables: <left>_<right>, alphabetical
				parts := strings.Split(t, "_")
				if len(parts) != 2 {
					continue
				}
				left := parts[0]
				right := parts[1]

				if left != base && right != base {
					continue
				}

				// field name becomes the "other" token (plural)
				other := left
				colSelect := "source_id"
				whereCol := "target_id"
				if left == base {
					other = right
					colSelect = "target_id"
					whereCol = "source_id"
				}

				field := other // keep plural as field (roles, organizations, etc.)
				if object[field] != nil {
					continue
				}

				// read refs from the correct side
				q := fmt.Sprintf("SELECT %s FROM %s WHERE %s=?", colSelect, t, whereCol)
				paramsJSON, _ := Utility.ToJson([]interface{}{id})
				if s, e := store.QueryContext(connectionId, db, q, paramsJSON); e == nil {
					data := make(map[string]interface{})
					if json.Unmarshal([]byte(s), &data) == nil {
						arr := make([]interface{}, 0)
						for _, row := range data["data"].([]interface{}) {
							refID := Utility.ToString(row.([]interface{})[0])
							if strings.Contains(refID, "@") {
								if strings.Split(refID, "@")[0] != domain {
									continue
								}
								refID = strings.Split(refID, "@")[0]
							}
							// $ref TypeName: capitalize the other token's first letter (compat with previous)
							b := []byte(other)
							if len(b) > 0 {
								b[0] = byte(unicode.ToUpper(rune(b[0])))
							}
							typeName := string(b)
							arr = append(arr, map[string]interface{}{"$ref": typeName, "$id": refID, "$db": db})
						}
						if len(arr) > 0 {
							object[field] = arr
						}
					}
				}
			}
		}
	}

	return objects, nil
}

func (store *SqlStore) formatQuery(table, query string) (string, error) {
	if query == "{}" {
		return fmt.Sprintf("SELECT * FROM %s", table), nil
	}

	parameters := make(map[string]interface{})
	if err := json.Unmarshal([]byte(query), &parameters); err != nil {
		return "", err
	}

	q := fmt.Sprintf("SELECT * FROM %s WHERE ", table)
	for key, value := range parameters {
		if key == "id" {
			key = "_id"
		}
		if key == "_id" {
			if s, ok := value.(string); ok && strings.Contains(s, "@") {
				value = strings.Split(s, "@")[0]
			}
		}
		switch reflect.TypeOf(value).Kind() {
		case reflect.String:
			q += fmt.Sprintf("%s = '%v' AND ", key, value)
		case reflect.Slice:
			if key == "$and" || key == "$or" {
				q += store.getParameters(key, value.([]interface{}))
			}
		case reflect.Map:
			for k, v := range value.(map[string]interface{}) {
				if k == "$regex" {
					q += fmt.Sprintf("%s LIKE '%v%%' AND ", key, v)
				}
			}
		}
	}
	q = strings.TrimSuffix(q, " AND ")
	return q, nil
}

// FindOne returns a single object that matches the query.
func (store *SqlStore) FindOne(ctx context.Context, connectionId string, database string, table string, query string, options string) (interface{}, error) {
	if len(query) == 0 {
		return nil, errors.New("query is empty")
	}
	if strings.HasPrefix(query, "{") && strings.HasSuffix(query, "}") {
		var err error
		query, err = store.formatQuery(table, query)
		if err != nil {
			return nil, err
		}
	}

	str, err := store.QueryContext(connectionId, database, query, "[]")
	if err != nil {
		return nil, err
	}

	data := make(map[string]interface{})
	if err := json.Unmarshal([]byte(str), &data); err != nil {
		return nil, err
	}

	options_ := make([]map[string]interface{}, 0)
	if len(options) > 0 {
		if err := json.Unmarshal([]byte(options), &options_); err != nil {
			return nil, err
		}
	}

	objects, err := store.recreateArrayOfObjects(connectionId, database, table, data, options_)
	if err != nil {
		return nil, err
	}
	if len(objects) > 0 {
		return objects[0], nil
	}
	return nil, errors.New("not found")
}

func (store *SqlStore) getParameters(condition string, values []interface{}) string {
	query := ""
	if condition == "$and" {
		query += "("
		for _, v := range values {
			value := v.(map[string]interface{})
			for key, v2 := range value {
				if reflect.TypeOf(v2).Kind() == reflect.String {
					query += fmt.Sprintf("%s = '%v' AND ", key, v2)
				}
			}
		}
		query = strings.TrimSuffix(query, " AND ")
		query += ")"
	}
	return query
}

// Find returns a list of objects that match the query.
func (store *SqlStore) Find(ctx context.Context, connectionId string, db string, table string, query string, options string) ([]any, error) {
	if len(query) == 0 || query == "{}" {
		query = fmt.Sprintf("SELECT * FROM %s", table)
	} else if strings.HasPrefix(query, "{") && strings.HasSuffix(query, "}") {
		var err error
		query, err = store.formatQuery(table, query)
		if err != nil {
			return nil, err
		}
	}

	str, err := store.QueryContext(connectionId, db, query, "[]")
	if err != nil {
		return nil, err
	}

	data := make(map[string]any)
	if err := json.Unmarshal([]byte(str), &data); err != nil {
		return nil, err
	}

	options_ := make([]map[string]any, 0)
	if len(options) > 0 {
		if err := json.Unmarshal([]byte(options), &options_); err != nil {
			return nil, err
		}
	}

	objects, err := store.recreateArrayOfObjects(connectionId, db, table, data, options_)
	if err != nil {
		return nil, err
	}
	if len(objects) > 0 {
		return objects, nil
	}
	return []any{}, nil
}

// ReplaceOne replaces a single entity matching the query with the provided value.
// If the table does not exist, it is created.
func (store *SqlStore) ReplaceOne(ctx context.Context, connectionId string, db string, table string, query string, value string, options string) error {
	entity := make(map[string]interface{})
	if err := json.Unmarshal([]byte(value), &entity); err != nil {
		return err
	}

	if !store.isTableExist(connectionId, db, table) {
		createTableSQL, arrayTableSQL := generateCreateTableSQL(table, entity)
		if _, err := store.ExecContext(connectionId, db, createTableSQL, nil, 0); err != nil {
			return err
		}
		for _, sql := range arrayTableSQL {
			if _, err := store.ExecContext(connectionId, db, sql, nil, 0); err != nil {
				return err
			}
		}
	}

	if strings.HasPrefix(query, "{") && strings.HasSuffix(query, "}") {
		var err error
		query, err = store.formatQuery(table, query)
		if err != nil {
			return err
		}
	}

	// Delete then insert (simplifies array/reference sync).
	_ = store.deleteOneSqlEntry(connectionId, db, table, query)

	_, err := store.insertData(connectionId, db, table, entity)
	return err
}

func generateUpdateTableQuery(tableName string, fields []interface{}, whereClause string) (string, error) {
	updateQuery := "UPDATE " + tableName + " SET "
	for i, field := range fields {
		updateQuery += field.(string) + " = ?"
		if i < len(fields)-1 {
			updateQuery += ", "
		}
	}
	if whereClause != "" {
		if strings.Contains(whereClause, "WHERE") {
			whereClause = strings.Split(whereClause, "WHERE")[1]
		}
		updateQuery += " WHERE " + whereClause
	}
	return updateQuery, nil
}

// Update applies a $set patch to all entities matching the query.
func (store *SqlStore) Update(ctx context.Context, connectionId string, db string, table string, query string, value string, options string) error {
	values_ := make(map[string]interface{})
	if err := json.Unmarshal([]byte(value), &values_); err != nil {
		return err
	}
	if values_["$set"] == nil {
		return errors.New("no $set operator allowed in Update")
	}

	var err error
	query, err = store.formatQuery(table, query)
	if err != nil {
		return err
	}

	// Fields (non-array only here).
	fields := make([]interface{}, 0)
	values := make([]interface{}, 0)
	for key, v := range values_["$set"].(map[string]interface{}) {
		if reflect.TypeOf(v).Kind() != reflect.Slice {
			fields = append(fields, key)
			values = append(values, v)
		}
	}

	q, err := generateUpdateTableQuery(table, fields, query)
	if err != nil {
		return err
	}

	if _, err := store.ExecContext(connectionId, db, q, values, 0); err != nil {
		return err
	}

	// Array/reference sync for bulk update is not handled to keep parity with original behavior.
	return nil
}

// UpdateOne applies a $set patch to a single entity and synchronizes primitive array tables.
// (Reference arrays are best updated via ReplaceOne/InsertOne in this minimal change.)
func (store *SqlStore) UpdateOne(ctx context.Context, connectionId string, db string, table string, query string, value string, options string) error {
	values_ := make(map[string]interface{})
	if err := json.Unmarshal([]byte(value), &values_); err != nil {
		return err
	}
	if values_["$set"] == nil {
		return errors.New("no $set operator allowed in UpdateOne")
	}

	var err error
	query, err = store.formatQuery(table, query)
	if err != nil {
		return err
	}

	currentEntity, err := store.FindOne(context.Background(), connectionId, db, table, query, "")
	if err != nil {
		return err
	}

	fields := make([]interface{}, 0)
	values := make([]interface{}, 0)
	arrayFields := make([]string, 0)

	for key, v := range values_["$set"].(map[string]interface{}) {
		if reflect.TypeOf(v).Kind() == reflect.Slice {
			arrayFields = append(arrayFields, key)
		} else {
			fields = append(fields, key)
			values = append(values, v)
		}
	}

	updateQuery, err := generateUpdateTableQuery(table, fields, query)
	if err != nil {
		return err
	}
	if _, err := store.ExecContext(connectionId, db, updateQuery, values, 0); err != nil {
		return err
	}

	// Sync primitive array tables for the updated entity.
	for _, field := range arrayFields {
		arrayTableName := table + "_" + field
		if store.isTableExist(connectionId, db, arrayTableName) {
			deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE %s_id=?", arrayTableName, table)
			if _, err := store.ExecContext(connectionId, db, deleteQuery, []interface{}{currentEntity.(map[string]interface{})["_id"]}, 0); err != nil {
				return err
			}
			sliceValue := reflect.ValueOf(values_["$set"].(map[string]interface{})[field])
			for i := 0; i < sliceValue.Len(); i++ {
				element := sliceValue.Index(i)
				insertQuery := fmt.Sprintf("INSERT INTO %s (value, %s_id) VALUES (?, ?);", arrayTableName, table)
				if _, err := store.ExecContext(connectionId, db, insertQuery, []interface{}{element.Interface(), currentEntity.(map[string]interface{})["_id"]}, 0); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (store *SqlStore) deleteSqlEntries(connectionId string, db string, table string, query string) error {
	entities, err := store.Find(context.Background(), connectionId, db, table, query, "")
	if err != nil {
		return err
	}

	for _, entity := range entities {
		if m, ok := entity.(map[string]interface{}); ok && m["_id"] != nil {
			q := fmt.Sprintf("SELECT * FROM '%s' WHERE _id='%s'", table, m["_id"].(string))
			if err := store.deleteOneSqlEntry(connectionId, db, table, q); err != nil {
				return err
			}
		}
	}
	return nil
}

func (store *SqlStore) deleteOneSqlEntry(connectionId string, db string, table string, query string) error {
	entity, err := store.FindOne(context.Background(), connectionId, db, table, query, "")
	if err != nil {
		return err
	}

	if strings.HasPrefix(query, "{") && strings.HasSuffix(query, "}") {
		if query, err = store.formatQuery(table, query); err != nil {
			return err
		}
	}

	// Delete the entity row.
	del := strings.Replace(query, "SELECT *", "DELETE", 1)
	if _, err := store.ExecContext(connectionId, db, del, nil, 0); err != nil {
		return err
	}

	// Delete related rows from array/reference tables.
	// We will:
	//  - remove primitive arrays: <table>_<field> WHERE <table>_id = ?
	//  - remove canonical refs: <left>_<right> WHERE (source_id|target_id) = ? depending on side
	id := Utility.ToString(entity.(map[string]interface{})["_id"])
	base := ensurePlural(table)

	// List tables once
	list := "SELECT name FROM sqlite_master WHERE type='table'"
	str, err := store.QueryContext(connectionId, db, list, "[]")
	if err != nil {
		return err
	}
	dbTables := make(map[string]interface{})
	if err := json.Unmarshal([]byte(str), &dbTables); err != nil {
		return err
	}

	for _, values := range dbTables["data"].([]interface{}) {
		for _, v := range values.([]interface{}) {
			name := v.(string)

			// Primitive arrays
			if strings.HasPrefix(name, table+"_") {
				delArr := fmt.Sprintf("DELETE FROM %s WHERE %s_id=?", name, table)
				_, _ = store.ExecContext(connectionId, db, delArr, []interface{}{id}, 0)
				continue
			}

			// Canonical refs
			parts := strings.Split(name, "_")
			if len(parts) != 2 {
				continue
			}
			left := parts[0]
			right := parts[1]
			if left != base && right != base {
				continue
			}
			if left == base {
				delRef := fmt.Sprintf("DELETE FROM %s WHERE source_id=?", name)
				_, _ = store.ExecContext(connectionId, db, delRef, []interface{}{id}, 0)
			} else {
				delRef := fmt.Sprintf("DELETE FROM %s WHERE target_id=?", name)
				_, _ = store.ExecContext(connectionId, db, delRef, []interface{}{id}, 0)
			}
		}
	}

	return nil
}

// Delete removes all entities matching the query.
func (store *SqlStore) Delete(ctx context.Context, connectionId string, db string, table string, query string, options string) error {
	return store.deleteSqlEntries(connectionId, db, table, query)
}

// DeleteOne removes a single entity matching the query.
func (store *SqlStore) DeleteOne(ctx context.Context, connectionId string, db string, table string, query string, options string) error {
	return store.deleteOneSqlEntry(connectionId, db, table, query)
}

// Aggregate is not implemented for the SQLite variant.
func (store *SqlStore) Aggregate(ctx context.Context, connectionId string, keyspace string, table string, pipeline string, optionsStr string) ([]interface{}, error) {
	return nil, errors.New("not implemented")
}

// CreateTable creates a new table with provided fields (all TEXT except _id which is TEXT PRIMARY KEY).
func (store *SqlStore) CreateTable(ctx context.Context, connectionId string, db string, table string, fields []string) error {
	createTableSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS \"%s\" (_id TEXT PRIMARY KEY, %s);", table, strings.Join(fields, ", "))
	_, err := store.ExecContext(connectionId, db, createTableSQL, nil, 0)
	return err
}

// CreateCollection is not implemented for the SQLite variant.
func (store *SqlStore) CreateCollection(ctx context.Context, connectionId string, database string, name string, optionsStr string) error {
	return errors.New("not implemented")
}

// DeleteCollection drops the table and all its auxiliary tables (<table>_*) and ref tables containing this table.
func (store *SqlStore) DeleteCollection(ctx context.Context, connectionId string, database string, collection string) error {
	// Drop main table.
	drop := fmt.Sprintf("DROP TABLE IF EXISTS %s", collection)
	if _, err := store.ExecContext(connectionId, database, drop, nil, 0); err != nil {
		return err
	}

	// Drop auxiliary and ref tables.
	list := "SELECT name FROM sqlite_master WHERE type='table'"
	str, err := store.QueryContext(connectionId, database, list, "[]")
	if err != nil {
		return err
	}

	data := make(map[string]interface{})
	if err := json.Unmarshal([]byte(str), &data); err != nil {
		return err
	}

	base := ensurePlural(collection)
	for _, values := range data["data"].([]interface{}) {
		for _, v := range values.([]interface{}) {
			name := v.(string)
			if strings.HasPrefix(name, collection+"_") {
				_, _ = store.ExecContext(connectionId, database, fmt.Sprintf("DROP TABLE IF EXISTS %s", name), nil, 0)
				continue
			}
			parts := strings.Split(name, "_")
			if len(parts) == 2 && (parts[0] == base || parts[1] == base) {
				_, _ = store.ExecContext(connectionId, database, fmt.Sprintf("DROP TABLE IF EXISTS %s", name), nil, 0)
			}
		}
	}

	return nil
}

// RunAdminCmd logs the request; not implemented for the SQLite variant.
func (store *SqlStore) RunAdminCmd(ctx context.Context, connectionId string, user string, password string, script string) error {
	slog.Info("RunAdminCmd (no-op)", "connectionId", connectionId, "user", user, "script", script)
	return nil
}
