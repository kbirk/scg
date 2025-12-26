#pragma once

#include <array>
#include <map>
#include <string>
#include <vector>
#include <istream>
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
		if (num_bits_to_read == 0) {
			val = 0;
			return nullptr;
		}

		uint32_t srcByteIndex = numBitsRead_ >> 3;
		uint8_t srcBitOffset = numBitsRead_ & 7;

		uint8_t bitsInFirstByte = 8 - srcBitOffset;

		if (num_bits_to_read <= bitsInFirstByte) {
			// Fits in one byte
			uint8_t v;
			if (auto err = readByte(v, srcByteIndex)) return err;
			v >>= srcBitOffset;
			uint8_t mask = (1 << num_bits_to_read) - 1;
			val = v & mask;
		} else {
			// Spans two bytes
			// Read first part
			uint8_t val1;
			if (auto err = readByte(val1, srcByteIndex)) return err;
			val1 >>= srcBitOffset;

			// Read second part
			uint8_t val2;
			if (auto err = readByte(val2, srcByteIndex + 1)) return err;

			uint8_t mask2 = (1 << (num_bits_to_read - bitsInFirstByte)) - 1;
			val = val1 | ((val2 & mask2) << bitsInFirstByte);
		}

		numBitsRead_ += num_bits_to_read;
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

	error::Error readByte(uint8_t& val, uint32_t byteIndex) override
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

	error::Error readByte(uint8_t& val, uint32_t byteIndex) override
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

	error::Error readByte(uint8_t& val, uint32_t byteIndex) override
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
