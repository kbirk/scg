#pragma once

#include <array>
#include <map>
#include <string>
#include <vector>
#include <cassert>
#include <type_traits>

#include "scg/pack.h"
#include "scg/error.h"
#include "scg/serialize.h"

namespace scg {
namespace serialize {

class IReader {
public:

	virtual ~IReader() = default;

	template <typename T>
	inline error::Error read(T& data)
	{
		return deserialize(data, *this);
	}

	template <size_t N>
	error::Error readBits(std::array<uint8_t, N>& val, uint32_t num_bits_to_read)
	{
		uint32_t total_bits_to_read = num_bits_to_read;

		while (num_bits_to_read > 0) {
			uint32_t src_byteIndex = get_byte_offset(numBitsRead_);
			uint8_t src_bit_index =  get_bit_offset(numBitsRead_);
			uint32_t dst_byteIndex = get_byte_offset(total_bits_to_read - num_bits_to_read);
			uint8_t dst_bit_index = get_bit_offset(total_bits_to_read - num_bits_to_read);
			uint32_t src_mask = 1 << src_bit_index;
			uint32_t dst_mask = 1 << dst_bit_index;
			uint8_t val_byte = 0;

			auto err = readByte(val_byte, src_byteIndex);
			if (err) {
				return err;
			}
			if (val_byte & src_mask) {
				val[dst_byteIndex] |= dst_mask;
			}
			numBitsRead_++;
			num_bits_to_read--;
		}

		return nullptr;
	}

	error::Error readBits(uint8_t& val, uint32_t num_bits_to_read)
	{
		std::array<uint8_t, 1> bs = {0};
		auto err = readBits(bs, num_bits_to_read);
		if (err) {
			return err;
		}

		val = bs[0];

		return nullptr;
	}

	error::Error readBits(uint16_t& val, uint32_t num_bits_to_read)
	{
		std::array<uint8_t, 2> bs = {0,0};
		auto err = readBits(bs, num_bits_to_read);
		if (err) {
			return err;
		}

		val =
			(static_cast<uint16_t>(bs[0]) << 8) |
			bs[1];

		return nullptr;
	}

	error::Error readBits(uint32_t& val, uint32_t num_bits_to_read)
	{
		std::array<uint8_t, 4> bs = {0,0,0,0};
		auto err = readBits(bs, num_bits_to_read);
		if (err) {
			return err;
		}

		val =
			(static_cast<uint32_t>(bs[0]) << 24) |
			(static_cast<uint32_t>(bs[1]) << 16) |
			(static_cast<uint32_t>(bs[2]) << 8) |
			bs[3];

		return nullptr;
	}

	error::Error readBits(uint64_t& val, uint32_t num_bits_to_read)
	{
		std::array<uint8_t, 8> bs = {0,0,0,0,0,0,0,0};
		auto err = readBits(bs, num_bits_to_read);
		if (err) {
			return err;
		}

		val =
			(static_cast<uint64_t>(bs[0]) << 56) |
			(static_cast<uint64_t>(bs[1]) << 48) |
			(static_cast<uint64_t>(bs[2]) << 40) |
			(static_cast<uint64_t>(bs[3]) << 32) |
			(static_cast<uint64_t>(bs[4]) << 24) |
			(static_cast<uint64_t>(bs[5]) << 16) |
			(static_cast<uint64_t>(bs[6]) << 8) |
			bs[7];

		return nullptr;
	}

protected:

	virtual error::Error readByte(uint8_t& val, uint32_t byteIndex) = 0;

	uint32_t numBitsRead_ = 0;

};


class ReaderView : public IReader {
public:

	inline ReaderView(const uint8_t* data, uint32_t size)
		: bytes_(data)
		, size_(size)
	{
	}

	inline explicit ReaderView(const std::vector<uint8_t>& data)
		: bytes_(&data[0])
		, size_(data.size())
	{
	}

protected:

	error::Error readByte(uint8_t& val, uint32_t byteIndex)
	{
		if (byteIndex >= size_) {
			return error::Error("Reader does not contain enough data to fill the argument");
		}
		val = bytes_[byteIndex];
		return nullptr;
	}

private:

	const uint8_t* bytes_;
	uint32_t size_;
};

class Reader : public IReader {
public:

	inline explicit Reader(const std::vector<uint8_t>& data)
		: bytes_(data)
	{
	}

protected:

	error::Error readByte(uint8_t& val, uint32_t byteIndex)
	{
		if (byteIndex >= bytes_.size()) {
			return error::Error("Reader does not contain enough data to fill the argument");
		}
		val = bytes_[byteIndex];
		return nullptr;
	}

private:

	std::vector<uint8_t> bytes_;
};

class StreamReader : scg::serialize::IReader {
public:

	StreamReader(std::istream& stream)
		: stream_(stream)
	{
	}

protected:

	error::Error readByte(uint8_t& val, uint32_t byteIndex)
	{
		if (byteIndex > currentIndex_) {
			currentIndex_ = byteIndex;

			stream_.read(reinterpret_cast<char*>(&currentByte_), 1);
			if (stream_.fail()) {
				return error::Error("Failed to read byte from stream");
			}
		}
		return nullptr;
	}

private:
	std::istream& stream_;
	uint8_t currentByte_ = 0;
	int64_t currentIndex_ = -1;
};

}
}
