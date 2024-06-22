#pragma once

#include <array>
#include <map>
#include <string>
#include <vector>
#include <cassert>
#include <type_traits>

#include "scg/pack.h"
#include "scg/error.h"

namespace scg {
namespace serialize {

class ReaderView {
public:

	ReaderView(const uint8_t* data, uint32_t size)
		: bytes_(data)
		, size_(size)
	{
	}

	ReaderView(const std::vector<uint8_t>& data)
		: bytes_(&data[0])
		, size_(data.size())
	{
	}

	template <std::size_t N>
	error::Error read(std::array<uint8_t, N>& dest)
	{
		if (pos_ + N > size_) {
			return error::Error("Reader does not contain enough data to fill the argument");
		}

		std::copy(bytes_ + pos_, bytes_ + pos_ + N, dest.begin());
		pos_ += N;
		return nullptr;
	}

	error::Error read(uint8_t* dest, uint32_t n)
	{
		if (pos_ + n > size_) {
			return error::Error("Reader does not contain enough data to fill the argument");
		}

		std::copy(bytes_ + pos_, bytes_ + pos_ + n, dest);
		pos_ += n;
		return nullptr;
	}

	error::Error read(std::vector<uint8_t>& dest, uint32_t n)
	{
		if  (pos_ + n > size_) {
			return error::Error("Reader does not contain enough data to fill the argument");
		}

		dest.insert(dest.end(), bytes_ + pos_, bytes_ + pos_ + n);
		pos_ += n;
		return nullptr;
	}

	template <typename T>
	error::Error read(T& data)
	{
		return deserialize(data, *this);
	}

private:

	const uint8_t* bytes_;
	uint32_t size_;
	size_t pos_ = 0;
};

class Reader {
public:

	Reader(const std::vector<uint8_t>& data)
		: bytes_(data)
	{
	}

	template <std::size_t N>
	error::Error read(std::array<uint8_t, N>& dest)
	{
		if (pos_ + N > bytes_.size()) {
			return error::Error("Reader does not contain enough data to fill the argument");
		}

		std::copy(bytes_.begin() + pos_, bytes_.begin() + pos_ + N, dest.begin());
		pos_ += N;
		return nullptr;
	}

	error::Error read(uint8_t* dest, uint32_t n)
	{
		if (pos_ + n > bytes_.size()) {
			return error::Error("Reader does not contain enough data to fill the argument");
		}

		std::copy(bytes_.begin() + pos_, bytes_.begin() + pos_ + n, dest);
		pos_ += n;
		return nullptr;
	}

	error::Error read(std::vector<uint8_t>& dest, uint32_t n)
	{
		if  (pos_ + n > bytes_.size()) {
			return error::Error("Reader does not contain enough data to fill the argument");
		}

		dest.insert(dest.end(), bytes_.begin() + pos_, bytes_.begin() + pos_ + n);
		pos_ += n;
		return nullptr;
	}

	template <typename T>
	error::Error read(T& data)
	{
		return deserialize(data, *this);
	}

private:

	std::vector<uint8_t> bytes_;
	size_t pos_ = 0;
};

}
}
