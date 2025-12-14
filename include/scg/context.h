#pragma once

#include <string>
#include <map>
#include <chrono>

#include "scg/serialize.h"
#include "scg/error.h"

namespace scg {
namespace context {

class Context {
public:

	Context() : hasDeadline_(false)
	{
	}

	static Context background()
	{
		return Context();
	}

	void setDeadline(std::chrono::system_clock::time_point deadline) {
		deadline_ = deadline;
		hasDeadline_ = true;
	}

	bool hasDeadline() const {
		return hasDeadline_;
	}

	std::chrono::system_clock::time_point getDeadline() const {
		return deadline_;
	}

	inline void put(const std::string& key, const std::vector<uint8_t>& val)
	{
		values_[key] = val;
	}

	inline void put(const std::string& key, const char* val)
	{
		using scg::serialize::serialize;
		using scg::serialize::bit_size;

		std::string str(val);

		auto size = bit_size(str);

		std::vector<uint8_t> data;
		data.reserve(scg::serialize::bits_to_bytes(size));
		scg::serialize::WriterView writer(data);
		serialize(writer, str);

		put(key, data);
	}

	template <typename T>
	inline void put(const std::string& key, const T& val)
	{
		using scg::serialize::serialize;
		using scg::serialize::bit_size;

		auto size = bit_size(val);

		std::vector<uint8_t> data;
		data.reserve(scg::serialize::bits_to_bytes(size));
		scg::serialize::WriterView writer(data);
		serialize(writer, val);

		put(key, data);
	}

	inline scg::error::Error get(std::string& t, const std::string& key) const
	{
		using scg::serialize::deserialize;

		auto it = values_.find(key);
		if (it == values_.end()) {
			return scg::error::Error("Key `" + key + "` not found");
		}
		const auto& bs = it->second;
		scg::serialize::ReaderView reader(bs);
		return deserialize(t, reader);
	}

	template <typename T>
	inline scg::error::Error get(T& t, const std::string& key) const
	{
		using scg::serialize::deserialize;

		auto it = values_.find(key);
		if (it == values_.end()) {
			return scg::error::Error("Key `" + key + "` not found");
		}
		const auto& bs = it->second;
		scg::serialize::ReaderView reader(bs);
		return deserialize(t, reader);
	}

	friend inline uint32_t bit_size(const Context& ctx)
	{
		using scg::serialize::bit_size; // adl trickery

		return bit_size(ctx.values_);
	}

	template <typename WriterType>
	friend inline void serialize(WriterType& writer, const Context& ctx)
	{
		using scg::serialize::serialize;

		serialize(writer, ctx.values_);
	}

	template <typename ReaderType>
	friend inline error::Error deserialize(Context& ctx, ReaderType& reader)
	{
		using scg::serialize::deserialize;

		return deserialize(ctx.values_, reader);
	}

private:

	std::map<std::string, std::vector<uint8_t>> values_;
	std::chrono::system_clock::time_point deadline_;
	bool hasDeadline_;
};

}
}
