#pragma once

#include <string>
#include <map>

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

	inline uint32_t byteSize() const
	{
		return serialize::byte_size(values_);
	}

	inline void serialize(serialize::FixedSizeWriter& writer) const
	{
		serialize::serialize(writer, values_);
	}

	inline error::Error deserialize(serialize::Reader& reader)
	{
		return serialize::deserialize(values_, reader);
	}

private:

	std::map<std::string, std::string> values_;
};

}
}
