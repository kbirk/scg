#pragma once

#include <string>
#include <cstring>
#include <utility>
#include <cstddef>
#include <cstdio>
#include <cstdarg>

namespace scg {
namespace error {

class Error {
public:

	inline Error() = default;

	inline Error(std::nullptr_t)
	{
	}

	inline explicit Error(const std::string& msg)
	{
		if (!msg.empty()) {
			size_t len = msg.size();
			msg_ = new char[len + 1];
			std::memcpy(msg_, msg.c_str(), len + 1);
		}
	}

	inline explicit Error(const char* msg)
	{
		if (msg && msg[0] != '\0') {
			size_t len = std::strlen(msg);
			msg_ = new char[len + 1];
			std::memcpy(msg_, msg, len + 1);
		}
	}

#if defined(__GNUC__) || defined(__clang__)
	__attribute__((format(printf, 1, 2)))
#endif
	static inline Error Errorf(const char* fmt, ...)
	{
		Error err;
		if (fmt && fmt[0] != '\0') {
			va_list args;
			va_start(args, fmt);
			va_list args_copy;
			va_copy(args_copy, args);
			int len = std::vsnprintf(nullptr, 0, fmt, args_copy);
			va_end(args_copy);
			if (len > 0) {
				err.msg_ = new char[len + 1];
				std::vsnprintf(err.msg_, len + 1, fmt, args);
			}
			va_end(args);
		}
		return err;
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
