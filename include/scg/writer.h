#pragma once

#include <array>
#include <map>
#include <string>
#include <vector>
#include <cassert>
#include <type_traits>
#include <cstring>
#include <algorithm>
#include <fstream>

#include "scg/pack.h"
#include "scg/error.h"
#include "scg/serialize.h"

namespace scg {
namespace serialize {

class Writer {
public:

	inline Writer() = default;

	// Writer pre-sizes its buffer to `size` bytes (normally bits_to_bytes of the
	// value's bit_size). A zero size is allowed (zero-bit messages); the buffer
	// grows on demand either way.
	inline explicit Writer(uint32_t size)
	{
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

		size_t dstByteIndex = numBitsWritten_ >> 3;
		uint8_t dstBitOffset = numBitsWritten_ & 7;

		// Ensure capacity
		ensureCapacity((numBitsWritten_ + num_bits_to_write + 7) / 8);

		// Calculate how many bits fit in the current byte
		uint8_t bitsInFirstByte = 8 - dstBitOffset;

		// Write first part
		bytes_[dstByteIndex] |= val << dstBitOffset;

		if (num_bits_to_write > bitsInFirstByte) {
			// Spans two bytes, write second part
			bytes_[dstByteIndex + 1] |= val >> bitsInFirstByte;
		}

		numBitsWritten_ += num_bits_to_write;
	}

	void writeByte(uint8_t val)
	{
		if ((numBitsWritten_ & 7) == 0) {
			size_t byteIndex = numBitsWritten_ >> 3;
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
			ensureCapacity((numBitsWritten_ >> 3) + size);
			size_t startByte = numBitsWritten_ >> 3;
			std::copy(data, data + size, bytes_.begin() + startByte);
			numBitsWritten_ += uint64_t(size) * 8;
		} else {
			ensureCapacity((numBitsWritten_ + uint64_t(size) * 8 + 7) / 8);
			writeBytesUnaligned(data, size, numBitsWritten_ & 7);
			numBitsWritten_ += uint64_t(size) * 8;
		}
	}

	inline const std::vector<uint8_t>& bytes() const
	{
		return bytes_;
	}

	// sizeBytes returns the number of bytes holding written bits: the
	// meaningful prefix of bytes(). The buffer can be pre-sized larger than the
	// payload, so bytes().size() may overstate it.
	inline size_t sizeBytes() const
	{
		return static_cast<size_t>((numBitsWritten_ + 7) >> 3);
	}

	// take moves the serialized buffer out of the writer without copying, e.g.
	// `return std::move(writer).take();`. The writer is left empty.
	inline std::vector<uint8_t> take() &&
	{
		numBitsWritten_ = 0;
		return std::move(bytes_);
	}

private:

	inline void ensureCapacity(size_t neededBytes)
	{
		if (neededBytes > bytes_.size()) {
			// Grow geometrically, but only when the capacity is actually
			// exceeded (reserving on every size increase would reallocate on
			// every write).
			if (neededBytes > bytes_.capacity()) {
				bytes_.reserve((std::max)(neededBytes, bytes_.capacity() * 2));
			}
			bytes_.resize(neededBytes, 0);
		}
	}

	void writeBytesUnaligned(const uint8_t* data, uint32_t size, uint8_t bitOffset)
	{
		size_t byteIndex = numBitsWritten_ >> 3;
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
	// 64-bit: a 32-bit bit position wraps at 512 MiB.
	uint64_t numBitsWritten_ = 0;
};

class WriterView {
public:

	// WriterView writes into a caller-owned buffer, growing it on demand. An empty
	// buffer is allowed (zero-bit messages serialize to zero bytes).
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

		size_t dstByteIndex = numBitsWritten_ >> 3;
		uint8_t dstBitOffset = numBitsWritten_ & 7;

		// Ensure capacity
		ensureCapacity((numBitsWritten_ + num_bits_to_write + 7) / 8);

		// Calculate how many bits fit in the current byte
		uint8_t bitsInFirstByte = 8 - dstBitOffset;

		// Write first part
		bytes_[dstByteIndex] |= val << dstBitOffset;

		if (num_bits_to_write > bitsInFirstByte) {
			// Spans two bytes, write second part
			bytes_[dstByteIndex + 1] |= val >> bitsInFirstByte;
		}

		numBitsWritten_ += num_bits_to_write;
	}

	void writeByte(uint8_t val)
	{
		if ((numBitsWritten_ & 7) == 0) {
			size_t byteIndex = numBitsWritten_ >> 3;
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
			ensureCapacity((numBitsWritten_ >> 3) + size);
			size_t startByte = numBitsWritten_ >> 3;
			std::copy(data, data + size, bytes_.begin() + startByte);
			numBitsWritten_ += uint64_t(size) * 8;
		} else {
			ensureCapacity((numBitsWritten_ + uint64_t(size) * 8 + 7) / 8);
			writeBytesUnaligned(data, size, numBitsWritten_ & 7);
			numBitsWritten_ += uint64_t(size) * 8;
		}
	}

	inline const std::vector<uint8_t>& bytes() const
	{
		return bytes_;
	}

	// sizeBytes returns the number of bytes holding written bits: the
	// meaningful prefix of bytes(). The buffer can be pre-sized larger than the
	// payload, so bytes().size() may overstate it.
	inline size_t sizeBytes() const
	{
		return static_cast<size_t>((numBitsWritten_ + 7) >> 3);
	}

private:

	inline void ensureCapacity(size_t neededBytes)
	{
		if (neededBytes > bytes_.size()) {
			// Grow geometrically, but only when the capacity is actually
			// exceeded (reserving on every size increase would reallocate on
			// every write).
			if (neededBytes > bytes_.capacity()) {
				bytes_.reserve((std::max)(neededBytes, bytes_.capacity() * 2));
			}
			bytes_.resize(neededBytes, 0);
		}
	}

	void writeBytesUnaligned(const uint8_t* data, uint32_t size, uint8_t bitOffset)
	{
		size_t byteIndex = numBitsWritten_ >> 3;
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
	uint64_t numBitsWritten_ = 0;
};

}
}
