package model

type GhostTopology struct {
	GeneratedAt string           `json:"generatedAt"`
	Source      string           `json:"source"`
	Nodes       []GhostNode      `json:"nodes"`
	Pods        []GhostPod       `json:"pods"`
	Services    []GhostService   `json:"services"`
	Ingresses   []GhostIngress   `json:"ingresses"`
	Edges       []GhostGraphEdge `json:"edges"`
}

type GhostNode struct {
	Name          string            `json:"name"`
	Status        string            `json:"status"`
	Unschedulable bool              `json:"unschedulable"`
	Labels        map[string]string `json:"labels,omitempty"`
	Taints        []string          `json:"taints,omitempty"`
	Capacity      GhostResources    `json:"capacity"`
	Allocatable   GhostResources    `json:"allocatable"`
	Used          GhostResources    `json:"used"`
	Headroom      GhostResources    `json:"headroom"`
}

type GhostPod struct {
	ID           string            `json:"id"`
	Namespace    string            `json:"namespace"`
	Name         string            `json:"name"`
	NodeName     string            `json:"nodeName,omitempty"`
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels,omitempty"`
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	Tolerations  []string          `json:"tolerations,omitempty"`
	Requests     GhostResources    `json:"requests"`
	Usage        GhostResources    `json:"usage"`
}

type GhostService struct {
	ID        string            `json:"id"`
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Selector  map[string]string `json:"selector,omitempty"`
	PodRefs   []string          `json:"podRefs"`
}

type GhostIngress struct {
	ID          string   `json:"id"`
	Namespace   string   `json:"namespace"`
	Name        string   `json:"name"`
	Hosts       []string `json:"hosts"`
	ServiceRefs []string `json:"serviceRefs"`
}

type GhostGraphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

type GhostResources struct {
	CPUMilli    int64 `json:"cpuMilli"`
	MemoryBytes int64 `json:"memoryBytes"`
}

type GhostSimulationRequest struct {
	Action         string `json:"action"`
	NodeName       string `json:"nodeName"`
	HorizonSeconds int    `json:"horizonSeconds,omitempty"`
}

type GhostSimulationResult struct {
	ID             string                 `json:"id"`
	Action         string                 `json:"action"`
	GeneratedAt    string                 `json:"generatedAt"`
	HorizonSeconds int                    `json:"horizonSeconds"`
	Verdict        GhostSimulationVerdict `json:"verdict"`
	Frames         []GhostTimelineFrame   `json:"frames"`
}

type GhostSimulationVerdict struct {
	Severity        string   `json:"severity"`
	Summary         string   `json:"summary"`
	Recommendations []string `json:"recommendations"`
}

type GhostTimelineFrame struct {
	OffsetSeconds int                  `json:"offsetSeconds"`
	Nodes         []GhostFrameNode     `json:"nodes"`
	Pods          []GhostFramePod      `json:"pods"`
	Events        []GhostTimelineEvent `json:"events"`
}

type GhostFrameNode struct {
	Name          string         `json:"name"`
	Status        string         `json:"status"`
	Unschedulable bool           `json:"unschedulable"`
	Headroom      GhostResources `json:"headroom"`
}

type GhostFramePod struct {
	ID        string `json:"id"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	NodeName  string `json:"nodeName,omitempty"`
	Status    string `json:"status"`
}

type GhostTimelineEvent struct {
	Kind      string `json:"kind"`
	Severity  string `json:"severity"`
	Resource  string `json:"resource"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp,omitempty"`
}
