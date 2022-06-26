#include "globular_config_client.h"
#include <iostream>

Globular::ConfigClient::ConfigClient(std::string name, std::string domain, unsigned int configurationPort):
    Globular::Client(name,domain, configurationPort),
    stub_(config::ConfigService::NewStub(this->channel))
{
    std::cout << "init the configuration (config) client at address " << domain << ":" << configurationPort << std::endl;
}

std::string Globular::ConfigClient::ConfigClient::getServiceConfiguration(std::string id){
     std::cout << "get service configuration " << id << std::endl;
     config::GetServiceConfigurationRequest rqst;
     rqst.set_path(id);
     config::GetServiceConfigurationResponse rsp;
     grpc::ClientContext ctx;
     this->getClientContext(ctx);

     Status status = this->stub_->GetServiceConfiguration(&ctx, rqst, &rsp);

     // return the token.
     if(status.ok()){
      return rsp.config();
     }else{
         std::cout << "fail to retreive service config " << id  << std::endl;
     }
     return "";
}

bool  Globular::ConfigClient::ConfigClient::setServiceConfiguration(std::string config){
    std::cout << "set service configuration"<< std::endl;
    config::SetServiceConfigurationRequest rqst;
    rqst.set_config(config);
    config::SetServiceConfigurationResponse rsp;
    grpc::ClientContext ctx;
    this->getClientContext(ctx);

    Status status = this->stub_->SetServiceConfiguration(&ctx, rqst, &rsp);

    // return the token.
    if(status.ok()){
        std::cout << "configuration was save "  << std::endl;
        return true;
    }else{
        std::cout << "fail to retreive save config "  << std::endl;
        return false;
    }

}
