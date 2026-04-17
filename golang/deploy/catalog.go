// Package deploy implements the build/publish workflow for Globular services.
package deploy

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ServiceEntry describes a single service in the catalog.
type ServiceEntry struct {
	Name         string   `yaml:"-"`
	Profiles     []string `yaml:"profiles,omitempty"`
	Priority     int      `yaml:"priority,omitempty"`
	Tier         int      `yaml:"tier,omitempty"`
	Dependencies []string `yaml:"dependencies,omitempty"`
	NeedsScylla  bool     `yaml:"needs_scylla,omitempty"`
	Capabilities []string `yaml:"capabilities,omitempty"`
	RunAsRoot    bool     `yaml:"run_as_root,omitempty"`
	ExtraPath    bool     `yaml:"extra_path,omitempty"`
}

// Catalog holds all service entries.
type Catalog struct {
	Services map[string]*ServiceEntry `yaml:"services"`
}

// LoadCatalog reads and parses the service catalog YAML file.
func LoadCatalog(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read catalog: %w", err)
	}
	var cat Catalog
	if err := yaml.Unmarshal(data, &cat); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	for name, entry := range cat.Services {
		entry.Name = name
		applyDefaults(entry)
	}
	return &cat, nil
}

// Get returns a service entry by name, or an error if not found.
func (c *Catalog) Get(name string) (*ServiceEntry, error) {
	// Try exact match first.
	if e, ok := c.Services[name]; ok {
		return e, nil
	}
	// Try with underscores replaced by dashes and vice versa.
	alt := strings.ReplaceAll(name, "-", "_")
	if e, ok := c.Services[alt]; ok {
		return e, nil
	}
	return nil, fmt.Errorf("service %q not found in catalog", name)
}

// ServiceNames returns all service names sorted alphabetically.
func (c *Catalog) ServiceNames() []string {
	names := make([]string, 0, len(c.Services))
	for name := range c.Services {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// applyDefaults fills in implicit rules for a service entry.
func applyDefaults(e *ServiceEntry) {
	if len(e.Profiles) == 0 {
		e.Profiles = []string{"core", "compute"}
	}
	if e.Priority == 0 {
		e.Priority = 1000
	}
	// Implicit event dependency for non-tier-0 services.
	if e.Tier > 0 || (e.Name != "event" && e.Name != "persistence" && e.Tier == 0) {
		hasEvent := false
		for _, d := range e.Dependencies {
			if d == "event" {
				hasEvent = true
				break
			}
		}
		if !hasEvent && e.Name != "event" && e.Name != "persistence" {
			e.Dependencies = append([]string{"event"}, e.Dependencies...)
		}
	}
}

// PackageName returns the canonical package name with hyphens (e.g., "ai_executor" → "ai-executor").
// The repository, desired-state, and installed-state all use hyphenated names.
func (e *ServiceEntry) PackageName() string {
	return strings.ReplaceAll(e.Name, "_", "-")
}

// ExecName returns the binary name for a service (e.g., "echo" → "echo_server").
func (e *ServiceEntry) ExecName() string {
	return e.Name + "_server"
}

// SystemdUnit returns the systemd unit name (e.g., "echo" → "globular-echo.service").
func (e *ServiceEntry) SystemdUnit() string {
	return "globular-" + e.PackageName() + ".service"
}

// User returns the system user for the service.
func (e *ServiceEntry) User() string {
	if e.RunAsRoot {
		return "root"
	}
	return "globular"
}

// Group returns the system group for the service.
func (e *ServiceEntry) Group() string {
	if e.RunAsRoot {
		return "root"
	}
	return "globular"
}

// SystemdDeps returns the list of systemd unit dependencies.
func (e *ServiceEntry) SystemdDeps() []string {
	var units []string
	for _, d := range e.Dependencies {
		units = append(units, "globular-"+strings.ReplaceAll(d, "_", "-")+".service")
	}
	return units
}
