using System;
using Grpc.Core;

namespace Globular
{

    public class LoadBalancingClient : Client
    {
        private Lb.LoadBalancingService.LoadBalancingServiceClient client;

        public LoadBalancingClient( string id, string domain, int configurationPort) : base(id, domain, configurationPort){
            this.client = new Lb.LoadBalancingService.LoadBalancingServiceClient(this.channel);
        }

    }

}