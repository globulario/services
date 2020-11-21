
using System;
using grpc = global::Grpc.Core;
using System.Threading.Tasks;
using System.Text.Json;
using System.IO;

// The first thing to do is derived the service base class with GlobularService class.
namespace Resource
{

    public class ResourceServiceImpl : ResourceService.ResourceServiceBase
    {

        public string Id { get; set; }
        public string Name { get; set; }
        public string Path { get; set; }
        public string Proto { get; set; }
        public int Port { get; set; }
        public int Proxy { get; set; }
        public string Protocol { get; set; }
        public bool AllowAllOrigins { get; set; }
        public string AllowedOrigins { get; set; }
        public string Domain { get; set; }
        public string CertAuthorityTrust { get; set; }
        public string CertFile { get; set; }
        public string KeyFile { get; set; }
        public bool TLS { get; set; }
        public string Version { get; set; }
        public string PublisherId { get; set; }
        public bool KeepUpToDate { get; set; }
        public bool KeepAlive { get; set; }

        // globular specific variable.
        public int ConfigurationPort; // The configuration port of globular.
        public string Root; // The globular root.
        public string Value { get; set; }

        public ResourceServiceImpl()
        {
            // Here I will set the default values.
            this.Port = 10029; // The default port value
            this.Proxy = 10030; // The reverse proxy port
            this.Id =  Guid.NewGuid().ToString(); // The service instance id.
            this.Name = "Resource.ResourceService"; // The service name
            this.Version = "0.0.1";
            this.PublisherId = "localhost"; // must be the publisher id here...
            this.Domain = "localhost";
            this.Protocol = "grpc";
            this.Version = "0.0.1";            
            this.Value = "Resource value!";
                
            // Retreive the prototype file path relative to where it was generated.
            this.Proto = global::Resource.ResourceReflection.Descriptor.Name;
        
        }

        private string getPath()
        {
            return Directory.GetCurrentDirectory();
        }


        // Here I will set the default config values...
        public ResourceServiceImpl init()
        {
            var configPath = this.getPath() + "/config.json";
            this.Path = System.Diagnostics.Process.GetCurrentProcess().MainModule.FileName;
            this.Path = this.Path.Replace("\\", "/");
            // Here I will read the file that contain the object.
            if (File.Exists(configPath))
            {
                var jsonStr = File.ReadAllText(configPath);
                var s = JsonSerializer.Deserialize(jsonStr, this.GetType());
                return (ResourceServiceImpl) s;
            }
            else
            {

                // Here I will complete the filepath with the Root value of the server.
                this.Proto = this.Root + "/" + this.Proto;
            }
            this.save(this);

            return this;
        }

        override 

        public void save(object server)
        {
            var configPath = getPath() + "/config.json";
            string jsonStr;
            jsonStr = JsonSerializer.Serialize(server);
            File.WriteAllText(configPath, jsonStr);
        }

    }
}