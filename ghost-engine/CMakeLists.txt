cmake_minimum_required(VERSION 3.22)
project(kubelens_ghost_engine LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

set(KUBELENS_THIRD_PARTY_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../third_party/usr")

add_executable(ghost-engine
  src/main.cc
  src/simulator.cc
  generated/ghost/v1/ghost.pb.cc
  generated/ghost/v1/ghost.grpc.pb.cc
)

target_include_directories(ghost-engine PRIVATE
  src
  generated
  "${KUBELENS_THIRD_PARTY_ROOT}/include"
)

target_link_directories(ghost-engine PRIVATE
  "${KUBELENS_THIRD_PARTY_ROOT}/lib64"
  "${KUBELENS_THIRD_PARTY_ROOT}/lib"
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

enable_testing()

add_executable(ghost-engine-simulator-tests
  tests/simulator_taints_test.cc
  src/simulator.cc
  generated/ghost/v1/ghost.pb.cc
  generated/ghost/v1/ghost.grpc.pb.cc
)

target_include_directories(ghost-engine-simulator-tests PRIVATE
  src
  generated
  "${KUBELENS_THIRD_PARTY_ROOT}/include"
)

target_link_directories(ghost-engine-simulator-tests PRIVATE
  "${KUBELENS_THIRD_PARTY_ROOT}/lib64"
  "${KUBELENS_THIRD_PARTY_ROOT}/lib"
)

target_link_libraries(ghost-engine-simulator-tests PRIVATE
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

target_compile_options(ghost-engine-simulator-tests PRIVATE -Wall -Wextra -Wpedantic)
add_test(NAME ghost-engine-simulator-tests COMMAND ghost-engine-simulator-tests)
set_tests_properties(ghost-engine-simulator-tests PROPERTIES
  ENVIRONMENT "LD_LIBRARY_PATH=${KUBELENS_THIRD_PARTY_ROOT}/lib64:${KUBELENS_THIRD_PARTY_ROOT}/lib"
)
