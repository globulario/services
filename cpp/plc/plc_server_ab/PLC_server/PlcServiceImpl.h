#pragma once

// The plc service.
#include "GlobularServer/globularserver.h"
#include <grpcpp/grpcpp.h>
#include "plc.pb.h"
#include "plc.grpc.pb.h"

#include <string>
#include <map>
#include <list>
#include <sstream>
#include "libplctag.h"

enum TagType {
	BOOL_TAG_TYPE = 0,
	SINT_TAG_TYPE = 1,
	INT_TAG_TYPE = 2,
	DINT_TAG_TYPE = 3,
	REAL_TAG_TYPE = 4,
	LREAL_TAG_TYPE = 5,
	LINT_TAG_TYPE = 6
};

enum CpuType {
	PLC = 0,
	PLC5 = 1,
	SLC = 2,
	SLC500 = 3,
	MICROLOGIX = 4,
	MLGX = 5,
	COMPACTLOGIX = 6,
	CLGX = 7,
	LGX = 8,
	CONTROLLOGIX = 9,
	CONTROLOGIX = 10,
	FLEXLOGIX = 11,
	FLGX = 12
};

enum ProtocolType {
	AB_EIP = 0,
	AB_CIP = 1
};

enum PortType {
	BACKPLANE = 0,
	NET_ETHERNET = 1,
	DH_PLUS_CHANNEL_A = 2,
	DH_PLUS_CHANNEL_B = 3,
	SERIAL = 4
};

/*
	A simple c++ struct to hold in memory the connection params.
*/
struct Connection {
	std::string id;
    ProtocolType protocol; // protocol type: ab_eip, ab_cip
    std::string ip;       // IP address: 192.168.1.10
    CpuType cpu;      // AB CPU model: plc,plc5,slc,slc500,micrologix,mlgx,compactlogix,clgx,lgx,controllogix,contrologix,flexlogix,flgx
    PortType    portType; // Communication Port Type: 1- Backplane, 2- Control Net/Ethernet, DH+ Channel A, DH+ Channel B, 3- Serial
    int    slot;     // Slot number where cpu is installed
	int64_t   timeout;  // Time out for reading/writing tags
	bool save; // mark connection for save or not...
};

/*
	Implementation of the PLC service.
 */
class PlcServiceImpl final : public plc::PlcService::Service , public Globular::GlobularService
{
	std::map<std::string, int> tags;
	std::map<std::string, std::string> paths;

    // keep map of connection.
	std::map<std::string, Connection> connections;

	// save configuration.
	void save();

	// Get CPU Communication path in a compatible format with the CIP library
	std::string GetCpuPath(Connection connection);

	std::string GetProtocol(Connection connection);

	std::string GetPortType(Connection connection);

	std::string GetCpuType(Connection connection);

	::grpc::Status OpenTag(std::string connectionId, std::string name, TagType type, int elementCount = 1);

	int GetTypeSize(TagType type);

public:
	// The constructor
    PlcServiceImpl( std::string id = "plc_server_ab",
                    std::string name = "plc.PlcService",
                    std::string domain = "localhost",
                    std::string publisher_id = "localhost",
                    bool allow_all_origins = false,
                    std::string allowed_origins = "",
                    bool tls = false,
                    std::string version = "0.0.1",
                    unsigned int defaultPort = 10081, unsigned int defaultProxy = 10082);

	// The desctructor.
	~PlcServiceImpl();

	::grpc::Status GetConnection(::grpc::ServerContext* context, const ::plc::GetConnectionRqst* request, ::plc::GetConnectionRsp* response);

	// Create a connection.
	::grpc::Status CreateConnection(::grpc::ServerContext* context, const ::plc::CreateConnectionRqst* request, ::plc::CreateConnectionRsp* response);
	
	// Delete a connection.
	::grpc::Status DeleteConnection(::grpc::ServerContext* context, const ::plc::DeleteConnectionRqst* request, ::plc::DeleteConnectionRsp* response);

	// Close a connection.
	::grpc::Status CloseConnection(::grpc::ServerContext* context, const ::plc::CloseConnectionRqst* request, ::plc::CloseConnectionRsp* response);

	// * Write tag *
	::grpc::Status WriteTag(::grpc::ServerContext* context, const ::plc::WriteTagRqst* request, ::plc::WriteTagRsp* response);
	
	// * Get tags *
	::grpc::Status ReadTag(::grpc::ServerContext* context, const ::plc::ReadTagRqst* request, ::plc::ReadTagRsp* response);

};

