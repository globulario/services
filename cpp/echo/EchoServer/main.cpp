#include <iostream>
#include "cxxopts.hpp"
#include "echoserviceimpl.h"

using namespace std;
//#pragma comment(lib,"ws2_32.lib")

int main(int argc, char** argv)
{

    // Instantiate a new server.

    cxxopts::Options options("Statistic process control service", "A c++ gRpc service implementation");
    auto result = options.parse(argc, argv);

    std::string id;
    std::string config_path;

    // Instantiate a new server.
    if(argc == 2){
        id = std::string(argv[1]);
    }else if(argc == 3){
        id = std::string(argv[1]);
        config_path = std::string(argv[2]);
    }

    EchoServiceImpl service(id, config_path);

    // Start the service.
    service.run(&service);

    return 0;
}
