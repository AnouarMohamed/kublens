package state

import "time"

type ResourceQuantities struct {
	CPUMilli    int64 `json:"cpuMilli"`
	MemoryBytes int64 `json:"memoryBytes"`
}

type UsagePoint struct {
	Timestamp time.Time
	Usage     ResourceQuantities
}

type ConditionInfo struct {
	Type               string
	Status             string
	Reason             string
	Message            string
	LastTransitionTime time.Time
}

type ContainerInfo struct {
	Name                 string
	Image                string
	Ready                bool
	State                string
	RestartCount         int32
	WaitingReason        string
	TerminatedReason     string
	TerminatedExitCode   int32
	TerminatedFinishedAt time.Time
	ResourceRequests     ResourceQuantities
	ResourceLimits       ResourceQuantities
}

type PodInfo struct {
	UID               string
	Name              string
	Namespace         string
	Labels            map[string]string
	Phase             string
	StatusReason      string
	StatusMessage     string
	NodeName          string
	NodeSelector      map[string]string
	Tolerations       []TolerationInfo
	StartTime         time.Time
	DeletionTimestamp *time.Time
	Restarts          int32
	RecentRestarts    int
	Conditions        []ConditionInfo
	Containers        []ContainerInfo
	QoSClass          string
	ResourceRequests  ResourceQuantities
	ResourceLimits    ResourceQuantities
	Usage             ResourceQuantities
	UsageHistory      []UsagePoint
}

type NodeInfo struct {
	UID           string
	Name          string
	Status        string
	Roles         []string
	Unschedulable bool
	Version       string
	CreatedAt     time.Time
	Conditions    []ConditionInfo
	Capacity      ResourceQuantities
	Allocatable   ResourceQuantities
	Usage         ResourceQuantities
	UsageHistory  []UsagePoint
	Labels        map[string]string
	Taints        []string
}

type TolerationInfo struct {
	Key      string
	Value    string
	Effect   string
	Operator string
}

type DeploymentInfo struct {
	UID               string
	Name              string
	Namespace         string
	DesiredReplicas   int32
	ReadyReplicas     int32
	UpdatedReplicas   int32
	AvailableReplicas int32
	Strategy          string
	Conditions        []ConditionInfo
	CreatedAt         time.Time
}

type ServicePortInfo struct {
	Name       string
	Port       int32
	Protocol   string
	TargetPort string
}

type ServiceInfo struct {
	UID       string
	Name      string
	Namespace string
	Type      string
	ClusterIP string
	Selector  map[string]string
	Ports     []ServicePortInfo
}

type EndpointSliceInfo struct {
	UID         string
	Name        string
	Namespace   string
	ServiceName string
	Addresses   []string
	PodTargets  []string
}

type IngressBackendInfo struct {
	ServiceName string
	ServicePort string
}

type IngressInfo struct {
	UID       string
	Name      string
	Namespace string
	Hosts     []string
	Backends  []IngressBackendInfo
}

type EventInfo struct {
	Type               string
	Reason             string
	Message            string
	Namespace          string
	InvolvedObjectKind string
	InvolvedObjectName string
	Count              int32
	FirstTimestamp     time.Time
	LastTimestamp      time.Time
	Source             string
}

type ClusterState struct {
	Pods           map[string]PodInfo
	Nodes          map[string]NodeInfo
	Deployments    map[string]DeploymentInfo
	Services       map[string]ServiceInfo
	EndpointSlices map[string]EndpointSliceInfo
	Ingresses      map[string]IngressInfo
	Events         []EventInfo
	LastUpdated    time.Time
}

func (p PodInfo) clone() PodInfo {
	out := p
	if p.DeletionTimestamp != nil {
		deletionTimestamp := *p.DeletionTimestamp
		out.DeletionTimestamp = &deletionTimestamp
	}
	out.Conditions = append([]ConditionInfo(nil), p.Conditions...)
	out.Containers = append([]ContainerInfo(nil), p.Containers...)
	out.UsageHistory = append([]UsagePoint(nil), p.UsageHistory...)
	out.Labels = cloneStringMap(p.Labels)
	out.NodeSelector = cloneStringMap(p.NodeSelector)
	out.Tolerations = append([]TolerationInfo(nil), p.Tolerations...)
	return out
}

func (n NodeInfo) clone() NodeInfo {
	out := n
	out.Conditions = append([]ConditionInfo(nil), n.Conditions...)
	out.UsageHistory = append([]UsagePoint(nil), n.UsageHistory...)
	if n.Labels != nil {
		out.Labels = make(map[string]string, len(n.Labels))
		for k, v := range n.Labels {
			out.Labels[k] = v
		}
	}
	out.Taints = append([]string(nil), n.Taints...)
	return out
}

func (s ServiceInfo) clone() ServiceInfo {
	out := s
	out.Selector = cloneStringMap(s.Selector)
	out.Ports = append([]ServicePortInfo(nil), s.Ports...)
	return out
}

func (e EndpointSliceInfo) clone() EndpointSliceInfo {
	out := e
	out.Addresses = append([]string(nil), e.Addresses...)
	out.PodTargets = append([]string(nil), e.PodTargets...)
	return out
}

func (i IngressInfo) clone() IngressInfo {
	out := i
	out.Hosts = append([]string(nil), i.Hosts...)
	out.Backends = append([]IngressBackendInfo(nil), i.Backends...)
	return out
}

func (d DeploymentInfo) clone() DeploymentInfo {
	out := d
	out.Conditions = append([]ConditionInfo(nil), d.Conditions...)
	return out
}
