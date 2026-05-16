package main

import (
	"fmt"
	"log"
	"net"

	"github.com/globulario/awareness/runtime"
	"github.com/globulario/services/golang/config"
)

// newLiveBridge builds a RuntimeBridge wired to real cluster gRPC sources.
// Addresses are resolved from etcd via config.GetServiceConfigurationById — never hardcoded.
// Static addresses in runtime_sources.yaml override etcd lookup when present.
// Any source that fails to connect is logged and falls back to noop — the bridge
// never returns an error so callers always get a usable (possibly degraded) bridge.
func newLiveBridge(st *awarenessState) *runtime.RuntimeBridge {
	cfg := loadRuntimeSourcesConfig(st.repoRoot)
	b := runtime.NewBridge(st.nodeID, "")

	tlsCfg := runtime.GrpcSourceConfig{
		CACert:     cfg.CACert,
		ClientCert: cfg.ClientCert,
		ClientKey:  cfg.ClientKey,
		Insecure:   cfg.Insecure,
	}

	// Resolve doctor address: static config overrides etcd lookup.
	doctorAddr := cfg.DoctorAddr
	if doctorAddr == "" {
		doctorAddr = resolveServiceAddr("cluster_doctor.ClusterDoctorService")
	}
	if doctorAddr != "" {
		src, err := runtime.NewGrpcDoctorSource(runtime.GrpcSourceConfig{
			Addr:       doctorAddr,
			CACert:     tlsCfg.CACert,
			ClientCert: tlsCfg.ClientCert,
			ClientKey:  tlsCfg.ClientKey,
			Insecure:   tlsCfg.Insecure,
		})
		if err != nil {
			log.Printf("awareness runtime: doctor source unavailable (%s): %v", doctorAddr, err)
		} else {
			b.Doctor = src
			log.Printf("awareness runtime: doctor source live at %s", doctorAddr)
		}
	}

	// Resolve controller address: used for state and service status.
	controllerAddr := cfg.ControllerAddr
	if controllerAddr == "" {
		controllerAddr = resolveServiceAddr("cluster_controller.ClusterControllerService")
	}
	if controllerAddr != "" {
		stateSrc, err := runtime.NewGrpcStateSource(runtime.GrpcSourceConfig{
			Addr:       controllerAddr,
			CACert:     tlsCfg.CACert,
			ClientCert: tlsCfg.ClientCert,
			ClientKey:  tlsCfg.ClientKey,
			Insecure:   tlsCfg.Insecure,
		})
		if err != nil {
			log.Printf("awareness runtime: state source unavailable (%s): %v", controllerAddr, err)
		} else {
			b.State = stateSrc
			log.Printf("awareness runtime: state source live at %s", controllerAddr)
		}

		svcSrc, err := runtime.NewGrpcServiceStatusSource(runtime.GrpcSourceConfig{
			Addr:       controllerAddr,
			CACert:     tlsCfg.CACert,
			ClientCert: tlsCfg.ClientCert,
			ClientKey:  tlsCfg.ClientKey,
			Insecure:   tlsCfg.Insecure,
		})
		if err != nil {
			log.Printf("awareness runtime: service status source unavailable (%s): %v", controllerAddr, err)
		} else {
			b.Services = svcSrc
			log.Printf("awareness runtime: service status source live at %s", controllerAddr)
		}
	}

	// Resolve workflow address.
	workflowAddr := cfg.WorkflowAddr
	if workflowAddr == "" {
		workflowAddr = resolveServiceAddr("workflow.WorkflowService")
	}
	if workflowAddr != "" {
		wfSrc, err := runtime.NewGrpcWorkflowSource(runtime.GrpcSourceConfig{
			Addr:       workflowAddr,
			CACert:     tlsCfg.CACert,
			ClientCert: tlsCfg.ClientCert,
			ClientKey:  tlsCfg.ClientKey,
			Insecure:   tlsCfg.Insecure,
		})
		if err != nil {
			log.Printf("awareness runtime: workflow source unavailable (%s): %v", workflowAddr, err)
		} else {
			b.Workflows = wfSrc
			log.Printf("awareness runtime: workflow source live at %s", workflowAddr)
		}
	}

	// Prometheus: not a Globular service — use configured address only.
	if cfg.PrometheusAddr != "" {
		promSrc, err := runtime.NewPrometheusMetricsSource(cfg.PrometheusAddr, "")
		if err != nil {
			log.Printf("awareness runtime: prometheus source unavailable (%s): %v", cfg.PrometheusAddr, err)
		} else {
			b.Metrics = promSrc
			log.Printf("awareness runtime: prometheus source live at %s", cfg.PrometheusAddr)
		}
	}

	return b
}

// resolveServiceAddr looks up a service's address from etcd by service ID/name.
// Returns "host:port" on success, empty string if not found or etcd is unavailable.
// Logs errors at debug level — callers fall back to noop on empty return.
func resolveServiceAddr(serviceID string) string {
	svcCfg, err := config.GetServiceConfigurationById(serviceID)
	if err != nil {
		log.Printf("awareness runtime: etcd lookup %q: %v", serviceID, err)
		return ""
	}
	addr, _ := svcCfg["Address"].(string)
	port, _ := toFloat64(svcCfg["Port"])
	if addr == "" && port == 0 {
		log.Printf("awareness runtime: etcd lookup %q: missing Address and Port", serviceID)
		return ""
	}
	// Address from etcd may already be "host:port" (from instance heartbeat).
	// If it parses as host:port, use it directly.
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return addr
	}
	// Address is bare hostname; append port.
	if addr != "" && port != 0 {
		return fmt.Sprintf("%s:%d", addr, int(port))
	}
	log.Printf("awareness runtime: etcd lookup %q: cannot build address (addr=%q port=%v)", serviceID, addr, port)
	return ""
}

func toFloat64(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	}
	return 0, false
}
