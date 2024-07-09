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

	error::Error readBits(uint8_t& val, uint32_t num_bits_to_read)
	{
		uint32_t total_bits_to_read = num_bits_to_read;

		val = 0;

		while (num_bits_to_read > 0) {
			uint32_t src_byteIndex = get_byte_offset(numBitsRead_);
			uint8_t src_bit_index =  get_bit_offset(numBitsRead_);
			uint8_t dst_bit_index = get_bit_offset(total_bits_to_read - num_bits_to_read);
			uint32_t src_mask = 1 << src_bit_index;
			uint32_t dst_mask = 1 << dst_bit_index;
			uint8_t val_byte = 0;

			auto err = readByte(val_byte, src_byteIndex);
			if (err) {
				return err;
			}
			if (val_byte & src_mask) {
				val |= dst_mask;
			}
			numBitsRead_++;
			num_bits_to_read--;
		}

		return nullptr;
	}

protected:

	virtual error::Error readByte(uint8_t& val, uint32_t byteIndex) = 0;

	uint32_t numBitsRead_ = 0;

};


class ReaderView : public IReader {
public:

	using IReader::read;
	using IReader::readBits;

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
			return error::Error("ReaderView does not contain enough data to fill the argument: " + std::to_string(byteIndex) + " >= " + std::to_string(size_) + " bytes");
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

	using IReader::read;
	using IReader::readBits;

	inline explicit Reader(const std::vector<uint8_t>& data)
		: bytes_(data)
	{
	}

protected:

	error::Error readByte(uint8_t& val, uint32_t byteIndex)
	{
		if (byteIndex >= bytes_.size()) {
			return error::Error("Reader does not contain enough data to fill the argument: " + std::to_string(byteIndex) + " >= " + std::to_string(bytes_.size()) + " bytes");
		}
		val = bytes_[byteIndex];
		return nullptr;
	}

private:

	std::vector<uint8_t> bytes_;
};

class StreamReader : scg::serialize::IReader {
public:

	using IReader::read;
	using IReader::readBits;

	StreamReader(std::istream& stream)
		: stream_(stream)
	{
	}

protected:

	error::Error readByte(uint8_t& val, uint32_t byteIndex)
	{
		if (byteIndex > currentIndex_) {
			assert((byteIndex == currentIndex_ + 1) && "StreamReader::readByte: byteIndex must be incremented by 1");
			currentIndex_ = byteIndex;

			stream_.read(reinterpret_cast<char*>(&currentByte_), 1);
			if (stream_.fail()) {
				return error::Error("Failed to read byte from stream");
			}
		}

		val = currentByte_;
		return nullptr;
	}

private:
	std::istream& stream_;
	uint8_t currentByte_ = 0;
	int64_t currentIndex_ = -1;
};

}
}
