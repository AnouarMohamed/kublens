# syntax=docker/dockerfile:1.7

FROM fedora:43 AS builder
WORKDIR /workspace

RUN dnf install -y abseil-cpp-devel c-ares-devel cmake gcc-c++ grpc-cpp grpc-devel libicu-devel make protobuf-devel re2-devel && \
    dnf clean all

COPY ghost-engine ./ghost-engine

RUN cmake -S /workspace/ghost-engine -B /workspace/ghost-engine/.docker-build && \
    cmake --build /workspace/ghost-engine/.docker-build --parallel

FROM fedora:43 AS runtime
WORKDIR /app

RUN dnf install -y abseil-cpp c-ares grpc-cpp libicu libstdc++ protobuf re2 && \
    dnf clean all && \
    useradd --system --uid 10001 --gid 0 --home-dir /nonexistent --shell /sbin/nologin ghost

COPY --from=builder /workspace/ghost-engine/.docker-build/ghost-engine /app/ghost-engine

EXPOSE 8091
USER 10001

ENTRYPOINT ["/app/ghost-engine"]
CMD ["0.0.0.0:8091"]
