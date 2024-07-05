#pragma once

#include <chrono>

#include "scg/serialize.h"

#include "nlohmann/json.hpp"

namespace scg {
namespace type {

class timestamp {

public:

	inline timestamp()
		: timepoint_(std::chrono::system_clock::now())
	{
	}

	inline explicit timestamp(const std::chrono::time_point<std::chrono::system_clock, std::chrono::nanoseconds>& timepoint)
		: timepoint_(timepoint)
	{
	}

	inline bool operator==(const timestamp& other) const
	{
		return timepoint_ == other.timepoint_;
	}

	inline bool operator!=(const timestamp& other) const
	{
		return timepoint_ != other.timepoint_;
	}

	inline bool operator<(const timestamp& other) const
	{
		return timepoint_ < other.timepoint_;
	}

	inline bool operator<=(const timestamp& other) const
	{
		return timepoint_ <= other.timepoint_;
	}

	inline bool operator>(const timestamp& other) const
	{
		return timepoint_ > other.timepoint_;
	}

	inline bool operator>=(const timestamp& other) const
	{
		return timepoint_ >= other.timepoint_;
	}

	inline std::chrono::time_point<std::chrono::system_clock, std::chrono::nanoseconds> timepoint() const
	{
		return timepoint_;
	}

	inline void set(const std::chrono::time_point<std::chrono::system_clock, std::chrono::nanoseconds>& tp)
	{
		timepoint_ = tp;
	}

	friend inline uint32_t bit_size(const timestamp& value)
	{
		using scg::serialize::bit_size;

		auto duration_since_epoch = value.timepoint_.time_since_epoch();
		auto seconds = std::chrono::duration_cast<std::chrono::seconds>(duration_since_epoch);
		auto nanoseconds = std::chrono::duration_cast<std::chrono::nanoseconds>(duration_since_epoch - seconds);

		return bit_size(uint64_t(seconds.count())) + bit_size(uint64_t(nanoseconds.count()));
	}

	template <typename WriterType>
	friend inline void serialize(WriterType& writer, const timestamp& value)
	{
		using scg::serialize::serialize;

		auto duration_since_epoch = value.timepoint_.time_since_epoch();
		auto seconds = std::chrono::duration_cast<std::chrono::seconds>(duration_since_epoch);
		auto nanoseconds = std::chrono::duration_cast<std::chrono::nanoseconds>(duration_since_epoch - seconds);

		serialize(writer, uint64_t(seconds.count()));
		serialize(writer, uint64_t(nanoseconds.count()));
	}

	template <typename ReaderType>
	friend inline error::Error deserialize(timestamp& value, ReaderType& reader)
	{
		using scg::serialize::deserialize;

		uint64_t seconds = 0;
		uint64_t nanoseconds = 0;

		auto err = deserialize(seconds, reader);
		if (err) {
			return err;
		}
		err = deserialize(nanoseconds, reader);
		if (err) {
			return err;
		}

		value.timepoint_ = std::chrono::time_point<std::chrono::system_clock, std::chrono::nanoseconds> (std::chrono::seconds(seconds) + std::chrono::nanoseconds(nanoseconds));
		return nullptr;
	}

	friend std::ostream& operator<<(std::ostream& os, const timestamp& value)
	{
		auto time_t_c = std::chrono::system_clock::to_time_t(value.timepoint_);
		auto tm = *std::localtime(&time_t_c);
		os << std::put_time(&tm, "%Y-%m-%d %H:%M:%S");
		return os;
	}

private:

	std::chrono::time_point<std::chrono::system_clock, std::chrono::nanoseconds> timepoint_;

};

// nlohmann json serialization

inline void to_json(nlohmann::json& j, const timestamp& ts)
{
	j["since_epoch_nano"] = uint64_t(std::chrono::duration_cast<std::chrono::nanoseconds>(ts.timepoint().time_since_epoch()).count());
}

inline void from_json(const nlohmann::json& j, timestamp& ts)
{
	auto since_epoch = std::chrono::nanoseconds{j["since_epoch_nano"].get<uint64_t>()};
	ts.set(std::chrono::time_point<std::chrono::system_clock, std::chrono::nanoseconds>{since_epoch});
}

}
}
