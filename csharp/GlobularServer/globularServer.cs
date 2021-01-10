using System;
using System.IO;
using System.Text.Json;
using Grpc.Core;
using Grpc.Core.Interceptors;
using System.Threading.Tasks;
using System.IdentityModel.Tokens.Jwt;


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
        public string Description { get; set; }
        public string[] Keywords { get; set; }
        public bool KeepUpToDate { get; set; }
        public bool KeepAlive { get; set; }

        // globular specific variable.
        public int ConfigurationPort; // The configuration port of globular.
        public string Root; // The globular root.

        private ResourceClient resourceClient;

        private RbacClient rbacClient;

        private LoadBalancingClient loadBalancingClient;

        private LogClient logClient;

        private EventClient eventClient;

        public ServerUnaryInterceptor interceptor;

        /// <summary>
        /// The default constructor.
        /// </summary>
        public GlobularService(string domain = "localhost")
        {
            System.Console.WriteLine("Create a new service with domain " + domain);
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
            this.Description = "";
            this.Keywords = new string[] { "Globular", "microservice", "csharp" };

            // Create the interceptor.
            System.Console.WriteLine("create new ServerUnaryInterceptor");
            this.interceptor = new Globular.ServerUnaryInterceptor(this);

            // Get the local globular server infomation.
            System.Console.WriteLine("Try to read GLOBULAR_ROOT file from " + System.IO.Path.GetTempPath());

            string path = System.IO.Path.GetTempPath() + "GLOBULAR_ROOT";
            string text = System.IO.File.ReadAllText(path);
            this.Root = text.Substring(0, text.LastIndexOf(":")).Replace("\\", "/");

            this.ConfigurationPort = Int32.Parse(text.Substring(text.LastIndexOf(":") + 1));
            System.Console.WriteLine("The service configuration port is " + this.ConfigurationPort);
        }

        protected ResourceClient getResourceClient(string address)
        {
            if (this.resourceClient == null)
            {
                // there must be a globular server runing in order to validate resources.
                // TODO set the configuration port in a configuration file.
                this.resourceClient = new ResourceClient("resource.ResourceService", address);
            }
            return this.resourceClient;
        }


        protected RbacClient getRbacClient(string address)
        {
            if (this.rbacClient == null)
            {
                // there must be a globular server runing in order to validate resources.
                System.Console.WriteLine("---> init the RBAC client at domain " + address);
                this.rbacClient = new RbacClient("rbac.RbacService", address);
            }
            return this.rbacClient;
        }

        protected EventClient getEventClient(string address)
        {
            if (this.eventClient == null)
            {
                // there must be a globular server runing in order to validate resources.
                System.Console.WriteLine("---> init the event client at domain " + address);
                this.eventClient = new EventClient("event.EventService", address);
            }
            return this.eventClient;
        }

        protected LoadBalancingClient getLoadBalancingClient(string address)
        {
            if (this.loadBalancingClient == null)
            {
                // there must be a globular server runing in order to validate resources.
                this.loadBalancingClient = new LoadBalancingClient("lb.LoadBalancingService", address);
            }
            return this.loadBalancingClient;
        }

        protected LogClient getLogClient(string address)
        {
            if (this.logClient == null)
            {
                // there must be a globular server runing in order to validate resources.
                // TODO set the configuration port in a configuration file.
                this.logClient = new LogClient("log.LogService", address);
            }

            return this.logClient;
        }
        private string getPath()
        {
            return Directory.GetCurrentDirectory();
        }

        private bool validateAction(string domain, string method, string subject, Rbac.SubjectType subjectType, Google.Protobuf.Collections.RepeatedField<Rbac.ResourceInfos> infos)
        {
            System.Console.WriteLine("Valdated access for Domain: " + domain + " Subject: " + subject + " Method: " + method);
            // Here I need to ge the ResourceInfos...
            var client = this.getRbacClient(domain);

            return client.ValidateAction(subject, subjectType, infos);
        }

        public bool validateActionRequest(Google.Protobuf.IMessage rqst, string domain, string method, string subject, Rbac.SubjectType subjectType)
        {
            // Here I need to ge the ResourceInfos...
            var client = this.getRbacClient(domain);

            // The first thing I will do it's to get the list of actions parameters...
            var infos = client.GetActionResourceInfos(method);

            // Get the list of fied's by order
            var fields = rqst.Descriptor.Fields.InFieldNumberOrder();

            for (var i = 0; i < infos.Count; i++)
            {
                // Here I will set the path value for resource to be able to validate it 
                // access
                infos[i].Path = fields[infos[i].Index].Accessor.GetValue(rqst).ToString();
            }

            System.Console.WriteLine("There is " + infos.Count + " actions infos...");
            validateAction(domain, method, subject, subjectType, infos);

            return true;
        }

        public bool validateToken(string token)
        {
            // Here I will get the expiration time and test of it's valid.
            var exp = this.getClaim(token, "exp");

            long epochTicks = new DateTime(1970, 1, 1).Ticks;
            long now = ((DateTime.UtcNow.Ticks - epochTicks) / TimeSpan.TicksPerSecond);
            return now < Convert.ToInt64(exp) ;

        }
        private string getClaim(string token, string claimType)
        {
            var tokenHandler = new JwtSecurityTokenHandler();
            var securityToken = tokenHandler.ReadToken(token) as JwtSecurityToken;
            var iter = securityToken.Claims.GetEnumerator();
            while (iter.MoveNext())
            {
                var claim = iter.Current;
                if (claim.Type == claimType)
                {
                    return claim.Value;
                }
            }

            return "";
        }

        public string getUserIdFromToken(string token)
        {
            return this.getClaim(token, "username");
        }

        /// <summary>
        /// Log information message to the 
        /// </summary>
        public void logMessage(string method, string message, Log.LogLevel level){
            var client = this.getLogClient(this.Domain + ":" + this.ConfigurationPort);
            client.LogMessage(this.Name, this.Id, method, level, message);
        }
        
        /// <summary>
        /// Initialyse from json object from a file.
        /// </summary>
        public object init(object server)
        {
            var configPath = this.getPath() + "/config.json";
            this.Path = System.Diagnostics.Process.GetCurrentProcess().MainModule.FileName;
            this.Path = this.Path.Replace("\\", "/");
            // Here I will read the file that contain the object.
            if (File.Exists(configPath))
            {
                var jsonStr = File.ReadAllText(configPath);
                var s = JsonSerializer.Deserialize(jsonStr, server.GetType());
                return s;
            }
            else
            {

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
            var configPath = getPath() + "/config.json";
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
            string clientId = "";
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

            var rqst = (Google.Protobuf.IMessage)request;

            // A domain must be given to get access to the resource manager.
            if (domain.Length == 0)
            {
                throw new RpcException(new Status(StatusCode.PermissionDenied, "Permission denied, no domain was given!"), metadatas);
            }

            if (application.Length > 0)
            {
                hasAccess = this.service.validateActionRequest(rqst, domain, method, application, Rbac.SubjectType.Application);
            }

            if (!hasAccess)
            {
                if (token.Length > 0)
                {
                    if (this.service.validateToken(token))
                    {
                        clientId = this.service.getUserIdFromToken(token);
                    }
                    hasAccess = this.service.validateActionRequest(rqst, domain, method, clientId, Rbac.SubjectType.Account);
                }
            }

            if (!hasAccess)
            {
                hasAccess = this.service.validateActionRequest(rqst, domain, method, domain, Rbac.SubjectType.Peer);
            }

            // Here I will validate the user for action.
            if (!hasAccess)
            {
                // here I the user and the application has no access to the method 
                // I will throw an exception.
                throw new RpcException(new Status(StatusCode.PermissionDenied, "Permission denied"), metadatas);
            }

            // this.service.
            var response = await base.UnaryServerHandler(request, context, continuation);
            return response;
        }
    }
}