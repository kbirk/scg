#pragma once

#include <chrono>

#include "scg/serialize.h"
#include "scg/writer.h"
#include "scg/reader.h"

#include "nlohmann/json.hpp"

namespace scg {

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

	inline uint32_t byteSize() const
	{
		return 16;
	}

	template <typename WriterType>
	void serialize(WriterType& writer) const
	{
		auto duration_since_epoch = timepoint_.time_since_epoch();
		auto seconds = std::chrono::duration_cast<std::chrono::seconds>(duration_since_epoch);
		auto nanoseconds = std::chrono::duration_cast<std::chrono::nanoseconds>(duration_since_epoch - seconds);

		serialize::serialize(writer, uint64_t(seconds.count()));
		serialize::serialize(writer, uint64_t(nanoseconds.count()));
	}

	template <typename ReaderType>
	error::Error deserialize(ReaderType& reader)
	{
		uint64_t seconds = 0;
		uint64_t nanoseconds = 0;

		auto err = serialize::deserialize(seconds, reader);
		if (err) {
			return err;
		}
		err = serialize::deserialize(nanoseconds, reader);
		if (err) {
			return err;
		}

		timepoint_ = std::chrono::time_point<std::chrono::system_clock, std::chrono::nanoseconds> (std::chrono::seconds(seconds) + std::chrono::nanoseconds(nanoseconds));
		return nullptr;
	}

	inline std::vector<uint8_t> toBytes() const
	{
		std::vector<uint8_t> data(byteSize());
		scg::serialize::WriterView writer(data);
		serialize(writer);
		return data;
	}

	inline error::Error fromBytes(const std::vector<uint8_t>& bytes)
	{
		scg::serialize::ReaderView reader(bytes);
		return deserialize(reader);
	}

private:

	std::chrono::time_point<std::chrono::system_clock, std::chrono::nanoseconds> timepoint_;

};

}

namespace nlohmann {

	template <>
	struct adl_serializer<scg::timestamp>
	{
		static void to_json(json& j, const scg::timestamp& timestamp)
		{
			j["since_epoch_nano"] = uint64_t(std::chrono::duration_cast<std::chrono::nanoseconds>(timestamp.timepoint().time_since_epoch()).count());
		}

		static void from_json(const json& j, scg::timestamp& timestamp)
		{
			auto since_epoch = std::chrono::nanoseconds{j["since_epoch_nano"].get<uint64_t>()};
			timestamp.set(std::chrono::time_point<std::chrono::system_clock, std::chrono::nanoseconds>{since_epoch});
		}
	};
}
