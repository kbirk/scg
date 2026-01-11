#pragma once

#include <string>
#include <cstring>
#include <utility>
#include <cstddef>

namespace scg {
namespace error {

class Error {
public:

	inline Error() = default;

	inline Error(std::nullptr_t)
	{
	}

	inline explicit Error(const char* msg)
	{
		if (msg && msg[0] != '\0') {
			size_t len = std::strlen(msg);
			msg_ = new char[len + 1];
			std::memcpy(msg_, msg, len + 1);
		}
	}

	inline explicit Error(const std::string& msg)
	{
		if (!msg.empty()) {
			size_t len = msg.size();
			msg_ = new char[len + 1];
			std::memcpy(msg_, msg.c_str(), len + 1);
		}
	}

	inline Error(const Error& other)
	{
		if (other.msg_) {
			size_t len = std::strlen(other.msg_);
			msg_ = new char[len + 1];
			std::memcpy(msg_, other.msg_, len + 1);
		}
	}

	inline Error(Error&& other) noexcept : msg_(other.msg_)
	{
		other.msg_ = nullptr;
	}

	inline Error& operator=(const Error& other)
	{
		if (this != &other) {
			delete[] msg_;
			msg_ = nullptr;
			if (other.msg_) {
				size_t len = std::strlen(other.msg_);
				msg_ = new char[len + 1];
				std::memcpy(msg_, other.msg_, len + 1);
			}
		}
		return *this;
	}

	inline Error& operator=(Error&& other) noexcept
	{
		if (this != &other) {
			delete[] msg_;
			msg_ = other.msg_;
			other.msg_ = nullptr;
		}
		return *this;
	}

	inline ~Error()
	{
		delete[] msg_;
	}

	inline operator bool() const
	{
		return msg_ != nullptr;
	}

	inline std::string message() const
	{
		return msg_ ? std::string(msg_) : "";
	}

private:

	char* msg_ = nullptr;
};

inline bool operator== (const Error& a, const Error& b)
{
	return a.message() == b.message();
}

inline bool operator== (const Error& a, std::nullptr_t)
{
	return !a;
}

inline bool operator== (std::nullptr_t, const Error& b)
{
	return !b;
}

inline bool operator!= (const Error& a, const Error& b)
{
	return !(a == b);
}

inline bool operator!= (const Error& a, std::nullptr_t) {
	return !(a == nullptr);
}

inline bool operator!= (std::nullptr_t, const Error& b)
{
	return !(b == nullptr);
}

}
}
