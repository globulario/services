package deploy

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"unicode"
)

// singletonServiceDirectives is the closed set of systemd [Service] directives
// where duplicate occurrences silently take last-wins. Shipping a unit with
// two `Type=` lines (the exact bug `acdcb436` shipped a hotfix for) made
// systemd accept the second value and Globular missed the drift because no
// component was diffing rendered-vs-effective unit configuration.
//
// Phase 5 of the Diagnostic Honesty Refactor: catch the duplicate at gen
// time so the spec build fails fast instead of producing a unit that
// behaves correctly today and surprises us tomorrow.
//
// This list is conservative — it covers the directives systemd treats as
// singletons in practice. Multi-OK directives (ExecStartPre, Environment,
// After, Wants, etc.) are deliberately absent so legitimate repetition
// stays legitimate.
var singletonServiceDirectives = map[string]struct{}{
	"Type":             {},
	"User":             {},
	"Group":            {},
	"WorkingDirectory": {},
	"Restart":          {},
	"RestartSec":       {},
	"TimeoutStartSec":  {},
	"TimeoutStopSec":   {},
	"KillMode":         {},
	"KillSignal":       {},
	"PIDFile":          {},
	"RootDirectory":    {},
	"RuntimeDirectory": {},
	"NotifyAccess":     {},
}

// isLikelyDirective heuristically decides whether a `Name=value` line is a
// systemd directive (CamelCase identifier left of `=`) versus an embedded
// shell expression, an environment assignment, or a `protoc Type=...`
// argument that happens to share the syntax. Returns true only when `name`
// is a non-empty alphanumeric identifier starting with a capital letter,
// which matches every directive in the systemd manual.
func isLikelyDirective(name string) bool {
	if name == "" {
		return false
	}
	if !unicode.IsUpper(rune(name[0])) {
		return false
	}
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// ValidateSystemdUnit scans a rendered unit (or a chunk of text containing
// one — e.g. a YAML spec with the unit embedded under `content: |`) for
// duplicate singleton directives in the [Service] section. Returns nil
// when the unit is well-formed.
//
// Detection algorithm: track the current section header (`[Service]`,
// `[Unit]`, `[Install]`), accumulate occurrence counts of singleton
// directives in [Service], and fail the first time any count exceeds 1.
//
// Phase 5 finding equivalent: systemd.unit_duplicate_directive (critical).
func ValidateSystemdUnit(content string) error {
	section := ""
	counts := make(map[string]int)
	for lineNo, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		// Section header — e.g. "[Service]". Reset isn't required because
		// each (section, directive) pair is its own counter key.
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line[1 : len(line)-1]
			continue
		}
		// Lines start at section==""; ignore until we enter a section.
		if section != "Service" {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		name := line[:eq]
		if !isLikelyDirective(name) {
			continue
		}
		if _, ok := singletonServiceDirectives[name]; !ok {
			continue
		}
		counts[name]++
		if counts[name] > 1 {
			return fmt.Errorf(
				"line %d: duplicate singleton directive %s= in [%s] — systemd silently uses the last value, masking the earlier one (finding: systemd.unit_duplicate_directive)",
				lineNo+1, name, section)
		}
	}
	return nil
}

// ValidateSpec rejects any spec or systemd unit content that contains a
// fragile WorkingDirectory line pointing at the Globular state dir without
// the '-' optional prefix.  A missing state dir causes systemd to abort with
// status=200/CHDIR before ExecStartPre can create the directory.
//
// Accepted:   WorkingDirectory=-/var/lib/globular/<service>
// Accepted:   WorkingDirectory=-{{.StateDir}}/<service>
// Rejected:   WorkingDirectory=/var/lib/globular/<service>
// Rejected:   WorkingDirectory={{.StateDir}}/<service>
func ValidateSpec(content string) error {
	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "WorkingDirectory=") {
			continue
		}
		val := strings.TrimPrefix(trimmed, "WorkingDirectory=")
		// Reject any path under the Globular state dir without the '-' prefix.
		if (strings.HasPrefix(val, "/var/lib/globular/") ||
			strings.HasPrefix(val, "{{.StateDir}}/")) {
			return fmt.Errorf("line %d: fragile WorkingDirectory=%q — must use '-' prefix (WorkingDirectory=-%s) to make the state dir optional; "+
				"without it systemd aborts with status=200/CHDIR when the directory is missing", i+1, val, val)
		}
	}
	return nil
}

