package persistence_store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"
	"unicode"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/config"
	_ "github.com/mattn/go-sqlite3" // Import the sqlite3 driver
)

// SqlConnection represents a connection to a SQL database.
type SqlConnection struct {
	Id        string            // Unique identifier for the connection
	Host      string            // Host address of the SQL server
	Token     string            // Authentication token
	Path      string            // Path to the database file
	databases map[string]*sql.DB // Map of database connections
}

// SqlStore represents the SQL store.
type SqlStore struct {
	connections map[string]SqlConnection // Map of SQL connections
}

func (store *SqlStore) GetStoreType() string {
	return "SQL"
}

// Connect to the SQL database.
func (store *SqlStore) Connect(id string, host string, port int32, user string, password string, database string, timeout int32, options_str string) error {
	if len(id) == 0 {
		return errors.New("the connection id is required")
	}

	if store.connections == nil {
		store.connections = make(map[string]SqlConnection)
	}

	if len(host) == 0 {
		return errors.New("the host is required")
	}

	if len(user) == 0 {
		return errors.New("the user is required")
	}

	if len(database) == 0 {
		return errors.New("the database is required")
	}

	// Authenticate the user and password.
	authentication_client, err := authentication_client.NewAuthenticationService_Client(host, "authentication.AuthenticationService")
	if err != nil {
		return err
	}

	// Authenticate the user, try 5 times.
	nbTry := 5
	var token string
	for nbTry > 0 {
		var err error
		token, err = authentication_client.Authenticate(user, password)
		if err != nil && nbTry == 0 {
			return err
		} else if err == nil {
			break
		}
		nbTry--
		time.Sleep(1 * time.Second)
	}

	// Ensure the data path exists.
	options := make(map[string]interface{})
	if len(options_str) > 0 {
		err := json.Unmarshal([]byte(options_str), &options)
		if err != nil {
			return err
		}
	}

	// Set default path
	path := config.GetDataDir() + "/sql-data"
	if options["path"] != nil {
		path = options["path"].(string)
	}

	// Create the directory if it does not exist.
	Utility.CreateDirIfNotExist(path)

	// Create the connection.
	var connection SqlConnection
	if _, ok := store.connections[id]; ok {
		connection = store.connections[id]
	} else {
		connection = SqlConnection{
			Id:        id,
			Host:      host,
			Token:     token,
			Path:      path,
			databases: make(map[string]*sql.DB),
		}
		store.connections[id] = connection
	}

	// Create the database if it does not exist.
	databasePath := connection.Path + "/" + database + ".db"
	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		return err
	}
	connection.databases[database] = db

	// Create the table if it does not exist.
	count, _ := store.Count(context.Background(), id, "", "user_data", `SELECT * FROM user_data WHERE _id='`+user+`'`, "")
	if count == 0 && id != "local_resource" {
		store.InsertOne(context.Background(), id, database, "user_data", map[string]interface{}{"_id": user, "first_name": "", "last_name": "", "middle_name": "", "profile_picture": "", "email": ""}, "")
	}

	return nil
}

func (store *SqlStore) ExecContext(connectionId string, database string, query string, parameters []any, tx_ int) (string, error) {
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
		conn.databases = make(map[string]*sql.DB)
	}

	if conn.databases[database] == nil {
		databasePath := conn.Path + "/" + database + ".db"
		db, err := sql.Open("sqlite3", databasePath)
		if err != nil {
			return "", err
		}
		conn.databases[database] = db
	}

	hasTx := tx_ == 1
	var result sql.Result
	var err error

	if hasTx {
		tx, err := conn.databases[database].BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
		if err != nil {
			return "", err
		}
		result, err = tx.ExecContext(context.Background(), query, parameters...)
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				err = errors.New(fmt.Sprint("update failed: %v, unable to rollback: %v\n", err, rollbackErr))
			} else {
				err = errors.New(fmt.Sprint("update failed: %v", err))
			}
			return "", err
		}
		if err := tx.Commit(); err != nil {
			return "", err
		}
	} else {
		result, err = conn.databases[database].ExecContext(context.Background(), query, parameters...)
		if err != nil {
			return "", err
		}
	}

	numRowsAffected, err := result.RowsAffected()
	if err != nil {
		return "", err
	}

	lastId, _ := result.LastInsertId()
	return fmt.Sprintf("{\"lastId\": %d, \"rowsAffected\": %d}", lastId, numRowsAffected), nil
}

