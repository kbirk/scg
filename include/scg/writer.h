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

class FixedSizeWriter {
public:

	FixedSizeWriter(uint32_t size)
	{
		bytes_.reserve(size);
	}

	void write(uint8_t data)
	{
		bytes_.push_back(data);
	}

	template <std::size_t N>
	void write(const std::array<uint8_t, N>& data)
	{
		bytes_.insert(bytes_.end(), data.begin(), data.end());
	}

	void write(const uint8_t* data, uint32_t n)
	{
		bytes_.insert(bytes_.end(), data, data + n);
	}

	template <typename T>
	void write(const T& data)
	{
		serialize(*this, data);
	}

	const std::vector<uint8_t>& bytes() const
	{
		assert(bytes_.size() == bytes_.capacity() && std::string("FixedSizeWriter::bytes() called before all data was written" + (std::to_string(bytes_.size()) + " != " + std::to_string(bytes_.capacity()))).c_str());

		return bytes_;
	}

private:

	std::vector<uint8_t> bytes_;
};


class WriterView {
public:

	WriterView(std::vector<uint8_t>& data)
		: bytes_(data)
	{
	}

	void write(uint8_t data)
	{
		bytes_[pos_++] = data;
	}

	template <std::size_t N>
	void write(const std::array<uint8_t, N>& data)
	{
		std::copy(data.begin(), data.end(), bytes_.begin() + pos_);
		pos_ += N;
	}

	void write(const uint8_t* data, uint32_t n)
	{
		std::copy(data, data + n, bytes_.begin() + pos_);
		pos_ += n;
	}

	template <typename T>
	void write(const T& data)
	{
		serialize(*this, data);
	}

	const std::vector<uint8_t>& bytes() const
	{
		return bytes_;
	}

private:

	std::vector<uint8_t>& bytes_;
	size_t pos_ = 0;
};


}
}
