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

class IWriter {
public:

	virtual ~IWriter() = default;

	virtual const std::vector<uint8_t>& bytes() const = 0;

	virtual void write(const uint8_t* data, uint32_t n) = 0;

	inline void write(uint8_t data)
	{
		write(&data, 1);
	}

	template <std::size_t N>
	inline void write(const std::array<uint8_t, N>& data)
	{
		write(data.data(), N);
	}

	template <typename T>
	inline void write(const T& data)
	{
		serialize(*this, data);
	}
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

	inline void write(const uint8_t* data, uint32_t n)
	{
		bytes_.insert(bytes_.end(), data, data + n);
	}

	inline const std::vector<uint8_t>& bytes() const
	{
		return bytes_;
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

	inline void write(const uint8_t* data, uint32_t n)
	{
		bytes_.insert(bytes_.end(), data, data + n);
	}

	inline const std::vector<uint8_t>& bytes() const
	{
		return bytes_;
	}

private:

	std::vector<uint8_t>& bytes_;
};


class FixedSizeWriter : public IWriter {
public:

	using IWriter::write;

	inline explicit FixedSizeWriter(uint32_t size)
	{
		bytes_.reserve(size);
	}

	inline void write(const uint8_t* data, uint32_t n)
	{
		bytes_.insert(bytes_.end(), data, data + n);
	}

	inline const std::vector<uint8_t>& bytes() const
	{
		assert(bytes_.size() == bytes_.capacity() && std::string("FixedSizeWriter::bytes() called before all data was written" + (std::to_string(bytes_.size()) + " != " + std::to_string(bytes_.capacity()))).c_str());

		return bytes_;
	}

private:

	std::vector<uint8_t> bytes_;
};

}
}
