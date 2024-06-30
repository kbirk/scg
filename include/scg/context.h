#pragma once

#include <string>
#include <map>

#include "scg/serialize.h"
#include "scg/error.h"

namespace scg {
namespace context {

class Context {
public:

	Context()
	{
	}

	static Context background()
	{
		return Context();
	}

	inline void put(const std::string& key, const std::vector<uint8_t>& val)
	{
		values_[key] = val;
	}

	inline void put(const std::string& key, const char* val)
	{
		using scg::serialize::serialize;
		using scg::serialize::byte_size;

		std::string str(val);

		auto size = byte_size(str);

		std::vector<uint8_t> data;
		data.reserve(size);
		scg::serialize::WriterView writer(data);
		serialize(writer, str);

		put(key, data);
	}

	template <typename T>
	inline void put(const std::string& key, const T& val)
	{
		using scg::serialize::serialize;
		using scg::serialize::byte_size;

		auto size = byte_size(val);

		std::vector<uint8_t> data;
		data.reserve(size);
		scg::serialize::WriterView writer(data);
		serialize(writer, val);

		put(key, data);
	}

	inline scg::error::Error get(std::string& t, const std::string& key)
	{
		using scg::serialize::deserialize;

		if (values_.find(key) == values_.end()) {
			return scg::error::Error("Key `" + key + "` not found");
		}
		auto& bs = values_[key];
		scg::serialize::ReaderView reader(bs);
		return deserialize(t, reader);
	}

	template <typename T>
	inline scg::error::Error get(T& t, const std::string& key)
	{
		using scg::serialize::deserialize;

		if (values_.find(key) == values_.end()) {
			return scg::error::Error("Key `" + key + "` not found");
		}
		auto& bs = values_[key];
		scg::serialize::ReaderView reader(bs);
		return deserialize(t, reader);
	}

	friend inline uint32_t byte_size(const Context& ctx)
	{
		using scg::serialize::byte_size;

		return byte_size(ctx.values_);
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
};

}
}
