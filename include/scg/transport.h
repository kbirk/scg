#pragma once

#include <vector>
#include <memory>
#include <functional>
#include "scg/error.h"

namespace scg {
namespace rpc {

// Connection interface for transport abstraction
class Connection {
public:
    virtual ~Connection() = default;

    // Send binary data over the connection
    virtual error::Error send(const std::vector<uint8_t>& data) = 0;

    // Set callback for when messages are received
    virtual void setMessageHandler(std::function<void(const std::vector<uint8_t>&)> handler) = 0;

    // Set callback for when connection fails
    virtual void setFailHandler(std::function<void(const error::Error&)> handler) = 0;

    // Set callback for when connection closes
    virtual void setCloseHandler(std::function<void()> handler) = 0;

    // Close the connection
    virtual error::Error close() = 0;
};

// Client transport interface
class ClientTransport {
public:
    virtual ~ClientTransport() = default;

    // Connect to the server and return a connection
    virtual std::pair<std::shared_ptr<Connection>, error::Error> connect() = 0;

    // Cleanup and shutdown the transport
    virtual void shutdown() = 0;
};

} // namespace rpc
} // namespace scg
