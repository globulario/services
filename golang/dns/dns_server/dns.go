package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/globulario/services/golang/dns/dnspb"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"github.com/miekg/dns"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

/* =========================
   A (IPv4) record endpoints
   ========================= */

// Set multiple domain names.
func (srv *server) SetDomains(ctx context.Context, rqst *dnspb.SetDomainsRequest) (*dnspb.SetDomainsResponse, error) {
	if len(rqst.Domains) == 0 {
		err := errors.New("no domains provided")
		srv.Logger.Error("SetDomains no domains", "err", err)
		return nil, status.Errorf(codes.InvalidArgument, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.Domains = rqst.Domains

	srv.Logger.Info("Domains set", "domains", strings.Join(srv.Domains, ", "))

	// store domains persistently
	domainsData, err := json.Marshal(srv.Domains)
	if err != nil {
		srv.Logger.Error("SetDomains marshal", "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.store.SetItem("domains", domainsData)
	if err != nil {
		srv.Logger.Error("SetDomains setItem", "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &dnspb.SetDomainsResponse{Result: true}, nil
}

// Get multiple domain names.
func (srv *server) GetDomains(context.Context, *dnspb.GetDomainsRequest) (*dnspb.GetDomainsResponse, error) {
	domainsData, err := srv.store.GetItem("domains")
	if err != nil {
		srv.Logger.Error("GetDomains getItem", "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	var domains []string
	err = json.Unmarshal(domainsData, &domains)
	if err != nil {
		srv.Logger.Error("GetDomains unmarshal", "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &dnspb.GetDomainsResponse{Domains: domains}, nil
}

// SetA adds or updates an A (IPv4 address) DNS record for the specified domain.
// It first checks if the domain is managed by this DNS server. If not, it returns an error.
// The function normalizes the domain name, generates a unique identifier, and merges the new IPv4 address
// with any existing addresses for the domain, avoiding duplicates. The updated list is then marshaled to JSON
// and stored. The TTL (time-to-live) for the record is also set. All operations are logged appropriately.
//
// Parameters:
//   - ctx: The context for the request, used for cancellation and deadlines.
//   - rqst: The SetARequest containing the domain, IPv4 address, and TTL.
//
// Returns:
//   - *dnspb.SetAResponse: The response containing a message with the domain name.
//   - error: An error if the operation fails at any step.
func (srv *server) SetA(ctx context.Context, rqst *dnspb.SetARequest) (*dnspb.SetAResponse, error) {
	if !srv.isManaged(rqst.Domain) {
		err := fmt.Errorf("the domain %s is not managed by this DNS", rqst.Domain)
		srv.Logger.Error("SetA unmanaged domain", "domain", rqst.Domain, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	domain := strings.ToLower(rqst.Domain)
	if !strings.HasSuffix(domain, ".") {
		domain += "."
	}
	uuid := Utility.GenerateUUID("A:" + domain)

	values := make([]string, 0)

	// Merge new value with existing list (if any).
	if data, err := srv.store.GetItem(uuid); err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			srv.Logger.Error("SetA unmarshal", "domain", domain, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if !Utility.Contains(values, rqst.A) {
		values = append(values, rqst.A)
	}

	data, err := json.Marshal(values)
	if err != nil {
		srv.Logger.Error("SetA marshal", "domain", domain, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if err := srv.store.SetItem(uuid, data); err != nil {
		srv.Logger.Error("SetA setItem", "domain", domain, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Persist TTL and log success.
	_ = srv.setTtl(uuid, rqst.Ttl)
	srv.Logger.Info("A record set",
		"domain", domain, "uuid", uuid, "ipv4", rqst.A, "ttl", rqst.Ttl)

	return &dnspb.SetAResponse{Message: domain}, nil
}

// RemoveA removes an A (IPv4 address) record from the DNS server for the specified domain.
// It first checks if the domain is managed by this DNS server. If not, it returns an error.
// The function retrieves the current list of A records for the domain, removes the specified
// IPv4 address if it exists, and updates the store accordingly. If no A records remain after
// removal, it deletes the record from the store and removes associated RBAC permissions.
// Returns a RemoveAResponse with the result or an error if any operation fails.
//
// Parameters:
//   - ctx: The context for request-scoped values, deadlines, and cancellation signals.
//   - rqst: The RemoveARequest containing the domain and IPv4 address to remove.
//
// Returns:
//   - *dnspb.RemoveAResponse: The response indicating the result of the operation.
//   - error: An error if the operation fails.
func (srv *server) RemoveA(ctx context.Context, rqst *dnspb.RemoveARequest) (*dnspb.RemoveAResponse, error) {
	_, token, err := security.GetClientId(ctx)
	if !srv.isManaged(rqst.Domain) {
		err := fmt.Errorf("the domain %s is not managed by this DNS", rqst.Domain)
		srv.Logger.Error("RemoveA unmanaged domain", "domain", rqst.Domain, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	domain := strings.ToLower(rqst.Domain)
	if !strings.HasSuffix(domain, ".") {
		domain += "."
	}
	uuid := Utility.GenerateUUID("A:" + domain)

	data, err := srv.store.GetItem(uuid)
	if err != nil {
		srv.Logger.Error("RemoveA getItem", "domain", domain, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	values := make([]string, 0)
	if err := json.Unmarshal(data, &values); err != nil {
		srv.Logger.Error("RemoveA unmarshal", "domain", domain, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if Utility.Contains(values, rqst.A) {
		values = Utility.RemoveString(values, rqst.A)
	}

	if len(values) == 0 {
		if err := srv.store.RemoveItem(uuid); err != nil {
			srv.Logger.Error("RemoveA removeItem", "domain", domain, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if rbac_client_, err := srv.getRbacClient(); err == nil {
			_ = rbac_client_.DeleteResourcePermissions(token, domain)
		}
		srv.Logger.Info("A record deleted", "domain", domain, "uuid", uuid, "ipv4", rqst.A)
	} else {
		data, err = json.Marshal(values)
		if err != nil {
			srv.Logger.Error("RemoveA marshal", "domain", domain, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if err := srv.store.SetItem(uuid, data); err != nil {
			srv.Logger.Error("RemoveA setItem", "domain", domain, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		srv.Logger.Info("A value removed", "domain", domain, "uuid", uuid, "ipv4", rqst.A, "remaining", len(values))
	}

	return &dnspb.RemoveAResponse{Result: true}, nil
}

// get_ipv4 returns all IPv4 addresses and TTL for a given domain.
func (srv *server) get_ipv4(domain string) ([]string, uint32, error) {
	domain = strings.ToLower(domain)
	if !strings.HasSuffix(domain, ".") {
		domain += "."
	}
	uuid := Utility.GenerateUUID("A:" + domain)
	data, err := srv.store.GetItem(uuid)
	if err != nil {
		return nil, 0, err
	}
	values := make([]string, 0)
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, 0, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return orderIPsByPrivacy(values), srv.getTtl(uuid), nil
}

// GetA retrieves the A (IPv4 address) records for the specified domain from the DNS store.
// It normalizes the domain name to lowercase and ensures it ends with a dot.
// The function generates a UUID key for the domain, fetches the associated data from the store,
// and unmarshals the JSON-encoded list of IP addresses. The IP addresses are then ordered
// according to privacy preferences before being returned in the response.
//
// Parameters:
//
//	ctx - The context for the request, used for cancellation and deadlines.
//	rqst - The GetARequest containing the domain name to query.
//
// Returns:
//
//	*dnspb.GetAResponse - The response containing the list of A records (IPv4 addresses).
//	error - An error if the retrieval or unmarshalling fails.
func (srv *server) GetA(ctx context.Context, rqst *dnspb.GetARequest) (*dnspb.GetAResponse, error) {
	domain := strings.ToLower(rqst.Domain)
	if !strings.HasSuffix(domain, ".") {
		domain += "."
	}
	uuid := Utility.GenerateUUID("A:" + domain)
	data, err := srv.store.GetItem(uuid)
	if err != nil {
		return nil, err
	}
	values := make([]string, 0)
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	values = orderIPsByPrivacy(values)
	return &dnspb.GetAResponse{A: values}, nil
}

/* ===========================
   AAAA (IPv6) record endpoints
   =========================== */

// SetAAAA adds or updates an AAAA (IPv6 address) DNS record for the specified domain.
// It first checks if the domain is managed by this DNS server. If not, it returns an error.
// The domain name is normalized to lowercase and ensured to have a trailing dot.
// The function generates a unique identifier for the AAAA record, retrieves any existing
// IPv6 addresses for the domain, and appends the new address if it does not already exist.
// The updated list of IPv6 addresses is then marshaled to JSON and stored.
// The TTL (time-to-live) for the record is set if provided.
// Logs are generated for errors and successful operations.
//
// Parameters:
//   - ctx: The context for the request.
//   - rqst: The SetAAAARequest containing the domain, IPv6 address, and TTL.
//
// Returns:
//   - *dnspb.SetAAAAResponse: The response containing a message with the domain.
//   - error: An error if the operation fails.
func (srv *server) SetAAAA(ctx context.Context, rqst *dnspb.SetAAAARequest) (*dnspb.SetAAAAResponse, error) {
	if !srv.isManaged(rqst.Domain) {
		err := fmt.Errorf("the domain %s is not managed by this DNS", rqst.Domain)
		srv.Logger.Error("SetAAAA unmanaged domain", "domain", rqst.Domain, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	domain := strings.ToLower(rqst.Domain)
	if !strings.HasSuffix(domain, ".") {
		domain += "."
	}
	uuid := Utility.GenerateUUID("AAAA:" + domain)

	values := make([]string, 0)
	if data, err := srv.store.GetItem(uuid); err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			srv.Logger.Error("SetAAAA unmarshal", "domain", domain, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	if !Utility.Contains(values, rqst.Aaaa) {
		values = append(values, rqst.Aaaa)
	}
	data, err := json.Marshal(values)
	if err != nil {
		srv.Logger.Error("SetAAAA marshal", "domain", domain, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := srv.store.SetItem(uuid, data); err != nil {
		srv.Logger.Error("SetAAAA setItem", "domain", domain, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	_ = srv.setTtl(uuid, rqst.Ttl)
	srv.Logger.Info("AAAA record set",
		"domain", domain, "uuid", uuid, "ipv6", rqst.Aaaa, "ttl", rqst.Ttl)

	return &dnspb.SetAAAAResponse{Message: domain}, nil
}

// RemoveAAAA removes an AAAA (IPv6) DNS record for the specified domain.
// It first normalizes the domain name, checks if the domain is managed by this DNS server,
// and then attempts to remove the specified AAAA record from the store. If the record is the
// last one for the domain, it also removes the associated permissions. Logs actions and errors
// throughout the process.
//
// Parameters:
//   - ctx: The context for the request, used for cancellation and deadlines.
//   - rqst: The request containing the domain and the AAAA record to remove.
//
// Returns:
//   - *dnspb.RemoveAAAAResponse: The response indicating the result of the operation.
//   - error: An error if the operation failed.
func (srv *server) RemoveAAAA(ctx context.Context, rqst *dnspb.RemoveAAAARequest) (*dnspb.RemoveAAAAResponse, error) {
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		srv.Logger.Error("RemoveAAAA getClientId", "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	
	domain := strings.ToLower(rqst.Domain)
	if !strings.HasSuffix(domain, ".") {
		domain += "."
	}
	if !srv.isManaged(rqst.Domain) {
		err := fmt.Errorf("the domain %s is not managed by this DNS", rqst.Domain)
		srv.Logger.Error("RemoveAAAA unmanaged domain", "domain", rqst.Domain, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	uuid := Utility.GenerateUUID("AAAA:" + domain)
	data, err := srv.store.GetItem(uuid)
	values := make([]string, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			srv.Logger.Error("RemoveAAAA unmarshal", "domain", domain, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if Utility.Contains(values, rqst.Aaaa) {
		values = Utility.RemoveString(values, rqst.Aaaa)
	}

	if len(values) == 0 {
		if err := srv.store.RemoveItem(uuid); err != nil {
			srv.Logger.Error("RemoveAAAA removeItem", "domain", domain, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if rbac_client_, err := srv.getRbacClient(); err == nil {
			_ = rbac_client_.DeleteResourcePermissions(token, domain)
		}
		srv.Logger.Info("AAAA record deleted", "domain", domain, "uuid", uuid, "ipv6", rqst.Aaaa)
	} else {
		data, err = json.Marshal(values)
		if err != nil {
			srv.Logger.Error("RemoveAAAA marshal", "domain", domain, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if err := srv.store.SetItem(uuid, data); err != nil {
			srv.Logger.Error("RemoveAAAA setItem", "domain", domain, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		srv.Logger.Info("AAAA value removed", "domain", domain, "uuid", uuid, "ipv6", rqst.Aaaa, "remaining", len(values))
	}

	return &dnspb.RemoveAAAAResponse{Result: true}, nil
}

// get_ipv6 returns all IPv6 addresses and TTL for a given domain.
func (srv *server) get_ipv6(domain string) ([]string, uint32, error) {
	domain = strings.ToLower(domain)
	if !strings.HasSuffix(domain, ".") {
		domain += "."
	}
	uuid := Utility.GenerateUUID("AAAA:" + domain)
	data, err := srv.store.GetItem(uuid)
	values := make([]string, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, 0, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {
		return nil, 0, err
	}
	if len(values) == 0 {
		return nil, 0, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no value found for domain "+domain)))
	}
	return values, srv.getTtl(uuid), nil
}

// GetAAAA retrieves the list of IPv6 (AAAA) records associated with the specified domain.
// The domain name is normalized to lowercase and ensured to have a trailing dot.
// It generates a UUID key for the domain, fetches the associated data from the store,
// and unmarshals the JSON-encoded list of IPv6 addresses. If no records are found or
// an error occurs during unmarshalling, an appropriate gRPC error is returned.
//
// Parameters:
//
//	ctx  - The context for the request, used for cancellation and deadlines.
//	rqst - The request containing the domain name to query.
//
// Returns:
//
//	*dnspb.GetAAAAResponse - The response containing the list of IPv6 addresses.
//	error                  - An error if the domain is not found or unmarshalling fails.
func (srv *server) GetAAAA(ctx context.Context, rqst *dnspb.GetAAAARequest) (*dnspb.GetAAAAResponse, error) {
	domain := strings.ToLower(rqst.Domain)
	if !strings.HasSuffix(domain, ".") {
		domain += "."
	}
	uuid := Utility.GenerateUUID("AAAA:" + domain)
	data, err := srv.store.GetItem(uuid)
	values := make([]string, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	if len(values) == 0 {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no value found for domain "+domain)))
	}
	return &dnspb.GetAAAAResponse{Aaaa: values}, nil
}

/* ===============
   TXT record API
   =============== */

// SetText sets or updates the TXT record values for a given identifier.
// It marshals the provided values to JSON and stores them in the underlying store.
// If a TXT record already exists for the given ID, the new values are merged with the existing ones.
// The function also sets the TTL (time-to-live) for the record.
// Returns a SetTextResponse indicating success, or an error if any operation fails.
//
// Parameters:
//
//	ctx  - The context for the request.
//	rqst - The SetTextRequest containing the ID, values, and TTL.
//
// Returns:
//
//	*dnspb.SetTextResponse - The response indicating the result of the operation.
//	error                  - An error if the operation fails.
func (srv *server) SetText(ctx context.Context, rqst *dnspb.SetTextRequest) (*dnspb.SetTextResponse, error) {
	values, err := json.Marshal(rqst.Values)
	if err != nil {
		srv.Logger.Error("SetText marshal", "id", rqst.Id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("TXT:" + id)

	// Merge with existing values (if any).
	if data, err := srv.store.GetItem(uuid); err == nil {
		values_ := make([]string, 0)
		if err := json.Unmarshal(data, &values_); err != nil {
			srv.Logger.Error("SetText unmarshal-existing", "id", id, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		values_ = append(values_, rqst.Values...)
		values, err = json.Marshal(values_)
		if err != nil {
			srv.Logger.Error("SetText marshal-merged", "id", id, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if err := srv.store.SetItem(uuid, values); err != nil {
		srv.Logger.Error("SetText setItem", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	_ = srv.setTtl(uuid, rqst.Ttl)
	srv.Logger.Info("TXT set", "id", id, "uuid", uuid, "values", len(rqst.Values), "ttl", rqst.Ttl)

	return &dnspb.SetTextResponse{Result: true}, nil
}

// getText returns TXT values and TTL for an identifier.
func (srv *server) getText(id string) ([]string, uint32, error) {
	id = strings.ToLower(id)
	uuid := Utility.GenerateUUID("TXT:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]string, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, 0, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {
		return nil, 0, err
	}

	srv.Logger.Debug("TXT fetched", "id", id, "uuid", uuid, "values", len(values))
	return values, srv.getTtl(uuid), nil
}

// GetText retrieves the TXT record values associated with the given ID from the DNS server's store.
// It generates a UUID based on the provided ID, fetches the corresponding item from the store,
// and attempts to unmarshal the data into a slice of strings. If successful, it returns the values
// in a GetTextResponse. If an error occurs during unmarshalling, it returns an internal error status.
//
// Parameters:
//
//	ctx - The context for the request, used for cancellation and deadlines.
//	rqst - The GetTextRequest containing the ID of the TXT record to retrieve.
//
// Returns:
//
//	*dnspb.GetTextResponse - The response containing the TXT record values.
//	error - An error if the operation fails, otherwise nil.
func (srv *server) GetText(ctx context.Context, rqst *dnspb.GetTextRequest) (*dnspb.GetTextResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("TXT:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]string, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	return &dnspb.GetTextResponse{Values: values}, nil
}

// RemoveText removes a TXT DNS record identified by the given request ID.
// It deletes the corresponding item from the store and attempts to remove any associated
// resource permissions using the RBAC client. Logs are generated for both errors and successful removals.
//
// Parameters:
//
//	ctx  - The context for controlling cancellation and deadlines.
//	rqst - The request containing the ID of the TXT record to remove.
//
// Returns:
//
//	*dnspb.RemoveTextResponse - The response indicating the result of the removal operation.
//	error                     - An error if the removal fails, otherwise nil.
func (srv *server) RemoveText(ctx context.Context, rqst *dnspb.RemoveTextRequest) (*dnspb.RemoveTextResponse, error) {
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		srv.Logger.Error("RemoveText getClientId", "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("TXT:" + id)
	if err := srv.store.RemoveItem(uuid); err != nil {
		srv.Logger.Error("RemoveText removeItem", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if rbac_client_, err := srv.getRbacClient(); err == nil {
		_ = rbac_client_.DeleteResourcePermissions(token, rqst.Id)
	}
	srv.Logger.Info("TXT removed", "id", id, "uuid", uuid)
	return &dnspb.RemoveTextResponse{Result: true}, nil
}

/* ==============
   NS record API
   ============== */

// SetNs sets a nameserver (NS) record for a given domain identifier.
// It normalizes the domain and nameserver to lowercase and ensures they end with a dot.
// The function retrieves any existing NS records for the domain, adds the new NS if not present,
// and stores the updated list. It also sets the TTL for the record.
// Returns a SetNsResponse with the result or an error if the operation fails.
//
// Parameters:
//
//	ctx  - context for request-scoped values, cancellation, and deadlines
//	rqst - SetNsRequest containing the domain identifier, nameserver, and TTL
//
// Returns:
//
//	*dnspb.SetNsResponse - response indicating success
//	error                - error if the operation fails
func (srv *server) SetNs(ctx context.Context, rqst *dnspb.SetNsRequest) (*dnspb.SetNsResponse, error) {
	id := strings.ToLower(rqst.Id)
	if !strings.HasSuffix(id, ".") {
		id += "."
	}
	uuid := Utility.GenerateUUID("NS:" + id)

	values := make([]string, 0)
	if data, err := srv.store.GetItem(uuid); err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			srv.Logger.Error("SetNs unmarshal", "id", id, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	ns := strings.ToLower(rqst.Ns)
	if !strings.HasSuffix(ns, ".") {
		ns += "."
	}
	if !Utility.Contains(values, ns) {
		values = append(values, ns)
	}

	data, err := json.Marshal(values)
	if err != nil {
		srv.Logger.Error("SetNs marshal", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := srv.store.SetItem(uuid, data); err != nil {
		srv.Logger.Error("SetNs setItem", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	_ = srv.setTtl(uuid, rqst.Ttl)
	srv.Logger.Info("NS set", "id", id, "uuid", uuid, "ns", ns, "ttl", rqst.Ttl)

	return &dnspb.SetNsResponse{Result: true}, nil
}

func (srv *server) getNs(id string) ([]string, uint32, error) {
	id = strings.ToLower(id)
	if !strings.HasSuffix(id, ".") {
		id += "."
	}
	uuid := Utility.GenerateUUID("NS:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]string, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, 0, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {
		parts := strings.Split(id, ".")
		if len(parts) > 2 {
			id = strings.Join(parts[1:], ".")
			return srv.getNs(id)
		}
		return nil, 0, err
	}
	return values, srv.getTtl(uuid), nil
}

// GetNs retrieves the list of name servers (NS) associated with the given identifier.
// It generates a UUID based on the provided ID, fetches the corresponding item from the store,
// and unmarshals the stored data into a slice of strings representing the NS records.
// Returns a GetNsResponse containing the NS records, or an error if unmarshalling fails.
func (srv *server) GetNs(ctx context.Context, rqst *dnspb.GetNsRequest) (*dnspb.GetNsResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("NS:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]string, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	return &dnspb.GetNsResponse{Ns: values}, nil
}

// RemoveNs removes a nameserver (NS) record for a given domain from the DNS server's storage.
// It takes a context and a RemoveNsRequest containing the domain ID and the NS to remove.
// The function normalizes the domain and NS names, retrieves the current NS values,
// and removes the specified NS if present. If no NS records remain for the domain,
// it deletes the domain entry and its associated permissions. Returns a RemoveNsResponse
// indicating success, or an error if the operation fails at any step.
func (srv *server) RemoveNs(ctx context.Context, rqst *dnspb.RemoveNsRequest) (*dnspb.RemoveNsResponse, error) {
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		srv.Logger.Error("RemoveNs getClientId", "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id := strings.ToLower(rqst.Id)
	if !strings.HasSuffix(id, ".") {
		id += "."
	}
	uuid := Utility.GenerateUUID("NS:" + id)

	data, err := srv.store.GetItem(uuid)
	values := make([]string, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			srv.Logger.Error("RemoveNs unmarshal", "id", id, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if len(values) == 0 {
		err := errors.New("no value found for domain " + id)
		srv.Logger.Error("RemoveNs empty", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	ns := strings.ToLower(rqst.Ns)
	if !strings.HasSuffix(ns, ".") {
		ns += "."
	}
	if Utility.Contains(values, ns) {
		values = Utility.RemoveString(values, ns)
	}

	if len(values) == 0 {
		if err := srv.store.RemoveItem(uuid); err != nil {
			srv.Logger.Error("RemoveNs removeItem", "id", id, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if rbac_client_, err := srv.getRbacClient(); err == nil {
			_ = rbac_client_.DeleteResourcePermissions(token, rqst.Id)
		}
		srv.Logger.Info("NS record deleted", "id", id, "uuid", uuid, "ns", ns)
	} else {
		data, err = json.Marshal(values)
		if err != nil {
			srv.Logger.Error("RemoveNs marshal", "id", id, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if err := srv.store.SetItem(uuid, data); err != nil {
			srv.Logger.Error("RemoveNs setItem", "id", id, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		srv.Logger.Info("NS value removed", "id", id, "uuid", uuid, "ns", ns, "remaining", len(values))
	}

	return &dnspb.RemoveNsResponse{Result: true}, nil
}

/* =================
   CNAME record API
   ================= */

// SetCName sets a CNAME (Canonical Name) record in the DNS server's storage.
// It generates a unique identifier for the CNAME record based on the provided ID,
// stores the CNAME value, and sets its TTL (time-to-live) if specified.
// Logs the operation and returns a response indicating success or an error if the operation fails.
//
// Parameters:
//
//	ctx  - The context for the request, used for cancellation and deadlines.
//	rqst - The SetCNameRequest containing the ID, CNAME target, and TTL.
//
// Returns:
//
//	*dnspb.SetCNameResponse - The response indicating the result of the operation.
//	error                   - An error if the operation fails.
func (srv *server) SetCName(ctx context.Context, rqst *dnspb.SetCNameRequest) (*dnspb.SetCNameResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("CName:" + id)
	if err := srv.store.SetItem(uuid, []byte(rqst.Cname)); err != nil {
		srv.Logger.Error("SetCName setItem", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	_ = srv.setTtl(uuid, rqst.Ttl)
	srv.Logger.Info("CNAME set", "id", id, "uuid", uuid, "target", rqst.Cname, "ttl", rqst.Ttl)
	return &dnspb.SetCNameResponse{Result: true}, nil
}

func (srv *server) getCName(id string) (string, uint32, error) {
	id = strings.ToLower(id)
	uuid := Utility.GenerateUUID("CName:" + id)
	data, err := srv.store.GetItem(uuid)
	if err != nil {
		return "", 0, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return string(data), srv.getTtl(uuid), nil
}

// GetCName retrieves the canonical name (CNAME) record associated with the given request ID.
// It generates a UUID based on the provided ID, fetches the corresponding item from the store,
// and returns the CNAME as a response. If an error occurs during retrieval, it returns an
// appropriate gRPC status error.
//
// Parameters:
//
//	ctx  - The context for controlling cancellation and deadlines.
//	rqst - The request containing the ID for which to retrieve the CNAME.
//
// Returns:
//
//	*dnspb.GetCNameResponse - The response containing the CNAME string.
//	error                   - An error if the retrieval fails.
func (srv *server) GetCName(ctx context.Context, rqst *dnspb.GetCNameRequest) (*dnspb.GetCNameResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("CName:" + id)
	data, err := srv.store.GetItem(uuid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &dnspb.GetCNameResponse{Cname: string(data)}, nil
}

// RemoveCName removes a CNAME record from the DNS server's store based on the provided request.
// It generates a UUID for the CNAME using the request ID, attempts to remove the item from the store,
// and deletes any associated resource permissions using the RBAC client if available.
// Logs both errors and successful removals. Returns a RemoveCNameResponse indicating the result.
//
// Parameters:
//
//	ctx  - The context for the request, used for cancellation and deadlines.
//	rqst - The RemoveCNameRequest containing the ID of the CNAME to remove.
//
// Returns:
//
//	*dnspb.RemoveCNameResponse - The response indicating whether the removal was successful.
//	error                      - An error if the removal failed, otherwise nil.
func (srv *server) RemoveCName(ctx context.Context, rqst *dnspb.RemoveCNameRequest) (*dnspb.RemoveCNameResponse, error) {
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		srv.Logger.Error("RemoveCName getClientId", "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("CName:" + id)
	if err := srv.store.RemoveItem(uuid); err != nil {
		srv.Logger.Error("RemoveCName removeItem", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if rbac_client_, err := srv.getRbacClient(); err == nil {
		_ = rbac_client_.DeleteResourcePermissions(token, rqst.Id)
	}
	srv.Logger.Info("CNAME removed", "id", id, "uuid", uuid)
	return &dnspb.RemoveCNameResponse{Result: true}, nil
}

/* =============
   MX record API
   ============= */

// SetMx sets or updates an MX (Mail Exchange) record for a given domain identifier.
// It ensures the domain and MX host end with a dot, generates a unique UUID for the record,
// retrieves any existing MX records, and updates or appends the new MX entry as needed.
// The updated list of MX records is then marshaled to JSON and stored.
// If a TTL (Time To Live) is provided, it is set for the record.
// Returns a SetMxResponse indicating success or an error if any operation fails.
//
// Parameters:
//
//	ctx  - The context for request-scoped values, cancellation, and deadlines.
//	rqst - The SetMxRequest containing the domain identifier, MX record, and TTL.
//
// Returns:
//
//	*dnspb.SetMxResponse - The response indicating the result of the operation.
//	error                - An error if the operation fails.
func (srv *server) SetMx(ctx context.Context, rqst *dnspb.SetMxRequest) (*dnspb.SetMxResponse, error) {
	id := strings.ToLower(rqst.Id)
	if !strings.HasSuffix(id, ".") {
		id += "."
	}
	if !strings.HasSuffix(rqst.Mx.Mx, ".") {
		rqst.Mx.Mx += "."
	}

	uuid := Utility.GenerateUUID("MX:" + id)
	values := make([]*dnspb.MX, 0)

	if data, err := srv.store.GetItem(uuid); err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			srv.Logger.Error("SetMx unmarshal", "id", id, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	found := false
	for i := range values {
		if values[i].Mx == rqst.Mx.Mx {
			values[i] = rqst.Mx
			found = true
			break
		}
	}
	if !found && rqst.Mx != nil {
		values = append(values, rqst.Mx)
	}

	data, err := json.Marshal(values)
	if err != nil {
		srv.Logger.Error("SetMx marshal", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := srv.store.SetItem(uuid, data); err != nil {
		srv.Logger.Error("SetMx setItem", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	_ = srv.setTtl(uuid, rqst.Ttl)
	srv.Logger.Info("MX set", "id", id, "uuid", uuid, "host", rqst.Mx.Mx, "pref", rqst.Mx.Preference, "ttl", rqst.Ttl)

	return &dnspb.SetMxResponse{Result: true}, nil
}

func (srv *server) getMx(id, mx string) ([]*dnspb.MX, uint32, error) {
	id = strings.ToLower(id)
	if !strings.HasSuffix(id, ".") {
		id += "."
	}
	mx = strings.ToLower(mx)
	if len(mx) > 0 && !strings.HasSuffix(mx, ".") {
		mx += "."
	}

	uuid := Utility.GenerateUUID("MX:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]*dnspb.MX, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, 0, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	if len(values) > 0 && len(mx) > 0 {
		for i := range values {
			if values[i].Mx == mx {
				return []*dnspb.MX{values[i]}, srv.getTtl(uuid), nil
			}
		}
	}
	return values, srv.getTtl(uuid), nil
}

// GetMx retrieves MX (Mail Exchange) records for a given domain ID from the server's store.
// The domain ID is normalized to lowercase and ensured to have a trailing dot.
// It generates a UUID key based on the domain ID and attempts to fetch the corresponding MX records.
// If a specific MX value is provided in the request, it filters and returns only the matching record.
// Returns a GetMxResponse containing the list of MX records or an error if unmarshalling fails.
func (srv *server) GetMx(ctx context.Context, rqst *dnspb.GetMxRequest) (*dnspb.GetMxResponse, error) {
	id := strings.ToLower(rqst.Id)
	if !strings.HasSuffix(id, ".") {
		id += "."
	}
	uuid := Utility.GenerateUUID("MX:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]*dnspb.MX, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	if len(rqst.Mx) > 0 {
		for i := range values {
			if values[i].Mx == rqst.Mx {
				return &dnspb.GetMxResponse{Result: []*dnspb.MX{values[i]}}, nil
			}
		}
	}
	return &dnspb.GetMxResponse{Result: values}, nil
}

// RemoveMx removes an MX (Mail Exchange) record for a given domain from the DNS server's storage.
// It takes a context and a RemoveMxRequest containing the domain ID and MX value to remove.
// The function retrieves the current list of MX records for the domain, removes the specified MX value,
// and updates the storage. If no MX records remain after removal, it deletes the domain's MX entry
// and associated RBAC permissions. Returns a RemoveMxResponse indicating success or an error if the operation fails.
func (srv *server) RemoveMx(ctx context.Context, rqst *dnspb.RemoveMxRequest) (*dnspb.RemoveMxResponse, error) {

	_, token, err := security.GetClientId(ctx)
	if err != nil {
		srv.Logger.Error("RemoveMx getClientId", "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("MX:" + id)

	data, err := srv.store.GetItem(uuid)
	values := make([]*dnspb.MX, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			srv.Logger.Error("RemoveMx unmarshal", "id", id, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if len(values) == 0 {
		err := errors.New("no value found for domain " + id)
		srv.Logger.Error("RemoveMx empty", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	for i := range values {
		if values[i].Mx == rqst.Mx {
			values = append(values[:i], values[i+1:]...)
			break
		}
	}

	data, err = json.Marshal(values)
	if err != nil {
		srv.Logger.Error("RemoveMx marshal", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(values) > 0 {
		if err := srv.store.SetItem(uuid, data); err != nil {
			srv.Logger.Error("RemoveMx setItem", "id", id, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		srv.Logger.Info("MX value removed", "id", id, "uuid", uuid, "host", rqst.Mx, "remaining", len(values))
	} else {
		if err := srv.store.RemoveItem(uuid); err != nil {
			srv.Logger.Error("RemoveMx removeItem", "id", id, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if rbac_client_, err := srv.getRbacClient(); err == nil {
			_ = rbac_client_.DeleteResourcePermissions(token, rqst.Id)
		}
		srv.Logger.Info("MX record deleted", "id", id, "uuid", uuid, "host", rqst.Mx)
	}

	return &dnspb.RemoveMxResponse{Result: true}, nil
}

/* =============
   SOA record API
   ============= */

// SetSoa sets or updates a Start of Authority (SOA) record for a given DNS zone.
//
// It normalizes the zone ID and the SOA nameserver (NS) to ensure they end with a dot.
// The function generates a unique identifier for the SOA record, retrieves any existing
// SOA records for the zone, and updates the record if the NS matches. If no matching
// NS is found, it appends the new SOA record. The updated list is then marshaled to JSON
// and stored. The TTL for the record is also set.
//
// Parameters:
//
//	ctx  - The context for the request.
//	rqst - The SetSoaRequest containing the zone ID, SOA record, and TTL.
//
// Returns:
//
//	*dnspb.SetSoaResponse - The response indicating success.
//	error                 - An error if the operation fails.
func (srv *server) SetSoa(ctx context.Context, rqst *dnspb.SetSoaRequest) (*dnspb.SetSoaResponse, error) {
	id := strings.ToLower(rqst.Id)
	if !strings.HasSuffix(id, ".") {
		id += "."
	}
	if !strings.HasSuffix(rqst.Soa.Ns, ".") {
		rqst.Soa.Ns += "."
	}
	uuid := Utility.GenerateUUID("SOA:" + id)

	values := make([]*dnspb.SOA, 0)
	if data, err := srv.store.GetItem(uuid); err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			srv.Logger.Error("SetSoa unmarshal", "id", id, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	for i := range values {
		ns := strings.ToLower(values[i].Ns)
		if !strings.HasSuffix(ns, ".") {
			ns += "."
		}
		if ns == rqst.Soa.Ns {
			values[i] = rqst.Soa
			rqst.Soa = nil
			break
		}
	}
	if rqst.Soa != nil {
		values = append(values, rqst.Soa)
	}

	data, err := json.Marshal(values)
	if err != nil {
		srv.Logger.Error("SetSoa marshal", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := srv.store.SetItem(uuid, data); err != nil {
		srv.Logger.Error("SetSoa setItem", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	_ = srv.setTtl(uuid, rqst.Ttl)
	srv.Logger.Info("SOA set", "id", id, "uuid", uuid, "ns", rqst.Soa.GetNs(), "ttl", rqst.Ttl)

	return &dnspb.SetSoaResponse{Result: true}, nil
}

func (srv *server) getSoa(id, ns string) ([]*dnspb.SOA, uint32, error) {
	id = strings.ToLower(id)
	if !strings.HasSuffix(id, ".") {
		id += "."
	}
	uuid := Utility.GenerateUUID("SOA:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]*dnspb.SOA, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, 0, err
		}
	} else {
		parts := strings.Split(id, ".")
		if len(parts) > 2 {
			id = strings.Join(parts[1:], ".")
			return srv.getSoa(id, ns)
		}
		return nil, 0, err
	}

	if len(ns) > 0 {
		for i := range values {
			if values[i].Ns == ns {
				return []*dnspb.SOA{values[i]}, srv.getTtl(uuid), nil
			}
		}
	}
	return values, srv.getTtl(uuid), nil
}

// GetSoa retrieves the SOA (Start of Authority) records associated with the given request ID.
// It generates a UUID based on the request ID, fetches the corresponding item from the store,
// and unmarshals the data into a slice of SOA records. If a specific nameserver (Ns) is provided
// in the request, it returns only the matching SOA record. Otherwise, it returns all SOA records
// found for the given ID. Returns an error if data unmarshalling fails or if there are issues
// accessing the store.
func (srv *server) GetSoa(ctx context.Context, rqst *dnspb.GetSoaRequest) (*dnspb.GetSoaResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("SOA:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]*dnspb.SOA, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	if len(rqst.Ns) > 0 {
		for i := range values {
			if values[i].Ns == rqst.Ns {
				return &dnspb.GetSoaResponse{Result: []*dnspb.SOA{values[i]}}, nil
			}
		}
	}
	return &dnspb.GetSoaResponse{Result: values}, nil
}

// RemoveSoa removes a Start of Authority (SOA) record for a given domain from the DNS server's storage.
// It takes a context and a RemoveSoaRequest containing the domain ID and nameserver (NS) to remove.
// The function normalizes the domain and NS, retrieves the current SOA records, and removes the matching NS entry.
// If no SOA records remain after removal, it deletes the SOA entry and associated RBAC permissions.
// Returns a RemoveSoaResponse indicating success, or an error if the operation fails.
func (srv *server) RemoveSoa(ctx context.Context, rqst *dnspb.RemoveSoaRequest) (*dnspb.RemoveSoaResponse, error) {

	_, token, err := security.GetClientId(ctx)
	if err != nil {
		srv.Logger.Error("RemoveSoa getClientId", "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id := strings.ToLower(rqst.Id)
	if !strings.HasSuffix(id, ".") {
		id += "."
	}
	uuid := Utility.GenerateUUID("SOA:" + id)

	data, err := srv.store.GetItem(uuid)
	values := make([]*dnspb.SOA, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			srv.Logger.Error("RemoveSoa unmarshal", "id", id, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	if len(values) == 0 {
		err := errors.New("no value found for domain " + id)
		srv.Logger.Error("RemoveSoa empty", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if !strings.HasSuffix(rqst.Ns, ".") {
		rqst.Ns += "."
	}
	for i := range values {
		ns := strings.ToLower(values[i].Ns)
		if !strings.HasSuffix(ns, ".") {
			ns += "."
		}
		if ns == rqst.Ns {
			values = append(values[:i], values[i+1:]...)
			break
		}
	}

	data, err = json.Marshal(values)
	if err != nil {
		srv.Logger.Error("RemoveSoa marshal", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(values) > 0 {
		if err := srv.store.SetItem(uuid, data); err != nil {
			srv.Logger.Error("RemoveSoa setItem", "id", id, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		srv.Logger.Info("SOA value removed", "id", id, "uuid", uuid, "ns", rqst.Ns, "remaining", len(values))
	} else {
		if err := srv.store.RemoveItem(uuid); err != nil {
			srv.Logger.Error("RemoveSoa removeItem", "id", id, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if rbac_client_, err := srv.getRbacClient(); err == nil {
			_ = rbac_client_.DeleteResourcePermissions(token, rqst.Id)
		}
		srv.Logger.Info("SOA record deleted", "id", id, "uuid", uuid, "ns", rqst.Ns)
	}

	return &dnspb.RemoveSoaResponse{Result: true}, nil
}

/* =============
   URI record API
   ============= */

// SetUri sets or updates a URI record for a given ID in the server's store.
// If a URI with the same target already exists, it is replaced; otherwise, the new URI is appended.
// The method serializes the updated list of URIs and stores it, setting the TTL as specified.
// Returns a SetUriResponse indicating success, or an error if any operation fails.
//
// Parameters:
//   - ctx: The context for the request.
//   - rqst: The SetUriRequest containing the ID, URI, and TTL.
//
// Returns:
//   - *dnspb.SetUriResponse: The response indicating the result of the operation.
//   - error: An error if the operation fails.
func (srv *server) SetUri(ctx context.Context, rqst *dnspb.SetUriRequest) (*dnspb.SetUriResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("URI:" + id)

	values := make([]*dnspb.URI, 0)
	if data, err := srv.store.GetItem(uuid); err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			srv.Logger.Error("SetUri unmarshal", "id", id, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	for i := range values {
		if values[i].Target == rqst.Uri.Target {
			values[i] = rqst.Uri
			rqst.Uri = nil
			break
		}
	}
	if rqst.Uri != nil {
		values = append(values, rqst.Uri)
	}

	data, err := json.Marshal(values)
	if err != nil {
		srv.Logger.Error("SetUri marshal", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := srv.store.SetItem(uuid, data); err != nil {
		srv.Logger.Error("SetUri setItem", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	_ = srv.setTtl(uuid, rqst.Ttl)
	srv.Logger.Info("URI set",
		"id", id, "uuid", uuid, "target", rqst.Uri.GetTarget(),
		"priority", rqst.Uri.GetPriority(), "weight", rqst.Uri.GetWeight(), "ttl", rqst.Ttl)
	return &dnspb.SetUriResponse{Result: true}, nil
}

func (srv *server) getUri(id, target string) ([]*dnspb.URI, uint32, error) {
	id = strings.ToLower(id)
	uuid := Utility.GenerateUUID("URI:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]*dnspb.URI, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, 0, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	if len(target) > 0 {
		for i := range values {
			if values[i].Target == target {
				return []*dnspb.URI{values[i]}, srv.getTtl(uuid), nil
			}
		}
	}
	return values, srv.getTtl(uuid), nil
}

// GetUri retrieves URI records associated with the given request ID from the server's store.
// It generates a UUID based on the request ID, fetches the corresponding data, and unmarshals it into a slice of URI objects.
// If a specific target is provided in the request, it filters and returns only the matching URI.
// Returns a GetUriResponse containing the matched URIs or an error if the operation fails.
func (srv *server) GetUri(ctx context.Context, rqst *dnspb.GetUriRequest) (*dnspb.GetUriResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("URI:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]*dnspb.URI, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	if len(rqst.Target) > 0 {
		for i := range values {
			if values[i].Target == rqst.Target {
				return &dnspb.GetUriResponse{Result: []*dnspb.URI{values[i]}}, nil
			}
		}
	}
	return &dnspb.GetUriResponse{Result: values}, nil
}

// RemoveUri removes a URI target from the list of URIs associated with the given ID.
// It retrieves the current list of URIs from the store, removes the specified target,
// and updates or deletes the record as appropriate. If the last URI is removed, the
// entire record is deleted and associated RBAC permissions are cleaned up.
// Returns a RemoveUriResponse with the result or an error if the operation fails.
//
// Parameters:
//   - ctx: The context for the request.
//   - rqst: The RemoveUriRequest containing the ID and target to remove.
//
// Returns:
//   - *dnspb.RemoveUriResponse: The response indicating the result of the operation.
//   - error: An error if the operation fails.
func (srv *server) RemoveUri(ctx context.Context, rqst *dnspb.RemoveUriRequest) (*dnspb.RemoveUriResponse, error) {

	_, token, err := security.GetClientId(ctx)
	if err != nil {
		srv.Logger.Error("RemoveUri getClientId", "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("URI:" + id)

	data, err := srv.store.GetItem(uuid)
	values := make([]*dnspb.URI, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			srv.Logger.Error("RemoveUri unmarshal", "id", id, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	if len(values) == 0 {
		err := errors.New("no value found for domain " + id)
		srv.Logger.Error("RemoveUri empty", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	for i := range values {
		if values[i].Target == rqst.Target {
			values = append(values[:i], values[i+1:]...)
			break
		}
	}

	data, err = json.Marshal(values)
	if err != nil {
		srv.Logger.Error("RemoveUri marshal", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(values) > 0 {
		if err := srv.store.SetItem(uuid, data); err != nil {
			srv.Logger.Error("RemoveUri setItem", "id", id, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		srv.Logger.Info("URI value removed", "id", id, "uuid", uuid, "target", rqst.Target, "remaining", len(values))
	} else {
		if err := srv.store.RemoveItem(uuid); err != nil {
			srv.Logger.Error("RemoveUri removeItem", "id", id, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if rbac_client_, err := srv.getRbacClient(); err == nil {
			_ = rbac_client_.DeleteResourcePermissions(token, rqst.Id)
		}
		srv.Logger.Info("URI record deleted", "id", id, "uuid", uuid, "target", rqst.Target)
	}

	return &dnspb.RemoveUriResponse{Result: true}, nil
}

/* ==============
   AFSDB record API
   ============== */

// SetAfsdb handles the request to set an AFSDB (Andrew File System Database) DNS record.
// It marshals the provided AFSDB data from the request, generates a unique identifier for the record,
// and stores it in the server's storage backend. The method also sets the TTL (time-to-live) for the record.
// Logging is performed for both successful and error cases. Returns a SetAfsdbResponse indicating success,
// or an error if the operation fails.
//
// Parameters:
//
//	ctx  - The context for the request, used for cancellation and deadlines.
//	rqst - The SetAfsdbRequest containing the AFSDB record data, record ID, and TTL.
//
// Returns:
//
//	*dnspb.SetAfsdbResponse - The response indicating the result of the operation.
//	error                   - An error if the operation fails.
func (srv *server) SetAfsdb(ctx context.Context, rqst *dnspb.SetAfsdbRequest) (*dnspb.SetAfsdbResponse, error) {
	values, err := json.Marshal(rqst.Afsdb)
	if err != nil {
		srv.Logger.Error("SetAfsdb marshal", "id", rqst.Id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("AFSDB:" + id)
	if err := srv.store.SetItem(uuid, values); err != nil {
		srv.Logger.Error("SetAfsdb setItem", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	_ = srv.setTtl(uuid, rqst.Ttl)
	srv.Logger.Info("AFSDB set", "id", id, "uuid", uuid, "subtype", rqst.Afsdb.GetSubtype(), "host", rqst.Afsdb.GetHostname(), "ttl", rqst.Ttl)
	return &dnspb.SetAfsdbResponse{Result: true}, nil
}

func (srv *server) getAfsdb(id string) (*dnspb.AFSDB, uint32, error) {
	id = strings.ToLower(id)
	uuid := Utility.GenerateUUID("AFSDB:" + id)
	data, err := srv.store.GetItem(uuid)
	if err != nil {
		return nil, 0, err
	}
	afsdb := new(dnspb.AFSDB)
	if err := json.Unmarshal(data, afsdb); err != nil {
		return nil, 0, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return afsdb, srv.getTtl(uuid), nil
}

// GetAfsdb retrieves an AFSDB (Andrew File System Database) record from the server's store
// based on the provided request ID. The function generates a UUID using the request ID,
// fetches the corresponding item from the store, and unmarshals the JSON data into an
// AFSDB protocol buffer message. If successful, it returns a GetAfsdbResponse containing
// the AFSDB record; otherwise, it returns an appropriate gRPC error.
//
// Parameters:
//
//	ctx  - The context for controlling cancellation and deadlines.
//	rqst - The GetAfsdbRequest containing the ID of the AFSDB record to retrieve.
//
// Returns:
//
//	*dnspb.GetAfsdbResponse - The response containing the AFSDB record if found.
//	error                   - An error if the record could not be retrieved or unmarshaled.
func (srv *server) GetAfsdb(ctx context.Context, rqst *dnspb.GetAfsdbRequest) (*dnspb.GetAfsdbResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("AFSDB:" + id)
	data, err := srv.store.GetItem(uuid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	afsdb := new(dnspb.AFSDB)
	if err := json.Unmarshal(data, afsdb); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &dnspb.GetAfsdbResponse{Result: afsdb}, nil
}

func (srv *server) RemoveAfsdb(ctx context.Context, rqst *dnspb.RemoveAfsdbRequest) (*dnspb.RemoveAfsdbResponse, error) {

	_, token, err := security.GetClientId(ctx)
	if err != nil {
		srv.Logger.Error("RemoveAfsdb getClientId", "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("AFSDB:" + id)
	if err := srv.store.RemoveItem(uuid); err != nil {
		srv.Logger.Error("RemoveAfsdb removeItem", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if rbac_client_, err := srv.getRbacClient(); err == nil {
		_ = rbac_client_.DeleteResourcePermissions(token, rqst.Id)
	}
	srv.Logger.Info("AFSDB removed", "id", id, "uuid", uuid)
	return &dnspb.RemoveAfsdbResponse{Result: true}, nil
}

/* =============
   CAA record API
   ============= */

// SetCaa sets or updates a CAA (Certification Authority Authorization) record for a given domain.
// It retrieves the existing CAA records for the specified ID, updates the record if the domain matches,
// or appends a new record if it does not exist. The updated list is then marshaled to JSON and stored.
// The function also sets the TTL (Time To Live) for the record and logs the operation.
//
// Parameters:
//
//	ctx  - The context for the request, used for cancellation and deadlines.
//	rqst - The SetCaaRequest containing the CAA record to set and the TTL.
//
// Returns:
//
//	*dnspb.SetCaaResponse - The response indicating the result of the operation.
//	error                 - An error if the operation fails at any step.
func (srv *server) SetCaa(ctx context.Context, rqst *dnspb.SetCaaRequest) (*dnspb.SetCaaResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("CAA:" + id)

	values := make([]*dnspb.CAA, 0)
	if data, err := srv.store.GetItem(uuid); err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			srv.Logger.Error("SetCaa unmarshal", "id", id, "err", err)
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	for i := range values {
		if values[i].Domain == rqst.Caa.Domain {
			values[i] = rqst.Caa
			rqst.Caa = nil
			break
		}
	}
	if rqst.Caa != nil {
		values = append(values, rqst.Caa)
	}

	data, err := json.Marshal(values)
	if err != nil {
		srv.Logger.Error("SetCaa marshal", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := srv.store.SetItem(uuid, data); err != nil {
		srv.Logger.Error("SetCaa setItem", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	_ = srv.setTtl(uuid, rqst.Ttl)
	srv.Logger.Info("CAA set", "id", id, "uuid", uuid, "tag", rqst.Caa.GetTag(), "domain", rqst.Caa.GetDomain(), "ttl", rqst.Ttl)
	return &dnspb.SetCaaResponse{Result: true}, nil
}

// -----------------------------
// CAA Helpers & Public Methods
// -----------------------------

// getCaa returns CAA records and TTL for an identifier. If domain is provided, it filters to that domain.
// Private helper; not part of the public gRPC surface.
func (srv *server) getCaa(id, domain string) ([]*dnspb.CAA, uint32, error) {
	id = strings.ToLower(id)
	uuid := Utility.GenerateUUID("CAA:" + id)

	data, err := srv.store.GetItem(uuid)
	if err != nil {
		if srv.Logger != nil {
			srv.Logger.Debug("caa:get no data", "id", id, "uuid", uuid, "err", err)
		}
		return nil, 0, err
	}

	caa := make([]*dnspb.CAA, 0)
	if err := json.Unmarshal(data, &caa); err != nil {
		if srv.Logger != nil {
			srv.Logger.Error("caa:get unmarshal failed", "id", id, "uuid", uuid, "err", err)
		}
		return nil, 0, err
	}

	if domain != "" {
		for i := range caa {
			if caa[i].Domain == domain {
				return []*dnspb.CAA{caa[i]}, srv.getTtl(uuid), nil
			}
		}
	}

	return caa, srv.getTtl(uuid), nil
}

// GetCaa retrieves CAA (Certification Authority Authorization) records from the server's storage.
// It accepts a GetCaaRequest containing an identifier and an optional domain filter.
// The function attempts to fetch and decode the CAA records associated with the given ID.
// If a domain is specified in the request, it filters the results to return only the matching CAA record.
// Returns a GetCaaResponse containing the matched CAA records, or an error if retrieval or decoding fails.
// Logs errors and debug information if a logger is configured.
func (srv *server) GetCaa(ctx context.Context, rqst *dnspb.GetCaaRequest) (*dnspb.GetCaaResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("CAA:" + id)

	data, err := srv.store.GetItem(uuid)
	if err != nil {
		if srv.Logger != nil {
			srv.Logger.Error("caa:get read failed", "id", id, "uuid", uuid, "err", err)
		}
		// NotFound vs Internal: if backend distinguishes, map accordingly.
		return nil, status.Errorf(codes.Internal, "failed to read CAA for %q: %v", id, err)
	}

	caa := make([]*dnspb.CAA, 0)
	if err := json.Unmarshal(data, &caa); err != nil {
		if srv.Logger != nil {
			srv.Logger.Error("caa:get decode failed", "id", id, "uuid", uuid, "err", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to decode stored CAA for %q: %v", id, err)
	}

	if rqst.Domain != "" {
		for i := range caa {
			if caa[i].Domain == rqst.Domain {
				if srv.Logger != nil {
					srv.Logger.Debug("caa:get filtered hit", "id", id, "domain", rqst.Domain)
				}
				return &dnspb.GetCaaResponse{Result: []*dnspb.CAA{caa[i]}}, nil
			}
		}
		// Domain filter miss is not an error; return empty.
		if srv.Logger != nil {
			srv.Logger.Info("caa:get filtered empty", "id", id, "domain", rqst.Domain)
		}
		return &dnspb.GetCaaResponse{Result: []*dnspb.CAA{}}, nil
	}

	if srv.Logger != nil {
		srv.Logger.Debug("caa:get all", "id", id, "count", len(caa))
	}
	return &dnspb.GetCaaResponse{Result: caa}, nil
}

// RemoveCaa removes a single CAA entry by rqst.Domain for rqst.Id. If the last entry is removed,
// the stored key is deleted and related RBAC permissions are purged.
// Public RPC — signature MUST NOT change.
func (srv *server) RemoveCaa(ctx context.Context, rqst *dnspb.RemoveCaaRequest) (*dnspb.RemoveCaaResponse, error) {

	_, token, err := security.GetClientId(ctx)
	if err != nil {
		srv.Logger.Error("caa:remove getClientId failed", "err", err)
		return nil, status.Errorf(codes.Internal, "failed to get client ID: %v", err)
	}

	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("CAA:" + id)

	data, err := srv.store.GetItem(uuid)
	if err != nil {
		if srv.Logger != nil {
			srv.Logger.Error("caa:remove read failed", "id", id, "uuid", uuid, "err", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to load CAA for %q: %v", id, err)
	}

	caa := make([]*dnspb.CAA, 0)
	if err := json.Unmarshal(data, &caa); err != nil {
		if srv.Logger != nil {
			srv.Logger.Error("caa:remove decode failed", "id", id, "uuid", uuid, "err", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to decode CAA for %q: %v", id, err)
	}
	if len(caa) == 0 {
		if srv.Logger != nil {
			srv.Logger.Warn("caa:remove no values", "id", id, "uuid", uuid)
		}
		return nil, status.Errorf(codes.NotFound, "no CAA values found for %q", id)
	}

	removed := false
	for i := range caa {
		if caa[i].Domain == rqst.Domain {
			caa = append(caa[:i], caa[i+1:]...)
			removed = true
			break
		}
	}
	if !removed {
		if srv.Logger != nil {
			srv.Logger.Info("caa:remove domain not found", "id", id, "uuid", uuid, "domain", rqst.Domain)
		}
		return &dnspb.RemoveCaaResponse{Result: false}, nil
	}

	if len(caa) > 0 {
		buf, err := json.Marshal(caa)
		if err != nil {
			if srv.Logger != nil {
				srv.Logger.Error("caa:remove encode failed", "id", id, "uuid", uuid, "err", err)
			}
			return nil, status.Errorf(codes.Internal, "failed to encode updated CAA for %q: %v", id, err)
		}
		if err := srv.store.SetItem(uuid, buf); err != nil {
			if srv.Logger != nil {
				srv.Logger.Error("caa:remove persist failed", "id", id, "uuid", uuid, "err", err)
			}
			return nil, status.Errorf(codes.Internal, "failed to store updated CAA for %q: %v", id, err)
		}
		if srv.Logger != nil {
			srv.Logger.Info("caa:removed", "id", id, "uuid", uuid, "domain", rqst.Domain, "remaining", len(caa))
		}
		return &dnspb.RemoveCaaResponse{Result: true}, nil
	}

	// No entries remain -> remove key and RBAC permissions.
	if err := srv.store.RemoveItem(uuid); err != nil {
		if srv.Logger != nil {
			srv.Logger.Error("caa:remove delete key failed", "id", id, "uuid", uuid, "err", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to delete empty CAA set for %q: %v", id, err)
	}
	if rbac_client_, err := srv.getRbacClient(); err == nil {
		_ = rbac_client_.DeleteResourcePermissions(token, rqst.Id)
	} else if srv.Logger != nil {
		srv.Logger.Warn("caa:remove rbac cleanup skipped", "id", id, "uuid", uuid, "err", err)
	}
	if srv.Logger != nil {
		srv.Logger.Info("caa:deleted", "id", id, "uuid", uuid, "domain", rqst.Domain)
	}
	return &dnspb.RemoveCaaResponse{Result: true}, nil
}

// -----------------------------------
// UDP DNS responder (miekg/dns) hook
// -----------------------------------

type handler struct{}

// ServeDNS handles incoming DNS queries and formulates appropriate DNS responses
// based on the query type. It supports various DNS record types including A, AAAA,
// AFSDB, CAA, CNAME, TXT, NS, MX, SOA, and URI. For each supported query type, it
// retrieves the relevant records using corresponding handler methods, constructs
// DNS response messages, and writes them back to the client. Errors encountered
// during record retrieval or response writing are logged using the configured logger.
// The method ensures authoritative responses where applicable and handles multiple
// questions for certain record types as needed.
//
// Parameters:
//   - w: dns.ResponseWriter used to send the DNS response.
//   - r: *dns.Msg representing the incoming DNS query message.
func (hd *handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	switch r.Question[0].Qtype {
	case dns.TypeA:
		msg := dns.Msg{}
		msg.SetReply(r)
		domain := msg.Question[0].Name
		msg.Authoritative = true
		addresses, ttl, err := srv.get_ipv4(domain)
		if err != nil && srv.Logger != nil {
			srv.Logger.Debug("dns:get A failed", "domain", domain, "err", err)
		}
		for _, address := range addresses {
			msg.Answer = append(msg.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ttl},
				A:   net.ParseIP(address),
			})
		}
		if err := w.WriteMsg(&msg); err != nil && srv.Logger != nil {
			srv.Logger.Error("dns:write failed", "qtype", "A", "domain", domain, "err", err)
		}

	case dns.TypeAAAA:
		msg := dns.Msg{}
		msg.SetReply(r)
		msg.Authoritative = true
		domain := msg.Question[0].Name
		addresses, ttl, err := srv.get_ipv6(domain)
		if err != nil && srv.Logger != nil {
			srv.Logger.Debug("dns:get AAAA failed", "domain", domain, "err", err)
		}
		for _, address := range addresses {
			msg.Answer = append(msg.Answer, &dns.AAAA{
				Hdr:  dns.RR_Header{Name: domain, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: ttl},
				AAAA: net.ParseIP(address),
			})
		}
		if err := w.WriteMsg(&msg); err != nil && srv.Logger != nil {
			srv.Logger.Error("dns:write failed", "qtype", "AAAA", "domain", domain, "err", err)
		}

	case dns.TypeAFSDB:
		msg := dns.Msg{}
		msg.SetReply(r)
		msg.Authoritative = true
		afsdb, ttl, err := srv.getAfsdb(msg.Question[0].Name)
		if err == nil {
			msg.Answer = append(msg.Answer, &dns.AFSDB{
				Hdr:      dns.RR_Header{Name: msg.Question[0].Name, Rrtype: dns.TypeAFSDB, Class: dns.ClassINET, Ttl: ttl},
				Subtype:  uint16(afsdb.Subtype),
				Hostname: afsdb.Hostname,
			})
		} else if srv.Logger != nil {
			srv.Logger.Debug("dns:get AFSDB failed", "name", msg.Question[0].Name, "err", err)
		}
		if err := w.WriteMsg(&msg); err != nil && srv.Logger != nil {
			srv.Logger.Error("dns:write failed", "qtype", "AFSDB", "err", err)
		}

	case dns.TypeCAA:
		msg := dns.Msg{}
		msg.SetReply(r)
		msg.Authoritative = true
		name := msg.Question[0].Name
		domain := ""
		if len(msg.Question) > 1 {
			domain = msg.Question[1].Name
		}
		values, ttl, err := srv.getCaa(name, domain)
		if err != nil && srv.Logger != nil {
			srv.Logger.Debug("dns:get CAA failed", "name", name, "domain", domain, "err", err)
		}
		for _, caa := range values {
			msg.Answer = append(msg.Answer, &dns.CAA{
				Hdr:   dns.RR_Header{Name: name, Rrtype: dns.TypeCAA, Class: dns.ClassINET, Ttl: ttl},
				Flag:  uint8(caa.Flag),
				Tag:   caa.Tag,
				Value: caa.Domain,
			})
		}
		if err := w.WriteMsg(&msg); err != nil && srv.Logger != nil {
			srv.Logger.Error("dns:write failed", "qtype", "CAA", "name", name, "err", err)
		}

	case dns.TypeCNAME:
		msg := dns.Msg{}
		msg.SetReply(r)
		cname, ttl, err := srv.getCName(msg.Question[0].Name)
		if err == nil {
			msg.Answer = append(msg.Answer, &dns.CNAME{
				Hdr:    dns.RR_Header{Name: msg.Question[0].Name, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: ttl},
				Target: cname,
			})
		} else if srv.Logger != nil {
			srv.Logger.Debug("dns:get CNAME failed", "name", msg.Question[0].Name, "err", err)
		}
		if err := w.WriteMsg(&msg); err != nil && srv.Logger != nil {
			srv.Logger.Error("dns:write failed", "qtype", "CNAME", "err", err)
		}

	case dns.TypeTXT:
		values, ttl, err := srv.getText(r.Question[0].Name)
		if err != nil && srv.Logger != nil {
			srv.Logger.Debug("dns:get TXT failed", "name", r.Question[0].Name, "err", err)
		}
		msg := new(dns.Msg)
		msg.SetReply(r)
		for _, txtValue := range values {
			msg.Answer = append(msg.Answer, &dns.TXT{
				Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: ttl},
				Txt: []string{txtValue},
			})
		}
		if err := w.WriteMsg(msg); err != nil && srv.Logger != nil {
			srv.Logger.Error("dns:write failed", "qtype", "TXT", "err", err)
		}

	case dns.TypeNS:
		values, ttl, err := srv.getNs(r.Question[0].Name)
		if err != nil && srv.Logger != nil {
			srv.Logger.Debug("dns:get NS failed", "name", r.Question[0].Name, "err", err)
		}
		msg := new(dns.Msg)
		msg.SetReply(r)
		for _, ns := range values {
			msg.Answer = append(msg.Answer, &dns.NS{
				Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: ttl},
				Ns:  ns,
			})
		}
		if err := w.WriteMsg(msg); err != nil && srv.Logger != nil {
			srv.Logger.Error("dns:write failed", "qtype", "NS", "err", err)
		}

	case dns.TypeMX:
		msg := dns.Msg{}
		msg.SetReply(r)
		mx := ""
		if len(msg.Question) > 1 {
			mx = msg.Question[1].Name
		}
		values, ttl, err := srv.getMx(msg.Question[0].Name, mx)
		if err != nil && srv.Logger != nil {
			srv.Logger.Debug("dns:get MX failed", "name", msg.Question[0].Name, "mx", mx, "err", err)
		}
		for _, mxr := range values {
			msg.Answer = append(msg.Answer, &dns.MX{
				Hdr:        dns.RR_Header{Name: msg.Question[0].Name, Rrtype: dns.TypeMX, Class: dns.ClassINET, Ttl: ttl},
				Preference: uint16(mxr.Preference),
				Mx:         mxr.Mx,
			})
		}
		if err := w.WriteMsg(&msg); err != nil && srv.Logger != nil {
			srv.Logger.Error("dns:write failed", "qtype", "MX", "err", err)
		}

	case dns.TypeSOA:
		msg := dns.Msg{}
		msg.SetReply(r)
		ns := ""
		if len(msg.Question) > 1 {
			ns = msg.Question[1].Name
		}
		values, ttl, err := srv.getSoa(msg.Question[0].Name, ns)
		if err != nil && srv.Logger != nil {
			srv.Logger.Debug("dns:get SOA failed", "name", msg.Question[0].Name, "ns", ns, "err", err)
		}
		domain := strings.ToLower(msg.Question[0].Name)
		if !strings.HasSuffix(domain, ".") {
			domain += "."
		}
		for _, soa := range values {
			if !strings.HasSuffix(soa.Mbox, ".") {
				soa.Mbox += "."
			}
			msg.Answer = append(msg.Answer, &dns.SOA{
				Hdr:     dns.RR_Header{Name: domain, Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: ttl},
				Ns:      soa.Ns,
				Mbox:    soa.Mbox,
				Serial:  soa.Serial,
				Refresh: soa.Refresh,
				Retry:   soa.Retry,
				Expire:  soa.Expire,
				Minttl:  soa.Minttl,
			})
		}
		if err := w.WriteMsg(&msg); err != nil && srv.Logger != nil {
			srv.Logger.Error("dns:write failed", "qtype", "SOA", "err", err)
		}

	case dns.TypeURI:
		msg := dns.Msg{}
		msg.SetReply(r)
		target := ""
		if len(msg.Question) > 1 {
			target = msg.Question[1].Name
		}
		values, ttl, err := srv.getUri(msg.Question[0].Name, target)
		if err != nil && srv.Logger != nil {
			srv.Logger.Debug("dns:get URI failed", "name", msg.Question[0].Name, "target", target, "err", err)
		}
		for _, uri := range values {
			msg.Answer = append(msg.Answer, &dns.URI{
				Hdr:      dns.RR_Header{Name: msg.Question[0].Name, Rrtype: dns.TypeURI, Class: dns.ClassINET, Ttl: ttl},
				Priority: uint16(uri.Priority),
				Weight:   uint16(uri.Weight),
				Target:   uri.Target,
			})
		}
		if err := w.WriteMsg(&msg); err != nil && srv.Logger != nil {
			srv.Logger.Error("dns:write failed", "qtype", "URI", "err", err)
		}
	}
}

// ServeDns starts a DNS server listening on the specified UDP port.
// It logs server start, stop, and error events if a logger is available.
// Returns an error if the server fails to start or encounters an issue during execution.
func ServeDns(port int) error {
	if srv != nil && srv.Logger != nil {
		srv.Logger.Info("dns:udp server starting", "port", port)
	}
	dnsServer := &dns.Server{Addr: ":" + strconv.Itoa(port), Net: "udp"}
	dnsServer.Handler = &handler{}
	if err := dnsServer.ListenAndServe(); err != nil {
		if srv != nil && srv.Logger != nil {
			srv.Logger.Error("dns:udp server failed", "port", port, "err", err)
		}
		return err
	}
	if srv != nil && srv.Logger != nil {
		srv.Logger.Info("dns:udp server stopped", "port", port)
	}
	return nil
}

// ---------------
// TTL persistence
// ---------------

// setTtl persists the TTL for a given record UUID.
func (srv *server) setTtl(uuid string, ttl uint32) error {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, ttl)
	uuid = Utility.GenerateUUID("TTL:" + uuid)
	return srv.store.SetItem(uuid, data)
}

// getTtl retrieves the TTL for a given record UUID, returning a default (60s) if none is set.
func (srv *server) getTtl(uuid string) uint32 {
	uuid = Utility.GenerateUUID("TTL:" + uuid)
	data, err := srv.store.GetItem(uuid)
	if err != nil {
		return 60
	}
	return binary.LittleEndian.Uint32(data)
}
