#ifndef GLOBULAR_CONFIG_H__
#define GLOBULAR_CONFIG_H__

#include <string>
#include <fstream>
#include <filesystem>
#include "json.hpp"

//////////////////////////////////////////////////////////////////////////////////////////
// Utilie functions.

/**
 * @brief replaceAll
 * @param str
 * @param from
 * @param to
 * @return
 */
std::string replaceAll(std::string str, const std::string& from, const std::string& to);

/**
 * @brief getexepath
 * @return
 */
std::string getexepath();

/**
 * @brief sleep
 * @param milliseconds
 */
void sleep(unsigned milliseconds);

/**
 * @brief getLocalConfigPath
 * @return
 */
std::string getLocalConfigPath();

/**
 * @brief getRootDir
 * @return
 */
const std::string getRootDir();

/**
 * @brief getConfigPath Get the configuration path from the exec path...
 * @return
 */
std::string getConfigPath(std::string serviceId, std::string domain);

// Retreive a service configuration json.
const std::string getConfigStr(std::string path);



/**
 * @brief Get local globualr address
 * @return
 */
std::string getLocalAddress();

std::string getLocalDomain();

std::string getLocalProtocol();

/**
 * @brief getLocalPort The port of the active protocol.
 * @return
 */
int getLocalPort();

int getHttpPort();

int getHttpsPort();

namespace Globular{
    class ConfigClient;
}

// A singleton that return the config client instance.
Globular::ConfigClient* getConfigClient(std::string domain, int port);

// Return the service configuration
std::string getServiceConfig(std::string serviceId, std::string domain, std::string config_path);

// Save a configuration
void setServiceConfig(std::string serviceId, std::string domain, std::string config, std::string config_path);

#endif // GLOBULAR_CONFIG_H__
