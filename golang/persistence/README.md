# Persistence Service
Create Read Update and Delete entities... Persistence service give you all require functionalities to make all your applications persistent.
That microservice define a generic datastore that can be implement by almost any storage technologie. At this time only mongoDB is available,
but SQL is planned to be use to.

## Create a connection
In order to be able to get data from the data store the first step is to create a connection with it.
Connections can be save in the configuration file (config.json). If saved, connection will be
recreate automaticaly by the server start time. Note that you must set the password in the _config.json_ if authentication is enabled in your datastore.
The _CreateConnection_ method automaticaly open the new connection with the datastore, so you don't need to call _Connect_ method.

Here is how to create a connection. The required parameters are,
* The _connection id_
* The _database_ name
* The data store _domain_ 
* The data store _port_
* The data store type 0 is _mongoDB_
* The _user_
* The _password_
* The connection _timeout_
* The options string
* Does the connection must be store in the configuration file

_Go_
```go Golang
user := "sa"
pwd := "adminadmin"
err := client.CreateConnection("mongo_db_test_connection", "mongo_db_test_connection", "localhost", 27017, 0, user, pwd, 500, "", true)
```

## Connect
That method must be call to open an existing connection, a connection defined in the _congig.json_ file.

Here an exemple how to connect to the backend, The required parameters are,
* the _connection_id_
* the connection _password_

_Go_
```go
err := client.Connect("mongo_db_test_connection", "adminadmin")
if err != nil {
    log.Println("fail to connect to the backend with error ", err)
}
```

## Ping
That function can help to test if the backend is reachable and your connection setting's are correctly filled in. If ping 
fail an error message will be given. Before pingning the datastore be sure your connection was open by calling the _Connect_ function.

_Go_
```go
err := client.Ping("mongo_db_test_connection")
if err != nil {
    log.Fatalln("fail to ping the backend with error ", err)
}
```

## Insert One
With your connection opend and ready, you probably want to save some data. To store one entity at time, InsertOne must be use.
Note that if your entity contain a large number of data you must considere using _InsertMany_, because gRPC frame are limited 
in size and the serialized data can't be larger than that size.

Here an exemple how to persist employe data. If you made use of _mongoDB_ as backend, the database and the collection will
be create automaticaly at first insertion, otherwise you must explicitly create it by calling _CreateDatabase_ and _CreateCollection_
explicitly.

_Go_
```go
Id := "mongo_db_test_connection"
Database := "TestMongoDB"
Collection := "Employees"
employe := map[string]interface{}{
    "hire_date": "2007-07-01", 
    "last_name": "Courtois", 
    "first_name": "Dave", 
    "birth_date": "1976-01-28", 
    "emp_no": 200000, 
    "gender": "M"}
    
id, err := client.InsertOne(Id, Database, Collection, employe, "")

if err != nil {
    log.Fatalf("fail to pesist entity with error %v", err)
}

log.Println("Entity persist with id ", id)
```

## Insert many
If you have an array of objects to save, instead of repeatedly calling _InsertOne_ you must considere calling 
_InsertMany_. It's simpler, faster, and required less code. That function must also be use to store object larger
that the gRPC message size limit, 4mb by default. Object containing image for example can easily exeed that limit.

_Go_
```go
entities :=
    []interface{}{
        map[string]interface{}{
            "userId":            "rirani",
            "jobTitleName":      "Developer",
            "firstName":         "Romin",
            "lastName":          "Irani",
            "preferredFullName": "Romin Irani",
            "employeeCode":      "E1",
            "region":            "CA",
            "phoneNumber":       "408-1234567",
            "emailAddress":      "romin.k.irani@gmail.com",
        },
        map[string]interface{}{
            "userId":            "nirani",
            "jobTitleName":      "Developer",
            "firstName":         "Neil",
            "lastName":          "Irani",
            "preferredFullName": "Neil Irani",
            "employeeCode":      "E2",
            "region":            "CA",
            "phoneNumber":       "408-1111111",
            "emailAddress":      "neilrirani@gmail.com",
        },
        map[string]interface{}{
            "userId":            "thanks",
            "jobTitleName":      "Program Directory",
            "firstName":         "Tom",
            "lastName":          "Hanks",
            "preferredFullName": "Tom Hanks",
            "employeeCode":      "E3",
            "region":            "CA",
            "phoneNumber":       "408-2222222",
            "emailAddress":      "tomhanks@gmail.com",
        },
    }

Id := "mongo_db_test_connection"
Database := "TestCreateAndDelete_DB"
Collection := "Employees"

err := client.InsertMany(Id, Database, Collection, entities, "")
if err != nil {
    log.Fatalf("Fail to insert many entities whit error %v", err)
}
```
