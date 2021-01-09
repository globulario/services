
using System;
using Globular;
using grpc = global::Grpc.Core;
using System.Threading.Tasks;

// The first thing to do is derived the service base class with GlobularService class.
namespace Echo
{
    public static partial class EchoService
    {
        public abstract partial class EchoServiceBase : GlobularService
        {

        }

    }

    public class EchoServiceImpl : EchoService.EchoServiceBase
    {
        public string Value { get; set; }

        public EchoServiceImpl()
        {
            System.Console.WriteLine("Create new EchoServiceImpl");

            // Here I will set the default values.
            this.Port = 10051; // The default port value
            this.Proxy = 10052; // The reverse proxy port
            this.Id =  Guid.NewGuid().ToString(); // The service instance id.
            this.Name = "echo.EchoService"; // The service name
            this.Version = "0.0.1";
            this.PublisherId = "globulario"; // must be the publisher id here...
            this.Domain = "globulario";
            this.Protocol = "grpc";
            this.Version = "0.0.1";            
            this.Value = "echo value!";
            this.Description = "Test service that can be use to learning how to use globular.";
                
            // Retreive the prototype file path relative to where it was generated.
            this.Proto = global::Echo.EchoReflection.Descriptor.Name;

        }

        // Overide method of the service to implement in C#
        public override Task<global::Echo.EchoResponse> Echo(global::Echo.EchoRequest request, grpc::ServerCallContext context)
        {
            Echo.EchoResponse rsp = new EchoResponse();
            rsp.Message = "echo " + request.Message;
            return Task.FromResult(rsp);
        }



        // Here I will set the default config values...
        public EchoServiceImpl init()
        {
            // call save on init
            return (EchoServiceImpl)base.init(this);
        }
    }
}
