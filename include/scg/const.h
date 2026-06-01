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
	// STREAM_PREFIX tags every streaming frame (OPEN/MSG/HALF_CLOSE/CLOSE). It is
	// distinct from the unary prefixes so the unary fast path is unchanged. "scg-stream"
	constexpr std::array<uint8_t, 16> STREAM_PREFIX = {
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x73, 0x63,
		0x67, 0x2D, 0x73, 0x74,
		0x72, 0x65, 0x61, 0x6D};

	constexpr uint8_t ERROR_RESPONSE = 0x01;
	constexpr uint8_t MESSAGE_RESPONSE = 0x02;

	// Streaming frame kinds, carried as a uint8 immediately after the stream id.
	constexpr uint8_t STREAM_FRAME_OPEN = 0x01;        // client -> server: open (ctx, serviceID, methodID)
	constexpr uint8_t STREAM_FRAME_MESSAGE = 0x02;     // bidirectional: a single serialized message
	constexpr uint8_t STREAM_FRAME_HALF_CLOSE = 0x03;  // sender done sending, still receiving
	constexpr uint8_t STREAM_FRAME_CLOSE = 0x04;       // terminal: status + message
	constexpr uint8_t STREAM_FRAME_PING = 0x05;        // connection-level keepalive probe (stream id ignored)
	constexpr uint8_t STREAM_FRAME_PONG = 0x06;        // connection-level keepalive reply (stream id ignored)

	// Stream close statuses, carried in a CLOSE frame.
	constexpr uint8_t STREAM_STATUS_OK = 0x00;
	constexpr uint8_t STREAM_STATUS_ERROR = 0x01;
}
}
