# Persistence Service
Create Read Update and Delete entities... Persistence service give you all required functionalities that make all your application entities persistent.
That microservice define a generic datastore that can be implement by almost any storage technologie. At this time only mongoDB is available,
but SQL is planned to be done next.

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
err := client.CreateConnection("connection_id", "connection_id", "localhost", 27017, 0, user, pwd, 500, "", true)
```

## Connect
That method must be call to open an existing connection, a connection defined in the _congig.json_ file.

Here an exemple how to connect to the backend, The required parameters are,
* the _connection_id_
* the connection _password_

_Go_
```go
err := client.Connect("connection_id", "adminadmin")
if err != nil {
    log.Println("fail to connect to the backend with error ", err)
}
```

## Ping
That function can help to test if the backend is reachable and your connection setting's are correctly filled in. If ping 
fail an error message will be given. Before pingning the datastore be sure your connection was open by calling the _Connect_ function.

The parameters are,
* The connection _id_

_Go_
```go
err := client.Ping("connection_id")
if err != nil {
    log.Fatalln("fail to ping the backend with error ", err)
}
```

## Insert One
With your connection opend and ready, you probably want to save some data. To store one entity at time, InsertOne must be use.
Note that if your entity contain a large number of data you must considere using _InsertMany_, because gRPC frame are limited 
in size and the serialized data can't be larger than that size. It will return an error if already existing entity is present.
You can use _upsert_ option to replace existing entity.

Here an exemple how to persist employe data. If you made use of _mongoDB_ as backend, the database and the collection will
be create automaticaly at first insertion, otherwise you must explicitly create it by calling _CreateDatabase_ and _CreateCollection_
explicitly.

_Go_
```go
Id := "connection_id"
Database := "TestMongoDB"
Collection := "Employees"

employe := map[string]interface{}{
    "hire_date": "2007-07-01", 
    "last_name": "Courtois", 
    "first_name": "Dave", 
    "birth_date": "1976-01-28", 
    "emp_no": 200000, 
    "gender": "M"}
    
id, err := client.InsertOne(Id, Database, Collection, employe, `[{"upsert":true}]`)

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

Id := "connection_id"
Database := "TestDB"
Collection := "Employees"

err := client.InsertMany(Id, Database, Collection, entities, "")
if err != nil {
    log.Fatalf("Fail to insert many entities whit error %v", err)
}
```

## Replace One
Instead of delete and entity and create it, you must use replace one. If you want to change an object propertie
you must considere using _UpdateOne_ instead. You can also accomplish same thing with _InsertOne_ with the _upsert_ option set to true.

```go
Id := "connection_id"
Database := "TestDB"
Collection := "Employees"

entity := map[string]interface{}{
    "_id":               "nirani",
    "jobTitleName":      "Full Stack Developper",
    "firstName":         "Neil",
    "lastName":          "Irani",
    "preferredFullName": "Neil Irani",
    "employeeCode":      "E2",
    "region":            "CA",
    "phoneNumber":       "408-1111111",
    "emailAddress":      "neilrirani@gmail.com"}

err := client.ReplaceOne(Id, Database, Collection, `{"_id":"nirani"}`, entity, "")
if err != nil {
    log.Fatalf("Fail to replace entity %v", err)
}
```

## Update One
This is the function to use to replace one or more entity attributes values. If you want to replace almost all values of an entity you must considere using _ReplaceOne_ method instead.

```go
Id := "connection_id"
Database := "TestDB"
Collection := "Employees"

err := client.UpdateOne(Id, Database, Collection, `{"_id":"nirani"}`, `{ "$set":{"employeeCode":"E2.2"},"$set":{"phoneNumber":"408-1231234"}}`, "")
if err != nil {
    log.Fatalf("Fail to update entity %v", err)
}
```

## Update (Many)
If you want to change property or properties of many entities at once _Update_ method is the way to do it. 

Here for exemple I will append new attribute name _sate_ and set it value to _California_ for all employe with _region_ attribute with value _CA_.
```go
Id := "connection_id"
Database := "TestDB"
Collection := "Employees"
Query := `{"region": "CA"}`
Value := `{"$set":{"state":"California"}}`

err := client.Update(Id, Database, Collection, Query, Value, "")
```

## Find One
To find one object at time you can use _FindOne_. That function return an entity if it exist or an error if not.
The _Query_ parameter is will depend of the backend, I the fowlling example the backend is _mongoDB_. Note that the entity size must not
exeeded the gRPC message size limit, if so made use of _Find_ insetead.

```go
Id := "connection_id"
Database := "TestDB"
Collection := "Employees"
Query := `{"first_name": "Dave"}`

values, err := client.FindOne(Id, Database, Collection, Query, "")
if err != nil {
    log.Fatalf("TestFind fail %v", err)
}
```

## Find (Many)
If you need to get more than one entity, _Find_ is the method to use. That method can also be use to find large entity, entity larger than
the gRPC message size limit (4mb by default). It made use of a stream to return values so ther is no restriction on size.

Here's an exemple of how to find all employe from _CA region_.
```go
Id := "connection_id"
Database := "TestDB"
Collection := "Employees"
Query := `{"region": "CA"}`

values, err := client.Find(Id, Database, Collection, Query, `[{"Projection":{"firstName":1}}]`)
if err != nil {
    log.Fatalf("fail to find entities with error %v", err)
}
```

## Aggregate
If you need to transform data _Aggregate_ is the way to go. The way to make the aggragation will depend of 
the datastore.

```go
Id := "connection_id"
Database := "TestDB"
Collection := "Employees"

results, err := client.Aggregate(Id, Database, Collection, `[{"$count":"region"}]`, "")
if err != nil {
    log.Fatalf("fail to create aggregation with error %v", err)
}
```

## Delete One
To delete exactly one entity from the data store.

```go
Id := "mongo_db_test_connection"
Database := "TestCreateAndDelete_DB"
Collection := "Employees"
Query := `{"_id":"nirani"}`

err := client.DeleteOne(Id, Database, Collection, Query, "")
if err != nil {
    log.Fatalf("Fail to delete one entity with error %v", err)
}

```

## Delete (Many)
To delete more that one entity at time _Delete_ is the method to use.
There's an example how to do it,

```go
Id := "mongo_db_test_connection"
Database := "TestCreateAndDelete_DB"
Collection := "Employees"
Query := `{"region": "CA"}`

err := client.Delete(Id, Database, Collection, Query, "")
if err != nil {
    log.Fatalf("Fail to remove entities %v", err)
}
```


## Disconnect
That method must be call to close a connection.

Here an exemple how to connect to the backend, The required parameters are,
* the _connection_id_

_Go_
```go
err := client.Disconnect("connection_id")
if err != nil {
    log.Println("fail to disconnect from the backend with error ", err)
}
```

## Delete Collection
Here's the way to drop a collection

```go
Id := "test_connection"
Database := "TestDB"
Collection := "Employees"
err := client.DeleteCollection(Id, Database, Collection)
if err != nil {
    log.Println("fail to delete collection! ", err)
}
```

## Delete Database
Here's the way to drop a database

```go
Id := "test_connection"
Database := "TestDB"
err := client.DeleteDatabase(Id, Database)
if err != nil {
    log.Println("fail to delete database! ", err)
}
```

## Delete Connection
There is how you can delete a connection. You can alson remove a connection 

```go
err := client.DeleteConnection("mongo_db_test_connection")
if err != nil {
    log.Println("fail to delete connection! ", err)
}
```