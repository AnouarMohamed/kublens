# Technical Spec: Project Ghost Cluster (The Simulation Engine)

## Overview

**Project Ghost Cluster** is a high-fidelity, discrete-event simulation (DES) module for KubLens-AI. It enables "Predictive Operations" by allowing users to simulate cluster-wide changes in a sandbox that mirrors their real production environment.

**Implementation status:** MVP slice implemented. The executable slice is a read-only node-drain simulation behind a Go API and isolated gRPC engine boundary. It persists recent runs, exposes confidence and limitations, and applies basic scheduler filters for readiness, unschedulable nodes, node selectors, taints/tolerations, and requested CPU/memory headroom. eBPF-driven telemetry, full scheduler plugin parity, network propagation, and cascade scoring remain later phases.

## 1. Core Architecture

### Phase A: The State Mirror & Topology Extractor (Go)

The Go backend takes the current `ClusterState` and enriches it with a dependency graph.

- **Topology Mapping:** Parses Services, Endpoints, and Ingresses to understand which pods talk to which services.
- **Serialization:** Uses **Protobuf** to serialize the enriched state for high-speed ingestion by the C++ engine.
- **Context Preservation:** The current MVP respects taints, tolerations, node readiness, node selectors, and requested CPU/memory headroom. Pod anti-affinity and full scheduler plugin parity remain later fidelity work.

### Phase B: The DES Engine (C++)

The heart of the feature. A C++20 module designed for maximum throughput.

- **Scheduler Simulation:** The C++ engine implements deterministic node-drain placement for the current MVP. Full `kube-scheduler` parity, including anti-affinity, topology spread, preemption, and disruption policy behavior, remains later fidelity work.
- **Resource Propagation:** Future fidelity work will calculate how a CPU spike in `Service A` affects `Service B` based on mapped topology.
- **I/O & Network Modeling:** Future fidelity work will use high-frequency eBPF metrics to simulate disk and network saturation points.

### Phase C: Predictive Scoring (Python)

The C++ engine outputs a "Simulated Timeline." The Python Predictor analyzes this timeline.

- **Risk Verdict:** "Critical: Simulation shows Node-04 will OOM in 180 seconds if this scale-up proceeds."
- **Cascade Analysis:** Detects "Thundering Herd" scenarios where one pod crash leads to a cluster-wide failure.

### Phase D: Immersive UI (React + Three.js)

A "Ghost Mode" toggle in the UI that shifts the visual state to the future.

- **Timeline Scrubbing:** A global state allows the user to scrub forward in time through the simulated frames.
- **3D Dependency Graph:** A Three.js visualization showing real-time traffic flows and resource pressure points in the "Ghost" state.

## 2. Technical Requirements

- **Integration:** C++ code integrated via **CGO** (shared library) or a standalone **gRPC** service.
- **High-Speed Bus:** Go `EventBus` buffer increased to handle high-frequency telemetry.
- **Determinism:** The engine must be pure; given the same state and change, it must produce the same simulation result.

## 3. Implementation Roadmap (The "Boredom Killer" Plan)

1. **Sprint 1 (Go Foundation):** Shipped for the node-drain slice with topology export into the Ghost API boundary.
2. **Sprint 2 (C++ DES Engine):** Shipped for deterministic node-drain placement with resource, selector, and taint/toleration filters.
3. **Sprint 3 (The Predictor Bridge):** Future work for cascade failure detection from simulated timelines.
4. **Sprint 4 (3D Visualizer):** Future work for richer topology/time-scrubbing visualization.
5. **Sprint 5 (Closed Loop):** Future work; current behavior remains read-only simulation and human-reviewed remediation.

## 4. Key Metrics for Simulation

- **Latency (p99):** Predicted service-to-service latency.
- **Resource Depth:** How much "Headroom" remains on each node before eviction triggers.
- **Convergence Time:** How long it takes for the cluster to stabilize after a massive rollout.
