using System;
using Grpc.Core;

namespace Globular
{

    public class LogClient : Client
    {
        private Log.LogService.LogServiceClient client;

        public LogClient(string id, string address) : base(id, address)
        {
            this.client = new Log.LogService.LogServiceClient(this.channel);
        }

        public void LogMessage(string application, string user, string method, Log.LogLevel level, string message)
        {
            long epochTicks = new DateTime(1970, 1, 1).Ticks;
            long now = ((DateTime.UtcNow.Ticks - epochTicks) / TimeSpan.TicksPerSecond);

            var rqst = new Log.LogRqst();
            var info = new Log.LogInfo();
            info.Application = application;
            info.UserName = user;
            info.Method = method;
            info.Date = now;
            info.Level = level;
            info.Message = message;
            rqst.Info = info;

            // Log the message...
            this.client.Log(rqst, this.GetClientContext());
        }
    }
}