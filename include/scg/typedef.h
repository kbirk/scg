#pragma once

#include <functional>

#include "scg/serialize.h"

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

	constexpr operator T&() noexcept
	{
		return value_;
	}

	constexpr operator const T&() const noexcept
	{
		return value_;
	}

	constexpr strong_typedef<T, Tag>& operator++()
	{
		value_++;
		return *this;
	}

	constexpr strong_typedef<T, Tag> operator++(int)
	{
		strong_typedef<T, Tag> old = *this;
		value_++;
		return old;
	}

	constexpr strong_typedef<T, Tag>& operator--()
	{
		value_--;
		return *this;
	}

	constexpr strong_typedef<T, Tag> operator--(int)
	{
		strong_typedef<T, Tag> old = *this;
		value_--;
		return old;
	}

	template <typename S>
	constexpr strong_typedef<T, Tag>& operator+=(const S& s)
	{
		value_ += s;
		return *this;
	}

	template <typename S>
	constexpr strong_typedef<T, Tag>& operator-=(const S& s)
	{
		value_ -= s;
		return *this;
	}

	constexpr bool operator==(const strong_typedef<T, Tag>& x) const
	{
		return value_ == x.value_;
	}

	constexpr bool operator!=(const strong_typedef<T, Tag>& x) const
	{
		return value_ != x.value_;
	}

	constexpr bool operator<(const strong_typedef<T, Tag>& x) const
	{
		return value_ < x.value_;
	}

	constexpr bool operator>(const strong_typedef<T, Tag>& x) const
	{
		return value_ > x.value_;
	}

	constexpr bool operator<=(const strong_typedef<T, Tag>& x) const
	{
		return value_ <= x.value_;
	}

	constexpr bool operator>=(const strong_typedef<T, Tag>& x) const
	{
		return value_ >= x.value_;
	}

	friend constexpr strong_typedef operator&(const strong_typedef& lhs, const strong_typedef& rhs)
	{
		return strong_typedef(lhs.value_ & rhs.value_);
	}

	friend constexpr strong_typedef operator|(const strong_typedef& lhs, const strong_typedef& rhs)
	{
		return strong_typedef(lhs.value_ | rhs.value_);
	}

	friend constexpr strong_typedef operator^(const strong_typedef& lhs, const strong_typedef& rhs)
	{
		return strong_typedef(lhs.value_ ^ rhs.value_);
	}

	friend constexpr strong_typedef operator~(const strong_typedef& lhs)
	{
		return strong_typedef(~lhs.value_);
	}

	friend constexpr strong_typedef operator<<(const strong_typedef& lhs, int32_t shift)
	{
		return strong_typedef(lhs.value_ << shift);
	}

	friend constexpr strong_typedef operator>>(const strong_typedef& lhs, int32_t shift)
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

	friend inline uint32_t bit_size(const strong_typedef<T, Tag>& value)
	{
		using scg::serialize::bit_size; // adl trickery
		return bit_size(value.value_);
	}

	template <typename WriterType>
	friend inline void serialize(WriterType& writer, const strong_typedef<T, Tag>& value)
	{
		using scg::serialize::serialize;  // adl trickery
		serialize(writer, value.value_);
	}

	template <typename ReaderType>
	friend inline error::Error deserialize(strong_typedef<T, Tag>& value, ReaderType& reader)
	{
		using scg::serialize::deserialize;  // adl trickery
		return deserialize(value.value_, reader);
	}

private:

	T value_;
};

template<typename T, typename Tag, typename S>
constexpr strong_typedef<T, Tag> operator+(strong_typedef<T, Tag> lhs, const S& rhs)
{
	lhs += rhs;
	return lhs;
}

template<typename T, typename Tag, typename S>
constexpr strong_typedef<T, Tag> operator-(strong_typedef<T, Tag> lhs, const S& rhs)
{
	lhs -= rhs;
	return lhs;
}

// nlohmann json serialization

template <typename T, typename Tag>
inline void to_json(nlohmann::json& j, const scg::type::strong_typedef<T, Tag>& type)
{
	std::stringstream ss;
	ss << type;
	j = ss.str();
}

template <typename T, typename Tag>
inline void from_json(const nlohmann::json& j, scg::type::strong_typedef<T, Tag>& type)
{
	auto str = j.get<std::string>();
	std::stringstream ss(str);
	ss >> type;
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
