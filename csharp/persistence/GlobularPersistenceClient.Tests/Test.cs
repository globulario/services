using Xunit;
using System.Diagnostics;

namespace Globular
{
    public class PersistenceClient_Test
    {

        private PersistenceClient client = new PersistenceClient("persistence.PersistenceService", "localhost");
        private ResourceClient resourceClient = new ResourceClient("resource.ResourceService", "localhost");

        // Test create connection and also ping the connection to see if it exist and ready...
        /*
        [Fact]
        public void TestCreateConnection()
        {
            // Set the token.
            string token = resourceClient.Authenticate("sa", "adminadmin");
            System.Console.WriteLine("----> token: " + token);
            Persistence.Connection connection = new Persistence.Connection();
            connection.Id = "mongo_db_test_connection";
            connection.Name = "admin";
            connection.Host = "globular.io";
            connection.Port = 27017;
            connection.Store = Persistence.StoreType.Mongo;
            connection.User = "sa";
            connection.Password = "adminadmin";
            connection.Timeout = 3000;
            connection.Options = "";

            this.client.CreateConnection(connection, true, token);
        }
        */

        // Test find all...
        [Fact]
        public void TestFind()
        {   string token = resourceClient.Authenticate("sa", "adminadmin");
            var data = this.client.Find("local_resource", "local_resource", "Accounts", "{}", "", token);
            Assert.True(data.Length > 0);
            string str = System.Text.Encoding.UTF8.GetString(data);
            /** Here's the json array with all values in it ..." **/
            System.Console.WriteLine(str);
        }
        

        [Fact]
        public void TestFindOne()
        {
           string token = resourceClient.Authenticate("sa", "adminadmin");
           var data = this.client.FindOne("local_resource", "local_resource", "Accounts", "{\"_id\":\"dave\"}", "", token);
           Assert.True(data != null);
        }


    }
}