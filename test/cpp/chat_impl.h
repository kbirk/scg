#pragma once

// Chat streaming service implementation shared by the in-process C++ test suite
// and the standalone cross-language test servers, so the server behavior the
// streaming tests assert against is defined exactly once.
//
//   connect (bidi):  pushes "welcome" on open, echoes "echo:<text>" per message,
//                    a "fail" message ends the stream with an error, a "flood"
//                    message pushes 100 messages (to exercise backpressure), and
//                    a client half-close yields a "summary" with the echo count.
//   subscribe (server-streaming): pushes req.count "event-<i>" messages.
//   upload (client-streaming):    sums message seqs and returns the total.

#include <memory>
#include <string>
#include <utility>

#include "pingpong/pingpong.h"

class ChatServerImpl : public pingpong::ChatServer {
public:
	scg::error::Error connect(std::shared_ptr<pingpong::Chat_ConnectStreamServer> stream) override {
		pingpong::ChatMessage welcome;
		welcome.text = "welcome";
		welcome.seq = 0;
		auto err = stream->send(welcome);
		if (err) {
			return err;
		}

		int32_t count = 0;
		for (;;) {
			auto r = stream->recv();
			if (r.state == scg::rpc::StreamRecvState::Closed) {
				if (r.error) {
					return r.error; // client cancelled / connection dropped
				}
				// client half-closed: send a final summary and close cleanly
				pingpong::ChatMessage summary;
				summary.text = "summary";
				summary.seq = count;
				return stream->send(summary);
			}
			if (r.message.text == "fail") {
				return scg::error::Error("requested failure");
			}
			if (r.message.text == "flood") {
				// Push many messages rapidly to overflow a slow client's bounded buffer.
				for (int i = 0; i < 100; i++) {
					pingpong::ChatMessage f;
					f.text = "flood-" + std::to_string(i);
					f.seq = i;
					auto e = stream->send(f);
					if (e) {
						return e;
					}
				}
				continue;
			}
			count++;
			pingpong::ChatMessage echo;
			echo.text = "echo:" + r.message.text;
			echo.seq = r.message.seq + 1;
			err = stream->send(echo);
			if (err) {
				return err;
			}
		}
	}

	// Subscribe is a server-streaming handler: push req.count events and return.
	scg::error::Error subscribe(const pingpong::SubscribeRequest& req, std::shared_ptr<pingpong::Chat_SubscribeStreamServer> stream) override {
		for (int32_t i = 0; i < req.count; i++) {
			pingpong::ChatMessage m;
			m.text = "event-" + std::to_string(i);
			m.seq = i;
			auto err = stream->send(m);
			if (err) {
				return err;
			}
		}
		return nullptr;
	}

	// Upload is a client-streaming handler: sum the seqs and return a summary.
	std::pair<pingpong::UploadSummary, scg::error::Error> upload(std::shared_ptr<pingpong::Chat_UploadStreamServer> stream) override {
		int32_t total = 0;
		for (;;) {
			auto r = stream->recv();
			if (r.state == scg::rpc::StreamRecvState::Closed) {
				if (r.error) {
					return std::make_pair(pingpong::UploadSummary{}, r.error);
				}
				pingpong::UploadSummary summary;
				summary.total = total;
				return std::make_pair(summary, nullptr);
			}
			total += r.message.seq;
		}
	}
};
