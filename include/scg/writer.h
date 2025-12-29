#pragma once

#include <array>
#include <map>
#include <string>
#include <vector>
#include <cassert>
#include <type_traits>
#include <cstring>
#include <algorithm>

#include "scg/pack.h"
#include "scg/error.h"
#include "scg/serialize.h"

namespace scg {
namespace serialize {

class Writer {
public:

	inline explicit Writer(uint32_t size)
	{
		assert(size > 0 && "Writer must be created with non-zero size");

		bytes_.resize(size, 0);
	}

	template <typename T>
	inline void write(const T& data)
	{
		serialize(*this, data);
	}

	void clear()
	{
		numBitsWritten_ = 0;
		std::memset(bytes_.data(), 0, bytes_.size());
	}

	void writeBits(uint8_t val, uint32_t num_bits_to_write)
	{
		if (num_bits_to_write == 0) {
			return;
		}

		// Mask val to ensure we only write numBitsToWrite bits
		val &= (1 << num_bits_to_write) - 1;

		uint32_t dstByteIndex = numBitsWritten_ >> 3;
		uint8_t dstBitOffset = numBitsWritten_ & 7;

		// Ensure capacity
		ensureCapacity((numBitsWritten_ + num_bits_to_write + 7) / 8);

		// Calculate how many bits fit in the current byte
		uint8_t bitsInFirstByte = 8 - dstBitOffset;

		if (num_bits_to_write <= bitsInFirstByte) {
			// Fits in one byte
			orByte(dstByteIndex, val << dstBitOffset);
		} else {
			// Spans two bytes
			// Write first part
			orByte(dstByteIndex, val << dstBitOffset);

			// Write second part
			orByte(dstByteIndex + 1, val >> bitsInFirstByte);
		}

		numBitsWritten_ += num_bits_to_write;
	}

	void writeByte(uint8_t val)
	{
		if ((numBitsWritten_ & 7) == 0) {
			uint32_t byteIndex = numBitsWritten_ >> 3;
			ensureCapacity(byteIndex + 1);
			bytes_[byteIndex] = val;
			numBitsWritten_ += 8;
		} else {
			writeBits(val, 8);
		}
	}

	void writeBytes(const uint8_t* data, uint32_t size)
	{
		if (size == 0) {
			return;
		}
		if ((numBitsWritten_ & 7) == 0) {
			uint32_t needed = (numBitsWritten_ >> 3) + size;
			ensureCapacity(needed);
			uint32_t startByte = numBitsWritten_ >> 3;
			std::copy(data, data + size, bytes_.begin() + startByte);
			numBitsWritten_ += size * 8;
		} else {
			uint32_t needed = (numBitsWritten_ + size * 8 + 7) / 8;
			ensureCapacity(needed);
			writeBytesUnaligned(data, size, numBitsWritten_ & 7);
			numBitsWritten_ += size * 8;
		}
	}

	inline const std::vector<uint8_t>& bytes() const
	{
		return bytes_;
	}

private:

	void orByte(uint32_t index, uint8_t mask)
	{
		bytes_[index] |= mask;
	}

	void ensureCapacity(uint32_t size)
	{
		assert(size <= bytes_.size() && "Writer overflow: insufficient capacity");
	}

	void writeBytesUnaligned(const uint8_t* data, uint32_t size, uint8_t bitOffset)
	{
		uint32_t byteIndex = numBitsWritten_ >> 3;
		uint8_t shift = bitOffset;
		uint8_t invShift = 64 - shift;

		uint32_t i = 0;
		if (size >= 8) {
			for (; i <= size - 8; i += 8) {
				uint64_t val;
				std::memcpy(&val, data + i, 8);

				uint64_t low = val << shift;
				uint64_t high = val >> invShift;

				uint64_t dst_val;
				std::memcpy(&dst_val, bytes_.data() + byteIndex + i, 8);
				dst_val |= low;
				std::memcpy(bytes_.data() + byteIndex + i, &dst_val, 8);

				bytes_[byteIndex + i + 8] |= (uint8_t)high;
			}
		}

		uint8_t byteShift = bitOffset;
		uint8_t byteInvShift = 8 - byteShift;
		for (; i < size; ++i) {
			bytes_[byteIndex + i] |= data[i] << byteShift;
			bytes_[byteIndex + i + 1] |= data[i] >> byteInvShift;
		}
	}

	std::vector<uint8_t> bytes_;
	uint32_t numBitsWritten_ = 0;
};

class WriterView {
public:

	inline explicit WriterView(std::vector<uint8_t>& data)
		: bytes_(data)
	{
	}

	template <typename T>
	inline void write(const T& data)
	{
		serialize(*this, data);
	}

	void writeBits(uint8_t val, uint32_t num_bits_to_write)
	{
		if (num_bits_to_write == 0) {
			return;
		}

		// Mask val to ensure we only write numBitsToWrite bits
		val &= (1 << num_bits_to_write) - 1;

		uint32_t dstByteIndex = numBitsWritten_ >> 3;
		uint8_t dstBitOffset = numBitsWritten_ & 7;

		// Ensure capacity
		ensureCapacity((numBitsWritten_ + num_bits_to_write + 7) / 8);

		// Calculate how many bits fit in the current byte
		uint8_t bitsInFirstByte = 8 - dstBitOffset;

		if (num_bits_to_write <= bitsInFirstByte) {
			// Fits in one byte
			orByte(dstByteIndex, val << dstBitOffset);
		} else {
			// Spans two bytes
			// Write first part
			orByte(dstByteIndex, val << dstBitOffset);

			// Write second part
			orByte(dstByteIndex + 1, val >> bitsInFirstByte);
		}

		numBitsWritten_ += num_bits_to_write;
	}

