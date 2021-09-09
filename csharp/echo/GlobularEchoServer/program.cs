using System.Collections.Generic;
using System;
using Grpc.Core;
using Grpc.Core.Interceptors;
using System.Threading;
using System.Threading.Tasks;
using System.IO;

namespace Echo
{
    public class Prorgam
    {
        private static readonly AutoResetEvent _closing = new AutoResetEvent(false);
        private static Server server;


        // Event callback test...
        public static void onEchoEvent(Event.Event evt){
            System.Console.WriteLine("event received with string value " + evt.Data.ToStringUtf8());
        }

        public static void Main(string[] args)
        {
            Task.Factory.StartNew(() =>
            {
                System.Console.WriteLine("try to start echo server...");

                // Create a new echo server instance.
                var echoServer = new EchoServiceImpl();

                // init values from the configuration file.
                System.Console.WriteLine("init service configuration.");
                echoServer = echoServer.init();

                // Here is an exemple how to set log message information.
                //echoServer.logMessage("Main", "The C# echo server was started!", Log.LogLevel.InfoMessage);
                
                // example on how to subscribe to event.
                //echoServer.subscribe("on_echo_event", uuid.ToString(), new Action<Event.Event>(onEchoEvent));
                
                
                // Now here I will try to connect the server to an event channel...(this is for test...)
                var uuid = System.Guid.NewGuid();

                
                if (echoServer.TLS == true)
                {
                    // Read ssl certificate and initialyse credential with it.
                    List<KeyCertificatePair> certificates = new List<KeyCertificatePair>();
                    certificates.Add(new KeyCertificatePair(File.ReadAllText(echoServer.CertFile), File.ReadAllText(echoServer.KeyFile)));
 
                    // secure connection parameters.
                    var ssl = new SslServerCredentials(certificates, File.ReadAllText(echoServer.CertAuthorityTrust), false);
                   
                    // create the server.
                    server = new Server
                    {
                        Services = { EchoService.BindService(echoServer).Intercept(echoServer.interceptor) },
                        Ports = { new ServerPort( "0.0.0.0", echoServer.Port, ssl) }
                    };
                }
                else
                {
                    // non secure server.
                    server = new Server
                    {
                        Services = { EchoService.BindService(echoServer).Intercept(echoServer.interceptor) },
                        Ports = { new ServerPort("0.0.0.0", echoServer.Port, ServerCredentials.Insecure) }
                    };
                }


                Console.WriteLine("Echo server listening on port " + echoServer.Port);
               
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