var funcMap = template.FuncMap{
	"joinComma": func(s []string) string { return strings.Join(s, ", ") },
	"joinSpace": func(s []string) string { return strings.Join(s, " ") },
}

// specTemplate uses <<% %>> delimiters so that {{ }} passes through literally
// into the generated YAML (those are node-agent install-time template vars).
var specTemplate = template.Must(
	template.New("spec").Funcs(funcMap).Delims("<<%", "%>>").Parse(specTemplateText),
)

const specTemplateText = `version: 1

metadata:
  name: <<% .Name %>>
  profiles: [<<% joinComma .Profiles %>>]
  priority: <<% .Priority %>>

service:
  name: <<% .Name %>>
  exec: <<% .ExecName %>>

steps:
  - id: ensure-user-group
    type: ensure_user_group
    user: globular
    group: globular
    home: "{{.StateDir}}"
    shell: /usr/sbin/nologin
    system: true

  - id: ensure-dirs
    type: ensure_dirs
    dirs:
      - path: "{{.Prefix}}"
        owner: root
        group: root
        mode: 0755
      - path: "{{.Prefix}}/bin"
        owner: root
        group: root
        mode: 0755

      - path: "{{.StateDir}}"
        owner: globular
        group: globular
        mode: 0750
      - path: "{{.StateDir}}/services"
        owner: globular
        group: globular
        mode: 0750
      - path: "{{.StateDir}}/<<% .Name %>>"
        owner: globular
        group: globular
        mode: 0750

      # TLS directories (certificates must be provisioned by Day-0 bootstrap)
      - path: "{{.StateDir}}/pki"
        owner: globular
        group: globular
        mode: 0750
      # INV-PKI-1: Removed obsolete config/tls directory - all certs under pki/

  - id: install-<<% .Name %>>-payload
    type: install_package_payload
    install_bins: true
    install_config: false
    install_spec: false
    install_systemd: false

  - id: ensure-<<% .Name %>>-config
    type: ensure_service_config
    service_name: <<% .Name %>>
    exec: <<% .ExecName %>>
    address_host: auto
    rewrite_if_out_of_range: true

  - id: install-<<% .Name %>>-service
    type: install_services
    units:
      - name: <<% .SystemdUnit %>>
        owner: root
        group: root
        mode: 0644
        atomic: true
        content: |
          [Unit]
          Description=Globular <<% .Name %>>
          After=<<% joinSpace .AfterDeps %>>
          Wants=<<% joinSpace .WantsDeps %>>

          [Service]
          Type=simple
          User=<<% .User %>>
          Group=<<% .Group %>>
          WorkingDirectory=-{{.StateDir}}/<<% .Name %>>
          Environment=GLOBULAR_SERVICES_DIR={{.StateDir}}/services

          ExecStartPre=+/bin/sh -c 'mkdir -p {{.StateDir}}/<<% .Name %>> && chown <<% .User %>>:<<% .Group %>> {{.StateDir}}/<<% .Name %>>'
          ExecStartPre=/bin/sh -c 'for i in $(seq 1 60); do [ -f /var/lib/globular/pki/issued/services/service.crt ] && exit 0; sleep 1; done; echo "TLS cert not ready after 60s"; ls -la /var/lib/globular/pki/issued/services/ 2>&1 || echo "cert dir missing"; exit 1'
<<%- if .NeedsScylla %>>
          ExecStartPre=/bin/sh -c 'for i in $(seq 1 90); do ss -lnt | grep -q ":9042 " && exit 0; sleep 1; done; echo "scylla 9042 not ready after 90s"; ss -lnt 2>&1; systemctl status scylla-server.service --no-pager -l 2>&1 | tail -5; exit 1'
<<%- end %>>
<<%- if .HasCapNetBind %>>
          AmbientCapabilities=CAP_NET_BIND_SERVICE
<<%- end %>>
<<%- if .ExtraPath %>>
          Environment=PATH={{.Prefix}}/bin:/usr/local/bin:/usr/bin:/bin
<<%- end %>>
          ExecStart={{.Prefix}}/bin/<<% .ExecName %>>
          Restart=always
          RestartSec=2
          LimitNOFILE=524288

          [Install]
          WantedBy=multi-user.target

  - id: enable-<<% .Name %>>
    type: enable_services
    services:
      - <<% .SystemdUnit %>>

  - id: start-<<% .Name %>>
    type: start_services
    services:
      - <<% .SystemdUnit %>>
    binaries:
      <<% .SystemdUnit %>>: <<% .ExecName %>>

  - id: health-<<% .Name %>>
    type: health_checks
    services:
      - <<% .SystemdUnit %>>
`

