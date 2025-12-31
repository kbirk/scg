#pragma once

#include <array>
#include <map>
#include <string>
#include <vector>
#include <istream>
#include <cassert>
#include <type_traits>
#include <cstring>

#include "scg/pack.h"
#include "scg/error.h"
#include "scg/serialize.h"

namespace scg {
namespace serialize {

class ReaderView {
public:

	inline explicit ReaderView(const std::vector<uint8_t>& data)
		: bytes_(data.data())
		, size_(data.size())
	{
	}

	inline ReaderView(const uint8_t* data, uint32_t size)
		: bytes_(data)
		, size_(size)
	{
	}

	template <typename T>
	inline error::Error read(T& data)
	{
		return deserialize(data, *this);
	}

	error::Error readBits(uint8_t& val, uint32_t num_bits_to_read)
	{
		assert(num_bits_to_read <= 8 && "ReaderView::readBits only supports reading up to 8 bits at a time");

		if (num_bits_to_read == 0) {
			val = 0;
			return nullptr;
		}

		uint32_t srcByteIndex = numBitsRead_ >> 3;
		uint8_t srcBitOffset = numBitsRead_ & 7;

		uint8_t bitsInFirstByte = 8 - srcBitOffset;

		if (num_bits_to_read <= bitsInFirstByte) {
			// Fits in one byte
			if (srcByteIndex >= size_) {
				return error::Error("Reader does not contain enough data to fill the argument");
			}
			uint8_t v = bytes_[srcByteIndex];
			v >>= srcBitOffset;
			uint8_t mask = (1 << num_bits_to_read) - 1;
			val = v & mask;
		} else {
			// Spans two bytes
			if (srcByteIndex + 1 >= size_) {
				return error::Error("Reader does not contain enough data to fill the argument");
			}
			// Read first part
			uint8_t val1 = bytes_[srcByteIndex];
			val1 >>= srcBitOffset;

			// Read second part
			uint8_t val2 = bytes_[srcByteIndex + 1];

			uint8_t mask2 = (1 << (num_bits_to_read - bitsInFirstByte)) - 1;
			val = val1 | ((val2 & mask2) << bitsInFirstByte);
		}

		numBitsRead_ += num_bits_to_read;
		return nullptr;
	}

	error::Error readByte(uint8_t& val)
	{
		if ((numBitsRead_ & 7) == 0) {
			uint32_t srcByteIndex = numBitsRead_ >> 3;
			if (srcByteIndex >= size_) {
				return error::Error("Reader does not contain enough data to fill the argument");
			}
			val = bytes_[srcByteIndex];
			numBitsRead_ += 8;
			return nullptr;
		}
		return readBits(val, 8);
	}

	error::Error readBytes(uint8_t* data, uint32_t size)
	{
		if (size == 0) {
			return nullptr;
		}
		if ((numBitsRead_ & 7) == 0) {
			uint32_t startByte = numBitsRead_ >> 3;
			if (startByte + size > size_) {
				return error::Error("Reader does not contain enough data to fill the argument");
			}
			std::copy(bytes_ + startByte, bytes_ + startByte + size, data);
			numBitsRead_ += size * 8;
			return nullptr;
		}

		auto err = readBytesUnaligned(data, size, numBitsRead_ & 7);
		if (err) {
			return err;
		}
		numBitsRead_ += size * 8;
		return nullptr;
	}

private:

	error::Error readBytesUnaligned(uint8_t* data, uint32_t size, uint8_t bitOffset)
	{
		uint32_t byteIndex = numBitsRead_ >> 3;
		if (byteIndex + size >= size_) {
			return error::Error("Reader does not contain enough data to fill the argument");
		}

		uint8_t shift = bitOffset;
		uint8_t invShift = 64 - shift;

		uint32_t i = 0;
		if (size >= 8) {
			for (; i <= size - 8; i += 8) {
				uint64_t val1;
				std::memcpy(&val1, bytes_ + byteIndex + i, 8);

				uint8_t nextByte = bytes_[byteIndex + i + 8];
				uint64_t val2 = nextByte;

				uint64_t result = (val1 >> shift) | (val2 << invShift);
				std::memcpy(data + i, &result, 8);
			}
		}

		uint8_t byteShift = bitOffset;
		uint8_t byteInvShift = 8 - byteShift;
		for (; i < size; ++i) {
			data[i] = (bytes_[byteIndex + i] >> byteShift) | (bytes_[byteIndex + i + 1] << byteInvShift);
		}
		return nullptr;
	}

