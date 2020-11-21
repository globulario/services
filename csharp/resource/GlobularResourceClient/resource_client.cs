esource System;
esource Grpc.Core;

namespace Globular
{
    public class ResourceClient : Client
    {
        private Resource.ResourceService.ResourceServiceClient client;

        /// <summary>
        /// The resource client is use by the interceptor to validate user access.
        /// </summary>
        /// <param name="id"></param> The name or the id of the services.
        /// <param name="domain"></param> The domain of the services
        /// <param name="configurationPort"></param> The domain of the services
        /// <returns></returns>
        public ResourceClient( string id, string domain, int configurationPort) : base(id, domain, configurationPort)
        {
            // Here I will create grpc connection with the service...
            this.client = new Resource.ResourceService.ResourceServiceClient(this.channel);
        }

        public string Authenticate(string user, string password){
            Resource.AuthenticateRqst rqst = new Resource.AuthenticateRqst();
            rqst.Name = user;
            rqst.Password = password;
            var rsp = this.client.Authenticate(rqst, this.GetClientContext());
            return rsp.Token;
        }

        /// <summary>
        /// Validate if the user can access a given method.
        /// </summary>
        /// <param name="token">The user token</param>
        /// <param name="method">The method </param>
        /// <returns></returns>
        public bool ValidateUserAccess(string token, string method)
        {
            Resource.ValidateUserAccessRqst rqst = new Resource.ValidateUserAccessRqst();
            rqst.Token = token;
            rqst.Method = method;
            var rsp = this.client.ValidateUserAccess(rqst, this.GetClientContext());
            return rsp.Result;
        }

        /// <summary>
        /// Validate if an application have access a given method.
        /// </summary>
        /// <param name="token"></param>
        /// <param name="method"></param>
        /// <returns></returns>
        public bool ValidateApplicationAccess(string name, string method)
        {
            Resource.ValidateApplicationAccessRqst rqst = new Resource.ValidateApplicationAccessRqst();
            rqst.Name = name;
            rqst.Method = method;
            var rsp = this.client.ValidateApplicationAccess(rqst, this.GetClientContext());
            return rsp.Result;
        }

        /// <summary>
        /// Validate if the user can access a given method.
        /// </summary>
        /// <param name="token">The user token</param>
        /// <param name="method">The method </param>
        /// <returns></returns>
        public bool ValidateUserResourceAccess(string token, string path, string method, int permission)
        {
            Resource.ValidateUserResourceAccessRqst rqst = new Resource.ValidateUserResourceAccessRqst();
            rqst.Token = token;
            rqst.Method = method;
            rqst.Path = path; // the path of the resource... 
            rqst.Permission = permission;

            var rsp = this.client.ValidateUserResourceAccess(rqst, this.GetClientContext());
            return rsp.Result;
        }

        /// <summary>
        /// Validate if an application have access a given method.
        /// </summary>
        /// <param name="name"></param>
        /// <param name="method"></param>
        /// <param name="permission"></param>
        /// <returns></returns>
        public bool ValidateApplicationResourceAccess (string name, string path, string method, int permission)
        {
            Resource.ValidateApplicationResourceAccessRqst rqst = new Resource.ValidateApplicationResourceAccessRqst();
            rqst.Name = name;
            rqst.Method = method;
            rqst.Path = path;
            rqst.Permission = permission;

            var rsp = this.client.ValidateApplicationResourceAccess(rqst, this.GetClientContext());
            return rsp.Result;
        }

        /// <summary>
        /// Set a resource path.
        /// </summary>
        /// <param name="path">The path of the resource in form /toto/titi/tata</param>
        public void SetResource(string path, string name, int modified, int size){
            Resource.SetResourceRqst rqst = new Resource.SetResourceRqst();
            Resource.Resource resource = new Resource.Resource();
            resource.Path = path;
            resource.Name = name;
            resource.Modified = modified;
            resource.Size = size;
            rqst.Resource = resource;
            this.client.SetResource(rqst);
        }

        /// <summary>
        /// Remove a resource from globular. It also remove asscociated permissions.
        /// </summary>
        /// <param name="path"></param>
        public void RemoveRessouce(string path, string name){
            Resource.RemoveResourceRqst rqst = new Resource.RemoveResourceRqst();
            Resource.Resource resource = new Resource.Resource();
            resource.Path = path;
            resource.Name = name;
            rqst.Resource = resource;
            this.client.RemoveResource(rqst);
        }

        /// <summary>
        /// Get the resource Action permission for a given resource.
        /// </summary>
        /// <param name="path">The resource path</param>
        /// <param name="action">The gRPC action</param>
        /// <returns></returns>
        public Int32 GetActionPermission(string action) {
            //Resource.GetActionPermissionRqst rqst = new Resource.GetActionPermissionRqst();
            //rqst.Action = action;
            //var rsp = this.client.GetActionPermission(rqst);
            // return rsp.Permission;

            // TODO make correction here...
            return -1;
            
        }

        /// <summary>
        /// That method id use to log information/error
        /// </summary>
        /// <param name="application">The name of the application (given in the context)</param>
        /// <param name="token">Ths user token (logged end user)</param>
        /// <param name="method">The method called</param>
        /// <param name="message">The message info</param>
        /// <param name="type">Information or Error</param>
        public void Log(string application, string token, string method, string message, int type = 0)
        {
            var rqst = new Resource.LogRqst();
            var info = new Resource.LogInfo();
            info.Application = application;
            info.UserId = token; // can be a token or the user id...
            info.Method = method;
            if(type == 0){
                 info.Type = Resource.LogType.InfoMessage;
            }else{
                info.Type = Resource.LogType.ErrorMessage;
            }
            info.Message = message;
            rqst.Info = info;

            // Set the log.
            this.client.Log(rqst, this.GetClientContext());
        }
    }
}
