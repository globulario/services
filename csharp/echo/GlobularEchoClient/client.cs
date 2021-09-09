using System;
using Grpc.Core;

namespace Globular
{

    public class EchoClient : Client
    {
        private Echo.EchoService.EchoServiceClient client;

        public EchoClient(string id, string address) : base(id, address)
        {
            this.client = new Echo.EchoService.EchoServiceClient(this.channel);
        }

        public string Echo(string message)
        {
            long epochTicks = new DateTime(1970, 1, 1).Ticks;
            long now = ((DateTime.UtcNow.Ticks - epochTicks) / TimeSpan.TicksPerSecond);

            var rqst = new Echo.EchoRequest();
            rqst.Message = message;

            // Echo the message...
            Echo.EchoResponse rsp  = this.client.Echo(rqst, this.GetClientContext());
            return rsp.Message;
        }
    }
}