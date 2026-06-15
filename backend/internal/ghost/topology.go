package ghost

import (
	"sort"
	"strings"
	"time"

	"kubelens-backend/internal/model"
	"kubelens-backend/internal/state"
)

func BuildTopologyFromState(snapshot state.ClusterState, now time.Time) model.GhostTopology {
	topology := model.GhostTopology{
		GeneratedAt: now.UTC().Format(time.RFC3339),
		Source:      "state-cache",
		Nodes:       make([]model.GhostNode, 0, len(snapshot.Nodes)),
		Pods:        make([]model.GhostPod, 0, len(snapshot.Pods)),
		Services:    make([]model.GhostService, 0, len(snapshot.Services)),
		Ingresses:   make([]model.GhostIngress, 0, len(snapshot.Ingresses)),
		Edges:       []model.GhostGraphEdge{},
	}

	usedByNode := map[string]state.ResourceQuantities{}
	for _, pod := range snapshot.Pods {
		if pod.NodeName != "" {
			usedByNode[pod.NodeName] = add(usedByNode[pod.NodeName], pod.ResourceRequests)
		}
		topology.Pods = append(topology.Pods, ghostPodFromState(pod))
		if pod.NodeName != "" {
			topology.Edges = append(topology.Edges, model.GhostGraphEdge{
				From: "pod:" + podKey(pod.Namespace, pod.Name),
				To:   "node:" + pod.NodeName,
				Kind: "scheduled_on",
			})
		}
	}

	for _, node := range snapshot.Nodes {
		used := usedByNode[node.Name]
		headroom := state.ResourceQuantities{
			CPUMilli:    maxInt64(0, node.Allocatable.CPUMilli-used.CPUMilli),
			MemoryBytes: maxInt64(0, node.Allocatable.MemoryBytes-used.MemoryBytes),
		}
		topology.Nodes = append(topology.Nodes, model.GhostNode{
			Name:          node.Name,
			Status:        node.Status,
			Unschedulable: node.Unschedulable,
			Labels:        cloneMap(node.Labels),
			Taints:        append([]string(nil), node.Taints...),
			Capacity:      ghostResources(node.Capacity),
			Allocatable:   ghostResources(node.Allocatable),
			Used:          ghostResources(used),
			Headroom:      ghostResources(headroom),
		})
	}

	podRefsByService := servicePodRefs(snapshot.Services, snapshot.EndpointSlices, snapshot.Pods)
	for _, svc := range snapshot.Services {
		id := podKey(svc.Namespace, svc.Name)
		refs := append([]string(nil), podRefsByService[id]...)
		sort.Strings(refs)
		topology.Services = append(topology.Services, model.GhostService{
			ID:        id,
			Namespace: svc.Namespace,
			Name:      svc.Name,
			Type:      svc.Type,
			Selector:  cloneMap(svc.Selector),
			PodRefs:   refs,
		})
		for _, podRef := range refs {
			topology.Edges = append(topology.Edges, model.GhostGraphEdge{
				From: "service:" + id,
				To:   "pod:" + podRef,
				Kind: "selects",
			})
		}
	}

	for _, ing := range snapshot.Ingresses {
		id := podKey(ing.Namespace, ing.Name)
		serviceRefs := make([]string, 0, len(ing.Backends))
		for _, backend := range ing.Backends {
			ref := podKey(ing.Namespace, backend.ServiceName)
			if backend.ServiceName != "" {
				serviceRefs = append(serviceRefs, ref)
				topology.Edges = append(topology.Edges, model.GhostGraphEdge{
					From: "ingress:" + id,
					To:   "service:" + ref,
					Kind: "routes_to",
				})
			}
		}
		topology.Ingresses = append(topology.Ingresses, model.GhostIngress{
			ID:          id,
			Namespace:   ing.Namespace,
			Name:        ing.Name,
			Hosts:       append([]string(nil), ing.Hosts...),
			ServiceRefs: serviceRefs,
		})
	}

	sortTopology(&topology)
	return topology
}

