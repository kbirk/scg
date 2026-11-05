#pragma once

#include <chrono>
#include <cstdint>
#include <ctime>
#include <iomanip>
#include <ostream>
#include <string>

#include "scg/serialize.h"

#ifndef SCG_MINIMAL_INCLUDES
// Kept for source compatibility with code that relies on this transitive
// include; define SCG_MINIMAL_INCLUDES to skip it (the JSON hooks below are
// templates and do not need it).
#include "nlohmann/json.hpp"
#endif

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

	// Wire format: uint64(seconds) then uint64(nanoseconds), where seconds is the
	// floor of the time since the epoch and nanoseconds is in [0, 1e9). This is
	// the canonical form Go produces (time.Unix / Nanosecond). Pre-1970 values
	// therefore have negative seconds (two's complement) and non-negative
	// nanoseconds. The decoder also accepts the legacy C++ truncated form
	// (nanoseconds in (-1e9, 0)). Instants outside the int64 nanosecond range
	// saturate on decode; the minimum is encoded as Go's zero time.
	friend inline uint32_t bit_size(const timestamp& value)
	{
		using scg::serialize::bit_size;

		int64_t seconds = 0;
		int64_t nanoseconds = 0;
		split(value.timepoint_, seconds, nanoseconds);
		return bit_size(uint64_t(seconds)) + bit_size(uint64_t(nanoseconds));
	}

	template <typename WriterType>
	friend inline void serialize(WriterType& writer, const timestamp& value)
	{
		using scg::serialize::serialize;

		int64_t seconds = 0;
		int64_t nanoseconds = 0;
		split(value.timepoint_, seconds, nanoseconds);
		serialize(writer, uint64_t(seconds));
		serialize(writer, uint64_t(nanoseconds));
	}

	template <typename ReaderType>
	friend inline error::Error deserialize(timestamp& value, ReaderType& reader)
	{
		using scg::serialize::deserialize;

		uint64_t rawSeconds = 0;
		uint64_t rawNanoseconds = 0;

		auto err = deserialize(rawSeconds, reader);
		if (err) {
			return err;
		}
		err = deserialize(rawNanoseconds, reader);
		if (err) {
			return err;
		}

		// Two's complement reinterpretation (both encoders write int64 values).
		int64_t seconds = static_cast<int64_t>(rawSeconds);
		int64_t nanoseconds = static_cast<int64_t>(rawNanoseconds);

		value.timepoint_ = std::chrono::time_point<std::chrono::system_clock, std::chrono::nanoseconds>(
			std::chrono::nanoseconds(saturatedNanoseconds(seconds, nanoseconds)));
		return nullptr;
	}

	friend std::ostream& operator<<(std::ostream& os, const timestamp& value)
	{
		// to_time_t wants a system_clock::time_point. timepoint_ is nanosecond-precision;
		// libstdc++'s system_clock::duration is already nanoseconds (implicit match), but
		// MSVC's is 100ns ticks, so cast to the clock's native duration explicitly.
		auto time_t_c = std::chrono::system_clock::to_time_t(
			std::chrono::time_point_cast<std::chrono::system_clock::duration>(value.timepoint_));
		// localtime() returns a pointer to shared static storage; use the
		// thread-safe variants.
		std::tm tm{};
#if defined(_WIN32)
		localtime_s(&tm, &time_t_c);
#else
		localtime_r(&time_t_c, &tm);
#endif
		os << std::put_time(&tm, "%Y-%m-%d %H:%M:%S");
		return os;
	}

