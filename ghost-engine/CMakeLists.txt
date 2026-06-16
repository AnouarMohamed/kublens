cmake_minimum_required(VERSION 3.22)
project(kubelens_ghost_engine LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

# Linker directories for transitive shared library resolution at build time
link_directories("${CMAKE_CURRENT_SOURCE_DIR}/../third_party/usr/lib64")

# Manually define protobuf target
add_library(protobuf::libprotobuf SHARED IMPORTED)
set_target_properties(protobuf::libprotobuf PROPERTIES
  IMPORTED_LOCATION "${CMAKE_CURRENT_SOURCE_DIR}/../third_party/usr/lib64/libprotobuf.so"
  INTERFACE_INCLUDE_DIRECTORIES "${CMAKE_CURRENT_SOURCE_DIR}/../third_party/usr/include"
)

# Manually define gRPC target
add_library(gRPC::grpc++ SHARED IMPORTED)
set_target_properties(gRPC::grpc++ PROPERTIES
  IMPORTED_LOCATION "${CMAKE_CURRENT_SOURCE_DIR}/../third_party/usr/lib64/libgrpc++.so"
  INTERFACE_INCLUDE_DIRECTORIES "${CMAKE_CURRENT_SOURCE_DIR}/../third_party/usr/include"
)

add_executable(ghost-engine
  src/main.cc
  src/simulator.cc
  generated/ghost/v1/ghost.pb.cc
  generated/ghost/v1/ghost.grpc.pb.cc
)

target_include_directories(ghost-engine PRIVATE
  src
  generated
  "${CMAKE_CURRENT_SOURCE_DIR}/../third_party/usr/include"
)

target_link_libraries(ghost-engine PRIVATE
  gRPC::grpc++
  protobuf::libprotobuf
  absl_synchronization
  absl_time
  absl_strings
  absl_base
  absl_throw_delegate
  absl_int128
  absl_raw_logging_internal
)

target_compile_options(ghost-engine PRIVATE -Wall -Wextra -Wpedantic)
