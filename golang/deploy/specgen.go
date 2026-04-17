package deploy

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

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
          WorkingDirectory={{.StateDir}}/<<% .Name %>>
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
          Type=notify
          WatchdogSec=60
          Restart=on-failure
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
	return buf.String(), nil
}
