using System;
using Grpc.Core;

namespace Globular
{

    public class LoadBalancingClient : Client
    {
        private Lb.LoadBalancingService.LoadBalancingServiceClient client;

        public LoadBalancingClient( string id, string address) : base(id, address){
            this.client = new Lb.LoadBalancingService.LoadBalancingServiceClient(this.channel);
        }

    }

}