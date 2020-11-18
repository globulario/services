using Grpc.Core;
using Grpc.Core.Interceptors;
using System;
using System.Collections.Generic;
using System.IO;
using System.Threading;
using System.Threading.Tasks;

namespace Ressource
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
                var ressourceServer = new RessourceServiceImpl();
                // init values from the configuration file.
                ressourceServer = ressourceServer.init();
                if (ressourceServer.TLS == true)
                {
                    // Read ssl certificate and initialyse credential with it.
                    var cacert = File.ReadAllText(ressourceServer.CertAuthorityTrust);
                    var servercert = File.ReadAllText(ressourceServer.CertFile);
                    var serverkey = File.ReadAllText(ressourceServer.KeyFile);
                    var keypair = new KeyCertificatePair(servercert, serverkey);
                    // secure connection parameters.
                    var ssl = new SslServerCredentials(new List<KeyCertificatePair>() { keypair }, cacert, false);
                    // create the server.
                    server = new Server
                    {
                        Ports = { new ServerPort(ressourceServer.Domain, ressourceServer.Port, ssl) }
                    };
                }
                else
                {
                    // non secure server.
                    server = new Server
                    {
                        Ports = { new ServerPort(ressourceServer.Domain, ressourceServer.Port, ServerCredentials.Insecure) }
                    };
                }

                Console.WriteLine("Ressource server listening on port " + ressourceServer.Port);

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