private:

	static constexpr int64_t NANOS_PER_SECOND = 1000000000;

	// toNanoseconds combines seconds and nanoseconds (canonical [0, 1e9) or the
	// legacy truncated (-1e9, 1e9) form) into int64 nanoseconds since the epoch.
	// Returns false when the result is not representable, without overflowing.
	// saturatedNanoseconds converts any (seconds, nanoseconds) pair to int64
	// nanoseconds since the epoch, clamping values outside the representable
	// range (about +/-292 years around 1970) to the nearest limit instead of
	// failing. Go peers routinely send such values: Go's zero time.Time is year 1,
	// so rejecting them would fail the whole message. Decoding therefore never
	// errors on the timestamp value itself. Uses only portable integer math.
	static inline int64_t saturatedNanoseconds(int64_t seconds, int64_t nanoseconds)
	{
		// Fold whole seconds out of a non-canonical nanoseconds field.
		const int64_t carry = nanoseconds / NANOS_PER_SECOND;
		nanoseconds -= carry * NANOS_PER_SECOND; // now in (-1e9, 1e9)
		if (carry > 0 && seconds > INT64_MAX - carry) {
			return INT64_MAX;
		}
		if (carry < 0 && seconds < INT64_MIN - carry) {
			return INT64_MIN;
		}
		seconds += carry;

		int64_t total = 0;
		if (toNanoseconds(seconds, nanoseconds, total)) {
			return total;
		}
		// |nanoseconds| < 1e9, so only the seconds magnitude can be out of
		// range, and its sign gives the direction.
		return seconds < 0 ? INT64_MIN : INT64_MAX;
	}

	static inline bool toNanoseconds(int64_t seconds, int64_t nanoseconds, int64_t& total)
	{
		if (nanoseconds <= -NANOS_PER_SECOND || nanoseconds >= NANOS_PER_SECOND) {
			return false;
		}
		// floor(INT64_MIN / 1e9) and floor(INT64_MAX / 1e9)
		constexpr int64_t minSeconds = -9223372037LL;
		constexpr int64_t maxSeconds = 9223372036LL;
		if (seconds < minSeconds - 1 || seconds > maxSeconds + 1) {
			return false;
		}
		// Normalize to the canonical form: nanoseconds in [0, 1e9).
		if (nanoseconds < 0) {
			nanoseconds += NANOS_PER_SECOND;
			seconds -= 1;
		}
		if (seconds < minSeconds || seconds > maxSeconds) {
			return false;
		}
		if (seconds == maxSeconds) {
			if (nanoseconds > INT64_MAX - maxSeconds * NANOS_PER_SECOND) {
				return false;
			}
			total = seconds * NANOS_PER_SECOND + nanoseconds;
		} else if (seconds == minSeconds) {
			// seconds * 1e9 alone would overflow; fold one second into the
			// (non-positive) nanosecond part first.
			int64_t rest = nanoseconds - NANOS_PER_SECOND; // in [-1e9, 0)
			int64_t head = (seconds + 1) * NANOS_PER_SECOND;
			if (rest < INT64_MIN - head) {
				return false;
			}
			total = head + rest;
		} else {
			total = seconds * NANOS_PER_SECOND + nanoseconds;
		}
		return true;
	}

	// the Unix time of Go's zero time.Time, 0001-01-01T00:00:00Z
	static constexpr int64_t GO_ZERO_TIME_UNIX_SECONDS = -62135596800LL;

	// split computes floor(seconds) and nanoseconds in [0, 1e9). The minimum
	// timepoint is where the decoder saturates everything earlier, including
	// Go's zero time.Time, so it is written as Go's zero time: a Go peer's
	// "unset" time survives a round trip through C++ (scg/json.h maps it the
	// same way).
	static inline void split(const std::chrono::time_point<std::chrono::system_clock, std::chrono::nanoseconds>& tp, int64_t& seconds, int64_t& nanoseconds)
	{
		int64_t total = std::chrono::duration_cast<std::chrono::nanoseconds>(tp.time_since_epoch()).count();
		if (total == INT64_MIN) {
			seconds = GO_ZERO_TIME_UNIX_SECONDS;
			nanoseconds = 0;
			return;
		}
		seconds = total / NANOS_PER_SECOND;
		nanoseconds = total % NANOS_PER_SECOND;
		if (nanoseconds < 0) {
			nanoseconds += NANOS_PER_SECOND;
			seconds -= 1;
		}
	}

	std::chrono::time_point<std::chrono::system_clock, std::chrono::nanoseconds> timepoint_;

};

// nlohmann json serialization. Templated on the json type so this header does
// not need to include nlohmann/json.hpp; nlohmann finds these via ADL.

template <typename BasicJsonType>
inline void to_json(BasicJsonType& j, const timestamp& ts)
{
	j["since_epoch_nano"] = uint64_t(std::chrono::duration_cast<std::chrono::nanoseconds>(ts.timepoint().time_since_epoch()).count());
}

template <typename BasicJsonType>
inline void from_json(const BasicJsonType& j, timestamp& ts)
{
	auto since_epoch = std::chrono::nanoseconds{static_cast<int64_t>(j.at("since_epoch_nano").template get<uint64_t>())};
	ts.set(std::chrono::time_point<std::chrono::system_clock, std::chrono::nanoseconds>{since_epoch});
}

}
}
