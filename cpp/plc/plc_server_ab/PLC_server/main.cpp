
#include "PlcServiceImpl.h"
#include "cxxopts.hpp" // argument parser.

using grpc::Server;
using grpc::ServerBuilder;
using grpc::ServerContext;
using grpc::Status;

using namespace std;

int main(int argc, char** argv) {

	cxxopts::Options options("plc service", "A gRpc service to communicate with PLC.");
	
	auto result = options.parse(argc, argv);

    PlcServiceImpl service;
    if(argc == 2){
      int port = atoi(argv[1]);
      service.setPort(port);
    }

    // Start the service.
    service.run(&service);

	return 0;
}
