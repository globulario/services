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

bool Globular::ResourceClient::validateUserAccess(std::string token, std::string method){
    resource::ValidateUserAccessRqst rqst;
    rqst.set_token(token);
    rqst.set_method(method);
    grpc::ClientContext ctx;
    this->getClientContext(ctx);
    resource::ValidateUserAccessRsp rsp;

    Status status = this->stub_->ValidateUserAccess(&ctx, rqst, &rsp);
    if(status.ok()){
        return rsp.result();
    }else{
        return false;
    }
}

bool Globular::ResourceClient::validateApplicationAccess(std::string name, std::string method){
    resource::ValidateApplicationAccessRqst rqst;
    rqst.set_name(name);
    rqst.set_method(method);

    grpc::ClientContext ctx;
    this->getClientContext(ctx);
    resource::ValidateApplicationAccessRsp rsp;
    Status status = this->stub_->ValidateApplicationAccess(&ctx, rqst, &rsp);
    if(status.ok()){
        return rsp.result();
    }else{
        return false;
    }
}

bool  Globular::ResourceClient::validateApplicationResourceAccess(std::string application, std::string path, std::string method, int32_t permission){
    resource::ValidateApplicationResourceAccessRqst rqst;
    rqst.set_name(application);
    rqst.set_method(method);
    rqst.set_path(path);
    rqst.set_permission(permission);

    grpc::ClientContext ctx;
    this->getClientContext(ctx);
    resource::ValidateApplicationResourceAccessRsp rsp;
    Status status = this->stub_->ValidateApplicationResourceAccess(&ctx, rqst, &rsp);
    if(status.ok()){
        return rsp.result();
    }else{
        return false;
    }
}

bool  Globular::ResourceClient::validateUserResourceAccess(std::string token, std::string path, std::string method, int32_t permission){
    resource::ValidateUserResourceAccessRqst rqst;
    rqst.set_token(token);
    rqst.set_method(method);
    rqst.set_path(path);
    rqst.set_permission(permission);

    grpc::ClientContext ctx;
    this->getClientContext(ctx);
    resource::ValidateUserResourceAccessRsp rsp;
    Status status = this->stub_->ValidateUserResourceAccess(&ctx, rqst, &rsp);
    if(status.ok()){
        return rsp.result();
    }else{
        return false;
    }
}

void  Globular::ResourceClient::SetResource(std::string path, std::string name, int modified, int size){
    resource::SetResourceRqst rqst;
    resource::Resource* r = rqst.mutable_resource();
    r->set_path(path);
    r->set_name(name);
    r->set_modified(modified);
    r->set_size(size);

    grpc::ClientContext ctx;
    this->getClientContext(ctx);
    resource::SetResourceRsp rsp;
    Status status = this->stub_->SetResource(&ctx, rqst, &rsp);
    if(!status.ok()){
        std::cout << "Fail to set resource " << name << std::endl;
    }
}

void  Globular::ResourceClient::removeRessouce(std::string path, std::string name){
    resource::RemoveResourceRqst rqst;
    resource::Resource* r = rqst.mutable_resource();
    r->set_path(path);
    r->set_name(name);

    grpc::ClientContext ctx;
    this->getClientContext(ctx);
    resource::RemoveResourceRsp rsp;
    Status status = this->stub_->RemoveResource(&ctx, rqst, &rsp);
    if(!status.ok()){
        std::cout << "Fail to remove resource " << name << std::endl;
    }
}

std::vector<::resource::ActionParameterResourcePermission>  Globular::ResourceClient::getActionPermission(std::string method){
    resource::GetActionPermissionRqst rqst;
    rqst.set_action(method);

    grpc::ClientContext ctx;
    this->getClientContext(ctx);
    resource::GetActionPermissionRsp rsp;
    Status status = this->stub_->GetActionPermission(&ctx, rqst, &rsp);
    std::vector<::resource::ActionParameterResourcePermission>  results;
    if(status.ok()){
        for(auto i=0; i < rsp.actionparameterresourcepermissions().size(); i++){
            results.push_back(rsp.actionparameterresourcepermissions()[i]);
        }
    }
    return results;
}

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
