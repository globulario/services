#ifndef GLOBULARCLIENT_H
#define GLOBULARCLIENT_H

#include <string>
#include <vector>
#include <map>
#include <list>
#include <grpcpp/client_context.h>

using grpc::ClientContext;
using grpc::Channel;

namespace  Globular {

struct ServiceConfig{
    std::string Address;
    std::string Id;
    std::string Name;
    std::string Path;
    std::string ConfigPath;
    long ModTime;
    std::string Proto;
    unsigned int Port;
    unsigned int Proxy;
    std::string Domain;
    std::string Description;
    std::vector<std::string> Keywords;
    std::vector<std::string> Discoveries;
    std::vector<std::string> Repositories;

    // TLS
    bool TLS;
    std::string CertAuthorityTrust;
    std::string CertFile;
    std::string KeyFile;

};

struct ServerConfig {
    std::string Domain;
    std::string Name;
    std::string Mac;
    std::string Protocol;
    std::string PortsRange;
    std::string CertStableURL;
    std::string CertURL;
    unsigned int PortHttp;
    unsigned int PortHttps;
    std::string AdminEmail;
    unsigned int SessionTimeout;
    unsigned int CertExpirationDelay;
    std::string Version;
    std::vector<std::string> Discoveries;
    std::vector<std::string> DNS; // list of dns servers where the server is registered.
};


class Client
{
     ServiceConfig *config;

     /**
      * @brief getRemoteServiceConfig
      * @param serviceId
      * @param domain
      * @param configurationPort
      * @return
      */
     std::string getServiceConfig(std::string serviceId, std::string domain, unsigned int configurationPort);

    /**
     * @brief getCaCertificate
     * @param domain
     * @param ConfigurationPort
     * @return
     */
    std::string getCaCertificate(std::string domain, unsigned int ConfigurationPort);

    /**
     * @brief signCaCertificate Make certificate request signed by Certificate Authority.
     * @param domain The domain of the CA
     * @param ConfigurationPort The configuation port of the server that act the CA.
     * @param csr The certificate signing request.
     * @return A client certificate.
     */
    std::string signCaCertificate(std::string domain, unsigned int ConfigurationPort, std::string csr);

    /**
     * @brief generateClientPrivateKey Generate client private key.
     * @param path The path of the key file.
     * @param pwd The password.
     */
    void generateClientPrivateKey(std::string path, std::string pwd);


    /**
     * @brief generateClientCertificateSigningRequest
     * @param path The signing request.
     * @param domain The domain of the
     */
    void generateClientCertificateSigningRequest(std::string path, std::string domain);

    /**
     * @brief keyToPem
     * @param name
     * @param path
     * @param pwd
     */
    void keyToPem(std::string name, std::string path, std::string pwd);

public:
    Client(std::string name, std::string domain, unsigned int configurationPort);

    std::string getName() const;
    std::string getDomain() const;
    unsigned int getPort() const;

    // TLS functions.

    // if set at true that means the connection is secure.
    bool getTls() const;
    void setTls(bool value);

    // The CA certificate
    std::string getCaFile() const;
    void setCaFile(const std::string &value);

    // The Key file.
    std::string getKeyFile() const;
    void setKeyFile(const std::string &value);

    // The certificate file.
    std::string getCertFile() const;
    void setCertFile(const std::string &value);

    // init the client informations.
    void initClient(std::string id, std::string domain, unsigned int port);

    // Close the connection.
    void close();

   void getClientContext( ClientContext& , std::string token = "", std::string application  = "", std::string domain = "", std::string path = "");

protected:
    std::shared_ptr<Channel> channel;
};

}

#endif // GLOBULARCLIENT_H
