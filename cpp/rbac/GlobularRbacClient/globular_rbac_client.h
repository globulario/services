#ifndef GLOBULAR_RBAC_CLIENT_H
#define GLOBULAR_RBAC_CLIENT_H
#include "globularclient.h"
#include <thread>

#include <grpc/grpc.h>
#include <grpcpp/channel.h>
#include <grpcpp/client_context.h>
#include <grpcpp/create_channel.h>
#include <grpcpp/security/credentials.h>

#include "../rbacpb/rbac.pb.h"
#include "../rbacpb/rbac.grpc.pb.h"

// GRPC stuff.
using grpc::Channel;
using grpc::ClientContext;
using grpc::ClientReader;
using grpc::ClientReaderWriter;
using grpc::ClientWriter;
using grpc::Status;

namespace Globular {

class RbacClient : Client
{
    // the underlying grpc resource client.
    std::unique_ptr<rbac::RbacService::Stub> stub_;

public:

    // The constructor.
    RbacClient(std::string name, std::string domain="localhost", unsigned int configurationPort=80);

    // Now the resource client functionnalites.
};

}

#endif // GLOBULAR_RBAC_CLIENT_H
