package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

func (srv *server) getPeerInfos(address, mac string) (*resourcepb.Peer, error) {

	client, err := getResourceClient(address)
	if err != nil {
		logger.Error("fail to connect with remote resource service", "error", err)
		return nil, err
	}

	peers, err := client.GetPeers(`{"mac":"` + mac + `"}`)
	if err != nil {
		return nil, err
	}

	if len(peers) == 0 {
		return nil, errors.New("no peer found with mac address " + mac + " at address " + address)
	}

	return peers[0], nil

}

/** Retreive the peer public key */
func (srv *server) getPeerPublicKey(address, mac string) (string, error) {

	if len(mac) == 0 {
		mac = srv.Mac
	}

	if mac == srv.Mac {
		key, err := security.GetPeerKey(mac)
		if err != nil {
			return "", err
		}

		return string(key), nil
	}

	client, err := getResourceClient(address)
	if err != nil {
		return "", err
	}

	return client.GetPeerPublicKey(mac)
}

func initPeer(values interface{}) *resourcepb.Peer {
	values_ := values.(map[string]interface{})
	state := resourcepb.PeerApprovalState(int32(Utility.ToInt(values_["state"])))

	PortHttp := int32(80)
	if values_["PortHttp"] != nil {
		PortHttp = int32(Utility.ToInt(values_["PortHttp"]))
	} else if values_["port_http"] != nil {
		PortHttp = int32(Utility.ToInt(values_["port_http"]))
	}

	PortHttps := int32(443)
	if values_["PortHttps"] != nil {
		PortHttps = int32(Utility.ToInt(values_["PortHttps"]))
	} else if values_["port_https"] != nil {
		PortHttps = int32(Utility.ToInt(values_["port_https"]))
	}

	hostname := values_["hostname"].(string)
	domain := values_["domain"].(string)

	ExternalIpAddress := ""
	if values_["external_ip_address"] != nil {
		ExternalIpAddress = values_["external_ip_address"].(string)
	} else if values_["ExternalIpAddress"] != nil {
		ExternalIpAddress = values_["ExternalIpAddress"].(string)
	}

	LocalIpAddress := ""
	if values_["local_ip_address"] != nil {
		LocalIpAddress = values_["local_ip_address"].(string)
	} else if values_["LocalIpAddress"] != nil {
		LocalIpAddress = values_["LocalIpAddress"].(string)
	}

	mac := values_["mac"].(string)
	p := &resourcepb.Peer{Protocol: values_["protocol"].(string), PortHttp: PortHttp, PortHttps: PortHttps, Hostname: hostname, Domain: domain, ExternalIpAddress: ExternalIpAddress, LocalIpAddress: LocalIpAddress, Mac: mac, Actions: make([]string, 0), State: state}

	var actions_ []interface{}
	switch values_["actions"].(type) {
	case primitive.A:
		actions_ = []interface{}(values_["actions"].(primitive.A))
	case []interface{}:
		actions_ = values_["actions"].([]interface{})
	}

	for j := 0; j < len(actions_); j++ {
		p.Actions = append(p.Actions, actions_[j].(string))
	}

	return p
}

func getLocalPeer() *resourcepb.Peer {
	// Now I will return peers actual informations.
	hostname, _ := os.Hostname()
	domain, _ := config.GetDomain()
	localConfig, _ := config.GetLocalConfig(true)

	local_peer_ := new(resourcepb.Peer)
	local_peer_.TypeName = "Peer"
	local_peer_.Protocol = localConfig["Protocol"].(string)
	local_peer_.PortHttp = int32(Utility.ToInt(localConfig["PortHttp"]))
	local_peer_.PortHttps = int32(Utility.ToInt(localConfig["PortHttps"]))
	local_peer_.Hostname = hostname
	local_peer_.Domain = domain
	local_peer_.ExternalIpAddress = Utility.MyIP()
	local_peer_.LocalIpAddress = config.GetLocalIP()
	local_peer_.Mac, _ = config.GetMacAddress()
	local_peer_.State = resourcepb.PeerApprovalState_PEER_PENDING

	return local_peer_
}

