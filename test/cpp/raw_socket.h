#pragma once

// Test-only helpers for misbehaving peers (Linux/POSIX sockets). Not part of
// the runtime.

#include <arpa/inet.h>
#include <netinet/in.h>
#include <poll.h>
#include <sys/socket.h>
#include <unistd.h>

#include <chrono>
#include <cstdint>
#include <string>
#include <thread>
#include <vector>

namespace rawtest {

// RawSocket is a plain blocking TCP client used to misbehave on purpose.
class RawSocket {
public:
	explicit RawSocket(int port) {
		fd_ = socket(AF_INET, SOCK_STREAM, 0);
		sockaddr_in addr{};
		addr.sin_family = AF_INET;
		addr.sin_port = htons(static_cast<uint16_t>(port));
		inet_pton(AF_INET, "127.0.0.1", &addr.sin_addr);
		for (int i = 0; i < 50; i++) {
			if (::connect(fd_, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)) == 0) {
				connected_ = true;
				break;
			}
			std::this_thread::sleep_for(std::chrono::milliseconds(20));
		}
	}
	~RawSocket() {
		if (fd_ >= 0) {
			::close(fd_);
		}
	}
	bool connected() const { return connected_; }
	int fd() const { return fd_; }
	void setRecvBuffer(int bytes) { setsockopt(fd_, SOL_SOCKET, SO_RCVBUF, &bytes, sizeof(bytes)); }
	bool write(const std::vector<uint8_t>& data) {
		size_t off = 0;
		while (off < data.size()) {
			ssize_t n = ::send(fd_, data.data() + off, data.size() - off, MSG_NOSIGNAL);
			if (n <= 0) {
				return false;
			}
			off += size_t(n);
		}
		return true;
	}
	void shutdownWrite() { ::shutdown(fd_, SHUT_WR); }
	// waitClosed reads (discarding) until the peer closes or timeoutMs passes.
	bool waitClosed(int timeoutMs) {
		auto deadline = std::chrono::steady_clock::now() + std::chrono::milliseconds(timeoutMs);
		uint8_t buf[4096];
		while (std::chrono::steady_clock::now() < deadline) {
			pollfd p{fd_, POLLIN, 0};
			int r = ::poll(&p, 1, 50);
			if (r > 0) {
				ssize_t n = ::recv(fd_, buf, sizeof(buf), 0);
				if (n <= 0) {
					return true;
				}
			}
		}
		return false;
	}

private:
	int fd_ = -1;
	bool connected_ = false;
};

inline std::vector<uint8_t> framed(const std::vector<uint8_t>& payload) {
	uint32_t len = static_cast<uint32_t>(payload.size());
	std::vector<uint8_t> out(4 + payload.size());
	out[0] = uint8_t(len >> 24);
	out[1] = uint8_t(len >> 16);
	out[2] = uint8_t(len >> 8);
	out[3] = uint8_t(len);
	std::copy(payload.begin(), payload.end(), out.begin() + 4);
	return out;
}

// readHeaders reads until the end of an HTTP header block ("\r\n\r\n").
inline std::string readHttpHeaders(int fd, int timeoutMs)
{
	std::string out;
	auto deadline = std::chrono::steady_clock::now() + std::chrono::milliseconds(timeoutMs);
	char c = 0;
	while (std::chrono::steady_clock::now() < deadline) {
		pollfd p{fd, POLLIN, 0};
		if (::poll(&p, 1, 50) <= 0) {
			continue;
		}
		if (::recv(fd, &c, 1, 0) != 1) {
			break;
		}
		out.push_back(c);
		if (out.size() >= 4 && out.compare(out.size() - 4, 4, "\r\n\r\n") == 0) {
			break;
		}
	}
	return out;
}

// wsClientFrame builds a masked binary WebSocket frame (zero mask key, so the
// payload bytes are sent unchanged).
inline std::vector<uint8_t> wsClientFrame(const std::vector<uint8_t>& payload)
{
	std::vector<uint8_t> out;
	out.push_back(0x82); // FIN + binary
	size_t n = payload.size();
	if (n < 126) {
		out.push_back(static_cast<uint8_t>(0x80 | n));
	} else if (n <= 0xFFFF) {
		out.push_back(0x80 | 126);
		out.push_back(static_cast<uint8_t>(n >> 8));
		out.push_back(static_cast<uint8_t>(n));
	} else {
		out.push_back(0x80 | 127);
		for (int i = 7; i >= 0; i--) {
			out.push_back(static_cast<uint8_t>(uint64_t(n) >> (8 * i)));
		}
	}
	for (int i = 0; i < 4; i++) {
		out.push_back(0); // mask key
	}
	out.insert(out.end(), payload.begin(), payload.end());
	return out;
}

} // namespace rawtest
