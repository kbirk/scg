#pragma once

#include <string>
#include <map>

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

	inline void put(const std::string& key, const std::string& val)
	{
		values_[key] = val;
	}

	inline void put(const std::map<std::string, std::string>& values)
	{
		values_.insert(values.begin(), values.end());
	}

	inline std::string get(const std::string& key)
	{
		if (values_.find(key) == values_.end()) {
			return "";
		}
		return values_[key];
	}

	inline std::map<std::string, std::string> get()
	{
		return values_;
	}

	friend inline uint32_t byte_size(const Context& ctx)
	{
		return serialize::byte_size(ctx.values_);
	}

	template <typename WriterType>
	friend inline void serialize(WriterType& writer, const Context& ctx)
	{
		serialize::serialize(writer, ctx.values_);
	}

	template <typename ReaderType>
	friend inline error::Error deserialize(Context& ctx, ReaderType& reader)
	{
		return serialize::deserialize(ctx.values_, reader);
	}

private:

	std::map<std::string, std::string> values_;
};

}
}
