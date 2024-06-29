#pragma once

#include <string>

#include "nlohmann/json.hpp"

template <typename T>
std::string to_string(const T&)
{
	nlohmann::json j;
	to_json(j, value);
	return j.dump(4);
}
