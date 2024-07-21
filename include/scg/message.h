#pragma once

#include "scg/error.h"

#include <cstdint>
#include <vector>

namespace scg {
namespace type {

	struct Message {
		virtual ~Message() = default;

		virtual std::vector<uint8_t> toJSON() const = 0;
		virtual void fromJSON(const std::vector<uint8_t>& data)  = 0;

		virtual std::vector<uint8_t> toBytes() const  = 0;
		virtual scg::error::Error fromBytes(const std::vector<uint8_t>& data)  = 0;
		virtual scg::error::Error fromBytes(const uint8_t* data, uint32_t size)  = 0;

		virtual std::vector<uint8_t> toBytesWithPrefix() const  = 0;
		virtual scg::error::Error fromBytesWithPrefix(const std::vector<uint8_t>& data)  = 0;
		virtual scg::error::Error fromBytesWithPrefix(const uint8_t* data, uint32_t size)  = 0;
	};

}
}
