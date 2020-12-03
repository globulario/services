#ifndef GLOBULARResourceCLIENT_H
#define GLOBULARResourceCLIENT_H
#include "globularclient.h"
#include <thread>

#include <grpc/grpc.h>
#include <grpcpp/channel.h>
#include <grpcpp/client_context.h>
#include <grpcpp/create_channel.h>
#include <grpcpp/security/credentials.h>

#include "resource.pb.h"
#include "resource.grpc.pb.h"

// GRPC stuff.
using grpc::Channel;
using grpc::ClientContext;
using grpc::ClientReader;
using grpc::ClientReaderWriter;
using grpc::ClientWriter;
using grpc::Status;

namespace Globular {

class ResourceClient : Client
{
    // the underlying grpc resource client.
    std::unique_ptr<resource::ResourceService::Stub> stub_;

public:

    // The constructor.
    ResourceClient(std::string name, std::string domain="localhost", unsigned int configurationPort=80);

    // Now the resource client functionnalites.

    /**
     * @brief authenticate Authenticate a user on the services.
     * @param user The user id
     * @param password The user password
     * @return the token (valid for a given delay)
     */
    std::string authenticate(std::string user, std::string password);


    /**
     * @brief Log
     * @param application The application name
     * @param method The gRpc method path. ex. /module/methodName/
     * @param message The message to log.
     * @param type can be 0 for INFO_MESSAGE and 1 for ERROR_MESSAGE.
     */
    // void Log(std::string application, std::string method, std::string message, int type = 0);
};

}

#endif // GLOBULARResourceCLIENT_H
