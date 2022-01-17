#include "globular_config_client.h"
#include <iostream>

Globular::ConfigClient::ConfigClient(std::string name, std::string domain, unsigned int configurationPort):
    Globular::Client(name,domain, configurationPort),
    stub_(config::ConfigService::NewStub(this->channel))
{
    std::cout << "init the configuration (config) client at address " << domain << ":" << configurationPort << std::endl;
}
