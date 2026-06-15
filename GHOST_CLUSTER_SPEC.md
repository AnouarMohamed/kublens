# Technical Spec: Project Ghost Cluster (The Simulation Engine)

## Overview

**Project Ghost Cluster** is a high-fidelity, discrete-event simulation (DES) module for KubLens-AI. It enables "Predictive Operations" by allowing users to simulate cluster-wide changes in a sandbox that mirrors their real production environment.

**Implementation status:** Roadmap / MVP planning. The first executable slice is a read-only node-drain simulation behind a Go API and isolated gRPC engine boundary; eBPF-driven telemetry, network propagation, and cascade scoring are later phases.

## 1. Core Architecture

### Phase A: The State Mirror & Topology Extractor (Go)

The Go backend takes the current `ClusterState` and enriches it with a dependency graph.

- **Topology Mapping:** Parses Services, Endpoints, and Ingresses to understand which pods talk to which services.
- **Serialization:** Uses **Protobuf** to serialize the enriched state for high-speed ingestion by the C++ engine.
- **Context Preservation:** Ensures the simulation respects Taints, Tolerations, and Pod Anti-Affinity.

### Phase B: The DES Engine (C++)

The heart of the feature. A C++20 module designed for maximum throughput.

- **Scheduler Simulation:** Re-implements the `kube-scheduler` logic in C++. It predicts exactly where pods will land if a node is drained.
- **Resource Propagation:** Calculates how a CPU spike in `Service A` will affect the latency of `Service B` based on mapped topology.
- **I/O & Network Modeling:** Uses high-frequency metrics from eBPF to simulate disk and network saturation points.

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

1. **Sprint 1 (Go Foundation):** Refactor the backend into Domain Services. Build the **Topology Mapper** and **Protobuf Exporter**.
2. **Sprint 2 (C++ DES Engine):** Set up the C++20 project. Build a basic "Resource Allocator" and Discrete Event Scheduler.
3. **Sprint 3 (The Predictor Bridge):** Connect C++ output to Python for cascade failure detection.
4. **Sprint 4 (3D Visualizer):** Build the Three.js topology view in React. Integrate the "Ghost Mode" visual state.
5. **Sprint 5 (Closed Loop):** Scrubbing through the future. Finalize the time-scrubbing UI and real-time state updates.

## 4. Key Metrics for Simulation

- **Latency (p99):** Predicted service-to-service latency.
- **Resource Depth:** How much "Headroom" remains on each node before eviction triggers.
- **Convergence Time:** How long it takes for the cluster to stabilize after a massive rollout.
