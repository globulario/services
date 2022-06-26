using System;
using Grpc.Core;

namespace Globular
{

    public class ConfigClient : Client
    {
        private Config.ConfigService.ConfigServiceClient client;

        public ConfigClient(string id, string address) : base(id, address)
        {
            this.client = new Config.ConfigService.ConfigServiceClient(this.channel);
        }

        /**
         * Retreive a service configuration.
         */
        public string GetServiceConfiguration(string id)
        {
            try
            {
                var rqst = new Config.GetServiceConfigurationRequest();
                rqst.Path = id;
                System.Console.WriteLine("Get Client configuration: " + id);

                Config.GetServiceConfigurationResponse rsp = this.client.GetServiceConfiguration(rqst, this.GetClientContext());
                return rsp.Config;
            }
            catch(Exception e)
            {
                 System.Console.WriteLine("get service configuration fail with err or " + e);
                throw new System.InvalidOperationException("No serivce found with id " + id);
            }
        }

        /**
         * Save a service configuration.
         */
        public void SetServiceConfiguration(string config)
        {
            try
            {
                var rqst = new Config.SetServiceConfigurationRequest();
                rqst.Config = config;
                Config.SetServiceConfigurationResponse rsp = this.client.SetServiceConfiguration(rqst, this.GetClientContext());
            }
            catch
            {
                throw new System.InvalidOperationException("Fail to save configuration");
            }
        }
    }

}