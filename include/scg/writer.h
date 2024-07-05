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

	template <size_t N>
	inline void writeBits(const std::array<uint8_t, N>& val, uint32_t num_bits_to_write)
	{
		uint32_t total_bits_to_write = num_bits_to_write;

		while (num_bits_to_write > 0) {
			uint32_t dst_byte_index = get_byte_offset(numBitsWritten_);
			uint8_t dst_bit_index =  get_bit_offset(numBitsWritten_);
			uint32_t src_byte_index = get_byte_offset(total_bits_to_write - num_bits_to_write);
			uint8_t src_bit_index = get_bit_offset(total_bits_to_write - num_bits_to_write);

			assert((src_bit_index <= 7) && "Invalid bit index");
			assert((dst_bit_index <= 7) && "Invalid bit index");

			uint8_t src_mask = 1 << src_bit_index;
			uint8_t dst_mask = 1 << dst_bit_index;
			uint8_t val_byte = val[src_byte_index];

			if (val_byte & src_mask) {
				writeBit(dst_byte_index, dst_mask);
			}

			numBitsWritten_++;
			num_bits_to_write--;
		}
	}

	inline void writeBits(uint8_t val, uint32_t num_bits_to_write)
	{
		std::array<uint8_t, 1> bs = {
			val
		};
		writeBits(bs, num_bits_to_write);
	}

	inline void writeBits(uint16_t val, uint32_t num_bits_to_write)
	{
		std::array<uint8_t, 2> bs = {
			static_cast<uint8_t>(val >> 8),
			static_cast<uint8_t>(val)
		};
		writeBits(bs, num_bits_to_write);
	}

	inline void writeBits(uint32_t val, uint32_t num_bits_to_write)
	{
		std::array<uint8_t, 4> bs = {
			static_cast<uint8_t>(val >> 24),
			static_cast<uint8_t>(val >> 16),
			static_cast<uint8_t>(val >> 8),
			static_cast<uint8_t>(val)
		};
		writeBits(bs, num_bits_to_write);
	}

	inline void writeBits(uint64_t val, uint32_t num_bits_to_write)
	{
		std::array<uint8_t, 8> bs = {
			static_cast<uint8_t>(val >> 56),
			static_cast<uint8_t>(val >> 48),
			static_cast<uint8_t>(val >> 40),
			static_cast<uint8_t>(val >> 32),
			static_cast<uint8_t>(val >> 24),
			static_cast<uint8_t>(val >> 16),
			static_cast<uint8_t>(val >> 8),
			static_cast<uint8_t>(val)
		};
		writeBits(bs, num_bits_to_write);
	}

protected:

	virtual void writeBit(uint32_t byteIndex, uint8_t mask) = 0;

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
			currentByte_ = 0;
			currentByteIndex_ = byteIndex;
		}

		currentByte_ |= mask;

		stream_.seekp(currentByte_);
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
