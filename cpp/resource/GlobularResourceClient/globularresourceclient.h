#ifndef GLOBULARResourceCLIENT_H
#define GLOBULARResourceCLIENT_H
#include "../../GlobularClient/globularclient.h"
#include <thread>

#include <grpc/grpc.h>
#include <grpcpp/channel.h>
#include <grpcpp/client_context.h>
#include <grpcpp/create_channel.h>
#include <grpcpp/security/credentials.h>

#include "../resourcepb/resource.pb.h"
#include "../resourcepb/resource.grpc.pb.h"

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
};

}

#endif // GLOBULARResourceCLIENT_H
