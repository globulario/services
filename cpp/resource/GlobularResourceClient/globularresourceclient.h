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
     * @brief validateUserAccess Validate if a user can access or not a given method.
     * @param token The token receive from authenticate.
     * @param method The gRpc method path. ex. /module/methodName/
     * @return
     */
    bool validateUserAccess(std::string token, std::string method);


    /**
     * @brief validateApplicationAccess Validate if application can access or not a given method.
     * @param name The name the application (must be unique on the server).
     * @param method The gRpc method path. ex. /module/methodName/
     * @return
     */
    bool validateApplicationAccess(std::string name, std::string method);

    /**
     * @brief validateUserResourceAccess Validate if user can access a given resource on the server.
     * @param token The token received from authentication
     * @param path The path of the resource (must be unique on the server)
     * @param method The gRpc method path. ex. /module/methodName/
     * @param permission The permission number, see chmod number... (ReadWriteDelete)
     * @return
     */
    bool validateUserResourceAccess(std::string token, std::string path, std::string method, int32_t permission);

    /**
     * @brief validateApplicationResourceAccess
     * @param application The name of the application to be validate.
     * @param path The path of the resource (must be unique on the server)
     * @param method The gRpc method path. ex. /module/methodName/
     * @param permission The permission number, see chmod number... (ReadWriteDelete)
     * @return
     */
    bool validateApplicationResourceAccess(std::string application, std::string path, std::string method, int32_t permission);


    /**
     * @brief SetResource
     * @param path The path of the resource (must be unique on the server)
     * @param name The name of the resource
     * @param modified The modified date.
     * @param size The size of the resource.
     */
    void SetResource(std::string path, std::string name, int modified, int size);

    /**
     * @brief removeRessouce
     * @param path The path where the resource is located.
     * @param name The name of the resource.
     */
    void removeRessouce(std::string path, std::string name);


    /**
     * @brief getActionPermission
     * @param method The gRpc method path. ex. /module/methodName/
     * @return
     */
    std::vector<::resource::ActionParameterResourcePermission> getActionPermission(std::string method);

    /**
     * @brief Log
     * @param application The application name
     * @param method The gRpc method path. ex. /module/methodName/
     * @param message The message to log.
     * @param type can be 0 for INFO_MESSAGE and 1 for ERROR_MESSAGE.
     */
    void Log(std::string application, std::string method, std::string message, int type = 0);
};

}

#endif // GLOBULARResourceCLIENT_H
