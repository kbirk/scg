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

	inline void writeBits(uint8_t val, uint32_t num_bits_to_write)
	{
		uint32_t total_bits_to_write = num_bits_to_write;

		while (num_bits_to_write > 0) {
			uint32_t dst_byte_index = get_byte_offset(numBitsWritten_);
			uint8_t dst_bit_index =  get_bit_offset(numBitsWritten_);
			uint8_t src_bit_index = get_bit_offset(total_bits_to_write - num_bits_to_write);

			assert((src_bit_index <= 7) && "Invalid bit index");
			assert((dst_bit_index <= 7) && "Invalid bit index");

			uint8_t src_mask = 1 << src_bit_index;
			uint8_t dst_mask = 1 << dst_bit_index;

			if (val & src_mask) {
				writeBit(dst_byte_index, dst_mask);
			} else {
				writeBit(dst_byte_index, 0x00); // in case it needs to grow
			}

			numBitsWritten_++;
			num_bits_to_write--;
		}
	}

protected:

	virtual void writeBit(uint32_t destByteIndex, uint8_t bitMask) = 0;

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

	inline void writeBit(uint32_t byteIndex, uint8_t mask)
	{
		if (byteIndex >= bytes_.size()) {
			bytes_.push_back(0);
		}
		bytes_[byteIndex] |= mask;
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

	inline void writeBit(uint32_t byteIndex, uint8_t mask)
	{
		if (byteIndex >= bytes_.size()) {
			bytes_.push_back(0);
		}
		bytes_[byteIndex] |= mask;
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

	inline void writeBit(uint32_t byteIndex, uint8_t mask)
	{
		if (byteIndex > currentByteIndex_) {
			assert((byteIndex == currentByteIndex_ + 1) && "StreamWriter::writeBit() called with a byte index that is not the next byte in the stream");
			currentByte_ = 0;
			currentByteIndex_ = byteIndex;
		}

		currentByte_ |= mask;

		stream_.seekp(currentByteIndex_);
		stream_.write(reinterpret_cast<const char*>(&currentByte_), 1);
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

	inline void writeBit(uint32_t byteIndex, uint8_t mask)
	{
		assert(byteIndex < bytes_.size() && "FixedSizeWriter::getDestinationByte() called with an index greater than the capacity");

		bytes_[byteIndex] |= mask;
	}


private:

	std::vector<uint8_t> bytes_;
};

}
}
