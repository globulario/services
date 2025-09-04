package main

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/globulario/services/golang/ldap/ldappb"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"github.com/go-ldap/ldap/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// LDAP specific functionality
/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// Synchronize the resource with LDAP.
func (srv *server) Synchronize(ctx context.Context, rqst *ldappb.SynchronizeRequest) (*ldappb.SynchronizeResponse, error) {
	err := srv.synchronize()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &ldappb.SynchronizeResponse{}, nil
}

// Append synchronize information.
//
//	"LdapSyncInfos": {
//	    "my__ldap":
//	      {
//	        "ConnectionId": "my__ldap",
//	        "GroupSyncInfo": {
//	          "Base": "OU=Access_Groups,OU=Groups,OU=MON,OU=CA,DC=UD6,DC=UF6",
//	          "Id": "name",
//	          "Query": "((objectClass=group))"
//	        },
//	        "Refresh": 1,
//	        "UserSyncInfo": {
//	          "Base": "OU=Users,OU=MON,OU=CA,DC=UD6,DC=UF6",
//	          "Email": "mail",
//	          "Id": "userPrincipalName",
//	          "Query": "(|(objectClass=person)(objectClass=user))"
//	        }
//	      }
//	 }
func (srv *server) SetLdapSyncInfo(ctx context.Context, rqst *ldappb.SetLdapSyncInfoRequest) (*ldappb.SetLdapSyncInfoResponse, error) {
	info := make(map[string]interface{}, 0)

	info["ConnectionId"] = rqst.Info.ConnectionId
	info["Refresh"] = rqst.Info.Refresh
	info["GroupSyncInfo"] = make(map[string]interface{}, 0)
	info["GroupSyncInfo"].(map[string]interface{})["Id"] = rqst.Info.GroupSyncInfo.Id
	info["GroupSyncInfo"].(map[string]interface{})["Base"] = rqst.Info.GroupSyncInfo.Base
	info["GroupSyncInfo"].(map[string]interface{})["Query"] = rqst.Info.GroupSyncInfo.Query
	info["UserSyncInfo"] = make(map[string]interface{}, 0)
	info["UserSyncInfo"].(map[string]interface{})["Id"] = rqst.Info.UserSyncInfo.Id
	info["UserSyncInfo"].(map[string]interface{})["Base"] = rqst.Info.UserSyncInfo.Base
	info["UserSyncInfo"].(map[string]interface{})["Query"] = rqst.Info.UserSyncInfo.Query

	if srv.LdapSyncInfos == nil {
		srv.LdapSyncInfos = make(map[string]interface{}, 0)
	}

	// Store the info.
	srv.LdapSyncInfos[rqst.Info.ConnectionId] = info

	return &ldappb.SetLdapSyncInfoResponse{}, nil
}

// Delete synchronize information
func (srv *server) DeleteLdapSyncInfo(ctx context.Context, rqst *ldappb.DeleteLdapSyncInfoRequest) (*ldappb.DeleteLdapSyncInfoResponse, error) {
	if srv.LdapSyncInfos != nil {
		if srv.LdapSyncInfos[rqst.Id] != nil {
			delete(srv.LdapSyncInfos, rqst.Id)
		}
	}

	return &ldappb.DeleteLdapSyncInfoResponse{}, nil
}

// Retreive synchronize informations
func (srv *server) GetLdapSyncInfo(ctx context.Context, rqst *ldappb.GetLdapSyncInfoRequest) (*ldappb.GetLdapSyncInfoResponse, error) {
	infos := make([]*ldappb.LdapSyncInfo, 0)

	for _, info := range srv.LdapSyncInfos {
		info_ := new(ldappb.LdapSyncInfo)

		info_.Id = info.(map[string]interface{})["Id"].(string)
		info_.ConnectionId = info.(map[string]interface{})["ConnectionId"].(string)
		info_.Refresh = info.(map[string]interface{})["Refresh"].(int32)
		info_.GroupSyncInfo = new(ldappb.GroupSyncInfo)
		info_.GroupSyncInfo.Id = info.(map[string]interface{})["GroupSyncInfo"].(map[string]interface{})["Id"].(string)
		info_.GroupSyncInfo.Base = info.(map[string]interface{})["GroupSyncInfo"].(map[string]interface{})["Base"].(string)
		info_.GroupSyncInfo.Query = info.(map[string]interface{})["GroupSyncInfo"].(map[string]interface{})["Query"].(string)
		info_.UserSyncInfo = new(ldappb.UserSyncInfo)
		info_.UserSyncInfo.Id = info.(map[string]interface{})["UserSyncInfo"].(map[string]interface{})["Id"].(string)
		info_.UserSyncInfo.Base = info.(map[string]interface{})["UserSyncInfo"].(map[string]interface{})["Base"].(string)
		info_.UserSyncInfo.Email = info.(map[string]interface{})["UserSyncInfo"].(map[string]interface{})["Email"].(string)
		info_.UserSyncInfo.Query = info.(map[string]interface{})["UserSyncInfo"].(map[string]interface{})["Query"].(string)
		if len(rqst.Id) > 0 {
			if rqst.Id == info_.Id {
				infos = append(infos, info_)
				break
			}
		} else {
			// append all infos.
			infos = append(infos, info_)
		}
	}
	// return the
	return &ldappb.GetLdapSyncInfoResponse{Infos: infos}, nil
}

