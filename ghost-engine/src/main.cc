#include <chrono>
#include <csignal>
#include <iostream>
#include <string>
#include <thread>

namespace {
bool running = true;

void handle_signal(int) {
  running = false;
}
}  // namespace

int main(int argc, char** argv) {
  std::string address = "0.0.0.0:8091";
  if (argc > 1) {
    address = argv[1];
  }

  std::signal(SIGINT, handle_signal);
  std::signal(SIGTERM, handle_signal);

  std::cout << "ghost-engine scaffold listening target " << address << "\n";
  std::cout << "Generate gRPC stubs from proto/ghost/v1/ghost.proto before enabling production serving.\n";

  while (running) {
    std::this_thread::sleep_for(std::chrono::milliseconds(250));
  }
  return 0;
}
