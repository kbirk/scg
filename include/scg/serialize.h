#pragma once

#include <array>
#include <map>
#include <unordered_map>
#include <set>
#include <unordered_set>
#include <string>
#include <vector>
#include <cassert>
#include <type_traits>

#include "scg/pack.h"
#include "scg/error.h"

namespace scg {
namespace serialize {

inline constexpr uint32_t bit_size(bool value)
{
	return 1;
}

template <typename WriterType>
inline void serialize(WriterType& writer, bool value)
{
	writer.writeBits(value ? uint8_t(1) : uint8_t(0), 1);
}

template <typename ReaderType>
inline error::Error deserialize(bool& value, ReaderType& reader)
{
	uint8_t v = 0;
	auto err = reader.readBits(v, 1);
	if (err) {
		return err;
	}
	value = v != 0;
	return nullptr;
}

inline constexpr uint32_t bit_size(uint8_t value)
{
	return 8;
}

template <typename WriterType>
inline void serialize(WriterType& writer, uint8_t value)
{
	writer.writeBits(value, 8);
}

template <typename ReaderType>
inline error::Error deserialize(uint8_t& value, ReaderType& reader)
{
	return reader.readBits(value, 8);
}

inline constexpr uint32_t bit_size(uint16_t value)
{
	return var_uint_bit_size(value, 2);
}

template <typename WriterType>
inline void serialize(WriterType& writer, uint16_t value)
{
	var_encode_uint(writer, value, 2);
}

template <typename ReaderType>
inline error::Error deserialize(uint16_t& value, ReaderType& reader)
{
	uint64_t val = 0;
	auto err = var_decode_uint(val, reader, 2);
	if (err) {
		return err;
	}
	value = val;
	return nullptr;
}

inline constexpr uint32_t bit_size(uint32_t value)
{
	return var_uint_bit_size(value, 4);
}

template <typename WriterType>
inline void serialize(WriterType& writer, uint32_t value)
{
	var_encode_uint(writer, value, 4);
}

template <typename ReaderType>
inline error::Error deserialize(uint32_t& value, ReaderType& reader)
{
	uint64_t val = 0;
	auto err = var_decode_uint(val, reader, 4);
	if (err) {
		return err;
	}
	value = val;
	return nullptr;
}

inline constexpr uint32_t bit_size(uint64_t value)
{
	return var_uint_bit_size(value, 8);
}

template <typename WriterType>
inline void serialize(WriterType& writer, uint64_t value)
{
	var_encode_uint(writer, value, 8);
}

template <typename ReaderType>
inline error::Error deserialize(uint64_t& value, ReaderType& reader)
{
	return var_decode_uint(value, reader, 8);
}

inline constexpr uint32_t bit_size(int8_t value)
{
	return bit_size(uint8_t(value));
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

inline constexpr uint32_t bit_size(int16_t value)
{
	return var_int_bit_size(value, 2);
}

template <typename WriterType>
inline void serialize(WriterType& writer, int16_t value)
{
	var_encode_int(writer, value, 2);
}

template <typename ReaderType>
inline error::Error deserialize(int16_t& value, ReaderType& reader)
{
	int64_t val = 0;
	auto err = var_decode_int(val, reader, 2);
	if (err) {
		return err;
	}
	value = val;
	return nullptr;
}

inline constexpr uint32_t bit_size(int32_t value)
{
	return var_int_bit_size(value, 4);
}

template <typename WriterType>
inline void serialize(WriterType& writer, int32_t value)
{
	var_encode_int(writer, value, 4);
}

template <typename ReaderType>
inline error::Error deserialize(int32_t& value, ReaderType& reader)
{
	int64_t val = 0;
	auto err = var_decode_int(val, reader, 4);
	if (err) {
		return err;
	}
	value = val;
	return nullptr;
}

inline constexpr uint32_t bit_size(int64_t value)
{
	return var_int_bit_size(value, 8);
}

template <typename WriterType>
inline void serialize(WriterType& writer, int64_t value)
{
	var_encode_int(writer, value, 8);
}

template <typename ReaderType>
inline error::Error deserialize(int64_t& value, ReaderType& reader)
{
	return var_decode_int(value, reader, 8);
}

inline constexpr uint32_t bit_size(float32_t value)
{
	return bytes_to_bits(4);
}

template <typename WriterType>
inline void serialize(WriterType& writer, float32_t value)
{
	uint32_t packed = pack754_32(value);
	writer.writeBits(packed >> 24, 8);
	writer.writeBits(packed >> 16, 8);
	writer.writeBits(packed >> 8, 8);
	writer.writeBits(packed, 8);
}

template <typename ReaderType>
inline error::Error deserialize(float32_t& value, ReaderType& reader)
{
	std::array<uint8_t, 4> bytes;
	reader.readBits(bytes[0], 8);
	reader.readBits(bytes[1], 8);
	reader.readBits(bytes[2], 8);
	reader.readBits(bytes[3], 8);

	uint32_t packed =
		(static_cast<uint32_t>(bytes[0]) << 24) |
		(static_cast<uint32_t>(bytes[1]) << 16) |
		(static_cast<uint32_t>(bytes[2]) << 8) |
		bytes[3];
	value = unpack754_32(packed);

	return nullptr;
}

inline constexpr uint32_t bit_size(float64_t value)
{
	return bytes_to_bits(8);
}

template <typename WriterType>
inline void serialize(WriterType& writer, float64_t value)
{
	uint64_t packed = pack754_64(value);
	writer.writeBits(packed >> 56, 8);
	writer.writeBits(packed >> 48, 8);
	writer.writeBits(packed >> 40, 8);
	writer.writeBits(packed >> 32, 8);
	writer.writeBits(packed >> 24, 8);
	writer.writeBits(packed >> 16, 8);
	writer.writeBits(packed >> 8, 8);
	writer.writeBits(packed, 8);
}

template <typename ReaderType>
inline error::Error deserialize(float64_t& value, ReaderType& reader)
{
	std::array<uint8_t, 8> bytes;
	reader.readBits(bytes[0], 8);
	reader.readBits(bytes[1], 8);
	reader.readBits(bytes[2], 8);
	reader.readBits(bytes[3], 8);
	reader.readBits(bytes[4], 8);
	reader.readBits(bytes[5], 8);
	reader.readBits(bytes[6], 8);
	reader.readBits(bytes[7], 8);

	uint64_t packed =
		(static_cast<uint64_t>(bytes[0]) << 56) |
		(static_cast<uint64_t>(bytes[1]) << 48) |
		(static_cast<uint64_t>(bytes[2]) << 40) |
		(static_cast<uint64_t>(bytes[3]) << 32) |
		(static_cast<uint64_t>(bytes[4]) << 24) |
		(static_cast<uint64_t>(bytes[5]) << 16) |
		(static_cast<uint64_t>(bytes[6]) << 8) |
		bytes[7];
	value = unpack754_64(packed);

	return nullptr;
}

inline uint32_t bit_size(const std::string& value)
{
	auto size = bit_size(uint32_t(value.size()));
	for (const auto& c : value) {
		size += bit_size(uint8_t(c));
	}
	return size;
}


template <typename WriterType>
inline void serialize(WriterType& writer, const std::string& value)
{
	serialize(writer, uint32_t(value.size()));
	for (const auto& c : value) {
		serialize(writer, uint8_t(c));
	}
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
	for (uint32_t i=0; i<len; i++) {
		uint8_t c = 0;
		err = deserialize(c, reader);
		if (err) {
			return err;
		}
		value[i] = c;
	}

	return nullptr;
}

inline uint32_t bit_size(const error::Error& value)
{
	return bit_size(value.message);
}

template <typename WriterType>
inline void serialize(WriterType& writer, const error::Error& value)
{
	serialize(writer, value.message);
}

template <typename ReaderType>
inline error::Error deserialize(error::Error& value, ReaderType& reader)
{
	return deserialize(value.message, reader);
}

template <typename T,
	std::enable_if_t<std::is_enum<T>::value, int> = 0>
constexpr uint32_t bit_size(const T& t)
{
	return bit_size(uint16_t(t));
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
constexpr uint32_t bit_size(const T& value)
{
	static_assert(sizeof(T) == 0, "no `bit_size` override exists for type");
	return 0;
}

template <typename WriterType, typename T,
	std::enable_if_t<!std::is_enum<T>::value, int> = 0>
inline void serialize(WriterType& writer, const T& value)
{
	static_assert(sizeof(T) == 0, "no `serialize` override exists for type");
}

template <typename ReaderType, typename T,
	std::enable_if_t<!std::is_enum<T>::value, int> = 0>
inline error::Error deserialize(T& value, ReaderType& reader)
{
	static_assert(sizeof(T) == 0, "no `deserialize` override exists for type");
	return nullptr;
}

template <typename T>
inline uint32_t bit_size(const std::vector<T>& value)
{
	uint32_t size = bit_size(uint32_t(value.size()));
	for (const auto& item : value) {
		size += bit_size(item);
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

template <typename K, typename V>
inline uint32_t bit_size(const std::map<K,V>& value)
{
	uint32_t size = bit_size(uint32_t(value.size()));
	for (const auto& [k, v] : value) {
		size += bit_size(k) + bit_size(v);
	}
	return size;
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
inline uint32_t bit_size(const std::unordered_map<K,V>& value)
{
	uint32_t size = bit_size(uint32_t(value.size()));
	for (const auto& [k, v] : value) {
		size += bit_size(k) + bit_size(v);
	}
	return size;
}

template <typename K, typename V, typename WriterType>
inline void serialize(WriterType& writer, const std::unordered_map<K,V>& value)
{
	serialize(writer, uint32_t(value.size()));
	for (const auto& [key, value] : value) {
		serialize(writer, key);
		serialize(writer, value);
	}
}

template <typename K, typename V, typename ReaderType>
inline error::Error deserialize(std::unordered_map<K,V>& value, ReaderType& reader)
{
	uint32_t size;
	auto err = deserialize(size, reader);
	if (err) {
		return err;
	}
	value.reserve(size);

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

template <typename T>
inline uint32_t bit_size(const std::set<T>& value)
{
	uint32_t size = bit_size(uint32_t(value.size()));
	for (const auto& item : value) {
		size += bit_size(item);
	}
	return size;
}

template <typename WriterType, typename T>
inline void serialize(WriterType& writer, const std::set<T>& value)
{
	serialize(writer, uint32_t(value.size()));
	for (const auto& item : value) {
		serialize(writer, item);
	}
}

template <typename ReaderType, typename T>
inline error::Error deserialize(std::set<T>& value, ReaderType& reader)
{
	uint32_t size;
	auto err = deserialize(size, reader);
	if (err) {
		return err;
	}
	for (auto i = uint32_t(0); i < size; i++) {
		T t;
		err = deserialize(t, reader);
		if (err) {
			return err;
		}
		value.insert(t);
	}
	return nullptr;
}

template <typename T>
inline uint32_t bit_size(const std::unordered_set<T>& value)
{
	uint32_t size = bit_size(uint32_t(value.size()));
	for (const auto& item : value) {
		size += bit_size(item);
	}
	return size;
}

template <typename WriterType, typename T>
inline void serialize(WriterType& writer, const std::unordered_set<T>& value)
{
	serialize(writer, uint32_t(value.size()));
	for (const auto& item : value) {
		serialize(writer, item);
	}
}

template <typename ReaderType, typename T>
inline error::Error deserialize(std::unordered_set<T>& value, ReaderType& reader)
{
	uint32_t size;
	auto err = deserialize(size, reader);
	if (err) {
		return err;
	}
	value.reserve(size);
	for (auto i = uint32_t(0); i < size; i++) {
		T t;
		err = deserialize(t, reader);
		if (err) {
			return err;
		}
		value.insert(t);
	}
	return nullptr;
}


template <typename T, size_t N>
inline uint32_t bit_size(const std::array<T, N>& value)
{
	uint32_t size = 0;
	for (uint32_t i = 0; i < N; i++) {
		size += bit_size(value[i]);
	}
	return size;
}

template <typename WriterType, typename T, size_t N>
inline void serialize(WriterType& writer, const std::array<T, N>& value)
{
	for (uint32_t i = 0; i < N; i++) {
		serialize(writer, value[i]);
	}
}

template <typename ReaderType, typename T, size_t N>
inline error::Error deserialize(std::array<T, N>& value, ReaderType& reader)
{
	for (auto i = uint32_t(0); i < N; i++) {
		T t;
		auto err = deserialize(t, reader);
		if (err) {
			return err;
		}
		value[i] = t;
	}
	return nullptr;
}

}
}
