#pragma once

#include <array>
#include <map>
#include <string>
#include <vector>
#include <cassert>
#include <type_traits>

#include "scg/serialize.h"
#include "scg/pack.h"
#include "scg/error.h"
#include "scg/serialize.h"

namespace scg {
namespace serialize {

class IWriter {
public:
	virtual ~IWriter() = default;

	virtual const std::vector<uint8_t>& bytes() const = 0;

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

protected:

	virtual void orByte(uint32_t index, uint8_t mask) = 0;
	virtual void ensureCapacity(uint32_t size) = 0;

	uint32_t numBitsWritten_ = 0;
};


class Writer : public IWriter {
public:

	using IWriter::write;

	inline Writer()
	{
		bytes_.reserve(1024);
	}

	inline explicit Writer(uint32_t size)
	{
		bytes_.reserve(size);
	}

	inline const std::vector<uint8_t>& bytes() const
	{
		return bytes_;
	}

protected:

	void orByte(uint32_t index, uint8_t mask) override
	{
		bytes_[index] |= mask;
	}

	void ensureCapacity(uint32_t size) override
	{
		if (bytes_.size() < size) {
			bytes_.resize(size, 0);
		}
	}

private:

	std::vector<uint8_t> bytes_;
};

class WriterView : public IWriter {
public:

	using IWriter::write;
	using scg::serialize::IWriter::writeBits;

	inline explicit WriterView(std::vector<uint8_t>& data)
		: bytes_(data)
	{
	}

	inline const std::vector<uint8_t>& bytes() const
	{
		return bytes_;
	}

protected:

	void orByte(uint32_t index, uint8_t mask) override
	{
		bytes_[index] |= mask;
	}

	void ensureCapacity(uint32_t size) override
	{
		if (bytes_.size() < size) {
			bytes_.resize(size, 0);
		}
	}

private:

	std::vector<uint8_t>& bytes_;
};

class StreamWriter : scg::serialize::IWriter {
public:

	using scg::serialize::IWriter::write;
	using scg::serialize::IWriter::writeBits;

	StreamWriter(std::ostream& stream)
		: stream_(stream)
	{}

	inline const std::vector<uint8_t>& bytes() const
	{
		assert(false && "StreamWriter::bytes() called on a StreamWriter");
	}

protected:

	void orByte(uint32_t index, uint8_t mask) override
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

	void ensureCapacity(uint32_t size) override
	{
		// No-op for stream writer
	}

private:

	uint8_t currentByte_ = 0;
	int64_t currentByteIndex_ = -1;
	std::ostream& stream_;
};

class FixedSizeWriter : public IWriter {
public:

	using IWriter::write;
	using scg::serialize::IWriter::writeBits;

	inline explicit FixedSizeWriter(uint32_t size)
		: bytes_(size, 0)
	{
	}

	inline const std::vector<uint8_t>& bytes() const
	{
		assert(bytes_.size() == bytes_.capacity() && std::string("FixedSizeWriter::bytes() called before all data was written" + (std::to_string(bytes_.size()) + " != " + std::to_string(bytes_.capacity()))).c_str());

		return bytes_;
	}

	inline uint8_t* getDestinationByte(uint32_t byteIndex)
	{
		assert(byteIndex < bytes_.size() && "FixedSizeWriter::getDestinationByte() called with an index greater than the capacity");

		return &bytes_[byteIndex];
	}

protected:

	void orByte(uint32_t index, uint8_t mask) override
	{
		assert(index < bytes_.size() && "FixedSizeWriter::orByte() called with an index greater than the capacity");
		bytes_[index] |= mask;
	}

	void ensureCapacity(uint32_t size) override
	{
		assert(size <= bytes_.size() && "FixedSizeWriter overflow");
	}

private:

	std::vector<uint8_t> bytes_;
};

}
}
