# Ghost Engine Scaffold

This directory is the C++20 boundary for the future standalone Ghost Cluster gRPC service.

Current status:

- `proto/ghost/v1/ghost.proto` defines the service contract.
- `src/main.cc` is a compilable process scaffold.
- Production gRPC serving is intentionally not generated in this workspace because `protoc`, `grpc_cpp_plugin`, `protoc-gen-go`, and `protoc-gen-go-grpc` are not installed.

Expected generation flow once toolchains are available:

```bash
protoc \
  --cpp_out=ghost-engine/generated \
  --grpc_cpp_out=ghost-engine/generated \
  --plugin=protoc-gen-grpc="$(command -v grpc_cpp_plugin)" \
  proto/ghost/v1/ghost.proto
```
