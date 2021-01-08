using System;
using System.IO;
using System.Text.Json;
using Grpc.Core;
using Grpc.Core.Interceptors;
using System.Threading.Tasks;


// TODO for the validation, use a map to store valid method/token/resource/access
// the validation will be renew only if the token expire. And when a token expire
// the value in the map will be discard. That way it will put less charge on the server
// side.
namespace Globular
{

    /// <summary>
    /// That class contain the basic service class. Globular service are 
    /// plain gRPC service with required attributes to make it manageable.
    /// </summary>
    public class GlobularService
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

		
        private ResourceClient resourceClient;
        public ServerUnaryInterceptor interceptor;

        /// <summary>
        /// The default constructor.
        /// </summary>
        public GlobularService(string domain = "localhost")
        {
            // set default values.
            this.Domain = domain;
            this.Protocol = "grpc";
            this.Version = "0.0.1";
            this.PublisherId = "localhost";
            this.CertFile = "";
            this.KeyFile = "";
            this.CertAuthorityTrust = "";
            this.AllowAllOrigins = true;
            this.AllowedOrigins = "";
                        
            // Create the interceptor.
            this.interceptor = new Globular.ServerUnaryInterceptor(this);

            // Get the local globular server infomation.
            string path = System.IO.Path.GetTempPath() +  "GLOBULAR_ROOT";
            string text = System.IO.File.ReadAllText(  path );
            this.Root = text.Substring(0, text.LastIndexOf(":")).Replace("\\", "/");
            this.ConfigurationPort = Int32.Parse( text.Substring(text.LastIndexOf(":") + 1));
        }

        private ResourceClient getResourceClient(string domain)
        {
            if (this.resourceClient == null)
            {
                // there must be a globular server runing in order to validate resources.
                // TODO set the configuration port in a configuration file.
                resourceClient = new ResourceClient("resource.ResourceService", domain, this.ConfigurationPort );
            }
            return this.resourceClient;
        }

        private string getPath()
        {
            return Directory.GetCurrentDirectory();
        }

        /// <summary>
        /// Initialyse from json object from a file.
        /// </summary>
        public object init(object server)
        {
            var configPath = this.getPath() + "/config.json";
            this.Path =System.Diagnostics.Process.GetCurrentProcess().MainModule.FileName;
            this.Path = this.Path.Replace("\\", "/");
            // Here I will read the file that contain the object.
            if (File.Exists(configPath))
            {
                var jsonStr = File.ReadAllText(configPath);
                var s = JsonSerializer.Deserialize(jsonStr, server.GetType());
                return s;
            }else{

                // Here I will complete the filepath with the Root value of the server.
                this.Proto = this.Root + "/" + this.Proto;
            }
            this.save(server);
            return server;
        }

        /// <summary>
        /// Serialyse the object into json and save it in config.json file.
        /// </summary>
        public void save(object server)
        {
            var configPath = getPath()  + "/config.json";
            string jsonStr;
            jsonStr = JsonSerializer.Serialize(server);
            File.WriteAllText(configPath, jsonStr);
        }
    }

    public class ServerUnaryInterceptor : Interceptor
    {

        private GlobularService service;

        public ServerUnaryInterceptor(GlobularService srv)
        {
            this.service = srv;
        }

        public override async Task<TResponse> UnaryServerHandler<TRequest, TResponse>(TRequest request, ServerCallContext context, UnaryServerMethod<TRequest, TResponse> continuation)
        {
            // Do method validations here.
            Metadata metadatas = context.RequestHeaders;
            string application = "";
            string token = "";
            string path = "";
            string domain = "";
            string method = context.Method;
            bool hasAccess = false;
            
            // Get the metadata from the header.
            for (var i = 0; i < metadatas.Count; i++)
            {
                var metadata = metadatas[i];
                if (metadata.Key == "application")
                {
                    application = metadata.Value;
                }
                else if (metadata.Key == "token")
                {
                    token = metadata.Value;
                }
                else if (metadata.Key == "path")
                {
                    path = metadata.Value;
                }
                else if (metadata.Key == "domain")
                {
                    domain = metadata.Value;
                }
            }

            // A domain must be given to get access to the resource manager.
            if (domain.Length == 0)
            {
                throw new RpcException(new Status(StatusCode.PermissionDenied, "Permission denied, no domain was given!"), metadatas);
            }

            if (application.Length > 0)
            {
                hasAccess = this.service.validateApplicationAccess(domain, application, method);
            }

            if (!hasAccess)
            {
                hasAccess = this.service.validateUserAccess(domain, token, method);
            }


            // Here I will validate the user for action.
            if (!hasAccess)
            {
                // here I the user and the application has no access to the method 
                // I will throw an exception.
                throw new RpcException(new Status(StatusCode.PermissionDenied, "Permission denied"), metadatas);
            }

            // Now if the action has resource access permission defines...
            var permission = this.service.getActionPermission(domain, method);
            if (permission != -1)
            {
                // Now I will try to validate resource if there is none...
                if (path.Length > 0)
                {
                    var hasResourcePermission = this.service.validateUserResourceAccess(domain, token, path, method, permission);
                    if (!hasResourcePermission)
                    {
                        this.service.validateApplicationResourceAccess(domain, application, path, method, permission);
                    }
                    if (!hasResourcePermission)
                    {
                        throw new RpcException(new Status(StatusCode.PermissionDenied, "Permission denied"), metadatas);
                    }
                }
            }

            // this.service.
            var response = await base.UnaryServerHandler(request, context, continuation);
            return response;
        }
    }
}