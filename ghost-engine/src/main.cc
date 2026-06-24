#include <iostream>
#include <memory>
#include <string>
#include <thread>
#include <atomic>
#include <chrono>
#include <csignal>
#include <grpcpp/grpcpp.h>
#include "ghost/v1/ghost.grpc.pb.h"
#include "simulator.h"

class SimulationServiceImpl final : public ghost::v1::SimulationService::Service {
    grpc::Status Simulate(
        grpc::ServerContext* context,
        const ghost::v1::SimulationRequest* request,
        ghost::v1::SimulationResult* response
    ) override {
        (void)context;
        *response = ghost::Simulator::Run(*request);
        return grpc::Status::OK;
    }
};

namespace {
std::unique_ptr<grpc::Server> server;
std::atomic_bool shutdown_requested{false};

void handle_signal(int) {
    shutdown_requested.store(true, std::memory_order_relaxed);
}
}

int main(int argc, char** argv) {
    std::string address = "0.0.0.0:8091";
    if (argc > 1) {
        address = argv[1];
    }

    std::signal(SIGINT, handle_signal);
    std::signal(SIGTERM, handle_signal);

    SimulationServiceImpl service;

    grpc::ServerBuilder builder;
    builder.AddListeningPort(address, grpc::InsecureServerCredentials());
    builder.RegisterService(&service);

    server = builder.BuildAndStart();
    if (!server) {
        std::cerr << "Failed to bind to " << address << std::endl;
        return 1;
    }
    std::cout << "Ghost Engine Server listening on " << address << std::endl;

    std::thread shutdown_thread([] {
        while (!shutdown_requested.load(std::memory_order_relaxed)) {
            std::this_thread::sleep_for(std::chrono::milliseconds(100));
        }
        if (server) {
            std::cout << "Shutting down Ghost Engine gRPC server..." << std::endl;
            server->Shutdown();
        }
    });

    server->Wait();
    shutdown_requested.store(true, std::memory_order_relaxed);
    if (shutdown_thread.joinable()) {
        shutdown_thread.join();
    }
    return 0;
}
