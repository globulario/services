using System;
using Grpc.Core;

namespace Globular
{

    public class RbacClient : Client
    {
        private Rbac.RbacService.RbacServiceClient client;

        public RbacClient( string id, string domain, int configurationPort) : base(id, domain, configurationPort){
            this.client = new Rbac.RbacService.RbacServiceClient(this.channel);
        }

        

    }

}