	const uint8_t* bytes_;
	uint32_t size_;
	uint32_t numBitsRead_ = 0;
};

// Reader that owns the buffer
class Reader {
public:

	inline explicit Reader(const std::vector<uint8_t>& data)
		: bytes_(data)
	{
	}

	inline explicit Reader(std::vector<uint8_t>&& data)
		: bytes_(std::move(data))
	{
	}

	template <typename T>
	inline error::Error read(T& data)
	{
		return deserialize(data, *this);
	}

	error::Error readBits(uint8_t& val, uint32_t num_bits_to_read)
	{
		assert(num_bits_to_read <= 8 && "Reader::readBits only supports reading up to 8 bits at a time");

		if (num_bits_to_read == 0) {
			val = 0;
			return nullptr;
		}

		uint32_t srcByteIndex = numBitsRead_ >> 3;
		uint8_t srcBitOffset = numBitsRead_ & 7;

		uint8_t bitsInFirstByte = 8 - srcBitOffset;

		if (num_bits_to_read <= bitsInFirstByte) {
			// Fits in one byte
			if (srcByteIndex >= bytes_.size()) {
				return error::Error("Reader does not contain enough data to fill the argument");
			}
			uint8_t v = bytes_[srcByteIndex];
			v >>= srcBitOffset;
			uint8_t mask = (1 << num_bits_to_read) - 1;
			val = v & mask;
		} else {
			// Spans two bytes
			if (srcByteIndex + 1 >= bytes_.size()) {
				return error::Error("Reader does not contain enough data to fill the argument");
			}
			// Read first part
			uint8_t val1 = bytes_[srcByteIndex];
			val1 >>= srcBitOffset;

			// Read second part
			uint8_t val2 = bytes_[srcByteIndex + 1];

			uint8_t mask2 = (1 << (num_bits_to_read - bitsInFirstByte)) - 1;
			val = val1 | ((val2 & mask2) << bitsInFirstByte);
		}

		numBitsRead_ += num_bits_to_read;
		return nullptr;
	}

	error::Error readByte(uint8_t& val)
	{
		if ((numBitsRead_ & 7) == 0) {
			uint32_t srcByteIndex = numBitsRead_ >> 3;
			if (srcByteIndex >= bytes_.size()) {
				return error::Error("Reader does not contain enough data to fill the argument");
			}
			val = bytes_[srcByteIndex];
			numBitsRead_ += 8;
			return nullptr;
		}
		return readBits(val, 8);
	}

	error::Error readBytes(uint8_t* data, uint32_t size)
	{
		if (size == 0) {
			return nullptr;
		}
		if ((numBitsRead_ & 7) == 0) {
			uint32_t startByte = numBitsRead_ >> 3;
			if (startByte + size > bytes_.size()) {
				return error::Error("Reader does not contain enough data to fill the argument");
			}
			std::copy(bytes_.begin() + startByte, bytes_.begin() + startByte + size, data);
			numBitsRead_ += size * 8;
			return nullptr;
		}

		error::Error err = readBytesUnaligned(data, size, numBitsRead_ & 7);
		if (err) {
			return err;
		}
		numBitsRead_ += size * 8;
		return nullptr;
	}

private:

