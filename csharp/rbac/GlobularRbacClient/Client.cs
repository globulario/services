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

        public bool ValidateAccess(string subject, Rbac.SubjectType subjectType, string permission, string path){

            var rqst = new Rbac.ValidateAccessRqst();
            rqst.Subject = subject;
            rqst.Type = subjectType;
            rqst.Permission = permission;
            rqst.Path = path;
            
            var rsp = this.client.ValidateAccess(rqst, this.GetClientContext());
   
            return rsp.HasAccess && !rsp.AccessDenied;
        }

        public bool ValidateAction(string subject, string action, Rbac.SubjectType subjectType, Google.Protobuf.Collections.RepeatedField<Rbac.ResourceInfos> infos){

            var rqst = new Rbac.ValidateActionRqst();
            rqst.Subject = subject;
            rqst.Type = subjectType;
            rqst.Action = action;
            rqst.Infos.Add(infos);

            var rsp = this.client.ValidateAction(rqst, this.GetClientContext());

            return rsp.Result;
        }

        public Google.Protobuf.Collections.RepeatedField<Rbac.ResourceInfos> GetActionResourceInfos(string action ){

            var rqst = new Rbac.GetActionResourceInfosRqst();
            rqst.Action = action;

            var rsp = this.client.GetActionResourceInfos(rqst, this.GetClientContext());
            
            return rsp.Infos;
        }
    }

}