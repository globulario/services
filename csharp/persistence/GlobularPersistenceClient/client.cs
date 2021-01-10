using Grpc.Core;
using System;
using System.Threading.Tasks;
using System.Collections;
using System.Text.Json;
using Google.Protobuf.WellKnownTypes;
using Google.Protobuf;

// TODO fix Find, Aggregate and InsertMany

namespace Globular
{
    public class PersistenceClient : Client
    {
        // combine tow array of bytes.
        private static byte[] Combine(byte[] first, byte[] second)
        {
            byte[] ret = new byte[first.Length + second.Length];
            Buffer.BlockCopy(first, 0, ret, 0, first.Length);
            Buffer.BlockCopy(second, 0, ret, first.Length, second.Length);
            return ret;
        }
        private Persistence.PersistenceService.PersistenceServiceClient client;

        /// <summary>
        /// gRPC client for persistence service.
        /// </summary>
        /// <param name="id"></param> The name or the id of the services.
        /// <param name="address"></param> ex.<!-- localhost:80 or globular.<inheritdoc />-->
        /// <returns>Return the instance of the client with it connection ready to be use.</returns>
        public PersistenceClient(string id, string address) : base(id, address)
        {
            // Here I will create grpc connection with the service...
            this.client = new Persistence.PersistenceService.PersistenceServiceClient(this.channel);
        }

        /// <summary>
        /// Create a new persistence connection
        /// </summary>
        /// <param name="connection">The connection information</param>
        /// <param name="save">If true the connection will be save in the configuation file.</param>
        public void CreateConnection(Persistence.Connection connection, bool save, string token = "", string application = "")
        {
            // Here I will create the new connection.
            Persistence.CreateConnectionRqst rqst = new Persistence.CreateConnectionRqst();
            rqst.Connection = connection;
            rqst.Save = save;

            // Create a new connection
            this.client.CreateConnection(rqst, this.GetClientContext(token, application));
        }

        /// <summary>
        /// Delete a connection with a given id.
        /// </summary>
        /// <param name="connectionId">The connection to delete</param>
        public void DeleteConnection(string connectionId, string token = "", string application = "")
        {
            var rqst = new Persistence.DeleteConnectionRqst();
            rqst.Id = connectionId;
            this.client.DeleteConnection(rqst, this.GetClientContext(token, application));
        }

        /// <summary>
        /// Open a connection with the datastore.
        /// </summary>
        /// <param name="connectionId"></param>
        public void Connect(string connectionId, string token = "", string application = "")
        {
            var rqst = new Persistence.ConnectRqst();
            rqst.ConnectionId = connectionId;
            this.client.Connect(rqst, this.GetClientContext(token, application));
        }

        /// <summary>
        /// Disconnect from the  server.
        /// </summary>
        /// <param name="connectionId">The connection id</param>
        public void Disconnect(string connectionId, string token = "", string application = "")
        {
            var rqst = new Persistence.DisconnectRqst();
            rqst.ConnectionId = connectionId;
            this.client.Disconnect(rqst, this.GetClientContext(token, application));
        }

        /// <summary>
        /// Ping a given persistence service.
        /// </summary>
        /// <param name="connectionId">The connection id, not it name.</param>
        /// <returns>Must return 'pong'</returns>
        public string Ping(string connectionId, string token = "", string application = "")
        {
            // Here I will create the new connection.
            Persistence.PingConnectionRqst rqst = new Persistence.PingConnectionRqst();
            rqst.Id = connectionId;

            // Create a new connection
            var rsp = this.client.Ping(rqst, this.GetClientContext(token, application));
            return rsp.Result;
        }

        ///////////////////////////////////// Quering /////////////////////////////////////

        /// <summary>
        /// Find one object from the database.
        /// </summary>
        /// <param name="connectionId">The connection id</param>
        /// <param name="database">The database name</param>
        /// <param name="collection">The collection name</param>
        /// <param name="query">The filter</param>
        /// <param name="options">a list of option, must be a json array</param>
        /// <returns></returns>
        public Struct FindOne(string connectionId, string database, string collection, string query, string options, string token = "", string application = "")
        {
            var rqst = new Persistence.FindOneRqst();
            rqst.Id = connectionId;
            rqst.Database = database;
            rqst.Collection = collection;
            rqst.Query = query;
            rqst.Options = options;

            var rsp = this.client.FindOne(rqst, this.GetClientContext(token, application));
            return rsp.Result;
        }

