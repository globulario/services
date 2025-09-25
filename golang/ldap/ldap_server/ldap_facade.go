//go:build !js

package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	lmsg "github.com/lor00x/goldap/message"
	ldap "github.com/vjeantet/ldapserver"

	authentication_client "github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/config"
	resource_client "github.com/globulario/services/golang/resource/resource_client"
)

// ---- Public entrypoint ------------------------------------------------------

// StartLDAPFacade starts plain LDAP on :389 and LDAPS on :636.
// Uses Globular TLS certs and binds authenticate via Authentication service (sa + password).
func (s *server) StartLDAPFacade() error {
	baseDN := toBaseDN(s.Domain) // e.g. dc=globular,dc=io

	rc, err := s.getResourceClient()
	if err != nil {
		return err
	}

	addr, err := config.GetAddress()
	if err != nil || addr == "" {
		addr = "localhost:80"
	}

	lf := &ldapFacade{
		baseDN: baseDN,
		rc:     rc,
		addr:   addr,
		domain: s.Domain,
	}

	// Shared route mux across listeners
	routes := ldap.NewRouteMux()
	routes.Bind(lf.onBind)
	routes.Search(lf.onSearch)
	routes.Add(lf.onAdd)
	routes.Modify(lf.onModify)
	routes.Delete(lf.onDelete)

	// -------- Plain LDAP :389
	go func() {
		srv := ldap.NewServer()
		srv.Handle(routes)
		if err := srv.ListenAndServe(s.LdapListenAddr); err != nil {
			log.Println("LDAP server error:", err)
		}
	}()

	// -------- LDAPS :636 with Globular TLS certs
	go func() {
		srv := ldap.NewServer()
		srv.Handle(routes)

		cert, err := tls.LoadX509KeyPair(s.CertFile, s.KeyFile)
		if err != nil {
			log.Println("LDAPS: load key pair:", err)
			return
		}
		cfg := &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
		}
		_, err = tls.Listen("tcp", s.LdapsListenAddr, cfg)
		if err != nil {
			log.Println("LDAPS listen:", err)
			return
		}
		/*if err := srv.Serve(ln); err != nil {
			log.Println("LDAPS server error:", err)
		}*/
	}()

	log.Println("LDAP facade ready at :389 and :636 for base", baseDN)
	return nil
}

// ---- Internal types/helpers -------------------------------------------------

type ldapFacade struct {
	baseDN string
	rc     *resource_client.Resource_Client
	addr   string
	domain string
}

// Per-connection context (token is set on successful Bind)
type connState struct {
	token string
	user  string
	admin bool
}

// sessions: per-connection store keyed by client remote addr.
var sessions sync.Map // key string -> *connState

func sessKey(m *ldap.Message) string {
	if m != nil && m.Client != nil && m.Client.Addr() != nil {
		return m.Client.Addr().String()
	}
	return "default"
}

func toBaseDN(domain string) string {
	parts := strings.Split(domain, ".")
	var dn []string
	for _, p := range parts {
		if p == "" {
			continue
		}
		dn = append(dn, "dc="+strings.ToLower(p))
	}
	return strings.Join(dn, ",")
}

func (lf *ldapFacade) parseDN(dn string) (kind, id string) {
	ldn := strings.ToLower(dn)
	switch {
	case strings.Contains(ldn, "ou=people,"):
		return "user", getRDN("uid=", ldn)
	case strings.Contains(ldn, "ou=groups,"):
		return "group", getRDN("cn=", ldn)
	case strings.Contains(ldn, "ou=roles,"):
		return "role", getRDN("cn=", ldn)
	case strings.Contains(ldn, "ou=orgs,"):
		return "org", getRDN("o=", ldn)
	default:
		return "", ""
	}
}

func getRDN(prefix, dn string) string {
	for _, kv := range strings.Split(dn, ",") {
		kv = strings.TrimSpace(kv)
		if strings.HasPrefix(kv, prefix) {
			return strings.TrimPrefix(kv, prefix)
		}
	}
	return ""
}

