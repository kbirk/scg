#pragma once

#include <string>

namespace scg {
namespace error {

struct Error {

	Error()
		: message("")
	{
	}

	Error(std::nullptr_t)
		: message("")
	{
	}

	explicit Error(std::string msg)
		: message(msg)
	{
	}

	operator bool() const
	{
		return message != "";
	}

	std::string message;
};


bool operator== (const Error& a, const Error& b)
{
	return a.message == b.message;
}

bool operator== (const Error& a, std::nullptr_t)
{
	return a.message == "";
}

bool operator== (std::nullptr_t, const Error& b)
{
	return b.message == "";
}

bool operator!= (const Error& a, const Error& b)
{
	return !(a == b);
}

bool operator!= (const Error& a, std::nullptr_t)
{
	return !(a == nullptr);
}

bool operator!= (std::nullptr_t, const Error& b)
{
	return !(b == nullptr);
}

}
}
