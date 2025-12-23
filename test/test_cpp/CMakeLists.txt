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
include_directories("${CMAKE_CURRENT_SOURCE_DIR}")
include_directories(SYSTEM "${CMAKE_CURRENT_SOURCE_DIR}/../../third_party/include")

# Executables

# Serialization tests
add_executable(serialize_tests "serialize_tests.cpp")

# UUID tests
add_executable(uuid_tests "uuid_tests.cpp")

# Macro test
add_executable(macro_test "macro_test.cpp")

# =============================================================================
# Unified Test Suites (run in-process server + client for full test coverage)
# =============================================================================

# TCP tests (includes TCP and TCP-TLS)
add_executable(tcp_tests "tcp_tests.cpp")

# Unix socket tests
add_executable(unix_tests "unix_tests.cpp")

# WebSocket tests (includes WebSocket and WebSocket-TLS)
add_executable(ws_tests "ws_tests.cpp")

# =============================================================================
# Server Executables (for cross-language testing: Go Client + C++ Server)
# =============================================================================

# Server Websocket (for Go/other client testing)
add_executable(server_ws_test "server_ws_test.cpp")

# Server Websocket TLS (for Go/other client testing)
add_executable(server_ws_tls_test "server_ws_tls_test.cpp")

# Server TCP (for Go/other client testing)
add_executable(server_tcp_test "server_tcp_test.cpp")

# Server TCP TLS (for Go/other client testing)
add_executable(server_tcp_tls_test "server_tcp_tls_test.cpp")

# Server Unix (for Go/other client testing)
add_executable(server_unix_test "server_unix_test.cpp")

# =============================================================================
# Client Executables (for cross-language testing: C++ Client + Go Server)
# =============================================================================

# Client TCP (connects to Go TCP server)
add_executable(client_tcp_tests "client_tcp_tests.cpp")

# Client TCP TLS (connects to Go TCP TLS server)
add_executable(client_tcp_tls_tests "client_tcp_tls_tests.cpp")

# Client Unix (connects to Go Unix server)
add_executable(client_unix_tests "client_unix_tests.cpp")

# Client WebSocket (connects to Go WebSocket server)
add_executable(client_ws_tests "client_ws_tests.cpp")

# Client WebSocket TLS (connects to Go WebSocket TLS server)
add_executable(client_ws_tls_tests "client_ws_tls_tests.cpp")

# Streaming tests (C++ client and server)
add_executable(streaming_tcp_tests "streaming_tcp_tests.cpp")
add_executable(streaming_unix_tests "streaming_unix_tests.cpp")
add_executable(streaming_ws_tests "streaming_ws_tests.cpp")

# Streaming interop tests
add_executable(streaming_interop_client "streaming_interop_client.cpp")

# =============================================================================
# Link with libs
# =============================================================================

# Unified test suites
target_link_libraries(tcp_tests OpenSSL::SSL OpenSSL::Crypto)
target_link_libraries(unix_tests OpenSSL::SSL OpenSSL::Crypto)
target_link_libraries(ws_tests OpenSSL::SSL OpenSSL::Crypto)

# Server executables (cross-language testing)
target_link_libraries(server_ws_test OpenSSL::SSL OpenSSL::Crypto)
target_link_libraries(server_ws_tls_test OpenSSL::SSL OpenSSL::Crypto)
target_link_libraries(server_tcp_test OpenSSL::SSL OpenSSL::Crypto)
target_link_libraries(server_tcp_tls_test OpenSSL::SSL OpenSSL::Crypto)
target_link_libraries(server_unix_test OpenSSL::SSL OpenSSL::Crypto)

# Client executables (cross-language testing)
target_link_libraries(client_tcp_tests OpenSSL::SSL OpenSSL::Crypto)
target_link_libraries(client_tcp_tls_tests OpenSSL::SSL OpenSSL::Crypto)
target_link_libraries(client_unix_tests OpenSSL::SSL OpenSSL::Crypto)
target_link_libraries(client_ws_tests OpenSSL::SSL OpenSSL::Crypto)
target_link_libraries(client_ws_tls_tests OpenSSL::SSL OpenSSL::Crypto)

# Streaming test executables
target_link_libraries(streaming_tcp_tests OpenSSL::SSL OpenSSL::Crypto)
target_link_libraries(streaming_unix_tests OpenSSL::SSL OpenSSL::Crypto)
target_link_libraries(streaming_ws_tests OpenSSL::SSL OpenSSL::Crypto)

# Streaming interop executables
target_link_libraries(streaming_interop_client OpenSSL::SSL OpenSSL::Crypto)
