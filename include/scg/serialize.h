#pragma once

#include <array>
#include <map>
#include <string>
#include <vector>
#include <cassert>
#include <type_traits>

#include "scg/pack.h"
#include "scg/error.h"

namespace scg {
namespace serialize {

constexpr uint32_t byte_size(bool)
{
	return 1;
}

template <typename WriterType>
inline void serialize(WriterType& writer, bool value)
{
	writer.write(value ? uint8_t(1) : uint8_t(0));
}

template <typename ReaderType>
inline error::Error deserialize(bool& value, ReaderType& reader)
{
	std::array<uint8_t, 1> data;
	auto err = reader.read(data);
	if (err) {
		return err;
	}
	value = data[0] == 1 ? true : false;
	return nullptr;
}

constexpr uint32_t byte_size(uint8_t)
{
	return 1;
}

template <typename WriterType>
inline void serialize(WriterType& writer, uint8_t value)
{
	writer.write(value);
}

template <typename ReaderType>
inline error::Error deserialize(uint8_t& value, ReaderType& reader)
{
	std::array<uint8_t, 1> data;
	auto err = reader.read(data);
	if (err) {
		return err;
	}
	value = data[0];
	return nullptr;
}

constexpr uint32_t byte_size(uint16_t)
{
	return 2;
}

template <typename WriterType>
inline void serialize(WriterType& writer, uint16_t value)
{
	writer.write(std::array<uint8_t, 2>{
		uint8_t(value >> 8),
		uint8_t(value)
	});
}

template <typename ReaderType>
inline error::Error deserialize(uint16_t& value, ReaderType& reader)
{
	std::array<uint8_t, 2> data;
	auto err = reader.read(data);
	if (err) {
		return err;
	}
	value = uint16_t(data[0]) << 8 |
		uint16_t(data[1]);
	return nullptr;
}

constexpr uint32_t byte_size(uint32_t)
{
	return 4;
}

template <typename WriterType>
inline void serialize(WriterType& writer, uint32_t value)
{
	writer.write(std::array<uint8_t, 4>{
		uint8_t(value >> 24),
		uint8_t(value >> 16),
		uint8_t(value >> 8),
		uint8_t(value)
	});
}

template <typename ReaderType>
inline error::Error deserialize(uint32_t& value, ReaderType& reader)
{
	std::array<uint8_t, 4> data;
	auto err = reader.read(data);
	if (err) {
		return err;
	}
	value = uint32_t(data[0] << 24) |
		uint32_t(data[1] << 16) |
		uint32_t(data[2] << 8) |
		uint32_t(data[3]);
	return nullptr;
}

constexpr uint32_t byte_size(uint64_t)
{
	return 8;
}

template <typename WriterType>
inline void serialize(WriterType& writer, uint64_t value)
{
	writer.write(std::array<uint8_t, 8>{
		uint8_t(value >> 56),
		uint8_t(value >> 48),
		uint8_t(value >> 40),
		uint8_t(value >> 32),
		uint8_t(value >> 24),
		uint8_t(value >> 16),
		uint8_t(value >> 8),
		uint8_t(value)
	});
}

template <typename ReaderType>
inline error::Error deserialize(uint64_t& value, ReaderType& reader)
{
	std::array<uint8_t, 8> data;
	auto err = reader.read(data);
	if (err) {
		return err;
	}
	value = uint64_t(data[0]) << 56 |
		uint64_t(data[1]) << 48 |
		uint64_t(data[2]) << 40 |
		uint64_t(data[3]) << 32 |
		uint64_t(data[4]) << 24 |
		uint64_t(data[5]) << 16 |
		uint64_t(data[6]) << 8 |
		uint64_t(data[7]);
	return nullptr;
}

constexpr uint32_t byte_size(int8_t)
{
	return 1;
}

template <typename WriterType>
inline void serialize(WriterType& writer, int8_t value)
{
	serialize(writer, uint8_t(value));
}

template <typename ReaderType>
inline error::Error deserialize(int8_t& value, ReaderType& reader)
{
	uint8_t ui = 0;
	auto err = deserialize(ui, reader);
	if (err) {
		return err;
	}
	// change unsigned to signed
	if (ui <= 0x7fu) {
		value = ui;
	} else {
		value = -1 - (int8_t)(0xffu - ui);
	}
	return nullptr;
}

constexpr uint32_t byte_size(int16_t)
{
	return 2;
}

template <typename WriterType>
inline void serialize(WriterType& writer, int16_t value)
{
	serialize(writer, uint16_t(value));
}

template <typename ReaderType>
inline error::Error deserialize(int16_t& value, ReaderType& reader)
{
	uint16_t ui = 0;
	auto err = deserialize(ui, reader);
	if (err) {
		return err;
	}
	// change unsigned to signed
	if (ui <= 0x7fffu) {
		value = ui;
	} else {
		value = -1 - (int16_t)(0xffffu - ui);
	}
	return nullptr;
}

constexpr uint32_t byte_size(int32_t)
{
	return 4;
}

template <typename WriterType>
inline void serialize(WriterType& writer, int32_t value)
{
	serialize(writer, uint32_t(value));
}

template <typename ReaderType>
inline error::Error deserialize(int32_t& value, ReaderType& reader)
{
	uint32_t ui = 0;
	auto err = deserialize(ui, reader);
	if (err) {
		return err;
	}
	// change unsigned to signed
	if (ui <= 0x7fffffffu) {
		value = ui;
	} else {
		value = -1 - (int32_t)(0xffffffffu - ui);
	}
	return nullptr;
}

constexpr uint32_t byte_size(int64_t)
{
	return 8;
}

template <typename WriterType>
inline void serialize(WriterType& writer, int64_t value)
{
	serialize(writer, uint64_t(value));
}

template <typename ReaderType>
inline error::Error deserialize(int64_t& value, ReaderType& reader)
{
	uint64_t ui = 0;
	auto err = deserialize(ui, reader);
	if (err) {
		return err;
	}
	// change unsigned numbers to signed
	if (ui <= 0x7fffffffffffffffu) {
		value = ui;
	} else {
		value = -1 - (int64_t)(0xffffffffffffffffu - ui);
	}
	return nullptr;
}

constexpr uint32_t byte_size(float32_t)
{
	return 4;
}

template <typename WriterType>
inline void serialize(WriterType& writer, float32_t value)
{
	serialize(writer, pack754_32(value));
}

template <typename ReaderType>
inline error::Error deserialize(float32_t& value, ReaderType& reader)
{
	uint32_t ui = 0;
	auto err = deserialize(ui, reader);
	if (err) {
		return err;
	}
	value = unpack754_32(ui);

	return nullptr;
}

constexpr uint32_t byte_size(float64_t)
{
	return 8;
}

template <typename WriterType>
inline void serialize(WriterType& writer, float64_t value)
{
	serialize(writer, pack754_64(value));
}

template <typename ReaderType>
inline error::Error deserialize(float64_t& value, ReaderType& reader)
{
	uint64_t ui = 0;
	auto err = deserialize(ui, reader);
	if (err) {
		return err;
	}
	value = unpack754_64(ui);
	return nullptr;
}

inline uint32_t byte_size(std::string value)
{
	return 4 + value.size();
}

template <typename WriterType>
inline void serialize(WriterType& writer, std::string value)
{
	serialize(writer, uint32_t(value.size()));
	writer.write((uint8_t*)value.data(), value.size());
}

template <typename ReaderType>
inline error::Error deserialize(std::string& value, ReaderType& reader)
{
	uint32_t len = 0;
	auto err = deserialize(len, reader);
	if (err) {
		return err;
	}

	value.resize(len);
	return reader.read((uint8_t*)value.data(), len);
}

template <typename T,
	std::enable_if_t<std::is_enum<T>::value, int> = 0>
constexpr uint32_t byte_size(const T& t)
{
	return 2;
}

template <typename WriterType, typename T,
	std::enable_if_t<std::is_enum<T>::value, int> = 0>
inline void serialize(WriterType& writer, const T& value)
{
	serialize(writer, uint16_t(value));
}

template <typename ReaderType, typename T,
	std::enable_if_t<std::is_enum<T>::value, int> = 0>
inline error::Error deserialize(T& value, ReaderType& reader)
{
	uint16_t val = 0;
	auto err = deserialize(val, reader);
	if (err) {
		return err;
	}
	value = T(val);
	return nullptr;
}

template <typename T,
	std::enable_if_t<!std::is_enum<T>::value, int> = 0>
constexpr uint32_t byte_size(const T& t)
{
	return t.byteSize();
}

template <typename WriterType, typename T,
	std::enable_if_t<!std::is_enum<T>::value, int> = 0>
inline void serialize(WriterType& writer, const T& value)
{
	value.serialize(writer);
}

template <typename ReaderType, typename T,
	std::enable_if_t<!std::is_enum<T>::value, int> = 0>
inline error::Error deserialize(T& value, ReaderType& reader)
{
	return value.deserialize(reader);
}

template <typename T>
inline uint32_t byte_size(const std::vector<T>& value)
{
	uint32_t size = 4;
	for (const auto& item : value) {
		size += byte_size(item);
	}
	return size;
}

template <typename WriterType, typename T>
inline void serialize(WriterType& writer, const std::vector<T>& value)
{
	serialize(writer, uint32_t(value.size()));
	for (const auto& item : value) {
		serialize(writer, item);
	}
}

template <typename ReaderType, typename T>
inline error::Error deserialize(std::vector<T>& value, ReaderType& reader)
{
	uint32_t size;
	auto err = deserialize(size, reader);
	if (err) {
		return err;
	}
	value.resize(size);
	for (auto i = uint32_t(0); i < size; i++) {
		err = deserialize(value[i], reader);
		if (err) {
			return err;
		}
	}
	return nullptr;
}

template <typename WriterType>
inline void serialize(WriterType& writer, const std::vector<uint8_t>& value)
{
	serialize(writer, uint32_t(value.size()));
	writer.write(value.data(), value.size());
}

template <typename ReaderType>
inline error::Error deserialize(std::vector<uint8_t>& value, ReaderType& reader)
{
	uint32_t size;
	auto err = deserialize(size, reader);
	if (err) {
		return err;
	}

	value.reserve(size);
	return reader.read(value, size);
}

template <>
inline uint32_t byte_size(const std::vector<uint8_t>& value)
{
	return 4 + value.size();
}

template <typename K, typename V, typename WriterType>
inline void serialize(WriterType& writer, const std::map<K,V>& value)
{
	serialize(writer, uint32_t(value.size()));
	for (const auto& [key, value] : value) {
		serialize(writer, key);
		serialize(writer, value);
	}
}

template <typename K, typename V, typename ReaderType>
inline error::Error deserialize(std::map<K,V>& value, ReaderType& reader)
{
	uint32_t size;
	auto err = deserialize(size, reader);
	if (err) {
		return err;
	}

	for (auto i = uint32_t(0); i < size; i++) {
		K key;
		V val;
		err = deserialize(key, reader);
		if (err) {
			return err;
		}
		err = deserialize(val, reader);
		if (err) {
			return err;
		}
		value[key] = val;
	}
	return nullptr;
}

template <typename K, typename V>
inline uint32_t byte_size(const std::map<K,V>& value)
{
	uint32_t size = 4;
	for (const auto& [k, v] : value) {
		size += byte_size(k) + byte_size(v);
	}
	return size;
}

}
}