// ---- LDAP Handlers ----------------------------------------------------------

// Bind: authenticate via Authentication service; cache token in "sessions".
func (lf *ldapFacade) onBind(w ldap.ResponseWriter, m *ldap.Message) {
	r := m.GetBindRequest()
	dn := string(r.Name())
	pw := string(r.AuthenticationSimple())

	res := ldap.NewBindResponse(ldap.LDAPResultSuccess)

	// Accept anonymous bind (optional: read-only)
	if dn == "" && pw == "" {
		w.Write(res)
		return
	}

	// Map DN -> userid (uid from user DN; allow admin DN too)
	kind, id := lf.parseDN(dn)
	admin := false
	if kind != "user" {
		if strings.HasPrefix(strings.ToLower(dn), "cn=admin,") || strings.HasPrefix(strings.ToLower(dn), "uid=sa,") {
			id = "sa"
			admin = true
		} else {
			res.SetResultCode(ldap.LDAPResultInvalidCredentials)
			w.Write(res)
			return
		}
	}

	// Authenticate using the Authentication service
	authCli, err := authentication_client.NewAuthenticationService_Client(lf.addr, "authentication.AuthenticationService")
	if err != nil {
		res.SetResultCode(ldap.LDAPResultUnavailable)
		res.SetDiagnosticMessage("auth service unavailable")
		w.Write(res)
		return
	}

	token, err := authCli.Authenticate(id, pw)
	if err != nil || token == "" {
		res.SetResultCode(ldap.LDAPResultInvalidCredentials)
		res.SetDiagnosticMessage("invalid credentials")
		w.Write(res)
		return
	}

	// Stash token in our map
	sessions.Store(sessKey(m), &connState{token: token, user: id, admin: admin})

	w.Write(res)
}

