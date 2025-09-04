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
	Utility "github.com/globulario/utility"
	"github.com/miekg/dns"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

/* =========================
   A (IPv4) record endpoints
   ========================= */

// SetA stores (or appends) an IPv4 address for the given domain. It also sets the TTL for that record key.
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

// RemoveA removes a specific IPv4 address from the A record list. If no value remains, the key is deleted.
func (srv *server) RemoveA(ctx context.Context, rqst *dnspb.RemoveARequest) (*dnspb.RemoveAResponse, error) {
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
		if rbac_client_, err := srv.GetRbacClient(); err == nil {
			_ = rbac_client_.DeleteResourcePermissions(domain)
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

// GetA returns the list of IPv4 addresses associated with a domain.
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

// SetAAAA stores (or appends) an IPv6 address for the given domain and sets its TTL.
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

// RemoveAAAA removes a specific IPv6 address (or deletes the key if no values remain).
func (srv *server) RemoveAAAA(ctx context.Context, rqst *dnspb.RemoveAAAARequest) (*dnspb.RemoveAAAAResponse, error) {
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
		if rbac_client_, err := srv.GetRbacClient(); err == nil {
			_ = rbac_client_.DeleteResourcePermissions(domain)
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
		return nil, 0, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no value found for domain "+domain)))
	}
	return values, srv.getTtl(uuid), nil
}

// GetAAAA returns the list of IPv6 addresses associated with a domain.
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
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no value found for domain "+domain)))
	}
	return &dnspb.GetAAAAResponse{Aaaa: values}, nil
}

/* ===============
   TXT record API
   =============== */

// SetText appends TXT values for an identifier and stores TTL.
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

// GetText returns TXT values for an identifier.
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

