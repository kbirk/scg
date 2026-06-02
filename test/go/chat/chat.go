// Package chat holds the Chat streaming service implementation shared by
// the in-process test suite and the standalone cross-language test servers, so
// the server behavior the streaming tests assert against is defined exactly once.
package chat

import (
	"errors"
	"fmt"
	"io"

	"github.com/kbirk/scg/test/scg/generated/pingpong"
)

// ChatServer implements pingpong.ChatServer.
//
//	Connect (bidi):  pushes "welcome" on open, echoes "echo:<text>" per message,
//	                 a "fail" message ends the stream with an error, a "flood"
//	                 message pushes 100 messages (to exercise backpressure), and a
//	                 client half-close yields a final "summary" with the echo count.
//	Subscribe (server-streaming): pushes req.Count "event-<i>" messages.
//	Upload (client-streaming):    sums message seqs and returns the total.
type ChatServer struct{}

func (s *ChatServer) Connect(stream *pingpong.Chat_ConnectStreamServer) error {
	if err := stream.Send(&pingpong.ChatMessage{Text: "welcome", Seq: 0}); err != nil {
		return err
	}

	count := int32(0)
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return stream.Send(&pingpong.ChatMessage{Text: "summary", Seq: count})
		}
		if err != nil {
			return err
		}
		if msg.Text == "fail" {
			return errors.New("requested failure")
		}
		if msg.Text == "flood" {
			for i := 0; i < 100; i++ {
				if err := stream.Send(&pingpong.ChatMessage{Text: fmt.Sprintf("flood-%d", i), Seq: int32(i)}); err != nil {
					return err
				}
			}
			continue
		}
		count++
		if err := stream.Send(&pingpong.ChatMessage{Text: "echo:" + msg.Text, Seq: msg.Seq + 1}); err != nil {
			return err
		}
	}
}

func (s *ChatServer) Subscribe(req *pingpong.SubscribeRequest, stream *pingpong.Chat_SubscribeStreamServer) error {
	for i := int32(0); i < req.Count; i++ {
		if err := stream.Send(&pingpong.ChatMessage{Text: fmt.Sprintf("event-%d", i), Seq: i}); err != nil {
			return err
		}
	}
	return nil
}

func (s *ChatServer) Upload(stream *pingpong.Chat_UploadStreamServer) (*pingpong.UploadSummary, error) {
	total := int32(0)
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return &pingpong.UploadSummary{Total: total}, nil
		}
		if err != nil {
			return nil, err
		}
		total += msg.Seq
	}
}
