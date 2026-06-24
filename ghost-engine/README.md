# Ghost Engine Standalone Service

This directory contains the C++20 standalone service for **Project Ghost Cluster**. It runs a high-performance gRPC server that simulates cluster-wide maintenance operations (like `node_drain`) and returns timeline frames and verdicts.

## Current Status

- **Fully Finished**: Implements `ghost.v1.SimulationService` defined in `proto/ghost/v1/ghost.proto`.
- **Integrated**: The Go backend contains a `ghost.Client` that automatically serializes topology details, communicates with this service via gRPC, and maps results back to Go structures.
- **Local Toolchain Setup**: Dependencies (Protobuf, gRPC, and Abseil) are locally resolved and configured in `third_party/` to avoid requiring system-wide root/sudo privileges.

## How to Build

1. Make sure you have `g++`, `make`, and `cmake` installed.
2. Configure and compile using CMake:
   ```bash
   mkdir -p build && cd build
   cmake ..
   LD_LIBRARY_PATH=../../third_party/usr/lib64 make
   ```

## How to Build the Container Image

Build from the repository root so the Docker build can access the vendored `third_party` runtime libraries:

```bash
docker build -t kubelens-ghost-engine:v0.4.1 -f ghost-engine/Dockerfile .
```

## How to Run

1. Run the service on a specific target address (default is `0.0.0.0:8091`):
   ```bash
   LD_LIBRARY_PATH=../../third_party/usr/lib64 ./ghost-engine 0.0.0.0:8091
   ```

2. Or run the containerized service:

   ```bash
   docker run --rm -p 8091:8091 kubelens-ghost-engine:v0.4.1
   ```

3. Make sure `GHOST_ENABLED=true` and `GHOST_ENGINE_ADDR=localhost:8091` are configured in your backend environment `.env` to enable production gRPC forwarding from the Go backend.
