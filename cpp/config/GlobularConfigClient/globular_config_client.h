#ifndef GLOBULAR_Config__CLIENT_H
#define GLOBULAR_Config__CLIENT_H
#include "../../GlobularClient/globularclient.h"
#include <thread>

#include <grpc/grpc.h>
#include <grpcpp/channel.h>
#include <grpcpp/client_context.h>
#include <grpcpp/create_channel.h>
#include <grpcpp/security/credentials.h>

#include "../configpb/config.pb.h"
#include "../configpb/config.grpc.pb.h"

// GRPC stuff.
using grpc::Channel;
using grpc::ClientContext;
using grpc::ClientReader;
using grpc::ClientReaderWriter;
using grpc::ClientWriter;
using grpc::Status;

namespace Globular {

class ConfigClient : Client {
    // the underlying grpc resource client.
    std::unique_ptr<config::ConfigService::Stub> stub_;

public:

    // The constructor.
    ConfigClient(std::string name, std::string domain="localhost", unsigned int configurationPort=80);

    // Now the resource client functionnalites.
};

}

#endif // GLOBULAR_Config__CLIENT_H