// AcceptPeer handles the acceptance of a peer into the system.
// It updates the peer's approval state in the persistence store, assigns required DNS actions,
// updates the local hosts file, and sets resource ownership for the peer's domain.
// The function also publishes events to signal peer changes both locally and remotely.
// Returns an AcceptPeerRsp indicating the result of the operation or an error if any step fails.
//
// Parameters:
//   ctx - The context for the request.
//   rqst - The AcceptPeerRqst containing peer information.
//
// Returns:
//   *resourcepb.AcceptPeerRsp - The response indicating success.
//   error - An error if the operation fails.
func (srv *server) AcceptPeer(ctx context.Context, rqst *resourcepb.AcceptPeerRqst) (*resourcepb.AcceptPeerRsp, error) {
	// Get the persistence connection
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + Utility.GenerateUUID(rqst.Peer.Mac) + `"}`

	// Now I will retrieve the peer informations.
	setState := map[string]interface{}{"$set": map[string]interface{}{"state": resourcepb.PeerApprovalState_PEER_ACCETEP}}
	setStateStr, err := Utility.ToJson(setState)
	if err != nil {
		return nil, err
	}

	err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Peers", q, setStateStr, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Add actions require by peer...
	srv.addPeerActions(rqst.Peer.Mac, []string{"/dns.DnsService/SetA"})
	srv.addPeerActions(rqst.Peer.Mac, []string{"/dns.DnsService/SetAAAA"})
	srv.addPeerActions(rqst.Peer.Mac, []string{"/dns.DnsService/SetCAA"})
	srv.addPeerActions(rqst.Peer.Mac, []string{"/dns.DnsService/SetText"})
	srv.addPeerActions(rqst.Peer.Mac, []string{"/dns.DnsService/RemoveText"})

	// set the remote peer in /etc/hosts
	srv.setLocalHosts(rqst.Peer)

	// Here I will append the resource owner...
	domain := rqst.Peer.Hostname
	if rqst.Peer.Domain != "localhost" {
		domain += "." + rqst.Peer.Domain
	}

	// in case local dns is use that peers will be able to change values releated to it domain.
	// but no other peer will be able to do it...
	srv.addResourceOwner(domain, "domain", rqst.Peer.Mac, rbacpb.SubjectType_PEER)
	jsonStr, err := protojson.Marshal(rqst.Peer)
	if err != nil {
		return nil, err
	}

	// signal peers changes...
	srv.publishEvent("update_peers_evt", jsonStr, srv.Address)

	address_ := rqst.Peer.Hostname
	if rqst.Peer.Domain != "localhost" {
		address_ += "." + rqst.Peer.Domain
	}

	if rqst.Peer.Protocol == "https" {
		address_ += ":" + Utility.ToString(rqst.Peer.PortHttps)
	} else {
		address_ += ":" + Utility.ToString(rqst.Peer.PortHttp)
	}

	jsonStr, err = protojson.Marshal(getLocalPeer())
	if err != nil {
		return nil, err
	}
	srv.publishRemoteEvent(address_, "update_peers_evt", jsonStr)

	return &resourcepb.AcceptPeerRsp{Result: true}, nil
}

func (srv *server) addPeerActions(mac string, actions_ []string) error {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	var q string
	if p.GetStoreType() == "MONGO" {
		q = `{"mac":"` + mac + `"}`
	} else if p.GetStoreType() == "SCYLLA" || p.GetStoreType() == "SQL" {
		q = `SELECT * FROM Peers WHERE mac='` + mac + `'`
	} else {
		return errors.New("unknown database type " + p.GetStoreType())
	}

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Peers", q, ``)
	if err != nil {
		return err
	}

	peer := values.(map[string]interface{})

	needSave := false
	if peer["actions"] == nil {
		peer["actions"] = actions_
		needSave = true
	} else {

		var actions []interface{}
		switch peer["actions"].(type) {
		case primitive.A:
			actions = []interface{}(peer["actions"].(primitive.A))
		case []interface{}:
			actions = peer["actions"].([]interface{})
		}

		for j := 0; j < len(actions_); j++ {
			exist := false
			for i := 0; i < len(actions); i++ {
				if actions[i].(string) == actions_[j] {
					exist = true
					break
				}
			}
			if !exist {
				actions = append(actions, actions_[j])
				needSave = true
			}
		}
		peer["actions"] = actions
	}

	if needSave {
		jsonStr := serialyseObject(peer)
		err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Peers", q, string(jsonStr), ``)
		if err != nil {
			return err
		}
	}

	// signal peers changes...
	srv.publishEvent("update_peer_"+mac+"_evt", []byte{}, srv.Address)

	return nil
}

