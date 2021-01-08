using System;
using Grpc.Core;

namespace Globular
{

    public class LogClient : Client
    {
        private Log.LogService.LogServiceClient client;

        public LogClient( string id, string address) : base(id, address){
            this.client = new Log.LogService.LogServiceClient(this.channel);
        }

    }

}