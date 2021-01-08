using System;
using Grpc.Core;

namespace Globular
{

    public class LogClient : Client
    {
        private Log.LogService.LogServiceClient client;

        public LogClient( string id, string domain, int configurationPort) : base(id, domain, configurationPort){
            this.client = new Log.LogService.LogServiceClient(this.channel);
        }

    }

}