        /// <summary>
        /// Find multiple values from the data store.
        /// </summary>
        /// <param name="connectionId">The connection Id to be used</param>
        /// <param name="database">The database name</param>
        /// <param name="collection">The collection name</param>
        /// <param name="query">The query</param>
        /// <param name="options">a list of option, must be a json array</param>
        /// <returns></returns>
        public byte[] Find(string connectionId, string database, string collection, string query, string options, string token = "", string application = "")
        {
            var rqst = new Persistence.FindRqst();
            rqst.Id = connectionId;
            rqst.Database = database;
            rqst.Collection = collection;
            rqst.Query = query;
            rqst.Options = options;

            var call = this.client.Find(rqst, this.GetClientContext(token, application));

            // Make the function synchrone...
            byte[] data = new byte[0];
            bool hasNext = true;

            // read until no more values found...
            while (hasNext)
            {
                var task = Task.Run(() => call.ResponseStream.MoveNext());
                task.Wait(); // wait for the next value...
                hasNext = task.Result;
                if (hasNext)
                {
                    // string str = call.ResponseStream.Current.Data;
                    data = Combine(data, call.ResponseStream.Current.Data.ToByteArray());
                }
            }

            return data;
        }

        public byte[] Aggregate(string connectionId, string database, string collection, string pipeline, string options, string token = "", string application = "")
        {
            var rqst = new Persistence.AggregateRqst();
            rqst.Id = connectionId;
            rqst.Database = database;
            rqst.Collection = collection;
            rqst.Pipeline = pipeline;
            rqst.Options = options;

            var call = this.client.Aggregate(rqst, this.GetClientContext(token, application));

            // Make the function synchrone...
            byte[] data = new byte[0];
            bool hasNext = true;

            // read until no more values found...
            while (hasNext)
            {
                var task = Task.Run(() => call.ResponseStream.MoveNext());
                task.Wait(); // wait for the next value...
                hasNext = task.Result;
                if (hasNext)
                {
                    // string str = call.ResponseStream.Current.Data;
                    data = Combine(data, call.ResponseStream.Current.Data.ToByteArray());
                }
            }

            return data;
        }

        /// <summary>
        /// Count the number of document that match a given query
        /// </summary>
        /// <param name="connectionId">The connection id</param>
        /// <param name="database">The datase</param>
        /// <param name="collection">The collection</param>
        /// <param name="query">The query</param>
        /// <param name="options">A list of options in form of json string</param>
        /// <returns></returns>
        public long Count(string connectionId, string database, string collection, string query, string options, string token = "", string application = "")
        {
            var rqst = new Persistence.CountRqst();
            rqst.Id = connectionId;
            rqst.Database = database;
            rqst.Collection = collection;
            rqst.Query = query;
            rqst.Options = options;

            var rsp = this.client.Count(rqst, this.GetClientContext(token, application));
            return rsp.Result;
        }

        /// <summary>
        /// Insert one document in the database and return the newly create document id.
        /// </summary>
        /// <param name="connectionId">The connection id</param>
        /// <param name="database">The database name</param>
        /// <param name="collection">The collection</param>
        /// <param name="jsonStr">The oject stringnify value</param>
        /// <param name="options">The options</param>
        /// <returns></returns>
        public string InsertOne(string connectionId, string database, string collection, string jsonStr, string options, string token = "", string application = "")
        {
            var rqst = new Persistence.InsertOneRqst();
            rqst.Id = connectionId;
            rqst.Database = database;
            rqst.Collection = collection;
            rqst.Data = jsonStr;
            rqst.Options = options;

            var rsp = this.client.InsertOne(rqst, this.GetClientContext(token, application));
            return rsp.Id;
        }

        public void ReplaceOne(string connectionId, string database, string collection, string query, string value, string options, string token = "", string application = "")
        {
            var rqst = new Persistence.ReplaceOneRqst();
            rqst.Id = connectionId;
            rqst.Database = database;
            rqst.Collection = collection;
            rqst.Query = query;
            rqst.Value = value;
            rqst.Options = options;

            this.client.ReplaceOne(rqst, this.GetClientContext(token, application));
        }

