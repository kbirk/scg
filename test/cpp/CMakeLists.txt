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

# Sanitizers: configure with e.g. -DSCG_SANITIZE=address,undefined or
# -DSCG_SANITIZE=thread. On kernels with high ASLR entropy, run the binaries
# under `setarch -R` (ASan/TSan otherwise fail intermittently at startup).
set(SCG_SANITIZE "" CACHE STRING "Sanitizers to build the tests with (address, thread, undefined, or a comma list)")
if(SCG_SANITIZE)
	message(STATUS "SCG_SANITIZE = ${SCG_SANITIZE}")
	set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -fsanitize=${SCG_SANITIZE} -fno-omit-frame-pointer")
	set(CMAKE_EXE_LINKER_FLAGS "${CMAKE_EXE_LINKER_FLAGS} -fsanitize=${SCG_SANITIZE}")
	if(SCG_SANITIZE MATCHES "undefined")
		set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -fno-sanitize-recover=undefined")
	endif()
	if(SCG_SANITIZE MATCHES "thread")
		# asio uses std::atomic_thread_fence, which GCC warns TSan cannot model.
		include(CheckCXXCompilerFlag)
		check_cxx_compiler_flag(-Wno-tsan SCG_HAS_WNO_TSAN)
		if(SCG_HAS_WNO_TSAN)
			set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -Wno-tsan")
		endif()
	endif()
endif()

# Find OpenSSL
find_package(OpenSSL REQUIRED)


# Source include dirs
include_directories("${CMAKE_CURRENT_SOURCE_DIR}/../../include")
include_directories("${CMAKE_CURRENT_SOURCE_DIR}/../../test/scg/generated")
include_directories("${CMAKE_CURRENT_SOURCE_DIR}")
include_directories(SYSTEM "${CMAKE_CURRENT_SOURCE_DIR}/../../third_party/include")

# Executables

# Serialization tests
add_executable(serialize_tests "serialize_tests.cpp")

# UUID tests
add_executable(uuid_tests "uuid_tests.cpp")

# Error tests
add_executable(error_tests "error_tests.cpp")

# Macro test
add_executable(macro_test "macro_test.cpp")

# Middleware chain tests
add_executable(middleware_tests "middleware_tests.cpp")

# Downstream compatibility (extension overloads, macros, golden encoding)
add_executable(downstream_compat_tests "downstream_compat_tests.cpp")

# Client connect bookkeeping and deadline arithmetic (no network)
add_executable(client_unit_tests "client_unit_tests.cpp")

# Every public header must compile on its own (built as part of `all`).
set(SCG_INCLUDE_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../../include")
file(GLOB_RECURSE SCG_PUBLIC_HEADERS RELATIVE "${SCG_INCLUDE_DIR}" "${SCG_INCLUDE_DIR}/scg/*.h")
set(SCG_HEADER_CHECK_SOURCES "")
foreach(header ${SCG_PUBLIC_HEADERS})
	string(REGEX REPLACE "[/.]" "_" check_name "${header}")
	set(check_src "${CMAKE_CURRENT_BINARY_DIR}/header_check/${check_name}.cpp")
	file(WRITE "${check_src}.in" "#include \"${header}\"\n")
	configure_file("${check_src}.in" "${check_src}" COPYONLY)
	list(APPEND SCG_HEADER_CHECK_SOURCES "${check_src}")
endforeach()
add_library(header_self_contained OBJECT ${SCG_HEADER_CHECK_SOURCES})

# Generated Api / Fake seam tests (no networking)
add_executable(api_fake_tests "api_fake_tests.cpp")

# =============================================================================
# Unified Test Suites (run in-process server + client for full test coverage)
# =============================================================================

# TCP tests (includes TCP and TCP-TLS)
add_executable(tcp_tests "tcp_tests.cpp")

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

# =============================================================================
# Client Executables (for cross-language testing: C++ Client + Go Server)
# =============================================================================

# Client TCP (connects to Go TCP server)
add_executable(client_tcp_tests "client_tcp_tests.cpp")

# Client TCP TLS (connects to Go TCP TLS server)
add_executable(client_tcp_tls_tests "client_tcp_tls_tests.cpp")

# Client WebSocket (connects to Go WebSocket server)
add_executable(client_ws_tests "client_ws_tests.cpp")

# Client WebSocket TLS (connects to Go WebSocket TLS server)
add_executable(client_ws_tls_tests "client_ws_tls_tests.cpp")

# =============================================================================
# Link with libs
# =============================================================================

# Unified test suites
target_link_libraries(tcp_tests OpenSSL::SSL OpenSSL::Crypto)
target_link_libraries(ws_tests OpenSSL::SSL OpenSSL::Crypto)

# Server executables (cross-language testing)
target_link_libraries(server_ws_test OpenSSL::SSL OpenSSL::Crypto)
target_link_libraries(server_ws_tls_test OpenSSL::SSL OpenSSL::Crypto)
target_link_libraries(server_tcp_test OpenSSL::SSL OpenSSL::Crypto)
target_link_libraries(server_tcp_tls_test OpenSSL::SSL OpenSSL::Crypto)

# Client executables (cross-language testing)
target_link_libraries(client_tcp_tests OpenSSL::SSL OpenSSL::Crypto)
target_link_libraries(client_tcp_tls_tests OpenSSL::SSL OpenSSL::Crypto)
target_link_libraries(client_ws_tests OpenSSL::SSL OpenSSL::Crypto)
target_link_libraries(client_ws_tls_tests OpenSSL::SSL OpenSSL::Crypto)
