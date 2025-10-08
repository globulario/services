//go:build !js

package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"reflect" // ← add this
	"strings"
	"sync"
	"time"

	lmsg "github.com/lor00x/goldap/message"
	ldap "github.com/vjeantet/ldapserver"

	authentication_client "github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/config"
	resource_client "github.com/globulario/services/golang/resource/resource_client"
)

// ---- SEARCH helpers: scope / filter / reflection ----------------------------

// case-insensitive DN compare helpers
func normDN(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func parentDN(dn string) string {
    parts := strings.SplitN(dn, ",", 2)
    if len(parts) == 2 { return parts[1] }
    return ""
}
// LDAP scope values (avoid depending on lmsg constants)
const (
	scopeBaseObject   = 0 // baseObject(0)
	scopeSingleLevel  = 1 // singleLevel(1)
	scopeWholeSubtree = 2 // wholeSubtree(2)
)

func inScope(entryDN, baseDN string, scope int) bool {
	e := normDN(entryDN)
	b := normDN(baseDN)
	switch scope {
	case scopeBaseObject:
		return e == b
	case scopeSingleLevel:
		return normDN(parentDN(e)) == b
	case scopeWholeSubtree:
		return e == b || strings.HasSuffix(e, ","+b)
	default:
		return true
	}
}

// very small filter parser: supports (attr=value) and (&(a=b)(c=d))
// recognized attrs: cn, uid, o, objectclass
func parseEqFilters(f string) map[string]string {
    f = strings.TrimSpace(f)
    if f == "" || f == "(objectClass=*)" {
        return nil
    }
    out := map[string]string{}
    lower := strings.ToLower(f)

    // strip outer (& ... ) if present
    if strings.HasPrefix(lower, "(&") && strings.HasSuffix(lower, ")") {
        inner := strings.TrimSuffix(strings.TrimPrefix(lower, "(&"), ")")
        // split into sub-filters "(a=b)(c=d)"
        for len(inner) > 0 {
            if inner[0] != '(' { break }
            end := strings.IndexByte(inner[1:], ')')
            if end < 0 { break }
            seg := inner[1 : 1+end]
            if kv := strings.SplitN(seg, "=", 2); len(kv) == 2 {
                k := strings.TrimSpace(kv[0])
                v := strings.TrimSpace(kv[1])
                if k != "" && v != "" {
                    out[k] = v
                }
            }
            inner = inner[2+end:]
        }
        return out
    }

    // single (a=b)
    if strings.HasPrefix(lower, "(") && strings.HasSuffix(lower, ")") {
        body := strings.TrimSuffix(strings.TrimPrefix(lower, "("), ")")
        if kv := strings.SplitN(body, "=", 2); len(kv) == 2 {
            k := strings.TrimSpace(kv[0])
            v := strings.TrimSpace(kv[1])
            if k != "" && v != "" {
                out[k] = v
            }
        }
    }
    return out
}

func matchesEq(attrs map[string]string, eq map[string]string) bool {
    if len(eq) == 0 { return true }
    for k, v := range eq {
        k = strings.ToLower(k)
        if k == "objectclass" {
            // allow "objectClass" constraint when present
            if got, ok := attrs["objectclass"]; !ok || !strings.Contains(strings.ToLower(got), strings.ToLower(v)) {
                return false
            }
            continue
        }
        got, ok := attrs[k]
        if !ok { return false }
        if strings.ToLower(got) != strings.ToLower(v) { return false }
    }
    return true
}

// reflect helpers to fetch IDs from resource_client if such methods exist.
// They safely return empty on absence / mismatch, so code still compiles & runs.

func (lf *ldapFacade) callIDs(method string, args ...interface{}) []string {
    rv := reflect.ValueOf(lf.rc)
    m := rv.MethodByName(method)
    if !m.IsValid() { return nil }

    // build args
    in := make([]reflect.Value, len(args))
    for i, a := range args {
        in[i] = reflect.ValueOf(a)
    }
    out := m.Call(in)
    if len(out) == 0 { return nil }

    // expect ([]string, error) or ([]T, error) or just []string / []T
    var slice reflect.Value
    if out[0].Kind() == reflect.Slice {
        slice = out[0]
    } else {
        return nil
    }

    // try to detect trailing error
    if len(out) > 1 && out[len(out)-1].Type().String() == "error" {
        if !out[len(out)-1].IsNil() {
            return nil
        }
    }

    ids := []string{}
    for i := 0; i < slice.Len(); i++ {
        el := slice.Index(i).Interface()
        switch v := el.(type) {
        case string:
            ids = append(ids, v)
        default:
            // try field "Id" / "ID"
            rv := reflect.ValueOf(v)
            if rv.Kind() == reflect.Ptr { rv = rv.Elem() }
            if rv.Kind() == reflect.Struct {
                f := rv.FieldByName("Id")
                if !f.IsValid() { f = rv.FieldByName("ID") }
                if f.IsValid() && f.Kind() == reflect.String {
                    ids = append(ids, f.String())
                }
            }
        }
    }
    return ids
}

// ORG
func (lf *ldapFacade) orgAccountIDs(orgID string) []string {
    if ids := lf.callIDs("GetOrganizationAccounts", orgID); len(ids) > 0 { return ids }
    if ids := lf.callIDs("GetOrganizationAccounts", lf.baseDN, orgID); len(ids) > 0 { return ids }
    return nil
}
func (lf *ldapFacade) orgGroupIDs(orgID string) []string {
    if ids := lf.callIDs("GetOrganizationGroups", orgID); len(ids) > 0 { return ids }
    if ids := lf.callIDs("GetOrganizationGroups", lf.baseDN, orgID); len(ids) > 0 { return ids }
    return nil
}
func (lf *ldapFacade) orgRoleIDs(orgID string) []string {
    if ids := lf.callIDs("GetOrganizationRoles", orgID); len(ids) > 0 { return ids }
    if ids := lf.callIDs("GetOrganizationRoles", lf.baseDN, orgID); len(ids) > 0 { return ids }
    return nil
}

// GROUP
func (lf *ldapFacade) groupAccountIDs(groupID string) []string {
    if ids := lf.callIDs("GetGroupMemberAccounts", groupID); len(ids) > 0 { return ids }
    if ids := lf.callIDs("GetGroupAccounts", groupID); len(ids) > 0 { return ids }
    return nil
}

// ROLE
func (lf *ldapFacade) roleAccountIDs(roleID string) []string {
    if ids := lf.callIDs("GetRoleAccounts", roleID); len(ids) > 0 { return ids }
    if ids := lf.callIDs("GetAccountsWithRole", roleID); len(ids) > 0 { return ids }
    return nil
}
// ---- Public entrypoint ------------------------------------------------------

// StartLDAPFacade starts plain LDAP on :389 and LDAPS on :636.
// Uses Globular TLS certs and binds authenticate via Authentication service (sa + password).
func (s *server) StartLDAPFacade() error {
	baseDN := toBaseDN(s.Domain)

	rc, err := s.getResourceClient()
	if err != nil {
		return fmt.Errorf("resource client: %w", err)
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

	// Shared routes for both listeners
	routes := ldap.NewRouteMux()
	routes.Bind(lf.onBind)
	routes.Search(lf.onSearch)
	routes.Add(lf.onAdd)
	routes.Modify(lf.onModify)
	routes.Delete(lf.onDelete)

	// ---------- Plain LDAP (:389)
	go func() {
		srv := ldap.NewServer()
		srv.Handle(routes)

		addr := s.LdapListenAddr
		if addr == "" {
			addr = ":389"
		}
		if err := srv.ListenAndServe(addr); err != nil {
			log.Println("LDAP server error:", err)
		} else {
			log.Printf("LDAP listening on %s (base DN %s)\n", addr, baseDN)
		}
	}()

	// ---------- LDAPS (:636) using the option hook to wrap the listener with TLS
	go func() {
		srv := ldap.NewServer()
		srv.Handle(routes)

		// Load server certificate/key (PEM files)
		cert, err := tls.LoadX509KeyPair(s.CertFile, s.KeyFile)
		if err != nil {
			log.Println("LDAPS: load key pair:", err)
			return
		}
		tcfg := &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
			// If clients verify SNI/hostname, ensure your cert SAN matches s.Domain:
			// ServerName: s.Domain,
		}

		addr := s.LdapsListenAddr
		if addr == "" {
			addr = ":636"
		}

		// Option that runs after ListenAndServe creates s.Listener:
		// we wrap that TCP listener with TLS.
		wrapTLS := func(sv *ldap.Server) {
			if sv.Listener != nil {
				sv.Listener = tls.NewListener(sv.Listener, tcfg)
			}
		}

		if err := srv.ListenAndServe(addr, wrapTLS); err != nil {
			log.Println("LDAPS server error:", err)
		} else {
			log.Printf("LDAPS listening on %s (base DN %s)\n", addr, baseDN)
		}
	}()

	log.Println("LDAP facade ready at", s.LdapListenAddr, "and", s.LdapsListenAddr, "for base", baseDN)
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

	if dn == "" && pw == "" {
		w.Write(res)
		return
	}

	kind, id := lf.parseDN(dn)
	admin := false
	if kind != "user" {
		ldn := strings.ToLower(dn)
		if strings.HasPrefix(ldn, "cn=admin,") ||
			strings.HasPrefix(ldn, "cn=sa,") || // ← add this line
			strings.HasPrefix(ldn, "uid=sa,") {
			id = "sa"
			admin = true
		} else {
			res.SetResultCode(ldap.LDAPResultInvalidCredentials)
			w.Write(res)
			return
		}
	}

	authCli, err := authentication_client.
		NewAuthenticationService_Client(lf.addr, "authentication.AuthenticationService")
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

	sessions.Store(sessKey(m), &connState{token: token, user: id, admin: admin})
	w.Write(res)
}

// getFilterString tries r.FilterString(), else falls back to fmt.
func getFilterString(r *lmsg.SearchRequest) string {
	if fs, ok := any(r).(interface{ FilterString() string }); ok {
		return fs.FilterString()
	}
	return fmt.Sprintf("%v", r.Filter())
}

// Search: supports subtree under ou=people, ou=groups, ou=roles, ou=orgs
func (lf *ldapFacade) onSearch(w ldap.ResponseWriter, m *ldap.Message) {
	r := m.GetSearchRequest()
	base := string(r.BaseObject())
	scopeInt := int(r.Scope())
	filterStr := getFilterString(&r)
	eq := parseEqFilters(filterStr)

	send := func(e lmsg.ProtocolOp) { w.Write(e) }
	sendDone := func() { w.Write(ldap.NewSearchResultDoneResponse(ldap.LDAPResultSuccess)) }

	lowerBase := strings.ToLower(base)

	// Root/base object
	if normDN(lowerBase) == normDN(lf.baseDN) && scopeInt == scopeBaseObject {
		e := ldap.NewSearchResultEntry(lf.baseDN)
		for _, v := range vals("top", "domain") {
			e.AddAttribute(lmsg.AttributeDescription("objectClass"), v)
		}
		send(e)
		for _, ou := range []string{"people", "groups", "roles", "orgs"} {
			dn := fmt.Sprintf("ou=%s,%s", ou, lf.baseDN)
			ouEntry := ldap.NewSearchResultEntry(dn)
			ouEntry.AddAttribute(lmsg.AttributeDescription("objectClass"), lmsg.AttributeValue("organizationalUnit"))
			ouEntry.AddAttribute(lmsg.AttributeDescription("objectClass"), lmsg.AttributeValue("top"))
			ouEntry.AddAttribute(lmsg.AttributeDescription("ou"), lmsg.AttributeValue(ou))
			if inScope(dn, base, scopeInt) && matchesEq(map[string]string{"objectclass": "organizationalUnit"}, eq) {
				send(ouEntry)
			}
		}
		sendDone()
		return
	}

	shouldSend := func(dn string, attrs map[string]string) bool {
		return inScope(dn, base, scopeInt) && matchesEq(attrs, eq)
	}

	// People
	if strings.Contains(lowerBase, "ou=people,") {
		accounts, _ := lf.rc.GetAccounts("{}")
		for _, a := range accounts {
			dn := fmt.Sprintf("uid=%s,ou=people,%s", a.Id, lf.baseDN)
			attrs := map[string]string{
				"uid":         a.Id,
				"cn":          a.Name,
				"objectclass": "inetOrgPerson organizationalPerson person top",
			}
			if !shouldSend(dn, attrs) { continue }

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
		sendDone()
		return
	}

	// Groups
	if strings.Contains(lowerBase, "ou=groups,") {
		groups, _ := lf.rc.GetGroups("{}")
		for _, g := range groups {
			dn := fmt.Sprintf("cn=%s,ou=groups,%s", g.Id, lf.baseDN)
			attrs := map[string]string{
				"cn":          g.Id,
				"objectclass": "groupOfNames top",
			}
			if !shouldSend(dn, attrs) { continue }

			e := ldap.NewSearchResultEntry(dn)
			e.AddAttribute(lmsg.AttributeDescription("objectClass"), lmsg.AttributeValue("groupOfNames"))
			e.AddAttribute(lmsg.AttributeDescription("objectClass"), lmsg.AttributeValue("top"))
			e.AddAttribute(lmsg.AttributeDescription("cn"), lmsg.AttributeValue(g.Id))

			// emit members (accounts) if discoverable
			if ids := lf.groupAccountIDs(g.Id); len(ids) > 0 {
				for _, uid := range ids {
					memDN := fmt.Sprintf("uid=%s,ou=people,%s", uid, lf.baseDN)
					e.AddAttribute(lmsg.AttributeDescription("member"), lmsg.AttributeValue(memDN))
				}
			}
			send(e)
		}
		sendDone()
		return
	}

	// Roles
	if strings.Contains(lowerBase, "ou=roles,") {
		roles, _ := lf.rc.GetRoles("{}")
		for _, r0 := range roles {
			dn := fmt.Sprintf("cn=%s,ou=roles,%s", r0.Id, lf.baseDN)
			attrs := map[string]string{
				"cn":          r0.Id,
				"objectclass": "globularRole top",
			}
			if !shouldSend(dn, attrs) { continue }

			e := ldap.NewSearchResultEntry(dn)
			e.AddAttribute(lmsg.AttributeDescription("objectClass"), lmsg.AttributeValue("top"))
			e.AddAttribute(lmsg.AttributeDescription("objectClass"), lmsg.AttributeValue("globularRole"))
			e.AddAttribute(lmsg.AttributeDescription("cn"), lmsg.AttributeValue(r0.Id))
			if len(r0.Actions) > 0 {
				for _, action := range vals(r0.Actions...) {
					e.AddAttribute(lmsg.AttributeDescription("globularAction"), action)
				}
			}
			// members (accounts) if discoverable
			if ids := lf.roleAccountIDs(r0.Id); len(ids) > 0 {
				for _, uid := range ids {
					memDN := fmt.Sprintf("uid=%s,ou=people,%s", uid, lf.baseDN)
					e.AddAttribute(lmsg.AttributeDescription("member"), lmsg.AttributeValue(memDN))
				}
			}
			send(e)
		}
		sendDone()
		return
	}

	// Orgs
	if strings.Contains(lowerBase, "ou=orgs,") {
		orgs, _ := lf.rc.GetOrganizations("{}")
		for _, o := range orgs {
			dn := fmt.Sprintf("o=%s,ou=orgs,%s", o.Id, lf.baseDN)
			attrs := map[string]string{
				"o":           o.Id,
				"objectclass": "organization top",
			}
			if !shouldSend(dn, attrs) { continue }

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

			// members (accounts, groups, roles) if discoverable
			if accs := lf.orgAccountIDs(o.Id); len(accs) > 0 {
				for _, uid := range accs {
					memDN := fmt.Sprintf("uid=%s,ou=people,%s", uid, lf.baseDN)
					e.AddAttribute(lmsg.AttributeDescription("member"), lmsg.AttributeValue(memDN))
				}
			}
			if grps := lf.orgGroupIDs(o.Id); len(grps) > 0 {
				for _, gid := range grps {
					memDN := fmt.Sprintf("cn=%s,ou=groups,%s", gid, lf.baseDN)
					e.AddAttribute(lmsg.AttributeDescription("member"), lmsg.AttributeValue(memDN))
				}
			}
			if roles := lf.orgRoleIDs(o.Id); len(roles) > 0 {
				for _, rid := range roles {
					memDN := fmt.Sprintf("cn=%s,ou=roles,%s", rid, lf.baseDN)
					e.AddAttribute(lmsg.AttributeDescription("uniqueMember"), lmsg.AttributeValue(memDN))
					e.AddAttribute(lmsg.AttributeDescription("member"), lmsg.AttributeValue(memDN))
				}
			}
			send(e)
		}
		sendDone()
		return
	}

	// default
	sendDone()
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
		if err == nil {
			// Initial members: accept both "member" and "uniqueMember"
			members := append([]string{}, attrs["member"]...)
			members = append(members, attrs["uniquemember"]...)
			for _, mem := range members {
				kindRef, refID := lf.parseDN(mem) // user/group/role
				if refID == "" {
					continue
				}
				if e := lf.addOrgMember(cs.token, id, kindRef, refID); e != nil {
					log.Printf("LDAP ORG add initial member %s (%s/%s): %v", id, kindRef, refID, e)
				}
			}
		}
		writeAddResult(w, err)
	}
}

// Route organization membership ops to the right resource_client method
// based on the DN kind parsed from member values (user/group/role).
func (lf *ldapFacade) addOrgMember(token, orgID, kind, refID string) error {
	switch kind {
	case "user":
		return lf.rc.AddOrganizationAccount(token, orgID, refID)
	case "group":
		return lf.rc.AddOrganizationGroup(token, orgID, refID)
	case "role":
		return lf.rc.AddOrganizationRole(token, orgID, refID)
	default:
		return nil
	}
}

func (lf *ldapFacade) removeOrgMember(token, orgID, kind, refID string) error {
	switch kind {
	case "user":
		return lf.rc.RemoveOrganizationAccount(token, orgID, refID)
	case "group":
		return lf.rc.RemoveOrganizationGroup(token, orgID, refID)
	case "role":
		return lf.rc.RemoveOrganizationRole(token, orgID, refID)
	default:
		return nil
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
			t := strings.ToLower(string(ch.Modification().Type_()))
			switch t {
			case "globularaction":
				if ch.Operation() == lmsg.ModifyRequestChangeOperationAdd {
					_ = lf.rc.AddRoleActions(id, toStrings(ch.Modification().Vals()))
				} else if ch.Operation() == lmsg.ModifyRequestChangeOperationDelete {
					for _, a := range toStrings(ch.Modification().Vals()) {
						_ = lf.rc.RemoveRoleAction(id, a)
					}
				}
			case "member":
				// Expect member values as user DNs like uid=<user>,ou=people,<baseDN>
				for _, v := range toStrings(ch.Modification().Vals()) {
					_, uid := lf.parseDN(v)
					if uid == "" {
						continue
					}
					if ch.Operation() == lmsg.ModifyRequestChangeOperationAdd {
						// TODO: replace with your actual resource client method
						_ = lf.rc.AddAccountRole(cs.token, uid, id)
					} else if ch.Operation() == lmsg.ModifyRequestChangeOperationDelete {
						// TODO: replace with your actual resource client method
						_ = lf.rc.RemoveAccountRole(cs.token, uid, id)
					}
				}
			}
		}
		w.Write(ldap.NewModifyResponse(ldap.LDAPResultSuccess))
	case "org":
		for _, ch := range r.Changes() {
			t := strings.ToLower(string(ch.Modification().Type_()))
			switch t {
			// Accept both "member" and "uniqueMember" for compatibility
			case "member", "uniquemember":
				for _, v := range toStrings(ch.Modification().Vals()) {
					kindRef, refID := lf.parseDN(v) // user / group / role by OU in the DN
					if refID == "" {
						continue
					}
					switch ch.Operation() {
					case lmsg.ModifyRequestChangeOperationAdd:
						if err := lf.addOrgMember(cs.token, id, kindRef, refID); err != nil {
							log.Printf("LDAP ORG add member %s (%s/%s): %v", id, kindRef, refID, err)
						}
					case lmsg.ModifyRequestChangeOperationDelete:
						if err := lf.removeOrgMember(cs.token, id, kindRef, refID); err != nil {
							log.Printf("LDAP ORG remove member %s (%s/%s): %v", id, kindRef, refID, err)
						}
					}
				}
				// (optional) allow org profile tweaks later (description/mail)
			}
		}
		w.Write(ldap.NewModifyResponse(ldap.LDAPResultSuccess))
	}
}

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
	if err != nil {
		log.Printf("LDAP ADD error: %v", err)
		w.Write(ldap.NewAddResponse(ldap.LDAPResultOther))
		return
	}
	w.Write(ldap.NewAddResponse(ldap.LDAPResultSuccess))
}

func writeModifyResult(w ldap.ResponseWriter, err error) {
	if err != nil {
		log.Printf("LDAP MODIFY error: %v", err)
		w.Write(ldap.NewModifyResponse(ldap.LDAPResultOther))
		return
	}
	w.Write(ldap.NewModifyResponse(ldap.LDAPResultSuccess))
}

func writeDeleteResult(w ldap.ResponseWriter, err error) {
	if err != nil {
		log.Printf("LDAP DELETE error: %v", err)
		w.Write(ldap.NewDeleteResponse(ldap.LDAPResultOther))
		return
	}
	w.Write(ldap.NewDeleteResponse(ldap.LDAPResultSuccess))
}

// ---- OPTIONAL: health check -------------------------------------------------

// PingLDAPFacade can be used to check that at least the server goroutines are running.
func (s *server) PingLDAPFacade() string {
	return time.Now().Format(time.RFC3339Nano)
}