// AddPeerActions adds a set of actions to a peer identified by its MAC address.
// It calls the internal addPeerActions method to perform the update and publishes an event
// to notify other components of the change. Returns a response indicating success or an error
// if the operation fails.
//
// Parameters:
//   ctx - The context for the request, used for cancellation and deadlines.
//   rqst - The request containing the peer's MAC address and the actions to add.
//
// Returns:
//   *resourcepb.AddPeerActionsRsp - The response indicating the result of the operation.
//   error - An error if the operation fails.
func (srv *server) AddPeerActions(ctx context.Context, rqst *resourcepb.AddPeerActionsRqst) (*resourcepb.AddPeerActionsRsp, error) {

	err := srv.addPeerActions(rqst.Mac, rqst.Actions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("update_peer_"+rqst.Mac+"_evt", []byte{}, srv.Address)

	return &resourcepb.AddPeerActionsRsp{Result: true}, nil

}

func (srv *server) deletePeer(token, address string) error {
	// Connect to remote server and call Register peer on it...
	client, err := getResourceClient(address)
	if err != nil {
		return err
	}

	return client.DeletePeer(token, srv.Mac)

}

// DeletePeer deletes a peer from the system based on the provided DeletePeerRqst.
// It performs the following steps:
//   - Retrieves the persistence store connection.
//   - Searches for the peer in the database using its MAC address.
//   - If found, initializes the peer object and deletes all associated access and permissions.
//   - Removes the peer's public key and entry from /etc/hosts.
//   - Publishes events to signal peer deletion to other components.
//   - Attempts to remove the peer from the remote end using a security token.
// Returns a DeletePeerRsp indicating success or an error if any step fails.
func (srv *server) DeletePeer(ctx context.Context, rqst *resourcepb.DeletePeerRqst) (*resourcepb.DeletePeerRsp, error) {
	// Get the persistence connection
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	q := `{"_id":"` + Utility.GenerateUUID(rqst.Peer.Mac) + `"}`

	// try to get the peer from the database.
	data, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Peers", q, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// the peer was not found.
	if data == nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no peer with mac "+rqst.Peer.Mac+" was found")))
	}

	// init the peer object.
	peer := initPeer(data)

	// Delete all peer access.
	srv.deleteAllAccess(peer.Mac, rbacpb.SubjectType_PEER)

	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Peers", q, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Delete permissions
	srv.deleteResourcePermissions(peer.Mac)
	srv.deleteAllAccess(rqst.Peer.Mac, rbacpb.SubjectType_PEER)

	// Delete peer public key...
	security.DeletePublicKey(peer.Mac)

	// remove from /etc/hosts
	srv.removeFromLocalHosts(peer)

	// Here I will append the resource owner...
	domain := peer.Hostname
	if len(peer.Domain) > 0 {
		domain += "." + peer.Domain
	}

	// signal peers changes...
	address := peer.Hostname
	if peer.Domain != "localhost" {
		address += "." + peer.Domain
	}

	if peer.Protocol == "https" {
		address += ":" + Utility.ToString(peer.PortHttps)
	} else {
		address += ":" + Utility.ToString(peer.PortHttp)
	}

	srv.publishEvent("delete_peer"+peer.Mac+"_evt", []byte{}, srv.Address)
	srv.publishEvent("delete_peer"+peer.Mac+"_evt", []byte{}, address)

	srv.publishEvent("delete_peer_evt", []byte(peer.Mac), srv.Address)
	srv.publishEvent("delete_peer_evt", []byte(peer.Mac), address)

	address_ := peer.Domain
	if peer.Protocol == "https" {
		address_ += ":" + Utility.ToString(peer.PortHttps)
	} else {
		address_ += ":" + Utility.ToString(peer.PortHttp)
	}

	// Also remove the peer at the other end...
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	srv.deletePeer(token, address_)

	return &resourcepb.DeletePeerRsp{
		Result: true,
	}, nil
}

