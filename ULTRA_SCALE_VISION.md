# KubLens-AI: Ultra-Scale Vision (2026+)

This document defines the strategic evolution of KubLens-AI from a diagnostics tool to an **Autonomous Cluster Intelligence Platform**. The goal is "Zero-Touch Operations" through simulation, deep visibility, and predictive remediation.

**Implementation status:** Strategic roadmap with shipped slices. The current execution path includes a narrow Ghost Cluster MVP and experimental eBPF telemetry ingestion with durable SQL retention, fleet drift reports/review proposals, and autonomous remediation proposals. Production eBPF agents, fleet correction, and closed-loop autonomy remain future hardening work.

## The Three Pillars of Ultra-Scale

### 1. Project Ghost Cluster (Simulation & Digital Twin)

**Status:** MVP slice shipped | **Languages:** Go, C++, React
**The Problem:** Engineers fear applying changes to production because they cannot predict side effects.
**The Solution:** A high-performance simulation engine that creates a "Digital Twin" of the cluster. It allows users to "play forward" the next 15-30 minutes of cluster behavior based on a proposed change (Scaling, Upgrading, Draining) before it is applied to the real world.
**Current slice:** Node-drain simulation is implemented with a C++ gRPC engine boundary, persisted runs, confidence/limitations, resource headroom, node selector, and taint/toleration filters.
**Key Tech:** C++ Discrete Event Simulation (DES), Protobuf State Serialization, WebAssembly (WASM) UI components.

### 2. Kernel-to-Cloud Deep Diagnostics (eBPF Visibility)

**Status:** Experimental ingestion slice shipped | **Languages:** C++, Go, Python
**The Problem:** K8s API metrics are "shallow." They tell you a pod is slow, but not _why_ (e.g., kernel lock contention, disk I/O wait, or silent packet loss).
**The Solution:** A C++ eBPF agent that sits on every node. It feeds high-frequency, low-overhead performance data (syscalls, network latency, file descriptor leaks) into the KubLens Predictor for "Root Cause Analysis" that is 10x deeper than standard Prometheus metrics.
**Current slice:** The backend exposes disabled-by-default, operator-authenticated telemetry ingestion and SQL-backed retention for recent node telemetry samples. A production node agent remains future work.

### 3. Fleet-Wide Drift Correction & Autonomous Remediation

**Status:** Experimental detection/proposal slice shipped | **Languages:** Go, Python
**The Problem:** Managing 50 clusters leads to "Configuration Drift." Small differences in manifests cause unpredictable failures.
**The Solution:** An autonomous controller that treats a fleet of clusters as a single entity. It uses the Python Predictor to detect "Behavioral Drift"—where two identical pods in different clusters behave differently—and automatically generates GitOps patches to align them.
**Current slice:** Fleet drift reports compare configured contexts, and warning-level drift can create review-only remediation proposals. Automatic GitOps correction and closed-loop autonomy remain future work.

## Multi-Language Architecture

- **Go (The Nervous System):** Handles orchestration, Kubernetes API interaction, and the high-speed **Topology Mapping**. It provides the Protobuf serialization layer for the simulation engine.
- **C++ (The Muscle):** Used for the Simulation Engine (Ghost Cluster) and high-speed eBPF packet/syscall processing.
- **Python (The Brain):** Used for ML-driven risk scoring, trend detection, and natural language assistant enrichment.
- **TypeScript/React (The Face):** Provides an immersive, real-time command center including **3D topology visualizations** (Three.js) and a **global time-scrubbing state**.

## Goal: The "Time Machine" Workflow

1. **Detect:** eBPF (C++) notices a latency spike in the kernel.
2. **Diagnose:** Predictor (Python) identifies a resource contention pattern.
3. **Simulate:** Ghost Cluster (C++) simulates a Pod Migration in the Digital Twin to see if it fixes the issue.
4. **Remediate:** Orchestrator (Go) applies the fix via GitOps once the simulation passes.