func BuildTopologyFromSummaries(pods []model.PodSummary, nodes []model.NodeSummary, now time.Time) model.GhostTopology {
	topology := model.GhostTopology{
		GeneratedAt: now.UTC().Format(time.RFC3339),
		Source:      "summary-fallback",
		Nodes:       make([]model.GhostNode, 0, len(nodes)),
		Pods:        make([]model.GhostPod, 0, len(pods)),
		Services:    []model.GhostService{},
		Ingresses:   []model.GhostIngress{},
		Edges:       []model.GhostGraphEdge{},
	}
	for _, node := range nodes {
		topology.Nodes = append(topology.Nodes, model.GhostNode{
			Name:          node.Name,
			Status:        string(node.Status),
			Unschedulable: node.Unschedulable,
			Capacity:      model.GhostResources{CPUMilli: 8000, MemoryBytes: 32 * 1024 * 1024 * 1024},
			Allocatable:   model.GhostResources{CPUMilli: 7800, MemoryBytes: 30 * 1024 * 1024 * 1024},
			Headroom:      model.GhostResources{CPUMilli: 7800, MemoryBytes: 30 * 1024 * 1024 * 1024},
		})
	}
	for _, pod := range pods {
		id := podKey(pod.Namespace, pod.Name)
		topology.Pods = append(topology.Pods, model.GhostPod{
			ID:        id,
			Namespace: pod.Namespace,
			Name:      pod.Name,
			NodeName:  pod.NodeName,
			Status:    string(pod.Status),
			Requests:  model.GhostResources{CPUMilli: 100, MemoryBytes: 128 * 1024 * 1024},
		})
		if pod.NodeName != "" {
			topology.Edges = append(topology.Edges, model.GhostGraphEdge{
				From: "pod:" + id,
				To:   "node:" + pod.NodeName,
				Kind: "scheduled_on",
			})
		}
	}
	sortTopology(&topology)
	return topology
}

func ghostPodFromState(pod state.PodInfo) model.GhostPod {
	tolerations := make([]string, 0, len(pod.Tolerations))
	for _, item := range pod.Tolerations {
		tolerations = append(tolerations, strings.TrimSpace(item.Key+"="+item.Value+":"+item.Effect))
	}
	return model.GhostPod{
		ID:           podKey(pod.Namespace, pod.Name),
		Namespace:    pod.Namespace,
		Name:         pod.Name,
		NodeName:     pod.NodeName,
		Status:       pod.Phase,
		Labels:       cloneMap(pod.Labels),
		NodeSelector: cloneMap(pod.NodeSelector),
		Tolerations:  tolerations,
		Requests:     ghostResources(pod.ResourceRequests),
		Usage:        ghostResources(pod.Usage),
	}
}

func servicePodRefs(
	services map[string]state.ServiceInfo,
	endpointSlices map[string]state.EndpointSliceInfo,
	pods map[string]state.PodInfo,
) map[string][]string {
	out := map[string][]string{}
	for _, slice := range endpointSlices {
		if slice.ServiceName == "" {
			continue
		}
		key := podKey(slice.Namespace, slice.ServiceName)
		out[key] = append(out[key], slice.PodTargets...)
	}
	for _, svc := range services {
		if len(svc.Selector) == 0 {
			continue
		}
		key := podKey(svc.Namespace, svc.Name)
		if len(out[key]) > 0 {
			continue
		}
		for _, pod := range pods {
			if pod.Namespace == svc.Namespace && labelsMatch(svc.Selector, pod.Labels) {
				out[key] = append(out[key], podKey(pod.Namespace, pod.Name))
			}
		}
	}
	return out
}

func labelsMatch(selector map[string]string, labels map[string]string) bool {
	for key, value := range selector {
		if labels[key] != value {
			return false
		}
	}
	return true
}

func sortTopology(topology *model.GhostTopology) {
	sort.Slice(topology.Nodes, func(i, j int) bool { return topology.Nodes[i].Name < topology.Nodes[j].Name })
	sort.Slice(topology.Pods, func(i, j int) bool { return topology.Pods[i].ID < topology.Pods[j].ID })
	sort.Slice(topology.Services, func(i, j int) bool { return topology.Services[i].ID < topology.Services[j].ID })
	sort.Slice(topology.Ingresses, func(i, j int) bool { return topology.Ingresses[i].ID < topology.Ingresses[j].ID })
	sort.Slice(topology.Edges, func(i, j int) bool {
		left := topology.Edges[i].From + topology.Edges[i].To + topology.Edges[i].Kind
		right := topology.Edges[j].From + topology.Edges[j].To + topology.Edges[j].Kind
		return left < right
	})
}

func ghostResources(q state.ResourceQuantities) model.GhostResources {
	return model.GhostResources{CPUMilli: q.CPUMilli, MemoryBytes: q.MemoryBytes}
}

func add(a, b state.ResourceQuantities) state.ResourceQuantities {
	return state.ResourceQuantities{CPUMilli: a.CPUMilli + b.CPUMilli, MemoryBytes: a.MemoryBytes + b.MemoryBytes}
}

func cloneMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func podKey(namespace string, name string) string {
	if namespace == "" {
		return name
	}
	return namespace + "/" + name
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