// specData holds all values needed to render the spec template.
type specData struct {
	Name          string
	Profiles      []string
	Priority      int
	ExecName      string
	SystemdUnit   string
	User          string
	Group         string
	AfterDeps     []string
	WantsDeps     []string
	NeedsScylla   bool
	HasCapNetBind bool
	ExtraPath     bool
}

// GenerateSpec produces a spec YAML string for the given service entry.
func GenerateSpec(entry *ServiceEntry) (string, error) {
	afterDeps := []string{"network-online.target"}
	wantsDeps := []string{"network-online.target"}

	sysDeps := entry.SystemdDeps()
	afterDeps = append(afterDeps, sysDeps...)
	wantsDeps = append(wantsDeps, sysDeps...)

	if entry.NeedsScylla {
		afterDeps = append(afterDeps, "scylla-server.service")
		wantsDeps = append(wantsDeps, "scylla-server.service")
	}

	hasCapNetBind := false
	for _, cap := range entry.Capabilities {
		if cap == "CAP_NET_BIND_SERVICE" {
			hasCapNetBind = true
			break
		}
	}

	data := specData{
		Name:          entry.Name,
		Profiles:      entry.Profiles,
		Priority:      entry.Priority,
		ExecName:      entry.ExecName(),
		SystemdUnit:   entry.SystemdUnit(),
		User:          entry.User(),
		Group:         entry.Group(),
		AfterDeps:     afterDeps,
		WantsDeps:     wantsDeps,
		NeedsScylla:   entry.NeedsScylla,
		HasCapNetBind: hasCapNetBind,
		ExtraPath:     entry.ExtraPath,
	}

	var buf bytes.Buffer
	if err := specTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template for %s: %w", entry.Name, err)
	}
	rendered := buf.String()
	// Phase 5: post-render check. Even though the template literal is
	// validated at init() (see init below), a template value injection
	// could still produce a duplicate directive at runtime. Fail closed.
	if err := ValidateSystemdUnit(rendered); err != nil {
		return "", fmt.Errorf("rendered spec for %s failed systemd unit validation: %w", entry.Name, err)
	}
	return rendered, nil
}

// init enforces the singleton-directive invariant against the spec template
// literal itself. A duplicate Type= or User= line accidentally committed to
// the template would otherwise ship to every node before any deploy command
// rendered it. Crashing the binary at startup is the right blast radius:
// the bug never reaches a service unit on disk.
func init() {
	// Render the literal with a placeholder entry that exercises every
	// optional branch so we validate the maximal output. Values are chosen
	// to be syntactically valid but distinguishable from any real package.
	probe := specData{
		Name:          "specgen-init-probe",
		Profiles:      []string{"core"},
		Priority:      0,
		ExecName:      "probe_server",
		SystemdUnit:   "globular-specgen-init-probe.service",
		User:          "globular",
		Group:         "globular",
		AfterDeps:     []string{"network-online.target"},
		WantsDeps:     []string{"network-online.target"},
		NeedsScylla:   true,
		HasCapNetBind: true,
		ExtraPath:     true,
	}
	var buf bytes.Buffer
	if err := specTemplate.Execute(&buf, probe); err != nil {
		panic(fmt.Sprintf("specgen template self-render failed: %v", err))
	}
	if err := ValidateSystemdUnit(buf.String()); err != nil {
		panic(fmt.Sprintf("specgen template contains a duplicate singleton directive — fix the template before this binary can start: %v", err))
	}
}
