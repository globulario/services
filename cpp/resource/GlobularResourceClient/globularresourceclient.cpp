#include "globularresourceclient.h"
#include <iostream>

Globular::ResourceClient::ResourceClient(std::string name, std::string domain, unsigned int configurationPort):
    Globular::Client(name,domain, configurationPort),
    stub_(resource::ResourceService::NewStub(this->channel))
{
    std::cout << "init the resource client at address " << domain << ":" << configurationPort << std::endl;
}
