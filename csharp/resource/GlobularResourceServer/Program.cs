using Grpc.Core;
using Grpc.Core.Interceptors;
using System;
using System.Collections.Generic;
using System.IO;
using System.Threading;
using System.Threading.Tasks;

namespace Resource
{
    public class Prorgam
    {
        private static readonly AutoResetEvent _closing = new AutoResetEvent(false);
        private static Server server;

        public static void Main(string[] args)
        {
            Task.Factory.StartNew(() =>
            {
                // Create a new echo server instance.
                var resourceServer = new ResourceServiceImpl();
                // init values from the configuration file.
                resourceServer = resourceServer.init();
                if (resourceServer.TLS == true)
                {
                    // Read ssl certificate and initialyse credential with it.
                    var cacert = File.ReadAllText(resourceServer.CertAuthorityTrust);
                    var servercert = File.ReadAllText(resourceServer.CertFile);
                    var serverkey = File.ReadAllText(resourceServer.KeyFile);
                    var keypair = new KeyCertificatePair(servercert, serverkey);
                    // secure connection parameters.
                    var ssl = new SslServerCredentials(new List<KeyCertificatePair>() { keypair }, cacert, false);
                    // create the server.
                    server = new Server
                    {
                        Ports = { new ServerPort(resourceServer.Domain, resourceServer.Port, ssl) }
                    };
                }
                else
                {
                    // non secure server.
                    server = new Server
                    {
                        Ports = { new ServerPort(resourceServer.Domain, resourceServer.Port, ServerCredentials.Insecure) }
                    };
                }

                Console.WriteLine("Resource server listening on port " + resourceServer.Port);

                // GRPC server.
                server.Start();
            });
            Console.CancelKeyPress += new ConsoleCancelEventHandler(OnExit);
            _closing.WaitOne();
        }

        protected static void OnExit(object sender, ConsoleCancelEventArgs args)
        {
            server.ShutdownAsync().Wait();
            _closing.Set();
        }
    }
}