	void writeByte(uint8_t val)
	{
		if ((numBitsWritten_ & 7) == 0) {
			uint32_t byteIndex = numBitsWritten_ >> 3;
			ensureCapacity(byteIndex + 1);
			bytes_[byteIndex] = val;
			numBitsWritten_ += 8;
		} else {
			writeBits(val, 8);
		}
	}

	void writeBytes(const uint8_t* data, uint32_t size)
	{
		if (size == 0) {
			return;
		}
		if ((numBitsWritten_ & 7) == 0) {
			uint32_t needed = (numBitsWritten_ >> 3) + size;
			ensureCapacity(needed);
			uint32_t startByte = numBitsWritten_ >> 3;
			std::copy(data, data + size, bytes_.begin() + startByte);
			numBitsWritten_ += size * 8;
		} else {
			uint32_t needed = (numBitsWritten_ + size * 8 + 7) / 8;
			ensureCapacity(needed);
			writeBytesUnaligned(data, size, numBitsWritten_ & 7);
			numBitsWritten_ += size * 8;
		}
	}

	inline const std::vector<uint8_t>& bytes() const
	{
		return bytes_;
	}

private:

	void orByte(uint32_t index, uint8_t mask)
	{
		bytes_[index] |= mask;
	}

	void ensureCapacity(uint32_t size)
	{
		assert(size <= bytes_.size() && "WriterView overflow: insufficient capacity");
	}

	void writeBytesUnaligned(const uint8_t* data, uint32_t size, uint8_t bitOffset)
	{
		uint32_t byteIndex = numBitsWritten_ >> 3;
		uint8_t shift = bitOffset;
		uint8_t invShift = 64 - shift;

		uint32_t i = 0;
		if (size >= 8) {
			for (; i <= size - 8; i += 8) {
				uint64_t val;
				std::memcpy(&val, data + i, 8);

				uint64_t low = val << shift;
				uint64_t high = val >> invShift;

				uint64_t dst_val;
				std::memcpy(&dst_val, bytes_.data() + byteIndex + i, 8);
				dst_val |= low;
				std::memcpy(bytes_.data() + byteIndex + i, &dst_val, 8);

				bytes_[byteIndex + i + 8] |= (uint8_t)high;
			}
		}

		uint8_t byteShift = bitOffset;
		uint8_t byteInvShift = 8 - byteShift;
		for (; i < size; ++i) {
			bytes_[byteIndex + i] |= data[i] << byteShift;
			bytes_[byteIndex + i + 1] |= data[i] >> byteInvShift;
		}
	}

	std::vector<uint8_t>& bytes_;
	uint32_t numBitsWritten_ = 0;
};

class StreamWriter {
public:

	inline explicit StreamWriter(std::ostream& stream)
		: stream_(stream)
	{
	}

	template <typename T>
	inline void write(const T& data)
	{
		serialize(*this, data);
	}

	void writeBits(uint8_t val, uint32_t num_bits_to_write)
	{
		if (num_bits_to_write == 0) {
			return;
		}

		// Mask val to ensure we only write numBitsToWrite bits
		val &= (1 << num_bits_to_write) - 1;

		uint32_t dstByteIndex = numBitsWritten_ >> 3;
		uint8_t dstBitOffset = numBitsWritten_ & 7;

		// Calculate how many bits fit in the current byte
		uint8_t bitsInFirstByte = 8 - dstBitOffset;

		if (num_bits_to_write <= bitsInFirstByte) {
			// Fits in one byte
			orByte(dstByteIndex, val << dstBitOffset);
		} else {
			// Spans two bytes
			// Write first part
			orByte(dstByteIndex, val << dstBitOffset);

			// Write second part
			orByte(dstByteIndex + 1, val >> bitsInFirstByte);
		}

		numBitsWritten_ += num_bits_to_write;
	}

	void writeByte(uint8_t val)
	{
		if ((numBitsWritten_ & 7) == 0) {
			orByte(numBitsWritten_ >> 3, val);
			numBitsWritten_ += 8;
		} else {
			writeBits(val, 8);
		}
	}

	void writeBytes(const uint8_t* data, uint32_t size)
	{
		if (size == 0) {
			return;
		}
		if ((numBitsWritten_ & 7) == 0) {
			writeBytesAligned(data, size);
			numBitsWritten_ += size * 8;
		} else {
			writeBytesUnaligned(data, size, numBitsWritten_ & 7);
			numBitsWritten_ += size * 8;
		}
	}

protected:

	void orByte(uint32_t index, uint8_t mask)
	{
		if (index > currentByteIndex_) {
			assert((index == currentByteIndex_ + 1) && "StreamWriter::orByte() called with a byte index that is not the next byte in the stream");
			currentByte_ = 0;
			currentByteIndex_ = index;
		}

		currentByte_ |= mask;

		stream_.seekp(currentByteIndex_);
		stream_.write(reinterpret_cast<const char*>(&currentByte_), 1);
	}

	void writeBytesAligned(const uint8_t* data, uint32_t size)
	{
		stream_.write(reinterpret_cast<const char*>(data), size);
		currentByteIndex_ += size;
		currentByte_ = 0;
	}

	void writeBytesUnaligned(const uint8_t* data, uint32_t size, uint8_t bitOffset)
	{
		uint32_t byteIndex = numBitsWritten_ >> 3;
		uint8_t shift = bitOffset;
		uint8_t invShift = 8 - shift;
		for (uint32_t i = 0; i < size; ++i) {
			orByte(byteIndex, data[i] << shift);
			orByte(byteIndex + 1, data[i] >> invShift);
			byteIndex++;
		}
	}

private:

	uint8_t currentByte_ = 0;
	int64_t currentByteIndex_ = -1;
	std::ostream& stream_;
	uint32_t numBitsWritten_ = 0;
};

}
}