// RemoveText deletes all TXT values for an identifier.
func (srv *server) RemoveText(ctx context.Context, rqst *dnspb.RemoveTextRequest) (*dnspb.RemoveTextResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("TXT:" + id)
	if err := srv.store.RemoveItem(uuid); err != nil {
		srv.Logger.Error("RemoveText removeItem", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if rbac_client_, err := srv.GetRbacClient(); err == nil {
		_ = rbac_client_.DeleteResourcePermissions(rqst.Id)
	}
	srv.Logger.Info("TXT removed", "id", id, "uuid", uuid)
	return &dnspb.RemoveTextResponse{Result: true}, nil
}

/* ==============
   NS record API
   ============== */

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

func (srv *server) RemoveNs(ctx context.Context, rqst *dnspb.RemoveNsRequest) (*dnspb.RemoveNsResponse, error) {
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
		if rbac_client_, err := srv.GetRbacClient(); err == nil {
			_ = rbac_client_.DeleteResourcePermissions(rqst.Id)
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

func (srv *server) GetCName(ctx context.Context, rqst *dnspb.GetCNameRequest) (*dnspb.GetCNameResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("CName:" + id)
	data, err := srv.store.GetItem(uuid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &dnspb.GetCNameResponse{Cname: string(data)}, nil
}

func (srv *server) RemoveCName(ctx context.Context, rqst *dnspb.RemoveCNameRequest) (*dnspb.RemoveCNameResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("CName:" + id)
	if err := srv.store.RemoveItem(uuid); err != nil {
		srv.Logger.Error("RemoveCName removeItem", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if rbac_client_, err := srv.GetRbacClient(); err == nil {
		_ = rbac_client_.DeleteResourcePermissions(rqst.Id)
	}
	srv.Logger.Info("CNAME removed", "id", id, "uuid", uuid)
	return &dnspb.RemoveCNameResponse{Result: true}, nil
}

/* =============
   MX record API
   ============= */

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

func (srv *server) RemoveMx(ctx context.Context, rqst *dnspb.RemoveMxRequest) (*dnspb.RemoveMxResponse, error) {
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
		if rbac_client_, err := srv.GetRbacClient(); err == nil {
			_ = rbac_client_.DeleteResourcePermissions(rqst.Id)
		}
		srv.Logger.Info("MX record deleted", "id", id, "uuid", uuid, "host", rqst.Mx)
	}

	return &dnspb.RemoveMxResponse{Result: true}, nil
}

/* =============
   SOA record API
   ============= */

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

func (srv *server) RemoveSoa(ctx context.Context, rqst *dnspb.RemoveSoaRequest) (*dnspb.RemoveSoaResponse, error) {
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
		if rbac_client_, err := srv.GetRbacClient(); err == nil {
			_ = rbac_client_.DeleteResourcePermissions(rqst.Id)
		}
		srv.Logger.Info("SOA record deleted", "id", id, "uuid", uuid, "ns", rqst.Ns)
	}

	return &dnspb.RemoveSoaResponse{Result: true}, nil
}

/* =============
   URI record API
   ============= */

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

func (srv *server) RemoveUri(ctx context.Context, rqst *dnspb.RemoveUriRequest) (*dnspb.RemoveUriResponse, error) {
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
		if rbac_client_, err := srv.GetRbacClient(); err == nil {
			_ = rbac_client_.DeleteResourcePermissions(rqst.Id)
		}
		srv.Logger.Info("URI record deleted", "id", id, "uuid", uuid, "target", rqst.Target)
	}

	return &dnspb.RemoveUriResponse{Result: true}, nil
}

/* ==============
   AFSDB record API
   ============== */

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
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("AFSDB:" + id)
	if err := srv.store.RemoveItem(uuid); err != nil {
		srv.Logger.Error("RemoveAfsdb removeItem", "id", id, "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if rbac_client_, err := srv.GetRbacClient(); err == nil {
		_ = rbac_client_.DeleteResourcePermissions(rqst.Id)
	}
	srv.Logger.Info("AFSDB removed", "id", id, "uuid", uuid)
	return &dnspb.RemoveAfsdbResponse{Result: true}, nil
}

/* =============
   CAA record API
   ============= */

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

// GetCaa returns all CAA records for rqst.Id, or only the one matching rqst.Domain if provided.
// Public RPC — signature MUST NOT change.
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
	if rbac_client_, err := srv.GetRbacClient(); err == nil {
		_ = rbac_client_.DeleteResourcePermissions(rqst.Id)
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

// ServeDNS handles incoming DNS queries and writes responses using data stored by the service.
// Errors are logged locally via slog.
func (hd *handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	switch r.Question[0].Qtype {
	case dns.TypeA:
		msg := dns.Msg{}
		msg.SetReply(r)
		domain := msg.Question[0].Name
		msg.Authoritative = true
		addresses, ttl, err := s.get_ipv4(domain)
		if err != nil && s.Logger != nil {
			s.Logger.Debug("dns:get A failed", "domain", domain, "err", err)
		}
		for _, address := range addresses {
			msg.Answer = append(msg.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ttl},
				A:   net.ParseIP(address),
			})
		}
		if err := w.WriteMsg(&msg); err != nil && s.Logger != nil {
			s.Logger.Error("dns:write failed", "qtype", "A", "domain", domain, "err", err)
		}

	case dns.TypeAAAA:
		msg := dns.Msg{}
		msg.SetReply(r)
		msg.Authoritative = true
		domain := msg.Question[0].Name
		addresses, ttl, err := s.get_ipv6(domain)
		if err != nil && s.Logger != nil {
			s.Logger.Debug("dns:get AAAA failed", "domain", domain, "err", err)
		}
		for _, address := range addresses {
			msg.Answer = append(msg.Answer, &dns.AAAA{
				Hdr:  dns.RR_Header{Name: domain, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: ttl},
				AAAA: net.ParseIP(address),
			})
		}
		if err := w.WriteMsg(&msg); err != nil && s.Logger != nil {
			s.Logger.Error("dns:write failed", "qtype", "AAAA", "domain", domain, "err", err)
		}

	case dns.TypeAFSDB:
		msg := dns.Msg{}
		msg.SetReply(r)
		msg.Authoritative = true
		afsdb, ttl, err := s.getAfsdb(msg.Question[0].Name)
		if err == nil {
			msg.Answer = append(msg.Answer, &dns.AFSDB{
				Hdr:      dns.RR_Header{Name: msg.Question[0].Name, Rrtype: dns.TypeAFSDB, Class: dns.ClassINET, Ttl: ttl},
				Subtype:  uint16(afsdb.Subtype),
				Hostname: afsdb.Hostname,
			})
		} else if s.Logger != nil {
			s.Logger.Debug("dns:get AFSDB failed", "name", msg.Question[0].Name, "err", err)
		}
		if err := w.WriteMsg(&msg); err != nil && s.Logger != nil {
			s.Logger.Error("dns:write failed", "qtype", "AFSDB", "err", err)
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
		values, ttl, err := s.getCaa(name, domain)
		if err != nil && s.Logger != nil {
			s.Logger.Debug("dns:get CAA failed", "name", name, "domain", domain, "err", err)
		}
		for _, caa := range values {
			msg.Answer = append(msg.Answer, &dns.CAA{
				Hdr:   dns.RR_Header{Name: name, Rrtype: dns.TypeCAA, Class: dns.ClassINET, Ttl: ttl},
				Flag:  uint8(caa.Flag),
				Tag:   caa.Tag,
				Value: caa.Domain,
			})
		}
		if err := w.WriteMsg(&msg); err != nil && s.Logger != nil {
			s.Logger.Error("dns:write failed", "qtype", "CAA", "name", name, "err", err)
		}

	case dns.TypeCNAME:
		msg := dns.Msg{}
		msg.SetReply(r)
		cname, ttl, err := s.getCName(msg.Question[0].Name)
		if err == nil {
			msg.Answer = append(msg.Answer, &dns.CNAME{
				Hdr:    dns.RR_Header{Name: msg.Question[0].Name, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: ttl},
				Target: cname,
			})
		} else if s.Logger != nil {
			s.Logger.Debug("dns:get CNAME failed", "name", msg.Question[0].Name, "err", err)
		}
		if err := w.WriteMsg(&msg); err != nil && s.Logger != nil {
			s.Logger.Error("dns:write failed", "qtype", "CNAME", "err", err)
		}

	case dns.TypeTXT:
		values, ttl, err := s.getText(r.Question[0].Name)
		if err != nil && s.Logger != nil {
			s.Logger.Debug("dns:get TXT failed", "name", r.Question[0].Name, "err", err)
		}
		msg := new(dns.Msg)
		msg.SetReply(r)
		for _, txtValue := range values {
			msg.Answer = append(msg.Answer, &dns.TXT{
				Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: ttl},
				Txt: []string{txtValue},
			})
		}
		if err := w.WriteMsg(msg); err != nil && s.Logger != nil {
			s.Logger.Error("dns:write failed", "qtype", "TXT", "err", err)
		}

	case dns.TypeNS:
		values, ttl, err := s.getNs(r.Question[0].Name)
		if err != nil && s.Logger != nil {
			s.Logger.Debug("dns:get NS failed", "name", r.Question[0].Name, "err", err)
		}
		msg := new(dns.Msg)
		msg.SetReply(r)
		for _, ns := range values {
			msg.Answer = append(msg.Answer, &dns.NS{
				Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: ttl},
				Ns:  ns,
			})
		}
		if err := w.WriteMsg(msg); err != nil && s.Logger != nil {
			s.Logger.Error("dns:write failed", "qtype", "NS", "err", err)
		}

	case dns.TypeMX:
		msg := dns.Msg{}
		msg.SetReply(r)
		mx := ""
		if len(msg.Question) > 1 {
			mx = msg.Question[1].Name
		}
		values, ttl, err := s.getMx(msg.Question[0].Name, mx)
		if err != nil && s.Logger != nil {
			s.Logger.Debug("dns:get MX failed", "name", msg.Question[0].Name, "mx", mx, "err", err)
		}
		for _, mxr := range values {
			msg.Answer = append(msg.Answer, &dns.MX{
				Hdr:        dns.RR_Header{Name: msg.Question[0].Name, Rrtype: dns.TypeMX, Class: dns.ClassINET, Ttl: ttl},
				Preference: uint16(mxr.Preference),
				Mx:         mxr.Mx,
			})
		}
		if err := w.WriteMsg(&msg); err != nil && s.Logger != nil {
			s.Logger.Error("dns:write failed", "qtype", "MX", "err", err)
		}

	case dns.TypeSOA:
		msg := dns.Msg{}
		msg.SetReply(r)
		ns := ""
		if len(msg.Question) > 1 {
			ns = msg.Question[1].Name
		}
		values, ttl, err := s.getSoa(msg.Question[0].Name, ns)
		if err != nil && s.Logger != nil {
			s.Logger.Debug("dns:get SOA failed", "name", msg.Question[0].Name, "ns", ns, "err", err)
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
		if err := w.WriteMsg(&msg); err != nil && s.Logger != nil {
			s.Logger.Error("dns:write failed", "qtype", "SOA", "err", err)
		}

	case dns.TypeURI:
		msg := dns.Msg{}
		msg.SetReply(r)
		target := ""
		if len(msg.Question) > 1 {
			target = msg.Question[1].Name
		}
		values, ttl, err := s.getUri(msg.Question[0].Name, target)
		if err != nil && s.Logger != nil {
			s.Logger.Debug("dns:get URI failed", "name", msg.Question[0].Name, "target", target, "err", err)
		}
		for _, uri := range values {
			msg.Answer = append(msg.Answer, &dns.URI{
				Hdr:      dns.RR_Header{Name: msg.Question[0].Name, Rrtype: dns.TypeURI, Class: dns.ClassINET, Ttl: ttl},
				Priority: uint16(uri.Priority),
				Weight:   uint16(uri.Weight),
				Target:   uri.Target,
			})
		}
		if err := w.WriteMsg(&msg); err != nil && s.Logger != nil {
			s.Logger.Error("dns:write failed", "qtype", "URI", "err", err)
		}
	}
}

// ServeDns starts the UDP DNS server on the specified port.
// Public API — signature MUST NOT change.
func ServeDns(port int) error {
	if s != nil && s.Logger != nil {
		s.Logger.Info("dns:udp server starting", "port", port)
	}
	srv := &dns.Server{Addr: ":" + strconv.Itoa(port), Net: "udp"}
	srv.Handler = &handler{}
	if err := srv.ListenAndServe(); err != nil {
		if s != nil && s.Logger != nil {
			s.Logger.Error("dns:udp server failed", "port", port, "err", err)
		}
		return err
	}
	if s != nil && s.Logger != nil {
		s.Logger.Info("dns:udp server stopped", "port", port)
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
