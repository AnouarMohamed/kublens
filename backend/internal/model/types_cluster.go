package model

// PodStatus represents a Kubernetes pod phase normalized for frontend use.
type PodStatus string

const (
	PodStatusRunning   PodStatus = "Running"
	PodStatusPending   PodStatus = "Pending"
	PodStatusFailed    PodStatus = "Failed"
	PodStatusSucceeded PodStatus = "Succeeded"
	PodStatusUnknown   PodStatus = "Unknown"
)

// NodeStatus represents a Kubernetes node readiness state.
type NodeStatus string

const (
	NodeStatusReady    NodeStatus = "Ready"
	NodeStatusNotReady NodeStatus = "NotReady"
	NodeStatusUnknown  NodeStatus = "Unknown"
)

type PodSummary struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	NodeName  string    `json:"nodeName,omitempty"`
	Status    PodStatus `json:"status"`
	CPU       string    `json:"cpu"`
	Memory    string    `json:"memory"`
	Age       string    `json:"age"`
	Restarts  int32     `json:"restarts"`
}

type ContainerEnv struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

type VolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
}

type ResourcePairs struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

type ContainerResources struct {
	Requests *ResourcePairs `json:"requests,omitempty"`
	Limits   *ResourcePairs `json:"limits,omitempty"`
}

type ContainerSpec struct {
	Name         string              `json:"name"`
	Image        string              `json:"image,omitempty"`
	Env          []ContainerEnv      `json:"env,omitempty"`
	VolumeMounts []VolumeMount       `json:"volumeMounts,omitempty"`
	Resources    *ContainerResources `json:"resources,omitempty"`
}

type NamedVolume struct {
	Name string `json:"name"`
}

type PodDetail struct {
	PodSummary
	Containers []ContainerSpec `json:"containers"`
	Volumes    []NamedVolume   `json:"volumes,omitempty"`
	NodeName   string          `json:"nodeName,omitempty"`
	HostIP     string          `json:"hostIP,omitempty"`
	PodIP      string          `json:"podIP,omitempty"`
}

type CPUPoint struct {
	Time  string `json:"time"`
	Value int    `json:"value"`
}

type NodeSummary struct {
	Name          string     `json:"name"`
	Status        NodeStatus `json:"status"`
	Roles         string     `json:"roles"`
	Unschedulable bool       `json:"unschedulable,omitempty"`
	Age           string     `json:"age"`
	Version       string     `json:"version"`
	CPUUsage      string     `json:"cpuUsage"`
	MemUsage      string     `json:"memUsage"`
	CPUHistory    []CPUPoint `json:"cpuHistory,omitempty"`
}

type ResourceCapacity struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	Pods   string `json:"pods"`
}

type NodeCondition struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	LastTransitionTime string `json:"lastTransitionTime"`
	Reason             string `json:"reason"`
	Message            string `json:"message"`
}

type NodeAddress struct {
	Type    string `json:"type"`
	Address string `json:"address"`
}

type NodeDetail struct {
	NodeSummary
	Capacity    ResourceCapacity `json:"capacity"`
	Allocatable ResourceCapacity `json:"allocatable"`
	Conditions  []NodeCondition  `json:"conditions"`
	Addresses   []NodeAddress    `json:"addresses"`
}

type NodeDrainPod struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Reason    string `json:"reason,omitempty"`
}

type NodeDrainBlocker struct {
	Kind      string       `json:"kind"`
	Message   string       `json:"message"`
	Pod       NodeDrainPod `json:"pod"`
	Reference string       `json:"reference,omitempty"`
}

type NodeDrainPreview struct {
	Node        string             `json:"node"`
	Evictable   []NodeDrainPod     `json:"evictable"`
	Skipped     []NodeDrainPod     `json:"skipped"`
	Blockers    []NodeDrainBlocker `json:"blockers"`
	SafeToDrain bool               `json:"safeToDrain"`
	GeneratedAt string             `json:"generatedAt"`
}

type NodeDrainRequest struct {
	Force  bool   `json:"force"`
	Reason string `json:"reason,omitempty"`
}

type K8sEvent struct {
	Type          string `json:"type"`
	Reason        string `json:"reason"`
	Age           string `json:"age"`
	From          string `json:"from"`
	Message       string `json:"message"`
	Namespace     string `json:"namespace,omitempty"`
	Resource      string `json:"resource,omitempty"`
	ResourceKind  string `json:"resourceKind,omitempty"`
	Count         int32  `json:"count,omitempty"`
	LastTimestamp string `json:"lastTimestamp,omitempty"`
}

type PodStats struct {
	Total   int `json:"total"`
	Running int `json:"running"`
	Pending int `json:"pending"`
	Failed  int `json:"failed"`
}

type NodeStats struct {
	Total    int `json:"total"`
	Ready    int `json:"ready"`
	NotReady int `json:"notReady"`
}

type ClusterCapacity struct {
	CPU     string `json:"cpu"`
	Memory  string `json:"memory"`
	Storage string `json:"storage"`
}

type ClusterStats struct {
	Pods    PodStats        `json:"pods"`
	Nodes   NodeStats       `json:"nodes"`
	Cluster ClusterCapacity `json:"cluster"`
}

type ResourceRecord struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Status    string `json:"status"`
	Age       string `json:"age"`
	Summary   string `json:"summary,omitempty"`
}

type ResourceList struct {
	Kind  string           `json:"kind"`
	Items []ResourceRecord `json:"items"`
}

type PodCreateRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Image     string `json:"image"`
}

type ScaleRequest struct {
	Replicas int32 `json:"replicas"`
}

type ResourceManifest struct {
	YAML string `json:"yaml"`
}

type ActionResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type ClusterContext struct {
	Name          string `json:"name"`
	IsRealCluster bool   `json:"isRealCluster"`
}

type ClusterContextList struct {
	Selected string           `json:"selected"`
	Items    []ClusterContext `json:"items"`
}

type ClusterSelectRequest struct {
	Name string `json:"name"`
}

type ClusterSelectResponse struct {
	Selected string `json:"selected"`
}
