#pragma once

#include <chrono>

#include "nlohmann/json.hpp"

namespace scg {

using timestamp = std::chrono::time_point<std::chrono::system_clock, std::chrono::nanoseconds>;

}

namespace nlohmann {

	template <>
	struct adl_serializer<scg::timestamp>
	{
		static void to_json(json& j, const scg::timestamp& tp)
		{
			j["since_epoch_nano"] = uint64_t(std::chrono::duration_cast<std::chrono::nanoseconds>(tp.time_since_epoch()).count());
		}

		static void from_json(const json& j, scg::timestamp& tp)
		{
			auto since_epoch = std::chrono::nanoseconds{j["since_epoch_nano"].get<uint64_t>()};
			tp = std::chrono::time_point<std::chrono::system_clock, std::chrono::nanoseconds>{since_epoch};
		}
	};
}
