using System;
using Grpc.Core;

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
        public ResourceClient(string id, string address) : base(id, address)
        {
            // Here I will create grpc connection with the service...
            this.client = new Resource.ResourceService.ResourceServiceClient(this.channel);
        }

        // TODO implement functions here...
    }
}