// Search: supports subtree under ou=people, ou=groups, ou=roles, ou=orgs
func (lf *ldapFacade) onSearch(w ldap.ResponseWriter, m *ldap.Message) {
	r := m.GetSearchRequest()
	base := strings.ToLower(string(r.BaseObject()))
	scope := r.Scope()

	// Helper to send entries (ProtocolOp)
	send := func(e lmsg.ProtocolOp) {
		w.Write(e)
	}

	// Root/base object
	if base == lf.baseDN && scope == lmsg.SearchRequestScopeBaseObject {
		e := ldap.NewSearchResultEntry(lf.baseDN)
		for _, v := range vals("top", "domain") {
			e.AddAttribute(lmsg.AttributeDescription("objectClass"), v)
		}
		send(e)
		// Advertise OUs
		for _, ou := range []string{"people", "groups", "roles", "orgs"} {
			dn := fmt.Sprintf("ou=%s,%s", ou, lf.baseDN)
			ouEntry := ldap.NewSearchResultEntry(dn)
			ouEntry.AddAttribute(lmsg.AttributeDescription("objectClass"), lmsg.AttributeValue("organizationalUnit"))
			ouEntry.AddAttribute(lmsg.AttributeDescription("objectClass"), lmsg.AttributeValue("top"))
			ouEntry.AddAttribute(lmsg.AttributeDescription("ou"), lmsg.AttributeValue(ou))
			send(ouEntry)
		}
		w.Write(ldap.NewSearchResultDoneResponse(ldap.LDAPResultSuccess))
		return
	}

	switch {
	case strings.Contains(base, "ou=people,"):
		accounts, _ := lf.rc.GetAccounts("{}")
		for _, a := range accounts {
			dn := fmt.Sprintf("uid=%s,ou=people,%s", a.Id, lf.baseDN)
			e := ldap.NewSearchResultEntry(dn)
			e.AddAttribute(lmsg.AttributeDescription("objectClass"), lmsg.AttributeValue("inetOrgPerson"))
			e.AddAttribute(lmsg.AttributeDescription("objectClass"), lmsg.AttributeValue("organizationalPerson"))
			e.AddAttribute(lmsg.AttributeDescription("objectClass"), lmsg.AttributeValue("person"))
			e.AddAttribute(lmsg.AttributeDescription("objectClass"), lmsg.AttributeValue("top"))
			e.AddAttribute(lmsg.AttributeDescription("uid"), lmsg.AttributeValue(a.Id))
			if a.Name != "" {
				e.AddAttribute(lmsg.AttributeDescription("cn"), lmsg.AttributeValue(a.Name))
				e.AddAttribute(lmsg.AttributeDescription("sn"), lmsg.AttributeValue(a.Name))
			}
			if a.Email != "" {
				e.AddAttribute(lmsg.AttributeDescription("mail"), lmsg.AttributeValue(a.Email))
			}
			send(e)
		}

	case strings.Contains(base, "ou=groups,"):
		groups, _ := lf.rc.GetGroups("{}")
		for _, g := range groups {
			dn := fmt.Sprintf("cn=%s,ou=groups,%s", g.Id, lf.baseDN)
			e := ldap.NewSearchResultEntry(dn)
			e.AddAttribute(lmsg.AttributeDescription("objectClass"), lmsg.AttributeValue("groupOfNames"))
			e.AddAttribute(lmsg.AttributeDescription("objectClass"), lmsg.AttributeValue("top"))
			e.AddAttribute(lmsg.AttributeDescription("cn"), lmsg.AttributeValue(g.Id))
			send(e)
		}

	case strings.Contains(base, "ou=roles,"):
		roles, _ := lf.rc.GetRoles("{}")
		for _, r := range roles {
			dn := fmt.Sprintf("cn=%s,ou=roles,%s", r.Id, lf.baseDN)
			e := ldap.NewSearchResultEntry(dn)
			for _, v := range vals("top", "globularRole") {
				e.AddAttribute(lmsg.AttributeDescription("objectClass"), v)
			}
			e.AddAttribute(lmsg.AttributeDescription("cn"), lmsg.AttributeValue(r.Id))
			if len(r.Actions) > 0 {
				for _, action := range vals(r.Actions...) {
					e.AddAttribute(lmsg.AttributeDescription("globularAction"), action)
				}
			}
			send(e)
		}

	case strings.Contains(base, "ou=orgs,"):
		orgs, _ := lf.rc.GetOrganizations("{}")
		for _, o := range orgs {
			dn := fmt.Sprintf("o=%s,ou=orgs,%s", o.Id, lf.baseDN)
			e := ldap.NewSearchResultEntry(dn)
			e.AddAttribute(lmsg.AttributeDescription("objectClass"), lmsg.AttributeValue("organization"))
			e.AddAttribute(lmsg.AttributeDescription("objectClass"), lmsg.AttributeValue("top"))
			e.AddAttribute(lmsg.AttributeDescription("o"), lmsg.AttributeValue(o.Id))
			if o.Description != "" {
				e.AddAttribute(lmsg.AttributeDescription("description"), lmsg.AttributeValue(o.Description))
			}
			if o.Email != "" {
				e.AddAttribute(lmsg.AttributeDescription("mail"), lmsg.AttributeValue(o.Email))
			}
			send(e)
		}
	}

	w.Write(ldap.NewSearchResultDoneResponse(ldap.LDAPResultSuccess))
}