        public void UpdateOne(string connectionId, string database, string collection, string query, string value, string options, string token = "", string application = "")
        {
            var rqst = new Persistence.UpdateOneRqst();
            rqst.Id = connectionId;
            rqst.Database = database;
            rqst.Collection = collection;
            rqst.Query = query;
            rqst.Value = value;
            rqst.Options = options;

            this.client.UpdateOne(rqst, this.GetClientContext(token, application));
        }


        public void Update(string connectionId, string database, string collection, string query, string value, string options, string token = "", string application = "")
        {
            var rqst = new Persistence.UpdateRqst();
            rqst.Id = connectionId;
            rqst.Database = database;
            rqst.Collection = collection;
            rqst.Query = query;
            rqst.Value = value;
            rqst.Options = options;

            this.client.Update(rqst, this.GetClientContext(token, application));
        }

        public void DeleteOne(string connectionId, string database, string collection, string query, string options, string token = "", string application = "")
        {
            var rqst = new Persistence.DeleteOneRqst();
            rqst.Id = connectionId;
            rqst.Database = database;
            rqst.Collection = collection;
            rqst.Query = query;
            rqst.Options = options;

            this.client.DeleteOne(rqst, this.GetClientContext(token, application));
        }

        public void Delete(string connectionId, string database, string collection, string query, string options, string token = "", string application = "")
        {
            var rqst = new Persistence.DeleteRqst();
            rqst.Id = connectionId;
            rqst.Database = database;
            rqst.Collection = collection;
            rqst.Query = query;
            rqst.Options = options;

            this.client.Delete(rqst, this.GetClientContext(token, application));
        }

        public void InsertMany(string connectionId, string database, string collection, ArrayList objects, string options, string token = "", string application = "")
        {

            // Open a stream with the server.
            var call = this.client.InsertMany(this.GetClientContext(token, application));

            // Here i will iterate over the list of object contain in the collection and persist 500 object at time.
            var chunkSize = 500;
            for (var i = 0; i < objects.Count; i += chunkSize)
            {
                var rqst = new Persistence.InsertManyRqst();
                rqst.Id = connectionId;
                rqst.Database = database;
                rqst.Collection = collection;

                string jsonStr;
                if (i + chunkSize < objects.Count)
                {
                    jsonStr = "[" + JsonSerializer.Serialize(objects.GetRange(i, chunkSize)) + "]";
                    
                }
                else
                {
                    jsonStr = "[" + JsonSerializer.Serialize(objects.GetRange(i, objects.Count - i)) + "]";
                }

                rqst.Data = ByteString.CopyFrom(System.Text.Encoding.UTF8.GetBytes(jsonStr));
                var task = Task.Run(() => call.RequestStream.WriteAsync(rqst));
                task.Wait(); // wait until the message was sent...
            }

            Task.Run(() => call.RequestStream.CompleteAsync()).Wait();

            var rsp = Task.Run(() => call.ResponseAsync);
            rsp.Wait();

            return;
        }

        public void DeleteCollection(string connectionId, string database, string collection, string token = "", string application = "")
        {
            var rqst = new Persistence.DeleteCollectionRqst();
            rqst.Id = connectionId;
            rqst.Database = database;
            rqst.Collection = collection;

            this.client.DeleteCollection(rqst, this.GetClientContext(token, application));
        }

        public void DeleteDatabase(string connectionId, string database, string token = "", string application = "")
        {
            var rqst = new Persistence.DeleteDatabaseRqst();
            rqst.Id = connectionId;
            rqst.Database = database;

            this.client.DeleteDatabase(rqst, this.GetClientContext(token, application));
        }

        public void RunAdminCmd(string connectionId, string user, string pwd, string script, string token = "", string application = "")
        {
            var rqst = new Persistence.RunAdminCmdRqst();
            rqst.ConnectionId = connectionId;
            rqst.Script = script;
            rqst.User = user;
            rqst.Password = pwd;

            this.client.RunAdminCmd(rqst, this.GetClientContext(token, application));
        }

    }

}
