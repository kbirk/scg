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

	virtual error::Error read(uint8_t* dest, uint32_t n) = 0;
	virtual error::Error read(std::vector<uint8_t>& dest, uint32_t n) = 0;

	template <std::size_t N>
	inline error::Error read(std::array<uint8_t, N>& dest)
	{
		return read(dest.data(), N);
	}

	template <typename T>
	inline error::Error read(T& data)
	{
		return deserialize(data, *this);
	}
};


class ReaderView : public IReader {
public:

	using IReader::read;

	inline ReaderView(const uint8_t* data, uint32_t size)
		: bytes_(data)
		, size_(size)
		, pos_(0)
	{
	}

	inline explicit ReaderView(const std::vector<uint8_t>& data)
		: bytes_(&data[0])
		, size_(data.size())
		, pos_(0)
	{
	}

	inline error::Error read(uint8_t* dest, uint32_t n)
	{
		if (pos_ + n > size_) {
			return error::Error("Reader does not contain enough data to fill the argument");
		}

		std::copy(bytes_ + pos_, bytes_ + pos_ + n, dest);
		pos_ += n;
		return nullptr;
	}

	inline error::Error read(std::vector<uint8_t>& dest, uint32_t n)
	{
		if  (pos_ + n > size_) {
			return error::Error("Reader does not contain enough data to fill the argument");
		}

		dest.insert(dest.end(), bytes_ + pos_, bytes_ + pos_ + n);
		pos_ += n;
		return nullptr;
	}

private:

	const uint8_t* bytes_;
	uint32_t size_;
	size_t pos_;
};

class Reader : public IReader {
public:

	using IReader::read;

	inline explicit Reader(const std::vector<uint8_t>& data)
		: bytes_(data)
		, pos_(0)
	{
	}

	inline error::Error read(uint8_t* dest, uint32_t n)
	{
		if (pos_ + n > bytes_.size()) {
			return error::Error("Reader does not contain enough data to fill the argument");
		}

		std::copy(bytes_.begin() + pos_, bytes_.begin() + pos_ + n, dest);
		pos_ += n;
		return nullptr;
	}

	inline error::Error read(std::vector<uint8_t>& dest, uint32_t n)
	{
		if  (pos_ + n > bytes_.size()) {
			return error::Error("Reader does not contain enough data to fill the argument");
		}

		dest.insert(dest.end(), bytes_.begin() + pos_, bytes_.begin() + pos_ + n);
		pos_ += n;
		return nullptr;
	}

private:

	std::vector<uint8_t> bytes_;
	size_t pos_;
};

class StreamReader : scg::serialize::IReader {
public:

	using scg::serialize::IReader::read;

	StreamReader(std::istream& stream)
		: stream_(stream)
	{}

	error::Error read(uint8_t* dest, uint32_t n)
	{
		// check that it has enough bytes
		stream_.read((char*)dest, n);
		if (stream_.fail()) {
			return error::Error("Failed to read " + std::to_string(n) + " bytes from stream");
		}
		return nullptr;
	}

	error::Error read(std::vector<uint8_t>& dest, uint32_t n)
	{
		dest.resize(n, 0);
		stream_.read((char*)(&dest[0]), n);
		if (stream_.fail()) {
			return error::Error("Failed to read " + std::to_string(n) + " bytes from stream");
		}
		return nullptr;
	}

private:
	std::istream& stream_;
};

}
}
