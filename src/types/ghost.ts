export interface GhostResources {
  cpuMilli: number;
  memoryBytes: number;
}

export interface GhostNode {
  name: string;
  status: string;
  unschedulable: boolean;
  labels?: Record<string, string>;
  taints?: string[];
  capacity: GhostResources;
  allocatable: GhostResources;
  used: GhostResources;
  headroom: GhostResources;
}

export interface GhostPod {
  id: string;
  namespace: string;
  name: string;
  nodeName?: string;
  status: string;
  labels?: Record<string, string>;
  nodeSelector?: Record<string, string>;
  tolerations?: string[];
  requests: GhostResources;
  usage: GhostResources;
}

export interface GhostService {
  id: string;
  namespace: string;
  name: string;
  type: string;
  selector?: Record<string, string>;
  podRefs: string[];
}

export interface GhostIngress {
  id: string;
  namespace: string;
  name: string;
  hosts: string[];
  serviceRefs: string[];
}

export interface GhostGraphEdge {
  from: string;
  to: string;
  kind: string;
}

export interface GhostTopology {
  generatedAt: string;
  source: string;
  nodes: GhostNode[];
  pods: GhostPod[];
  services: GhostService[];
  ingresses: GhostIngress[];
  edges: GhostGraphEdge[];
}

export interface GhostSimulationRequest {
  action: "node_drain";
  nodeName: string;
  horizonSeconds?: number;
}

export interface GhostSimulationResult {
  id: string;
  action: string;
  generatedAt: string;
  horizonSeconds: number;
  engine: string;
  topologyHash: string;
  confidence: number;
  limitations: string[];
  verdict: GhostSimulationVerdict;
  frames: GhostTimelineFrame[];
}

export interface GhostSimulationRecord {
  id: string;
  createdAt: string;
  request: GhostSimulationRequest;
  topologyHash: string;
  result: GhostSimulationResult;
}

export interface GhostSimulationListResponse {
  total: number;
  items: GhostSimulationRecord[];
}

export interface GhostSimulationVerdict {
  severity: "info" | "warning" | "critical" | string;
  summary: string;
  recommendations: string[];
}

export interface GhostTimelineFrame {
  offsetSeconds: number;
  nodes: GhostFrameNode[];
  pods: GhostFramePod[];
  events: GhostTimelineEvent[];
}

export interface GhostFrameNode {
  name: string;
  status: string;
  unschedulable: boolean;
  headroom: GhostResources;
}

export interface GhostFramePod {
  id: string;
  namespace: string;
  name: string;
  nodeName?: string;
  status: string;
}

export interface GhostTimelineEvent {
  kind: string;
  severity: "info" | "warning" | "critical" | string;
  resource: string;
  message: string;
  timestamp?: string;
}
