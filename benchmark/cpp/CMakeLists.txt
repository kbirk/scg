cmake_minimum_required(VERSION 3.5)
project(scg_benchmarks)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -O2 -Wall -Wextra -Werror -Wpedantic")
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -fno-omit-frame-pointer")
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -Wno-unused-parameter")

# Include directories
include_directories("${CMAKE_CURRENT_SOURCE_DIR}/../../include")
include_directories("${CMAKE_CURRENT_SOURCE_DIR}/../scg/generated")
include_directories(SYSTEM "${CMAKE_CURRENT_SOURCE_DIR}/../../third_party/include")

# Find gperftools
find_library(PROFILER_LIB profiler)

if(NOT PROFILER_LIB)
	message(WARNING "gperftools profiler library not found. Profiling will not work.")
endif()

add_executable(serialize_bench serialize_bench.cpp)
add_executable(message_bench message_bench.cpp)
add_executable(varint_bench varint_bench.cpp)
add_executable(rpc_bench rpc_bench.cpp)

# Find pthread for RPC benchmark
find_package(Threads REQUIRED)

if(PROFILER_LIB)
	target_link_libraries(serialize_bench ${PROFILER_LIB})
	target_link_libraries(message_bench ${PROFILER_LIB})
	target_link_libraries(varint_bench ${PROFILER_LIB})
	target_link_libraries(rpc_bench ${PROFILER_LIB} Threads::Threads)
else()
	target_link_libraries(rpc_bench Threads::Threads)
endif()