// Authenticate a user with LDAP srv.
func (srv *server) Authenticate(ctx context.Context, rqst *ldappb.AuthenticateRqst) (*ldappb.AuthenticateRsp, error) {
	id := rqst.Id
	login := rqst.Login
	pwd := rqst.Pwd

	if len(id) > 0 {
		// I will made use of bind to authenticate the user.
		_, err := srv.connect(id, login, pwd)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {
		for id, _ := range srv.Connections {
			_, err := srv.connect(id, login, pwd)
			if err == nil {
				return &ldappb.AuthenticateRsp{
					Result: true,
				}, nil
			}
		}
		// fail to authenticate.
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Authentication fail for user "+rqst.Login)))
	}

	return &ldappb.AuthenticateRsp{
		Result: true,
	}, nil
}

// Create a new SQL connection and store it for futur use. If the connection already
// exist it will be replace by the new one.
func (srv *server) CreateConnection(ctx context.Context, rsqt *ldappb.CreateConnectionRqst) (*ldappb.CreateConnectionRsp, error) {

	var c connection
	var err error

	// Set the connection info from the request.
	c.Id = rsqt.Connection.Id
	c.Host = rsqt.Connection.Host
	c.Port = rsqt.Connection.Port
	c.User = rsqt.Connection.User
	c.Password = rsqt.Connection.Password

	// set or update the connection and save it in json file.
	srv.Connections[c.Id] = c

	c.conn, err = srv.connect(c.Id, c.User, c.Password)
	defer c.conn.Close()

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// In that case I will save it in file.
	err = srv.Save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &ldappb.CreateConnectionRsp{
		Result: true,
	}, nil
}

// Remove a connection from the map and the file.
func (srv *server) DeleteConnection(ctx context.Context, rqst *ldappb.DeleteConnectionRqst) (*ldappb.DeleteConnectionRsp, error) {

	id := rqst.GetId()
	if _, ok := srv.Connections[id]; !ok {
		return &ldappb.DeleteConnectionRsp{
			Result: true,
		}, nil
	}

	if srv.Connections[id].conn != nil {
		// Close the connection.
		srv.Connections[id].conn.Close()
	}

	delete(srv.Connections, id)

	// In that case I will save it in file.
	err := srv.Save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// return success.
	return &ldappb.DeleteConnectionRsp{
		Result: true,
	}, nil

}

// Close connection.
func (srv *server) Close(ctx context.Context, rqst *ldappb.CloseRqst) (*ldappb.CloseRsp, error) {
	id := rqst.GetId()
	if _, ok := srv.Connections[id]; !ok {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Connection "+id+" dosent exist!")))
	}

	srv.Connections[id].conn.Close()

	// return success.
	return &ldappb.CloseRsp{
		Result: true,
	}, nil
}

// Synchronize the list user and group whith resources...
// Here is an exemple of ldap configuration...
//
//	"LdapSyncInfos": {
//	    "my__ldap":
//	      {
//	        "ConnectionId": "my__ldap",
//	        "GroupSyncInfo": {
//	          "Base": "OU=Access_Groups,OU=Groups,OU=MON,OU=CA,DC=UD6,DC=UF6",
//	          "Id": "name",
//	          "Query": "((objectClass=group))"
//	        },
//	        "Refresh": 1,
//	        "UserSyncInfo": {
//	          "Base": "OU=Users,OU=MON,OU=CA,DC=UD6,DC=UF6",
//	          "Email": "mail",
//	          "Id": "userPrincipalName",
//	          "Query": "(|(objectClass=person)(objectClass=user))"
//	        }
//	      }
//	 }
func (srv *server) synchronize() error {

	// Here I will get the local token to generate the groups
	token, err := security.GetLocalToken(srv.Mac)
	if err != nil {
		return err
	}

	for connectionId, syncInfo_ := range srv.LdapSyncInfos {
		syncInfo := syncInfo_.(map[string]interface{})
		GroupSyncInfo := syncInfo["GroupSyncInfos"].(map[string]interface{})
		groupsInfo, err := srv.search(connectionId, GroupSyncInfo["Base"].(string), GroupSyncInfo["Query"].(string), []string{GroupSyncInfo["Id"].(string), "distinguishedName"})
		if err != nil {
			slog.Error("fail to retreive group info", "error", err)
			return err
		}

		// Print group info.
		for i := range groupsInfo {
			name := groupsInfo[i][0].([]string)[0]
			id := Utility.GenerateUUID(groupsInfo[i][1].([]string)[0])
			_, groupErr := srv.getGroup(id)
			if groupErr != nil {
				srv.createGroup(token, id, name, "") // The group member will be set latter in that function.
			}
		}

		// Synchronize account and user info...
		UserSyncInfo := syncInfo["UserSyncInfos"].(map[string]interface{})
		accountsInfo, err := srv.search(connectionId, UserSyncInfo["Base"].(string), UserSyncInfo["Query"].(string), []string{UserSyncInfo["Id"].(string), UserSyncInfo["Email"].(string), "distinguishedName", "memberOf"})
		if err != nil {
			slog.Error("fail to retreive account info", "error", err)
			return err
		}

		for i := range accountsInfo {
			// Print the list of account...
			// I will not set the password...
			if len(accountsInfo[i]) > 0 {
				if len(accountsInfo[i][0].([]string)) > 0 {
					name := strings.ToLower(accountsInfo[i][0].([]string)[0])
					if len(accountsInfo[i][1].([]string)) > 0 {
						email := strings.ToLower(accountsInfo[i][1].([]string)[0])

						if len(email) > 0 {
							// Generate the
							id := name //strings.ToLower(accountsInfo[i][2].([]string)[0])

							if strings.Index(id, "@") > 0 {
								id = strings.Split(id, "@")[0]
							}

							if len(id) > 0 {

								// Try to create account...
								a, err := srv.getAccount(id)
								if err != nil {
									err := srv.registerAccount(srv.Domain, id, name, email, id)
									if err != nil {
										slog.Error("fail to register account", "id", id, "error", err)
									}
								}


								// Now I will update the groups user list...
								if err == nil {
									if len(accountsInfo[i][3].([]string)) > 0 && a != nil {
										groups := accountsInfo[i][3].([]string)
										// Append not existing group...
										for j := 0; j < len(groups); j++ {
											groupId := Utility.GenerateUUID(groups[j])

											if !Utility.Contains(a.Groups, groupId) {
												// Now I will remo
												err := srv.addGroupMemberAccount(token, groupId, a.Id)
												if err != nil {
													slog.Error("fail to add account", "id", a.Id, "group", groupId, "error", err)
												}
											}

										}

										// Remove group that no more part of the ldap group.
										for j := 0; j < len(a.Groups); j++ {
											groupId := a.Groups[j]
											if !Utility.Contains(groups, groupId) {
												// Now I will remo
												err := srv.removeGroupMemberAccount(token, groupId, a.Id)
												if err != nil {
													slog.Error("fail to remove account", "id", a.Id, "group", groupId, "error", err)
												}
											}

										}
									}
								}

							}
						} else {
							return errors.New("account " + strings.ToLower(accountsInfo[i][2].([]string)[0]) + " has no email configured! ")
						}
					}
				}
			}
		}
	}

	// No error...
	return nil
}

// The path of the configuration.
func (srv *server) GetConfigurationPath() string {
	return srv.ConfigPath
}

func (srv *server) SetConfigurationPath(path string) {
	srv.ConfigPath = path
}

/**
 * Search for a list of value over the ldap srv. if the base_dn is
 * not specify the default base is use. It return a list of values. This can
 * be interpret as a tow dimensional array.
 */
func (srv *server) search(id string, base_dn string, filter string, attributes []string) ([][]interface{}, error) {

	if _, ok := srv.Connections[id]; !ok {
		return nil, errors.New("Connection " + id + " dosent exist!")
	}

	// create the connection.
	c := srv.Connections[id]
	conn, err := srv.connect(id, srv.Connections[id].User, srv.Connections[id].Password)
	if err != nil {
		return nil, err
	}

	c.conn = conn
	srv.Connections[id] = c

	// close connection after search.
	defer c.conn.Close()

	//Now I will execute the query...
	search_request := ldap.NewSearchRequest(
		base_dn,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		attributes,
		nil)

	// Create simple search.
	sr, err := srv.Connections[id].conn.Search(search_request)

	if err != nil {
		return nil, err
	}

	// Store the founded values in results...
	var results [][]interface{}
	for i := 0; i < len(sr.Entries); i++ {
		entry := sr.Entries[i]
		var row []interface{}
		for j := 0; j < len(attributes); j++ {
			attributeName := attributes[j]
			attributeValues := entry.GetAttributeValues(attributeName)
			row = append(row, attributeValues)
		}
		results = append(results, row)
	}

	return results, nil
}

// Search over LDAP srv.
func (srv *server) Search(ctx context.Context, rqst *ldappb.SearchRqst) (*ldappb.SearchResp, error) {
	id := rqst.Search.GetId()
	if _, ok := srv.Connections[id]; !ok {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Connection "+id+" dosent exist!")))
	}

	results, err := srv.search(id, rqst.Search.BaseDN, rqst.Search.Filter, rqst.Search.Attributes)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Here I got the results.
	str, err := Utility.ToJson(results)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &ldappb.SearchResp{
		Result: string(str),
	}, nil
}
