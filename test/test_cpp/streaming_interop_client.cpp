#include "../files/output/streaming/streaming.h"
#include "scg/client.h"
#include "scg/tcp/transport_client.h"
#include <iostream>
#include <memory>

// Client-side stream handler
class ChatStreamClientHandler : public streaming::ChatStreamStreamClient {
public:
    explicit ChatStreamClientHandler(std::shared_ptr<scg::rpc::Stream> stream)
        : ChatStreamStreamClient(stream) {}

    std::pair<streaming::Empty, scg::error::Error> handleSendNotification(
        scg::context::Context&,
        const streaming::ServerNotification& req) override
    {
        std::cout << "[Client] Received notification: " << req.message << std::endl;
        return std::make_pair(streaming::Empty{}, nullptr);
    }
};

int main() {
    // Create TCP transport
    scg::tcp::ClientTransportConfig transportConfig;
    transportConfig.host = "127.0.0.1";
    transportConfig.port = 29999;
    auto transport = std::make_shared<scg::tcp::ClientTransportTCP>(transportConfig);

    // Create client
    auto client = std::make_shared<scg::rpc::Client>(scg::rpc::ClientConfig{transport});

    // Connect to server
    auto connectErr = client->connect();
    if (connectErr) {
        std::cerr << "Failed to connect: " << connectErr.message << std::endl;
        return 1;
    }
    std::cout << "Connected to server" << std::endl;

    // Open stream
    scg::context::Context openCtx;
    streaming::Empty openReq;
    auto [baseStream, openErr] = client->openStream<streaming::Empty>(
        openCtx, streaming::chatServiceID, streaming::chatService_OpenChatID, openReq);

    if (openErr) {
        std::cerr << "Failed to open stream: " << openErr.message << std::endl;
        return 1;
    }
    std::cout << "Stream opened" << std::endl;

    // Create stream handler
    auto stream = std::make_shared<ChatStreamClientHandler>(baseStream);

    // Send messages
    for (int i = 0; i < 5; i++) {
        streaming::ChatMessage msg;
        msg.text = "Test message " + std::to_string(i);
        msg.sender = "cpp-client";
        msg.timestamp = 0;

        scg::context::Context msgCtx;
        auto [resp, err] = stream->sendMessage(msgCtx, msg);
        if (err) {
            std::cerr << "Failed to send message " << i << ": " << err.message << std::endl;
            return 1;
        }
        std::cout << "Message " << i << " sent, status: " << resp.status << std::endl;
    }

    // Close stream
    stream->close();
    std::cout << "Stream closed" << std::endl;

    // Disconnect
    client->disconnect();
    std::cout << "Disconnected" << std::endl;

    return 0;
}
