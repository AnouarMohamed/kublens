package ghost

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	ghostv1 "kubelens-backend/gen/ghost/v1"
	"kubelens-backend/internal/model"
)

type Client struct {
	addr    string
	timeout time.Duration
}

func NewClient(addr string, timeout time.Duration) *Client {
	return &Client{
		addr:    addr,
		timeout: timeout,
	}
}

func (c *Client) Simulate(ctx context.Context, req model.GhostSimulationRequest, topology model.GhostTopology) (model.GhostSimulationResult, error) {
	dialCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, c.addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return model.GhostSimulationResult{}, err
	}
	defer conn.Close()

	client := ghostv1.NewSimulationServiceClient(conn)

	pbReq := &ghostv1.SimulationRequest{
		Action:         req.Action,
		NodeName:       req.NodeName,
		HorizonSeconds: int32(req.HorizonSeconds),
		Topology:       mapTopologyToPB(topology),
	}

	pbRes, err := client.Simulate(ctx, pbReq)
	if err != nil {
		return model.GhostSimulationResult{}, err
	}

	return mapResultFromPB(pbRes), nil
}

func mapTopologyToPB(topology model.GhostTopology) *ghostv1.TopologySnapshot {
	pbNodes := make([]*ghostv1.Node, len(topology.Nodes))
	for i, n := range topology.Nodes {
		pbNodes[i] = &ghostv1.Node{
			Name:          n.Name,
			Status:        n.Status,
			Unschedulable: n.Unschedulable,
			Labels:        n.Labels,
			Taints:        n.Taints,
			Capacity:      mapResourcesToPB(n.Capacity),
			Allocatable:   mapResourcesToPB(n.Allocatable),
			Used:          mapResourcesToPB(n.Used),
			Headroom:      mapResourcesToPB(n.Headroom),
		}
	}

	pbPods := make([]*ghostv1.Pod, len(topology.Pods))
	for i, p := range topology.Pods {
		pbPods[i] = &ghostv1.Pod{
			Id:           p.ID,
			Namespace:    p.Namespace,
			Name:         p.Name,
			NodeName:     p.NodeName,
			Status:       p.Status,
			Labels:       p.Labels,
			NodeSelector: p.NodeSelector,
			Tolerations:  p.Tolerations,
			Requests:     mapResourcesToPB(p.Requests),
			Usage:        mapResourcesToPB(p.Usage),
		}
	}

	pbServices := make([]*ghostv1.Service, len(topology.Services))
	for i, s := range topology.Services {
		pbServices[i] = &ghostv1.Service{
			Id:        s.ID,
			Namespace: s.Namespace,
			Name:      s.Name,
			Type:      s.Type,
			Selector:  s.Selector,
			PodRefs:   s.PodRefs,
		}
	}

	pbIngresses := make([]*ghostv1.Ingress, len(topology.Ingresses))
	for i, ing := range topology.Ingresses {
		pbIngresses[i] = &ghostv1.Ingress{
			Id:          ing.ID,
			Namespace:   ing.Namespace,
			Name:        ing.Name,
			Hosts:       ing.Hosts,
			ServiceRefs: ing.ServiceRefs,
		}
	}

	pbEdges := make([]*ghostv1.GraphEdge, len(topology.Edges))
	for i, e := range topology.Edges {
		pbEdges[i] = &ghostv1.GraphEdge{
			From: e.From,
			To:   e.To,
			Kind: e.Kind,
		}
	}

	return &ghostv1.TopologySnapshot{
		GeneratedAt: topology.GeneratedAt,
		Nodes:       pbNodes,
		Pods:        pbPods,
		Services:    pbServices,
		Ingresses:   pbIngresses,
		Edges:       pbEdges,
	}
}

func mapResourcesToPB(res model.GhostResources) *ghostv1.ResourceVector {
	return &ghostv1.ResourceVector{
		CpuMilli:    res.CPUMilli,
		MemoryBytes: res.MemoryBytes,
	}
}

func mapResourcesFromPB(res *ghostv1.ResourceVector) model.GhostResources {
	if res == nil {
		return model.GhostResources{}
	}
	return model.GhostResources{
		CPUMilli:    res.CpuMilli,
		MemoryBytes: res.MemoryBytes,
	}
}

func mapResultFromPB(res *ghostv1.SimulationResult) model.GhostSimulationResult {
	if res == nil {
		return model.GhostSimulationResult{}
	}

	frames := make([]model.GhostTimelineFrame, len(res.Frames))
	for i, f := range res.Frames {
		nodes := make([]model.GhostFrameNode, len(f.Nodes))
		for j, n := range f.Nodes {
			nodes[j] = model.GhostFrameNode{
				Name:          n.Name,
				Status:        n.Status,
				Unschedulable: n.Unschedulable,
				Headroom:      mapResourcesFromPB(n.Headroom),
			}
		}

		pods := make([]model.GhostFramePod, len(f.Pods))
		for j, p := range f.Pods {
			pods[j] = model.GhostFramePod{
				ID:        p.Id,
				Namespace: p.Namespace,
				Name:      p.Name,
				NodeName:  p.NodeName,
				Status:    p.Status,
			}
		}

		events := make([]model.GhostTimelineEvent, len(f.Events))
		for j, ev := range f.Events {
			events[j] = model.GhostTimelineEvent{
				Kind:      ev.Kind,
				Severity:  ev.Severity,
				Resource:  ev.Resource,
				Message:   ev.Message,
				Timestamp: ev.Timestamp,
			}
		}

		frames[i] = model.GhostTimelineFrame{
			OffsetSeconds: int(f.OffsetSeconds),
			Nodes:         nodes,
			Pods:          pods,
			Events:        events,
		}
	}

	return model.GhostSimulationResult{
		ID:             res.Id,
		Action:         res.Action,
		GeneratedAt:    res.GeneratedAt,
		HorizonSeconds: int(res.HorizonSeconds),
		Verdict: model.GhostSimulationVerdict{
			Severity:        res.Verdict.Severity,
			Summary:         res.Verdict.Summary,
			Recommendations: res.Verdict.Recommendations,
		},
		Frames: frames,
	}
}
