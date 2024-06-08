#pragma once

#include <cstdint>

namespace scg {
namespace rpc {
	constexpr std::array<uint8_t, 16> REQUEST_PREFIX = {
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x73, 0x63, 0x67,
		0x2D, 0x72, 0x65, 0x71,
		0x75, 0x65, 0x73, 0x74};
	constexpr std::array<uint8_t, 16> RESPONSE_PREFIX ={
		0x00, 0x00, 0x00, 0x00,
		0x73, 0x63, 0x67, 0x2D,
		0x72, 0x65, 0x73, 0x70,
		0x6F, 0x6E, 0x73, 0x65};

	constexpr uint32_t PREFIX_SIZE        = 16;
	constexpr uint32_t REQUEST_ID_SIZE    = 8;
	constexpr uint32_t SERVICE_ID_SIZE    = 8;
	constexpr uint32_t METHOD_ID_SIZE     = 8;
	constexpr uint32_t RESPONSE_TYPE_SIZE = 1;
	constexpr uint32_t REQUEST_HEADER_SIZE = PREFIX_SIZE + REQUEST_ID_SIZE + SERVICE_ID_SIZE + METHOD_ID_SIZE;

	constexpr uint8_t ERROR_RESPONSE = 0x01;
	constexpr uint8_t MESSAGE_RESPONSE = 0x02;
}
}
