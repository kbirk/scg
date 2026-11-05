#pragma once

#include <string>

#include "nlohmann/json.hpp"

// to_string renders any type with an nlohmann to_json overload as indented JSON.
template <typename T>
std::string to_string(const T& value)
{
	nlohmann::json j = value;
	return j.dump(4);
}
