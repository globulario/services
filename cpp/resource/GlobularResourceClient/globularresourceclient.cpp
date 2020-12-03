#include "globularresourceclient.h"
#include <iostream>

Globular::ResourceClient::ResourceClient(std::string name, std::string domain, unsigned int configurationPort):
    Globular::Client(name,domain, configurationPort),
    stub_(resource::ResourceService::NewStub(this->channel))
{
    std::cout << "init the resource client at address " << domain << ":" << configurationPort << std::endl;
}

std::string Globular::ResourceClient::authenticate(std::string user, std::string password){
   resource::AuthenticateRqst rqst;
   rqst.set_name(user);
   rqst.set_password(password);

   resource::AuthenticateRsp rsp;
   grpc::ClientContext ctx;
   this->getClientContext(ctx);
   Status status = this->stub_->Authenticate(&ctx, rqst, &rsp);

   // return the token.
   if(status.ok()){
    return rsp.token();
   }else{
       std::cout << "fail to autenticate user " << user  << std::endl;
       return "";
   }
}

/*
void  Globular::ResourceClient::Log(std::string application, std::string method, std::string message, int type){

    resource::LogRqst rqst;
    resource::LogInfo* info = rqst.mutable_info();
    info->set_type(resource::LogType(type));
    info->set_message(message);
    info->set_application(application);
    info->set_method(method);
    info->set_date(std::time(0));
    grpc::ClientContext ctx;
    this->getClientContext(ctx);
    resource::LogRsp rsp;
    Status status = this->stub_->Log(&ctx, rqst, &rsp);
    if(status.ok()){
        return;
    }else{
        std::cout << "Fail to log information " << application << ":" << method << std::endl;
    }
}
*/
