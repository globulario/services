#include <string>
#include <fstream>
#include <filesystem>
#include <vector>
#include "json.hpp"
#include "config.hpp"
#include "./config/GlobularConfigClient/globular_config_client.h"
#include "HTTPRequest.hpp"

std::string replaceAll(std::string str, const std::string& from, const std::string& to) {
    size_t start_pos = 0;
    while((start_pos = str.find(from, start_pos)) != std::string::npos) {
        str.replace(start_pos, from.length(), to);
        start_pos += to.length(); // Handles case where 'to' is a substring of 'from'
    }
    return str;
}


#ifdef WIN32
#include <windows.h>
std::string getexepath()
{
    char result[MAX_PATH];
    return replaceAll(std::string(result, GetModuleFileNameA(NULL, result, MAX_PATH)), "\\", "/");
}

void sleep(unsigned milliseconds)
{
    Sleep(milliseconds);
}

std::string getLocalConfigPath(){
    /*TCHAR windir[MAX_PATH];
    GetWindowsDirectory(windir, MAX_PATH);
    std::wstring windir_(&windir[0]); //convert to wstring
    std::string windir__(windir_.begin(), windir_.end()); //and convert to string.

    return windir__;//*/
    return "C:/Program Files/globular/config";
}

#else
#include <limits.h>
#include <unistd.h>
#include <linux/limits.h>

std::string getexepath()
{
    char result[PATH_MAX];
    ssize_t count = readlink("/proc/self/exe", result, PATH_MAX);
    return replaceAll(std::string(result, (count > 0) ? count : 0), "\\", "/");
}

std::string getLocalConfigPath(){

    return "/etc/globular/config"
}

#endif // WIN32

const std::string getRootDir(){
    std::string execPath = getexepath();
    std::size_t lastIndex = execPath.find_last_of("/");
    return execPath.substr(0, lastIndex);
}

// That function is use simply to given the ConfigPath value to use to start the service.
// this is a quick fix util the configuration service will be done.
std::string getRemoteParticalServiceConfig(std::string serviceId, std::string domain){

    std::stringstream ss;
    ss << "http://" << domain << ":" << getHttpPort() << "/config?id=" + serviceId;
    http::Request request(ss.str());
    const std::string& body = "";
    const std::vector<std::string>& headers = {};
    const std::chrono::milliseconds timeout = std::chrono::milliseconds{3000};
    try{
        const http::Response response = request.send("GET", body, headers, timeout);
        ss.flush();
        return std::string(response.body.begin(), response.body.end());
    }
    catch (...) {
        // Block of code to handle errors
        return "";
    }
}

// Get the configuration path from the exec path...
std::string getConfigPath(std::string serviceId, std::string domain){

        std::string partialConfig = getRemoteParticalServiceConfig(serviceId, domain);
        if(!partialConfig.empty()){
            auto j = nlohmann::json::parse(partialConfig);
            std::string configPath = j["ConfigPath"];
            if(!configPath.empty()){
                return configPath;
            }
        }


    std::string execPath = getexepath();
    std::size_t lastIndex = execPath.find_last_of("/");
    std::string configPath = execPath.substr(0, lastIndex) + "/config.json";
    return configPath;
}


// Retreive a service configuration json.
const std::string getConfigStr(std::string path){
    std::ifstream inFile;
    inFile.open(path); //open the input file

    if (inFile.good()) {
        std::stringstream strStream;
        strStream << inFile.rdbuf(); //read the file
        return strStream.str(); //str holds the content of the file
    }
    return "";
}

// Retreive the local configuration...
auto getLocalConfig(){
    std::string jsonStr =  getConfigStr(getLocalConfigPath() + "/config.json");
    return nlohmann::json::parse(jsonStr);
}

std::string getLocalDomain(){
    auto config_ = getLocalConfig();
    std::string address = config_["Name"];
    std::string domain = config_["Domain"];
    if(!address.empty()){
        if(!domain.empty()){
            address += "." + domain;
        }
    }else if (!domain.empty()) {
        address = domain;
    }

    return address;
}

std::string getLocalProtocol(){
    auto config_ = getLocalConfig();
    return config_["Protocol"].get<std::string>();
}

int getHttpPort(){
    auto config_ = getLocalConfig();
    return config_["PortHttp"].get<int>();;
}

int getHttpsPort(){
    auto config_ = getLocalConfig();
    return config_["PortHttps"].get<int>();
}

int getLocalPort(){
    int port;
    auto config_ = getLocalConfig();
    // Now the port...
    if(config_["Protocol"] == "http"){
        port = config_["PortHttp"].get<int>();
    }else{
        port = config_["PortHttps"].get<int>();
    }
    return port;
}

// The config client.
Globular::ConfigClient* config_client__ = 0;


// A singleton that return the config client instance.
Globular::ConfigClient* getConfigClient(std::string domain, int port){
    if(config_client__ == 0){
        // Get the configuration service.
        config_client__ = new Globular::ConfigClient("config.ConfigService", domain, port);
    }

    return config_client__;
}

// Return the service configuration
std::string getServiceConfig(std::string serviceId, std::string address, std::string config_path){
    // First option the configuration manager
    try {
        auto config_client_ = getConfigClient(address, getHttpPort());
        if(config_client_!= 0) {
            std::string config_ = config_client_->getServiceConfiguration(serviceId);
            if(!config_.empty()){
                return config_;
            }
        }
    }
    catch(...){
        // configuration from the file beside the exe...
        if(!config_path.empty()){
            return getConfigStr(config_path); // Start service from local file.
        }
        return getConfigStr(getConfigPath(serviceId, domain)); // Start service from local file.
    }
}

void setServiceConfig(std::string serviceId, std::string address, std::string config, std::string config_path){
    try {
        auto config_client_ = getConfigClient(address, getHttpPort());
        if(config_client_!= 0) {
            if(config_client_->setServiceConfiguration(config)){
                return;
            }
        }
    }
    catch(...){
        if(config_path.empty()){
            config_path = getConfigPath(serviceId, domain);
        }
        // save to the local file if configuration service fail to save it or is not found
        std::ofstream file;
        file.open(config_path);
        file << config;
        file.close();
    }
}
