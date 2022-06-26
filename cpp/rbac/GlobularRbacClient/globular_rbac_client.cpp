#include "globular_rbac_client.h"
#include <iostream>

Globular::RbacClient::RbacClient(std::string name, std::string domain, unsigned int configurationPort):
    Globular::Client(name,domain, configurationPort),
    stub_(rbac::RbacService::NewStub(this->channel))
{
    std::cout << "init the (R)ole (B)ase (A)ccess (C)ontrol client at address " << domain << ":" << configurationPort << std::endl;
}
