cmake_minimum_required(VERSION 3.22)
project(kubelens_ghost_engine LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

add_executable(ghost-engine
  src/main.cc
  src/simulator.cc
  generated/ghost/v1/ghost.pb.cc
  generated/ghost/v1/ghost.grpc.pb.cc
)

target_include_directories(ghost-engine PRIVATE
  src
  generated
)

target_link_libraries(ghost-engine PRIVATE
  grpc++
  protobuf
  absl_synchronization
  absl_time
  absl_strings
  absl_base
  absl_throw_delegate
  absl_int128
  absl_raw_logging_internal
  cares
  icuuc
  re2
)

target_compile_options(ghost-engine PRIVATE -Wall -Wextra -Wpedantic)
