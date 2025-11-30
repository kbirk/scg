cmake_minimum_required(VERSION 3.5)
project(scg_tests)

message(STATUS "CMAKE_SYSTEM_NAME = ${CMAKE_SYSTEM_NAME}")
message(STATUS "CMAKE_CXX_COMPILER_ID = ${CMAKE_CXX_COMPILER_ID}")
message(STATUS "CMAKE_CXX_COMPILER_VERSION = ${CMAKE_CXX_COMPILER_VERSION}")
message(STATUS "CMAKE_CURRENT_SOURCE_DIR = ${CMAKE_CURRENT_SOURCE_DIR}")

# Set LINUX flag
if(UNIX AND NOT APPLE)
    set(LINUX TRUE)
endif()

set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -std=c++17")

# Warning pedantic flags for all
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -Wall -Wextra -Werror -Wpedantic -Wfatal-errors")
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -Wno-unused-parameter")
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -O2")
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -g") # debug info
set(CMAKE_ENABLE_EXPORTS ON) # enables -rdynamic to export symbols for stacktraces

# Find OpenSSL
find_package(OpenSSL REQUIRED)

# Source include dirs
include_directories("${CMAKE_CURRENT_SOURCE_DIR}/../../include")
include_directories("${CMAKE_CURRENT_SOURCE_DIR}/../../test/files/output")
include_directories(SYSTEM "${CMAKE_CURRENT_SOURCE_DIR}/../../third_party/include")

# Executables

# Serialization tests
add_executable(serialize_tests "serialize_tests.cpp")

# UUID tests
add_executable(uuid_tests "uuid_tests.cpp")

# Client without TLS tests
add_executable(client_no_tls_tests "client_no_tls_tests.cpp")

# Client TCP tests
add_executable(client_tcp_tests "client_tcp_tests.cpp")

# Client with TLS tests
add_executable(client_tls_tests "client_tls_tests.cpp")

# Server without TLS test
add_executable(server_no_tls_test "server_no_tls_test.cpp")

# Server TCP test
add_executable(server_tcp_test "server_tcp_test.cpp")

# Client TCP TLS tests
add_executable(client_tcp_tls_tests "client_tcp_tls_tests.cpp")

# Server TCP TLS test
add_executable(server_tcp_tls_test "server_tcp_tls_test.cpp")

# Macro test
add_executable(macro_test "macro_test.cpp")

# Link with libs
target_link_libraries(client_tls_tests OpenSSL::SSL OpenSSL::Crypto)
target_link_libraries(server_no_tls_test OpenSSL::SSL OpenSSL::Crypto)
target_link_libraries(client_tcp_tests OpenSSL::SSL OpenSSL::Crypto)
target_link_libraries(server_tcp_test OpenSSL::SSL OpenSSL::Crypto)
target_link_libraries(client_tcp_tls_tests OpenSSL::SSL OpenSSL::Crypto)
target_link_libraries(server_tcp_tls_test OpenSSL::SSL OpenSSL::Crypto)
