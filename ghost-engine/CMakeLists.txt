cmake_minimum_required(VERSION 3.22)
project(kubelens_ghost_engine LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

add_executable(ghost-engine
  src/main.cc
)

target_compile_options(ghost-engine PRIVATE -Wall -Wextra -Wpedantic)