// GetPeerApprovalState retrieves the approval state of a peer identified by its MAC address and remote peer address.
// If the MAC address is not provided in the request, it attempts to obtain it from the server configuration.
// Returns a response containing the peer's approval state or an error if the peer information cannot be retrieved.
//
// Parameters:
//   ctx - The context for the request.
//   rqst - The request containing the MAC address and remote peer address.
//
// Returns:
//   *resourcepb.GetPeerApprovalStateRsp - The response containing the peer's approval state.
//   error - An error if the operation fails.
func (srv *server) GetPeerApprovalState(ctx context.Context, rqst *resourcepb.GetPeerApprovalStateRqst) (*resourcepb.GetPeerApprovalStateRsp, error) {
	mac := rqst.Mac
	if len(mac) == 0 {
		var err error
		mac, err = config.GetMacAddress()
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	peer, err := srv.getPeerInfos(rqst.RemotePeerAddress, mac)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.GetPeerApprovalStateRsp{State: peer.GetState()}, nil
}

func (srv *server) GetPeerPublicKey(ctx context.Context, rqst *resourcepb.GetPeerPublicKeyRqst) (*resourcepb.GetPeerPublicKeyRsp, error) {
	public_key, err := srv.getPeerPublicKey(rqst.RemotePeerAddress, rqst.Mac)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.GetPeerPublicKeyRsp{PublicKey: public_key}, nil
}

func (srv *server) getPeers(query string) ([]*resourcepb.Peer, error) {
	// Get the persistence connection
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	peers, err := p.Find(context.Background(), "local_resource", "local_resource", "Peers", query, "")
	if err != nil {
		return nil, err
	}

	// Filter out the server's own peer
	var result []*resourcepb.Peer
	for _, p := range peers {
		peerMap, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		mac, ok := peerMap["mac"].(string)
		if !ok {
			continue
		}
		if mac != srv.Mac {
			peerObj := initPeer(peerMap)
			result = append(result, peerObj)
		}
	}

	return result, nil
}

// GetPeers streams a list of peers from the persistence store based on the provided query and options.
// It retrieves peers from the "local_resource" collection, excluding the server's own peer (by MAC address).
// Results are sent in batches of up to 100 peers per response via the provided gRPC stream.
// If an error occurs during retrieval or streaming, an appropriate gRPC status error is returned.
//
// Parameters:
//   - rqst: The request containing the query and options for filtering peers.
//   - stream: The gRPC server stream used to send batches of peers.
//
// Returns:
//   - error: A gRPC status error if any operation fails, otherwise nil.
func (srv *server) GetPeers(rqst *resourcepb.GetPeersRqst, stream resourcepb.ResourceService_GetPeersServer) error {

	// Get the persistence connection
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	query := rqst.Query
	if len(query) == 0 {
		query = "{}"
	}

	peers, err := p.Find(context.Background(), "local_resource", "local_resource", "Peers", query, rqst.Options)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// No I will stream the result over the networks.
	maxSize := 100
	values := make([]*resourcepb.Peer, 0)

	for i := 0; i < len(peers); i++ {
		p := initPeer(peers[i])
		if p.Mac != srv.Mac {
			values = append(values, p)
		}
		if len(values) >= maxSize {
			err := stream.Send(
				&resourcepb.GetPeersRsp{
					Peers: values,
				},
			)
			if err != nil {
				return status.Errorf(
					codes.Internal,
					"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			values = make([]*resourcepb.Peer, 0)
		}
	}

	// Send reminding values.
	err = stream.Send(
		&resourcepb.GetPeersRsp{
			Peers: values,
		},
	)

	if err != nil {
		return status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return nil
}

func (srv *server) registerPeer(address string) (*resourcepb.Peer, string, error) {
	// Connect to remote server and call Register peer on it...
	client, err := getResourceClient(address)
	if err != nil {
		return nil, "", err
	}

	// get the local public key.
	key, err := security.GetLocalKey()
	if err != nil {
		logger.Info("log", "args", []interface{}{"fail to get local key with error ", err})
		return nil, "", err
	}

	// Get the configuration address with it http port...
	domain, _ := config.GetDomain()
	hostname, err := os.Hostname()
	if err != nil {
		return nil, "", err
	}

	macAddress, err := config.GetMacAddress()
	if err != nil {
		return nil, "", err
	}

	localConfig, err := config.GetLocalConfig(true)
	httpPort := Utility.ToInt(localConfig["PortHttp"])
	httpsPort := Utility.ToInt(localConfig["PortHttps"])
	protocol := localConfig["Protocol"].(string)

	if err != nil {
		logger.Info("log", "args", []interface{}{"fail to get local config ", err})
		return nil, "", err
	}

	return client.RegisterPeer(string(key), &resourcepb.Peer{Protocol: protocol, PortHttp: int32(httpPort), PortHttps: int32(httpsPort), Hostname: hostname, Mac: macAddress, Domain: domain, ExternalIpAddress: Utility.MyIP(), LocalIpAddress: config.GetLocalIP()})
}

// RegisterPeer registers a new peer in the system.
//
// It performs the following steps:
//   - Validates the request and peer information (peer object, local and external IP addresses).
//   - Prevents registering the server as its own peer.
//   - Checks if a peer with the same MAC address already exists; if so, returns its information and public key.
//   - If no MAC address is provided, registers the server itself on another peer, saves the received peer info, and publishes events.
//   - If a MAC address is provided, inserts the peer into the local resource database with a pending approval state.
//   - Saves the peer's public key and sets up resource ownership and actions.
//   - Publishes peer update events locally and remotely.
//
// Returns the registered peer information and its public key, or an error if registration fails.
func (srv *server) RegisterPeer(ctx context.Context, rqst *resourcepb.RegisterPeerRqst) (*resourcepb.RegisterPeerRsp, error) {

	if rqst.Peer == nil {
		return nil, errors.New("no peer object was given in the request")
	}

	if rqst.Peer.LocalIpAddress == "" {
		return nil, errors.New("no local ip address was given in the request")
	}

	if rqst.Peer.ExternalIpAddress == "" {
		return nil, errors.New("no external ip address was given in the request")
	}

	// set the remote peer in /etc/hosts
	srv.setLocalHosts(rqst.Peer)

	// Here I will first look if a peer with a same name already exist on the
	if srv.Mac == rqst.Peer.Mac {
		return nil, errors.New("can not register peer to itself")
	}

	// Get the persistence connection
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + Utility.GenerateUUID(rqst.Peer.Mac) + `"}`

	// Here I will first look if a peer with a same name already exist on the
	// resources...
	if len(rqst.Peer.Mac) > 0 {
		values, _ := p.FindOne(context.Background(), "local_resource", "local_resource", "Peers", q, "")
		if values != nil {
			p := initPeer(values)
			pubKey, err := security.GetPeerKey(p.Mac)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

			return &resourcepb.RegisterPeerRsp{
				Peer:      p,
				PublicKey: string(pubKey),
			}, nil
		}
	}

	// No authorization exist for that peer I will insert it.
	// Here will create the new peer.
	peer := make(map[string]interface{})
	peer["hostname"] = rqst.Peer.Hostname
	peer["domain"] = rqst.Peer.Domain
	peer["protocol"] = rqst.Peer.Protocol

	// If no mac address was given it mean the request came from a web application
	// so the intention is to register the server itself on another srv...
	// This can also be done with the command line tool but in that case all values will be
	// set on the peers...
	if len(rqst.Peer.Mac) == 0 {

		// In that case I will use the hostname and the domain to set the address
		address_ := rqst.Peer.Hostname
		if rqst.Peer.Domain != "localhost" {
			address_ += "." + rqst.Peer.Domain
		}

		if rqst.Peer.Protocol == "https" {
			address_ += ":" + Utility.ToString(rqst.Peer.PortHttps)
		} else {
			address_ += ":" + Utility.ToString(rqst.Peer.PortHttp)
		}

		// In that case I want to register the server to another srv.
		peer_, public_key, err := srv.registerPeer(address_)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		// Save the received values on the db
		peer := make(map[string]interface{})
		peer["_id"] = Utility.GenerateUUID(peer_.Mac) // The peer mac address will be use as peers id
		peer["domain"] = peer_.Domain

		// keep the address where the configuration can be found...
		// in case of docker instance that will be usefull to get peer addres config...
		peer["protocol"] = rqst.Peer.Protocol
		peer["PortHttps"] = rqst.Peer.PortHttps
		peer["PortHttp"] = rqst.Peer.PortHttp
		peer["hostname"] = peer_.Hostname
		peer["mac"] = peer_.Mac
		peer["local_ip_address"] = peer_.LocalIpAddress
		peer["external_ip_address"] = peer_.ExternalIpAddress
		peer["state"] = resourcepb.PeerApprovalState_PEER_ACCETEP
		peer["actions"] = []interface{}{}

		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Peers", peer, "")
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		// Here I wiil save the public key in the keys directory.
		err = security.SetPeerPublicKey(peer_.Mac, public_key)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		// set the remote peer in /etc/hosts
		srv.setLocalHosts(peer_)

		// in case local dns is use that peers will be able to change values releated to it domain.
		// but no other peer will be able to do it...
		srv.addResourceOwner(peer_.Domain, "domain", peer_.Mac, rbacpb.SubjectType_PEER)

		jsonStr, err := protojson.Marshal(peer_)
		if err != nil {
			return nil, err
		}

		// Update peer event.
		srv.publishEvent("update_peers_evt", jsonStr, srv.Address)

		address := rqst.Peer.Hostname
		if rqst.Peer.Domain != "localhost" {
			address += "." + rqst.Peer.Domain
		}

		if rqst.Peer.Protocol == "https" {
			address += ":" + Utility.ToString(rqst.Peer.PortHttps)
		} else {
			address += ":" + Utility.ToString(rqst.Peer.PortHttp)
		}

		// So here I need to publish my information as a pee

		// Publish local peer information...
		jsonStr, err = protojson.Marshal(getLocalPeer())
		if err != nil {
			return nil, err
		}

		srv.publishRemoteEvent(address, "update_peers_evt", jsonStr)

		// Set peer action
		srv.addPeerActions(peer_.Mac, []string{"/dns.DnsService/SetA"})
		srv.addPeerActions(peer_.Mac, []string{"/dns.DnsService/SetAAAA"})
		srv.addPeerActions(peer_.Mac, []string{"/dns.DnsService/SetCaa"})
		srv.addPeerActions(peer_.Mac, []string{"/dns.DnsService/SetNs"})
		srv.addPeerActions(peer_.Mac, []string{"/dns.DnsService/SetMx"})
		srv.addPeerActions(peer_.Mac, []string{"/dns.DnsService/SetSoa"})
		srv.addPeerActions(peer_.Mac, []string{"/dns.DnsService/SetText"})
		srv.addPeerActions(peer_.Mac, []string{"/dns.DnsService/RemoveText"})

		// Send back the peers informations.
		return &resourcepb.RegisterPeerRsp{Peer: peer_, PublicKey: public_key}, nil

	}

	// Here I will keep the peer info until it will be accepted by the admin of the other peer.
	peer["_id"] = Utility.GenerateUUID(rqst.Peer.Mac)
	peer["mac"] = rqst.Peer.Mac
	peer["hostname"] = rqst.Peer.Hostname
	peer["domain"] = rqst.Peer.Domain
	peer["protocol"] = rqst.Peer.Protocol
	peer["PortHttps"] = rqst.Peer.PortHttps
	peer["PortHttp"] = rqst.Peer.PortHttp
	peer["local_ip_address"] = rqst.Peer.LocalIpAddress
	peer["external_ip_address"] = rqst.Peer.ExternalIpAddress
	peer["state"] = resourcepb.PeerApprovalState_PEER_PENDING
	peer["actions"] = []interface{}{}

	// if the token is generate by the sa and it has permission i will accept the peer directly
	/*
		peer["state"] = resourcepb.PeerApprovalState_PEER_ACCETEP
		peer["actions"] = []interface{}{"/dns.DnsService/SetA"}
		peer["actions"] = []interface{}{"/dns.DnsService/SetAAAA"}
		peer["actions"] = []interface{}{"/dns.DnsService/SetCAA"}
		peer["actions"] = []interface{}{"/dns.DnsService/SetText"}
		peer["actions"] = []interface{}{"/dns.DnsService/RemoveText"}
	*/

	domain := rqst.Peer.Hostname
	if len(rqst.Peer.Domain) > 0 {
		domain += "." + rqst.Peer.Domain
	}

	srv.addResourceOwner(domain, "domain", rqst.Peer.Mac, rbacpb.SubjectType_PEER)

	// Insert the peer into the local resource database.
	_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Peers", peer, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Here I wiil save the public key in the keys directory.
	err = security.SetPeerPublicKey(rqst.Peer.Mac, rqst.PublicKey)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// actions will need to be set by admin latter...
	pubKey, err := security.GetPeerKey(getLocalPeer().Mac)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr, err := protojson.Marshal(rqst.Peer)
	if err != nil {
		return nil, err
	}

	// signal peers changes...
	srv.publishEvent("update_peers_evt", jsonStr, srv.GetAddress())

	address_ := rqst.Peer.Hostname
	if rqst.Peer.Domain != "localhost" {
		address_ += "." + rqst.Peer.Domain
	}

	if rqst.Peer.Protocol == "https" {
		address_ += ":" + Utility.ToString(rqst.Peer.PortHttps)
	} else {
		address_ += ":" + Utility.ToString(rqst.Peer.PortHttp)
	}

	jsonStr, err = protojson.Marshal(getLocalPeer())
	if err != nil {
		return nil, err
	}

	srv.publishRemoteEvent(address_, "update_peers_evt", jsonStr)

	srv.setLocalHosts(getLocalPeer())

	return &resourcepb.RegisterPeerRsp{
		Peer:      getLocalPeer(),
		PublicKey: string(pubKey),
	}, nil
}

// RejectPeer handles the rejection of a peer connection request.
// It updates the peer's state in the persistence store to indicate rejection,
// publishes an event to signal the change to local listeners, and notifies the remote peer.
// Returns a RejectPeerRsp with the result of the operation or an error if any step fails.
//
// Parameters:
//   ctx - The context for the request.
//   rqst - The RejectPeerRqst containing peer information to be rejected.
//
// Returns:
//   *resourcepb.RejectPeerRsp - The response indicating success or failure.
//   error - An error if the operation fails.
func (srv *server) RejectPeer(ctx context.Context, rqst *resourcepb.RejectPeerRqst) (*resourcepb.RejectPeerRsp, error) {

	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + Utility.GenerateUUID(rqst.Peer.Mac) + `"}`

	setState := `{ "$set":{"state":2}}`

	err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Peers", q, setState, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr, err := protojson.Marshal(rqst.Peer)
	if err != nil {
		return nil, err
	}

	// signal peers changes...
	srv.publishEvent("update_peers_evt", jsonStr, srv.Address)

	address_ := rqst.Peer.Hostname
	if rqst.Peer.Domain != "localhost" {
		address_ += "." + rqst.Peer.Domain
	}

	if rqst.Peer.Protocol == "https" {
		address_ += ":" + Utility.ToString(rqst.Peer.PortHttps)
	} else {
		address_ += ":" + Utility.ToString(rqst.Peer.PortHttp)
	}

	jsonStr, err = protojson.Marshal(getLocalPeer())
	if err != nil {
		return nil, err
	}
	srv.publishRemoteEvent(address_, "update_peers_evt", jsonStr)

	return &resourcepb.RejectPeerRsp{Result: true}, nil
}

// RemovePeerAction removes a specified action from the list of actions associated with a peer identified by its MAC address.
// It retrieves the peer from the persistence store, checks if the action exists, and removes it if present.
// If the action is successfully removed, the peer record is updated in the store and an update event is published.
// Returns a response indicating the result or an error if the operation fails.
func (srv *server) RemovePeerAction(ctx context.Context, rqst *resourcepb.RemovePeerActionRqst) (*resourcepb.RemovePeerActionRsp, error) {
	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	var q string
	if p.GetStoreType() == "MONGO" {
		q = `{"mac":"` + rqst.Mac + `"}`
	} else if p.GetStoreType() == "SCYLLA" || p.GetStoreType() == "SQL" {
		q = `SELECT * FROM Peers WHERE mac='` + rqst.Mac + `'`
	} else {
		return nil, errors.New("unknown database type " + p.GetStoreType())
	}

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Peers", q, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	peer := values.(map[string]interface{})

	needSave := false
	if peer["actions"] == nil {
		peer["actions"] = []string{rqst.Action}
		needSave = true
	} else {
		exist := false
		actions := make([]interface{}, 0)
		for i := 0; i < len(peer["actions"].(primitive.A)); i++ {
			if peer["actions"].(primitive.A)[i].(string) == rqst.Action {
				exist = true
			} else {
				actions = append(actions, peer["actions"].(primitive.A)[i])
			}
		}
		if exist {
			peer["actions"] = actions
			needSave = true
		} else {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Peer "+rqst.Mac+" not contain actions named "+rqst.Action+"!")))
		}
	}

	if needSave {
		jsonStr := serialyseObject(peer)
		err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Peers", q, string(jsonStr), ``)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// signal peers changes...
	srv.publishEvent("update_peer_"+rqst.Mac+"_evt", []byte{}, srv.Address)

	return &resourcepb.RemovePeerActionRsp{Result: true}, nil
}

// RemovePeersAction removes a specified action from the "actions" list of all peers in the "Peers" collection.
// If the action exists for a peer, it is removed and the peer is updated in the persistence store.
// An update event is published for each modified peer.
// Returns a response indicating success or an error if any operation fails.
//
// Parameters:
//   - ctx: The context for the request.
//   - rqst: The request containing the action to be removed.
//
// Returns:
//   - *resourcepb.RemovePeersActionRsp: The response indicating the result of the operation.
//   - error: An error if the operation fails.
func (srv *server) RemovePeersAction(ctx context.Context, rqst *resourcepb.RemovePeersActionRqst) (*resourcepb.RemovePeersActionRsp, error) {
	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	q := `{}`

	values, err := p.Find(context.Background(), "local_resource", "local_resource", "Peers", q, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	for i := 0; i < len(values); i++ {
		peer := values[i].(map[string]interface{})

		needSave := false
		if peer["actions"] == nil {
			peer["actions"] = []string{rqst.Action}
			needSave = true
		} else {
			exist := false
			actions := make([]interface{}, 0)
			for i := 0; i < len(peer["actions"].(primitive.A)); i++ {
				if peer["actions"].(primitive.A)[i].(string) == rqst.Action {
					exist = true
				} else {
					actions = append(actions, peer["actions"].(primitive.A)[i])
				}
			}
			if exist {
				peer["actions"] = actions
				needSave = true
			}
		}

		if needSave {
			q = `{"_id":"` + peer["_id"].(string) + `"}`
			jsonStr := serialyseObject(peer)
			err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Peers", q, string(jsonStr), ``)
			srv.publishEvent("update_peer_"+peer["_id"].(string)+"_evt", []byte{}, srv.Address)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
		}
	}

	return &resourcepb.RemovePeersActionRsp{Result: true}, nil
}

// UpdatePeer updates the information of a peer in the persistence store based on the provided UpdatePeerRqst.
// It retrieves the peer by its MAC address, updates its protocol, ports, and IP addresses, and persists the changes.
// The function handles differences in field naming conventions for different store types (e.g., SCYLLA vs. MONGO/SQL).
// After updating, it publishes events to notify about the peer update and returns a response indicating success.
//
// Parameters:
//   ctx - The context for the request.
//   rqst - The request containing the updated peer information.
//
// Returns:
//   *resourcepb.UpdatePeerRsp - The response indicating the result of the update operation.
//   error - An error if the update fails.
func (srv *server) UpdatePeer(ctx context.Context, rqst *resourcepb.UpdatePeerRqst) (*resourcepb.UpdatePeerRsp, error) {
	// Get the persistence connection
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	q := `{"_id":"` + Utility.GenerateUUID(rqst.Peer.Mac) + `"}`

	values, err := p.FindOne(ctx, "local_resource", "local_resource", "Peers", q, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// init the peer object.
	peer := initPeer(values)

	// Here I will update the peer information.
	peer.Protocol = rqst.Peer.Protocol
	peer.PortHttps = rqst.Peer.PortHttps
	peer.PortHttp = rqst.Peer.PortHttp
	peer.LocalIpAddress = rqst.Peer.LocalIpAddress
	peer.ExternalIpAddress = rqst.Peer.ExternalIpAddress

	jsonStr, err := json.Marshal(rqst.Peer)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// update peer values.
	setValues := map[string]interface{}{"$set": map[string]interface{}{"hostname": rqst.Peer.Hostname, "domain": rqst.Peer.Domain, "protocol": rqst.Peer.Protocol, "local_ip_address": rqst.Peer.LocalIpAddress, "external_ip_address": rqst.Peer.ExternalIpAddress}}

	if p.GetStoreType() == "SCYLLA" {
		// Scylla does not support camel case...
		setValues["$set"].(map[string]interface{})["port_https"] = rqst.Peer.PortHttps
		setValues["$set"].(map[string]interface{})["port_http"] = rqst.Peer.PortHttp
	} else {
		// MONGO and SQL
		setValues["$set"].(map[string]interface{})["PortHttps"] = rqst.Peer.PortHttps
		setValues["$set"].(map[string]interface{})["PortHttp"] = rqst.Peer.PortHttp
	}

	setValues_, err := Utility.ToJson(setValues)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Peers", q, setValues_, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// signal peers changes...
	srv.publishEvent("update_peer_"+rqst.Peer.Mac+"_evt", []byte{}, srv.Address)

	address := rqst.Peer.Hostname
	if len(rqst.Peer.Domain) > 0 {
		address += "." + rqst.Peer.Domain
	}

	if rqst.Peer.Protocol == "https" {
		address += ":" + Utility.ToString(rqst.Peer.PortHttps)
	} else {
		address += ":" + Utility.ToString(rqst.Peer.PortHttp)
	}
	srv.publishEvent("update_peer_"+rqst.Peer.Mac+"_evt", []byte{}, address)

	// give the peer information...
	srv.publishEvent("update_peers_evt", jsonStr, srv.Address)

	return &resourcepb.UpdatePeerRsp{Result: true}, nil
}