	error::Error readBytesUnaligned(uint8_t* data, uint32_t size, uint8_t bitOffset)
	{
		uint32_t byteIndex = numBitsRead_ >> 3;
		if (byteIndex + size >= bytes_.size()) {
			return error::Error("Reader does not contain enough data to fill the argument");
		}

		uint8_t shift = bitOffset;
		uint8_t invShift = 64 - shift;

		uint32_t i = 0;
		if (size >= 8) {
			for (; i <= size - 8; i += 8) {
				uint64_t val1;
				std::memcpy(&val1, bytes_.data() + byteIndex + i, 8);

				uint8_t nextByte = bytes_[byteIndex + i + 8];
				uint64_t val2 = nextByte;

				uint64_t result = (val1 >> shift) | (val2 << invShift);
				std::memcpy(data + i, &result, 8);
			}
		}

		uint8_t byteShift = bitOffset;
		uint8_t byteInvShift = 8 - byteShift;
		for (; i < size; ++i) {
			data[i] = (bytes_[byteIndex + i] >> byteShift) | (bytes_[byteIndex + i + 1] << byteInvShift);
		}
		return nullptr;
	}

	std::vector<uint8_t> bytes_;
	uint32_t numBitsRead_ = 0;
};

class StreamReader {
public:

	StreamReader(std::istream& stream)
		: stream_(stream)
	{
	}

	template <typename T>
	inline error::Error read(T& data)
	{
		return deserialize(data, *this);
	}

	error::Error readBits(uint8_t& val, uint32_t num_bits_to_read)
	{
		assert(num_bits_to_read <= 8 && "StreamReader::readBits only supports reading up to 8 bits at a time");

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
			auto err = readByte(v, srcByteIndex);
			if (err) {
				return err;
			}
			v >>= srcBitOffset;
			uint8_t mask = (1 << num_bits_to_read) - 1;
			val = v & mask;
		} else {
			// Spans two bytes
			// Read first part
			uint8_t val1;
			auto err = readByte(val1, srcByteIndex);
			if (err) {
				return err;
			}
			val1 >>= srcBitOffset;

			// Read second part
			uint8_t val2;
			err = readByte(val2, srcByteIndex + 1);
			if (err) {
				return err;
			}

			uint8_t mask2 = (1 << (num_bits_to_read - bitsInFirstByte)) - 1;
			val = val1 | ((val2 & mask2) << bitsInFirstByte);
		}

		numBitsRead_ += num_bits_to_read;
		return nullptr;
	}

	error::Error readByte(uint8_t& val)
	{
		if ((numBitsRead_ & 7) == 0) {
			uint32_t srcByteIndex = numBitsRead_ >> 3;
			auto err = readByte(val, srcByteIndex);
			if (err) {
				return err;
			}
			numBitsRead_ += 8;
			return nullptr;
		}
		return readBits(val, 8);
	}

	error::Error readBytes(uint8_t* data, uint32_t size)
	{
		if (size == 0) {
			return nullptr;
		}
		if ((numBitsRead_ & 7) == 0) {
			auto err = readBytesAligned(data, size);
			if (err) {
				return err;
			}
			numBitsRead_ += size * 8;
			return nullptr;
		}

		error::Error err = readBytesUnaligned(data, size, numBitsRead_ & 7);
		if (err) {
			return err;
		}
		numBitsRead_ += size * 8;
		return nullptr;
	}

private:

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

	error::Error readBytesAligned(uint8_t* data, uint32_t size)
	{
		stream_.read(reinterpret_cast<char*>(data), size);
		if (stream_.fail()) {
			return error::Error("Failed to read bytes from stream");
		}
		currentIndex_ += size;

		if (size > 0) {
			currentByte_ = data[size - 1];
		}
		return nullptr;
	}

	error::Error readBytesUnaligned(uint8_t* data, uint32_t size, uint8_t bitOffset)
	{
		uint32_t byteIndex = numBitsRead_ >> 3;
		uint8_t shift = bitOffset;
		uint8_t invShift = 8 - shift;
		for (uint32_t i = 0; i < size; ++i) {
			uint8_t val1 = 0, val2 = 0;
			error::Error err;
			err = readByte(val1, byteIndex);
			if (err) {
				return err;
			}
			err = readByte(val2, byteIndex + 1);
			if (err) {
				return err;
			}
			data[i] = (val1 >> shift) | (val2 << invShift);
			byteIndex++;
		}
		return nullptr;
	}

	std::istream& stream_;
	uint8_t currentByte_ = 0;
	int64_t currentIndex_ = -1;
	uint32_t numBitsRead_ = 0;
};

}
}
