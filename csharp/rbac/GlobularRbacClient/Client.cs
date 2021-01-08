using System;
using Grpc.Core;

namespace Globular
{

    public class RbacClient : Client
    {
        private Rbac.RbacService.RbacServiceClient client;

        /// <summary>
        /// The Role Base Access Control is use to control access to gRpc action 
        /// and also resource used by those action's.
        public RbacClient( string id, string address) : base(id, address){
            this.client = new Rbac.RbacService.RbacServiceClient(this.channel);
        }

        

    }

}