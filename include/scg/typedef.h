#pragma once

#include <functional>

#include <scg/serialize.h>

#include "nlohmann/json.hpp"

namespace scg {
namespace type {

template <typename T, class Tag>
class strong_typedef {
public:
	constexpr strong_typedef()
		: value_()
	{
	}

	constexpr explicit strong_typedef(T value)
		: value_(value)
	{
	}

	operator T&() noexcept
	{
		return value_;
	}

	operator const T&() const noexcept
	{
		return value_;
	}

	strong_typedef<T, Tag>& operator++()
	{
		value_++;
		return *this;
	}

	strong_typedef<T, Tag> operator++(int)
	{
		strong_typedef<T, Tag> old = *this;
		value_++;
		return old;
	}

	strong_typedef<T, Tag>& operator--()
	{
		value_--;
		return *this;
	}

	strong_typedef<T, Tag> operator--(int)
	{
		strong_typedef<T, Tag> old = *this;
		value_--;
		return old;
	}

	template <typename S>
	strong_typedef<T, Tag>& operator+=(const S& s)
	{
		value_ += s;
		return *this;
	}

	template <typename S>
	strong_typedef<T, Tag>& operator-=(const S& s)
	{
		value_ -= s;
		return *this;
	}

	bool operator==(const strong_typedef<T, Tag>& x) const
	{
		return value_ == x.value_;
	}

	bool operator!=(const strong_typedef<T, Tag>& x) const
	{
		return value_ != x.value_;
	}

	bool operator<(const strong_typedef<T, Tag>& x) const
	{
		return value_ < x.value_;
	}

	bool operator>(const strong_typedef<T, Tag>& x) const
	{
		return value_ > x.value_;
	}

	bool operator<=(const strong_typedef<T, Tag>& x) const
	{
		return value_ <= x.value_;
	}

	bool operator>=(const strong_typedef<T, Tag>& x) const
	{
		return value_ >= x.value_;
	}

	friend strong_typedef operator&(const strong_typedef& lhs, const strong_typedef& rhs)
	{
		return strong_typedef(lhs.value_ & rhs.value_);
	}

	friend strong_typedef operator|(const strong_typedef& lhs, const strong_typedef& rhs)
	{
		return strong_typedef(lhs.value_ | rhs.value_);
	}

	friend strong_typedef operator^(const strong_typedef& lhs, const strong_typedef& rhs)
	{
		return strong_typedef(lhs.value_ ^ rhs.value_);
	}

	friend strong_typedef operator~(const strong_typedef& lhs)
	{
		return strong_typedef(~lhs.value_);
	}

	friend strong_typedef operator<<(const strong_typedef& lhs, int32_t shift)
	{
		return strong_typedef(lhs.value_ << shift);
	}

	friend strong_typedef operator>>(const strong_typedef& lhs, int32_t shift)
	{
		return strong_typedef(lhs.value_ >> shift);
	}

	friend void swap(strong_typedef& a, strong_typedef& b) noexcept
	{
		using std::swap;
		swap(static_cast<T&>(a), static_cast<T&>(b));
	}

	friend std::ostream& operator<<(std::ostream& os, const strong_typedef<T, Tag>& x)
	{
		os << x.value_;
		return os;
	}

	inline friend std::istream& operator>>(std::istream& is, strong_typedef<T, Tag>& x)
	{
		is >> x.value_;
		return is;
	}

	inline uint32_t byteSize() const
	{
		return scg::serialize::byte_size(value_);
	}

	template <typename WriterType>
	void serialize(WriterType& writer) const
	{
		serialize::serialize(writer, value_);
	}

	template <typename ReaderType>
	error::Error deserialize(ReaderType& reader)
	{
		return serialize::deserialize(value_, reader);
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

	T value_;
};

template<typename T, typename Tag, typename S>
strong_typedef<T, Tag> operator+(strong_typedef<T, Tag> lhs, const S& rhs)
{
	lhs += rhs;
	return lhs;
}

template<typename T, typename Tag, typename S>
strong_typedef<T, Tag> operator-(strong_typedef<T, Tag> lhs, const S& rhs)
{
	lhs -= rhs;
	return lhs;
}

}
}

template <typename T, class Tag>
struct std::hash<scg::type::strong_typedef<T, Tag>> {
	std::size_t operator()(const scg::type::strong_typedef<T, Tag>& t) const
	{
		return std::hash<T>()(static_cast<T>(t));
	}
};

#define SCG_TYPEDEF(N, T) using N = scg::type::strong_typedef<T, struct N##_>

namespace nlohmann {

	template <typename T, typename Tag>
	struct adl_serializer<scg::type::strong_typedef<T, Tag>>
	{
		static void to_json(json& j, const scg::type::strong_typedef<T, Tag>& type)
		{
			std::stringstream ss;
			ss << type;
			j = ss.str();
		}

		static void from_json(const json& j, scg::type::strong_typedef<T, Tag>& type)
		{
			auto str = j.get<std::string>();
			std::stringstream ss(str);
			ss >> type;
		}
	};
}
