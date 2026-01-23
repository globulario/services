package clustercontrollerpb

// NOTE: This file provides minimal resource types to enable the new resource
// store and RPC scaffolding. It intentionally avoids full generated code while
// keeping field shapes compatible with future proto generation.

type ObjectMeta struct {
	Name            string            `json:"name,omitempty"`
	ResourceVersion string            `json:"resource_version,omitempty"`
	Generation      int64             `json:"generation,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Annotations     map[string]string `json:"annotations,omitempty"`
}

type ObjectStatus struct {
	ObservedGeneration int64 `json:"observed_generation,omitempty"`
}

type ClusterNetwork struct {
	Meta   *ObjectMeta         `json:"meta,omitempty"`
	Spec   *ClusterNetworkSpec `json:"spec,omitempty"`
	Status *ObjectStatus       `json:"status,omitempty"`
}

type ServiceDesiredVersionSpec struct {
	ServiceName string `json:"service_name,omitempty"`
	Version     string `json:"version,omitempty"`
}

type ServiceDesiredVersion struct {
	Meta   *ObjectMeta                `json:"meta,omitempty"`
	Spec   *ServiceDesiredVersionSpec `json:"spec,omitempty"`
	Status *ObjectStatus              `json:"status,omitempty"`
}

type NodeSpec struct {
	Labels map[string]string `json:"labels,omitempty"`
	Roles  []string          `json:"roles,omitempty"`
}

type Node struct {
	Meta   *ObjectMeta   `json:"meta,omitempty"`
	Spec   *NodeSpec     `json:"spec,omitempty"`
	Status *ObjectStatus `json:"status,omitempty"`
}

type WatchEvent struct {
	EventType             string                 `json:"event_type,omitempty"`
	ResourceVersion       string                 `json:"resource_version,omitempty"`
	ClusterNetwork        *ClusterNetwork        `json:"cluster_network,omitempty"`
	ServiceDesiredVersion *ServiceDesiredVersion `json:"service_desired_version,omitempty"`
}