// Add: user / group / role / org
func (lf *ldapFacade) onAdd(w ldap.ResponseWriter, m *ldap.Message) {
	cs := lf.mustSession(m)
	if cs == nil || cs.token == "" {
		w.Write(ldap.NewAddResponse(ldap.LDAPResultInsufficientAccessRights))
		return
	}

	r := m.GetAddRequest()
	kind, id := lf.parseDN(string(r.Entry()))
	if kind == "" {
		w.Write(ldap.NewAddResponse(ldap.LDAPResultUnwillingToPerform))
		return
	}

	attrs := map[string][]string{}
	for _, a := range r.Attributes() {
		attrs[strings.ToLower(string(a.Type_()))] = toStrings(a.Vals())
	}

	switch kind {
	case "user":
		name := first(attrs["cn"])
		mail := first(attrs["mail"])
		pwd := first(attrs["userpassword"])
		err := lf.rc.RegisterAccount(lf.domain, id, name, mail, pwd, pwd)
		writeAddResult(w, err)

	case "group":
		name := first(attrs["cn"])
		desc := first(attrs["description"])
		err := lf.rc.CreateGroup(cs.token, id, name, desc)
		if err == nil {
			for _, mem := range attrs["member"] {
				_, uid := lf.parseDN(mem)
				if uid != "" {
					_ = lf.rc.AddGroupMemberAccount(cs.token, id, uid)
				}
			}
		}
		writeAddResult(w, err)

	case "role":
		name := first(attrs["cn"])
		actions := attrs["globularaction"]
		err := lf.rc.CreateRole(cs.token, id, name, actions)
		writeAddResult(w, err)

	case "org":
		name := first(attrs["o"])
		desc := first(attrs["description"])
		mail := first(attrs["mail"])
		err := lf.rc.CreateOrganization(cs.token, id, name, mail, desc, "")
		writeAddResult(w, err)
	}
}

// Modify: update attributes, membership, role actions
func (lf *ldapFacade) onModify(w ldap.ResponseWriter, m *ldap.Message) {
	cs := lf.mustSession(m)
	if cs == nil || cs.token == "" {
		w.Write(ldap.NewModifyResponse(ldap.LDAPResultInsufficientAccessRights))
		return
	}

	r := m.GetModifyRequest()
	kind, id := lf.parseDN(string(r.Object()))
	if kind == "" {
		w.Write(ldap.NewModifyResponse(ldap.LDAPResultUnwillingToPerform))
		return
	}

	switch kind {
	case "user":
		acc, err := lf.rc.GetAccount(id)
		if err != nil || acc == nil {
			w.Write(ldap.NewModifyResponse(ldap.LDAPResultNoSuchObject))
			return
		}
		for _, ch := range r.Changes() {
			typ := strings.ToLower(string(ch.Modification().Type_()))
			switch typ {
			case "mail":
				acc.Email = first(toStrings(ch.Modification().Vals()))
			case "cn":
				acc.Name = first(toStrings(ch.Modification().Vals()))
			case "userpassword":
				newPwd := first(toStrings(ch.Modification().Vals()))
				_ = lf.rc.SetAccountPassword(id, cs.token, "", newPwd)
			}
		}
		err = lf.rc.SetAccount(cs.token, acc)
		writeModifyResult(w, err)

	case "group":
		for _, ch := range r.Changes() {
			op := ch.Operation()
			if strings.ToLower(string(ch.Modification().Type_())) == "member" {
				for _, v := range toStrings(ch.Modification().Vals()) {
					_, uid := lf.parseDN(v)
					if uid == "" {
						continue
					}
					if op == lmsg.ModifyRequestChangeOperationAdd {
						_ = lf.rc.AddGroupMemberAccount(cs.token, id, uid)
					} else if op == lmsg.ModifyRequestChangeOperationDelete {
						_ = lf.rc.RemoveGroupMemberAccount(cs.token, id, uid)
					}
				}
			}
		}
		w.Write(ldap.NewModifyResponse(ldap.LDAPResultSuccess))

	case "role":
		for _, ch := range r.Changes() {
			if strings.ToLower(string(ch.Modification().Type_())) == "globularaction" {
				if ch.Operation() == lmsg.ModifyRequestChangeOperationAdd {
					_ = lf.rc.AddRoleActions(id, toStrings(ch.Modification().Vals()))
				} else if ch.Operation() == lmsg.ModifyRequestChangeOperationDelete {
					for _, a := range toStrings(ch.Modification().Vals()) {
						_ = lf.rc.RemoveRoleAction(id, a)
					}
				}
			}
		}
		w.Write(ldap.NewModifyResponse(ldap.LDAPResultSuccess))

	case "org":
		for _, ch := range r.Changes() {
			t := strings.ToLower(string(ch.Modification().Type_()))
			switch t {
			case "member":
				for _, v := range toStrings(ch.Modification().Vals()) {
					_, uid := lf.parseDN(v)
					if uid == "" {
						continue
					}
					if ch.Operation() == lmsg.ModifyRequestChangeOperationAdd {
						_ = lf.rc.AddOrganizationAccount(cs.token, id, uid)
					} else if ch.Operation() == lmsg.ModifyRequestChangeOperationDelete {
						_ = lf.rc.RemoveOrganizationAccount(cs.token, id, uid)
					}
				}
			}
		}
		w.Write(ldap.NewModifyResponse(ldap.LDAPResultSuccess))
	}
}