func (store *SqlStore) QueryContext(connectionId string, database string, query string, parameters_ string) (string, error) {
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
		conn.databases = make(map[string]*sql.DB)
	}

	if conn.databases[database] == nil {
		databasePath := conn.Path + "/" + database + ".db"
		db, err := sql.Open("sqlite3", databasePath)
		if err != nil {
			return "", err
		}
		conn.databases[database] = db
	}

	parameters := make([]interface{}, 0)
	if len(parameters_) > 0 {
		err := json.Unmarshal([]byte(parameters_), &parameters)
		if err != nil {
			return "", err
		}
	}

	rows, err := conn.databases[database].QueryContext(context.Background(), query, parameters...)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return "", err
	}

	columnsType, err := rows.ColumnTypes()
	if err != nil {
		return "", err
	}

	header := make([]interface{}, len(columns))
	for i := 0; i < len(columnsType); i++ {
		column := columns[i]
		typeInfo := make(map[string]interface{})
		typeInfo["DatabaseTypeName"] = columnsType[i].DatabaseTypeName()
		typeInfo["Name"] = columnsType[i].DatabaseTypeName()

		if precision, scale, isDecimal := columnsType[i].DecimalSize(); isDecimal {
			typeInfo["Scale"] = scale
			typeInfo["Precision"] = precision
		}

		if length, hasLength := columnsType[i].Length(); hasLength {
			typeInfo["Precision"] = length
		}

		isNull, isNullable := columnsType[i].Nullable()
		typeInfo["IsNullable"] = isNullable
		if isNullable {
			typeInfo["IsNull"] = isNull
		}

		header[i] = map[string]interface{}{"name": column, "typeInfo": typeInfo}
	}

	count := len(columns)
	values := make([]interface{}, count)
	scanArgs := make([]interface{}, count)
	for i := range values {
		scanArgs[i] = &values[i]
	}

	rows_ := make([]interface{}, 0)
	for rows.Next() {
		row := make([]interface{}, count)
		err := rows.Scan(scanArgs...)
		if err != nil {
			return "", err
		}

		for i, v := range values {
			if v == nil {
				row[i] = nil
			} else {
				if Utility.IsNumeric(v) {
					row[i] = Utility.ToNumeric(v)
				} else if Utility.IsBool(v) {
					row[i] = Utility.ToBool(v)
				} else {
					row[i] = Utility.ToString(v)
				}
			}
		}
		rows_ = append(rows_, row)
	}

	result := make(map[string]interface{}, 0)
	result["header"] = header
	result["data"] = rows_
	result_, _ := Utility.ToJson(result)
	return result_, nil
}

func (store *SqlStore) Disconnect(connectionId string) error {
	_, exists := store.connections[connectionId]
	if !exists {
		return fmt.Errorf("connection with ID %s does not exist", connectionId)
	}

	for _, db := range store.connections[connectionId].databases {
		err := db.Close()
		if err != nil {
			return err
		}
	}

	delete(store.connections, connectionId)
	fmt.Println("Disconnected from SQL server", connectionId)
	return nil
}

func (store *SqlStore) Ping(ctx context.Context, connectionId string) error {
	return nil
}

func (store *SqlStore) CreateDatabase(ctx context.Context, connectionId string, name string) error {
	return errors.New("not implemented")
}

func (store *SqlStore) DeleteDatabase(ctx context.Context, connectionId string, db string) error {
	if len(db) == 0 {
		return errors.New("the database name is required")
	}
	databasePath := store.connections[connectionId].Path + "/" + db + ".db"
	return os.RemoveAll(databasePath)
}

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

	data := make(map[string]interface{}, 0)
	err = json.Unmarshal([]byte(str), &data)
	if err != nil {
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

	result := make(map[string]interface{}, 0)
	err = json.Unmarshal([]byte(str), &result)
	if err != nil {
		return false
	}

	return len(result["data"].([]interface{})) > 0
}

