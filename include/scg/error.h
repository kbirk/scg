#pragma once

#include <string>

namespace scg {
namespace error {

struct Error {

	inline Error()
		: message("")
	{
	}

	inline Error(std::nullptr_t)
		: message("")
	{
	}

	inline explicit Error(std::string msg)
		: message(msg)
	{
	}

	inline operator bool() const
	{
		return message != "";
	}

	std::string message;
};


inline bool operator== (const Error& a, const Error& b)
{
	return a.message == b.message;
}

inline bool operator== (const Error& a, std::nullptr_t)
{
	return a.message == "";
}

inline bool operator== (std::nullptr_t, const Error& b)
{
	return b.message == "";
}

inline bool operator!= (const Error& a, const Error& b)
{
	return !(a == b);
}

inline bool operator!= (const Error& a, std::nullptr_t)
{
	return !(a == nullptr);
}

inline bool operator!= (std::nullptr_t, const Error& b)
{
	return !(b == nullptr);
}

}
}