// Delete: remove user / group / role / org
// Delete: remove user / group / role / org
func (lf *ldapFacade) onDelete(w ldap.ResponseWriter, m *ldap.Message) {
    cs := lf.mustSession(m)
    if cs == nil || cs.token == "" {
        w.Write(ldap.NewDeleteResponse(ldap.LDAPResultInsufficientAccessRights))
        return
    }

    r := m.GetDeleteRequest()
    dn := string(r) // <- DelRequest is the DN itself
    kind, id := lf.parseDN(dn)
    if kind == "" {
        w.Write(ldap.NewDeleteResponse(ldap.LDAPResultUnwillingToPerform))
        return
    }

    var err error
    switch kind {
    case "user":
        err = lf.rc.DeleteAccount(id, cs.token)
    case "group":
        err = lf.rc.DeleteGroup(cs.token, id)
    case "role":
        err = lf.rc.DeleteRole(id)
    case "org":
        err = lf.rc.DeleteOrganization(cs.token, id)
    }
    writeDeleteResult(w, err)
}
// ---- small helpers ----------------------------------------------------------

func (lf *ldapFacade) mustSession(m *ldap.Message) *connState {
	if v, ok := sessions.Load(sessKey(m)); ok {
		if cs, ok2 := v.(*connState); ok2 {
			return cs
		}
	}
	return nil
}

func first(v []string) string {
	if len(v) == 0 {
		return ""
	}
	return v[0]
}

func toStrings(v []lmsg.AttributeValue) []string {
	out := make([]string, 0, len(v))
	for _, b := range v {
		out = append(out, string(b))
	}
	return out
}

func vals(vs ...string) []lmsg.AttributeValue {
	out := make([]lmsg.AttributeValue, 0, len(vs))
	for _, s := range vs {
		out = append(out, lmsg.AttributeValue(s))
	}
	return out
}

func writeAddResult(w ldap.ResponseWriter, err error) {
	code := ldap.LDAPResultSuccess
	if err != nil {
		code = ldap.LDAPResultOther
	}
	w.Write(ldap.NewAddResponse(code))
}

func writeModifyResult(w ldap.ResponseWriter, err error) {
	code := ldap.LDAPResultSuccess
	if err != nil {
		code = ldap.LDAPResultOther
	}
	w.Write(ldap.NewModifyResponse(code))
}

func writeDeleteResult(w ldap.ResponseWriter, err error) {
	code := ldap.LDAPResultSuccess
	if err != nil {
		code = ldap.LDAPResultOther
	}
	w.Write(ldap.NewDeleteResponse(code))
}

// ---- OPTIONAL: health check -------------------------------------------------

// PingLDAPFacade can be used to check that at least the server goroutines are running.
func (s *server) PingLDAPFacade() string {
	return time.Now().Format(time.RFC3339Nano)
}