func generateCreateTableSQL(tableName string, columns map[string]interface{}) (string, []string) {
	var columnsSQL []string
	var arrayTables []string

	for columnName, columnType := range columns {
		if columnType != nil {
			sqlType := getSQLType(reflect.TypeOf(columnType))
			isArray := reflect.Slice == reflect.TypeOf(columnType).Kind()

			if !isArray {
				if columnName != "typeName" {
					if columnName == "id" {
						columnName = "_id"
					}
					if columnName != "_id" {
						columnNameFormatted := fmt.Sprintf("\"%s\"", columnName)
						columnsSQL = append(columnsSQL, fmt.Sprintf("%s %s", columnNameFormatted, sqlType))
					}
				}
			} else {
				arrayTables = append(arrayTables, fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s_%s (value %s, %s_id TEXT, FOREIGN KEY (%s_id) REFERENCES %s(_id) ON DELETE CASCADE);", tableName, columnName, sqlType, tableName, tableName, tableName))
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

func generateMainInsertSQL(tableName string, data map[string]interface{}) (string, []interface{}) {
	var mainColumns []string
	var mainPlaceholders []string
	var mainValues []interface{}

	for columnName, columnValue := range data {
		if columnValue != nil {
			isArray := reflect.Slice == reflect.TypeOf(columnValue).Kind()
			if !isArray && columnName != "typeName" {
				if columnName == "id" {
					columnName = "_id"
				}
				mainColumns = append(mainColumns, fmt.Sprintf("\"%s\"", columnName))
				mainPlaceholders = append(mainPlaceholders, "?")
				mainValues = append(mainValues, columnValue)
			}
		}
	}

	mainInsertSQL := fmt.Sprintf("INSERT INTO \"%s\" (%s) VALUES (%s);", tableName, strings.Join(mainColumns, ", "), strings.Join(mainPlaceholders, ", "))
	return mainInsertSQL, mainValues
}

func (store *SqlStore) insertData(connectionId string, db string, tableName string, data map[string]interface{}) (map[string]interface{}, error) {
	var id string
	if data["id"] != nil {
		id = Utility.ToString(data["id"])
	} else if data["_id"] != nil {
		id = Utility.ToString(data["_id"])
	} else if data["Uid"] != nil {
		id = Utility.ToString(data["Uid"])
	} else if data["uid"] != nil {
		id = Utility.ToString(data["uid"])
	} else if data["uuid"] != nil {
		id = Utility.ToString(data["uuid"])
	} else if data["UUID"] != nil {
		id = Utility.ToString(data["UUID"])
	}

	if len(id) == 0 {
		return nil, errors.New("the id is required to insert data into the database")
	}

	data["_id"] = id

	if store.isTableExist(connectionId, db, tableName) {
		query := fmt.Sprintf("SELECT * FROM %s WHERE _id='%s'", tableName, id)
		values, err := store.FindOne(context.Background(), connectionId, db, tableName, query, "")
		if err == nil {
			return values.(map[string]interface{}), nil
		}
	}

	insertSQL, values := generateMainInsertSQL(tableName, data)
	str, err := store.ExecContext(connectionId, db, insertSQL, values, 0)
	if err != nil {
		return nil, err
	}

	result := make(map[string]interface{}, 0)
	err = json.Unmarshal([]byte(str), &result)
	if err != nil {
		return nil, err
	}

	for columnName, columnValue := range data {
		if columnValue != nil {
			if reflect.Slice == reflect.TypeOf(columnValue).Kind() {
				arrayTableName := tableName + "_" + columnName
				sliceValue := reflect.ValueOf(data[columnName])
				length := sliceValue.Len()
				for i := 0; i < length; i++ {
					element := sliceValue.Index(i)
					arrayInsertSQL := fmt.Sprintf("INSERT INTO %s (value, %s_id) VALUES (?, ?);", arrayTableName, tableName)
					parameters := []interface{}{element.Interface(), id}
					_, err := store.ExecContext(connectionId, db, arrayInsertSQL, parameters, 0)
					if err != nil {
						return nil, err
					}
				}
			}
		}
	}

	return data, nil
}

func (store *SqlStore) InsertOne(ctx context.Context, connectionId string, db string, table string, entity interface{}, options string) (interface{}, error) {
	entity_, err := Utility.ToMap(entity)
	if err != nil {
		return nil, err
	}

	if !store.isTableExist(connectionId, db, table) {
		createTableSQL, arrayTableSQL := generateCreateTableSQL(table, entity_)
		_, err = store.ExecContext(connectionId, db, createTableSQL, nil, 0)
		if err != nil {
			return nil, err
		}

		for _, sql := range arrayTableSQL {
			_, err := store.ExecContext(connectionId, db, sql, nil, 0)
			if err != nil {
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

func (store *SqlStore) InsertMany(ctx context.Context, connectionId string, db string, table string, entities []interface{}, options string) ([]interface{}, error) {
	if !store.isTableExist(connectionId, db, table) {
		entity_, err := Utility.ToMap(entities[0])
		if err != nil {
			return nil, err
		}

		createTableSQL, arrayTableSQL := generateCreateTableSQL(table, entity_)
		_, err = store.ExecContext(connectionId, db, createTableSQL, nil, 0)
		if err != nil {
			return nil, err
		}

		for _, sql := range arrayTableSQL {
			_, err := store.ExecContext(connectionId, db, sql, nil, 0)
			if err != nil {
				return nil, err
			}
		}
	}

	var results []interface{}
	for _, entity := range entities {
		entity_, err := Utility.ToMap(entity)
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

func isIntegerType(sqlType string) bool {
	return sqlType == "INTEGER"
}

func getIntValue(value interface{}) (int32, error) {
	v := Utility.ToInt(value)
	return int32(v), nil
}

func (store *SqlStore) recreateArrayOfObjects(connectionId, db, tableName string, dataHeader map[string]interface{}, options []map[string]interface{}) ([]interface{}, error) {
	data := dataHeader["data"]
	header := dataHeader["header"]

	var objects []interface{}
	var projection map[string]interface{}
	if len(options) > 0 {
		for _, option := range options {
			if option["Projection"] != nil {
				projection = option["Projection"].(map[string]interface{})
				projection["_id"] = 1
			}
		}
	}

	for _, dataRow := range data.([]interface{}) {
		dataRow := dataRow.([]interface{})
		object := make(map[string]interface{}, 0)
		object["typeName"] = tableName

		for index, fieldInfos := range header.([]interface{}) {
			fieldInfos := fieldInfos.(map[string]interface{})
			fieldName := fieldInfos["name"].(string)
			value := dataRow[index]
			typeInfos := fieldInfos["typeInfo"].(map[string]interface{})

			if projection != nil {
				if projection[fieldName] != nil {
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
			} else {
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
		}

		objects = append(objects, object)

		query := "SELECT name FROM sqlite_master WHERE type='table'"
		str, err := store.QueryContext(connectionId, db, query, "[]")
		if err != nil {
			return nil, err
		}

		tables := make(map[string]interface{}, 0)
		err = json.Unmarshal([]byte(str), &tables)
		if err != nil {
			return nil, err
		}

		domain, _ := config.GetDomain()
		for _, values := range tables["data"].([]interface{}) {
			for _, value := range values.([]interface{}) {
				field := strings.Replace(value.(string), tableName+"_", "", 1)
				if strings.HasPrefix(value.(string), tableName+"_") && object[field] == nil {
					query := fmt.Sprintf("SELECT value FROM %s WHERE %s_id=?", value.(string), tableName)
					parameters := []interface{}{object["_id"]}
					parameters_, _ := Utility.ToJson(parameters)
					str, err := store.QueryContext(connectionId, db, query, parameters_)
					if err == nil {
						data := make(map[string]interface{}, 0)
						err = json.Unmarshal([]byte(str), &data)
						if err != nil {
							return nil, err
						}
						object[field] = make([]interface{}, 0)
						for _, values := range data["data"].([]interface{}) {
							value := values.([]interface{})[0]
							object[field] = append(object[field].([]interface{}), value)
						}
					} else {
						query := fmt.Sprintf("SELECT * FROM %s WHERE source_id=?", value.(string))
						parameters := []interface{}{object["_id"]}
						parameters_, _ := Utility.ToJson(parameters)
						str, err := store.QueryContext(connectionId, db, query, parameters_)
						if err == nil {
							data := make(map[string]interface{}, 0)
							err = json.Unmarshal([]byte(str), &data)
							if err == nil {
								for _, values := range data["data"].([]interface{}) {
									ref_id := Utility.ToString(values.([]interface{})[1])
									if strings.Contains(ref_id, "@") {
										ref_id = strings.Split(ref_id, "@")[0]
										if strings.Split(ref_id, "@")[0] != domain {
											continue
										}
									}
									bytes := []byte(field)
									bytes[0] = byte(unicode.ToUpper(rune(bytes[0])))
									typeName := string(bytes)
									if typeName == "Members" {
										typeName = "Accounts"
									}
									if object[field] == nil {
										object[field] = make([]interface{}, 0)
									}
									object[field] = append(object[field].([]interface{}), map[string]interface{}{"$ref": typeName, "$id": ref_id, "$db": db})
								}
							}
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
		query = fmt.Sprintf("SELECT * FROM %s", table)
	} else {
		parameters := make(map[string]interface{}, 0)
		err := json.Unmarshal([]byte(query), &parameters)
		if err != nil {
			return "", err
		}

		query = fmt.Sprintf("SELECT * FROM %s WHERE ", table)
		for key, value := range parameters {
			if key == "id" {
				key = "_id"
			}
			if reflect.TypeOf(value).Kind() == reflect.String {
				query += fmt.Sprintf("%s = '%v' AND ", key, value)
			} else if reflect.TypeOf(value).Kind() == reflect.Slice {
				if key == "$and" || key == "$or" {
					query += store.getParameters(key, value.([]interface{}))
				}
			} else if reflect.TypeOf(value).Kind() == reflect.Map {
				for k, v := range value.(map[string]interface{}) {
					if k == "$regex" {
						query += fmt.Sprintf("%s LIKE '%v%%' AND ", key, v)
					}
				}
			}
		}
		query = strings.TrimSuffix(query, " AND ")
	}

	return query, nil
}

func (store *SqlStore) FindOne(ctx context.Context, connectionId string, database string, table string, query string, options string) (interface{}, error) {
	if len(query) == 0 {
		return nil, errors.New("query is empty")
	} else if strings.HasPrefix(query, "{") && strings.HasSuffix(query, "}") {
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

	data := make(map[string]interface{}, 0)
	err = json.Unmarshal([]byte(str), &data)
	if err != nil {
		return nil, err
	}

	options_ := make([]map[string]interface{}, 0)
	if len(options) > 0 {
		err = json.Unmarshal([]byte(options), &options_)
		if err != nil {
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
			for key, v := range value {
				if reflect.TypeOf(v).Kind() == reflect.String {
					query += fmt.Sprintf("%s = '%v' AND ", key, v)
				}
			}
		}
		query = strings.TrimSuffix(query, " AND ")
		query += ")"
	}
	return query
}

func (store *SqlStore) Find(ctx context.Context, connectionId string, db string, table string, query string, options string) ([]interface{}, error) {
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

	data := make(map[string]interface{}, 0)
	err = json.Unmarshal([]byte(str), &data)
	if err != nil {
		return nil, err
	}

	options_ := make([]map[string]interface{}, 0)
	if len(options) > 0 {
		err = json.Unmarshal([]byte(options), &options_)
		if err != nil {
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

	return []interface{}{}, nil
}

func (store *SqlStore) ReplaceOne(ctx context.Context, connectionId string, db string, table string, query string, value string, options string) error {
	entity := make(map[string]interface{}, 0)
	err := json.Unmarshal([]byte(value), &entity)
	if err != nil {
		return err
	}

	if !store.isTableExist(connectionId, db, table) {
		createTableSQL, arrayTableSQL := generateCreateTableSQL(table, entity)
		_, err := store.ExecContext(connectionId, db, createTableSQL, nil, 0)
		if err != nil {
			return err
		}

		for _, sql := range arrayTableSQL {
			_, err := store.ExecContext(connectionId, db, sql, nil, 0)
			if err != nil {
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

	store.deleteOneSqlEntry(connectionId, db, table, query)
	_, err = store.insertData(connectionId, db, table, entity)
	if err != nil {
		return err
	}

	return nil
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

func (store *SqlStore) Update(ctx context.Context, connectionId string, db string, table string, query string, value string, options string) error {
	values_ := make(map[string]interface{}, 0)
	err := json.Unmarshal([]byte(value), &values_)
	if err != nil {
		return err
	}

	if values_["$set"] == nil {
		return errors.New("no $set operator allowed in Update")
	}

	query, err = store.formatQuery(table, query)
	if err != nil {
		return err
	}

	fields := make([]interface{}, 0)
	values := make([]interface{}, 0)
	for key, value := range values_["$set"].(map[string]interface{}) {
		if reflect.TypeOf(value).Kind() != reflect.Slice {
			fields = append(fields, key)
			values = append(values, value)
		}
	}

	updateQuery, err := generateUpdateTableQuery(table, fields, query)
	if err != nil {
		return err
	}

	_, err = store.ExecContext(connectionId, db, updateQuery, values, 0)
	if err != nil {
		return err
	}

	currentEntity, err := store.FindOne(context.Background(), connectionId, db, table, query, "")
	if err != nil {
		return err
	}

	for columnName, columnValue := range values_["$set"].(map[string]interface{}) {
		if columnValue != nil && reflect.Slice == reflect.TypeOf(columnValue).Kind() {
			arrayTableName := table + "_" + columnName
			if store.isTableExist(connectionId, db, arrayTableName) {
				deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE %s_id=?", arrayTableName, table)
				parameters := []interface{}{currentEntity.(map[string]interface{})["_id"]}
				_, err := store.ExecContext(connectionId, db, deleteQuery, parameters, 0)
				if err != nil {
					return err
				}

				sliceValue := reflect.ValueOf(columnValue)
				length := sliceValue.Len()
				for i := 0; i < length; i++ {
					element := sliceValue.Index(i)
					insertQuery := fmt.Sprintf("INSERT INTO %s (value, %s_id) VALUES (?, ?);", arrayTableName, table)
					parameters := []interface{}{element.Interface(), currentEntity.(map[string]interface{})["_id"]}
					_, err := store.ExecContext(connectionId, db, insertQuery, parameters, 0)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func (store *SqlStore) UpdateOne(ctx context.Context, connectionId string, db string, table string, query string, value string, options string) error {
	values_ := make(map[string]interface{}, 0)
	err := json.Unmarshal([]byte(value), &values_)
	if err != nil {
		return err
	}

	if values_["$set"] == nil {
		return errors.New("no $set operator allowed in UpdateOne")
	}

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
	for key, value := range values_["$set"].(map[string]interface{}) {
		if reflect.TypeOf(value).Kind() != reflect.Slice {
			fields = append(fields, key)
			values = append(values, value)
		}
	}

	updateQuery, err := generateUpdateTableQuery(table, fields, query)
	if err != nil {
		return err
	}

	_, err = store.ExecContext(connectionId, db, updateQuery, values, 0)
	if err != nil {
		return err
	}

	for columnName, columnValue := range values_["$set"].(map[string]interface{}) {
		if columnValue != nil && reflect.Slice == reflect.TypeOf(columnValue).Kind() {
			arrayTableName := table + "_" + columnName
			if store.isTableExist(connectionId, db, arrayTableName) {
				deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE %s_id=?", arrayTableName, table)
				parameters := []interface{}{currentEntity.(map[string]interface{})["_id"]}
				_, err := store.ExecContext(connectionId, db, deleteQuery, parameters, 0)
				if err != nil {
					return err
				}

				sliceValue := reflect.ValueOf(columnValue)
				length := sliceValue.Len()
				for i := 0; i < length; i++ {
					element := sliceValue.Index(i)
					insertQuery := fmt.Sprintf("INSERT INTO %s (value, %s_id) VALUES (?, ?);", arrayTableName, table)
					parameters := []interface{}{element.Interface(), currentEntity.(map[string]interface{})["_id"]}
					_, err := store.ExecContext(connectionId, db, insertQuery, parameters, 0)
					if err != nil {
						return err
					}
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
		if entity.(map[string]interface{})["_id"] != nil {
			query := fmt.Sprintf("SELECT * FROM '%s' WHERE _id='%s'", table, entity.(map[string]interface{})["_id"].(string))
			err := store.deleteOneSqlEntry(connectionId, db, table, query)
			if err != nil {
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
		var err error
		query, err = store.formatQuery(table, query)
		if err != nil {
			return err
		}
	}

	query = strings.Replace(query, "SELECT *", "DELETE", 1)
	_, err = store.ExecContext(connectionId, db, query, nil, 0)
	if err != nil {
		return err
	}

	for columnName, columnValue := range entity.(map[string]interface{}) {
		if columnValue != nil {
			if reflect.Slice == reflect.TypeOf(columnValue).Kind() {
				arrayTableName := table + "_" + columnName
				deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE %s_id=?", arrayTableName, table)
				parameters := []interface{}{entity.(map[string]interface{})["_id"]}
				_, err := store.ExecContext(connectionId, db, deleteQuery, parameters, 0)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (store *SqlStore) Delete(ctx context.Context, connectionId string, db string, table string, query string, options string) error {
	return store.deleteSqlEntries(connectionId, db, table, query)
}

func (store *SqlStore) DeleteOne(ctx context.Context, connectionId string, db string, table string, query string, options string) error {
	return store.deleteOneSqlEntry(connectionId, db, table, query)
}

func (store *SqlStore) Aggregate(ctx context.Context, connectionId string, keyspace string, table string, pipeline string, optionsStr string) ([]interface{}, error) {
	return nil, errors.New("not implemented")
}

func (store *SqlStore) CreateTable(ctx context.Context, connectionId string, db string, table string, fields []string) error {
	createTableSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS \"%s\" (_id TEXT PRIMARY KEY, %s);", table, strings.Join(fields, ", "))
	_, err := store.ExecContext(connectionId, db, createTableSQL, nil, 0)
	if err != nil {
		return err
	}
	return nil
}

func (store *SqlStore) CreateCollection(ctx context.Context, connectionId string, database string, name string, optionsStr string) error {
	return errors.New("not implemented")
}

func (store *SqlStore) DeleteCollection(ctx context.Context, connectionId string, database string, collection string) error {
	query := fmt.Sprintf("DROP TABLE IF EXISTS %s", collection)
	_, err := store.ExecContext(connectionId, database, query, nil, 0)
	if err != nil {
		return err
	}

	query = fmt.Sprintf("SELECT name FROM sqlite_master WHERE type='table'")
	str, err := store.QueryContext(connectionId, database, query, "[]")
	if err != nil {
		return err
	}

	data := make(map[string]interface{}, 0)
	err = json.Unmarshal([]byte(str), &data)
	if err != nil {
		return err
	}

	for _, values := range data["data"].([]interface{}) {
		for _, value := range values.([]interface{}) {
			if strings.HasPrefix(value.(string), collection+"_") {
				query := fmt.Sprintf("DROP TABLE IF EXISTS %s", value.(string))
				_, err := store.ExecContext(connectionId, database, query, nil, 0)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (store *SqlStore) RunAdminCmd(ctx context.Context, connectionId string, user string, password string, script string) error {
	fmt.Println("RunAdminCmd ", connectionId, user, password, script)
	return nil
}
