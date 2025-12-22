package test

import (
	"testing"
	"time"

	"github.com/kbirk/scg/test/files/output/streaming"
)

// Test that we can compile and create the basic structures
func TestStreamingCodeGeneration(t *testing.T) {
	// Test message creation
	msg := &streaming.ChatMessage{
		Text:      "Test",
		Sender:    "Sender",
		Timestamp: 12345,
	}

	if msg.Text != "Test" {
		t.Errorf("Message field not set correctly")
	}

	// Test serialization
	bytes := msg.ToBytes()
	if len(bytes) == 0 {
		t.Error("Serialization produced empty bytes")
	}

	// Test deserialization
	msg2 := &streaming.ChatMessage{}
	err := msg2.FromBytes(bytes)
	if err != nil {
		t.Fatalf("Deserialization failed: %v", err)
	}

	if msg2.Text != msg.Text || msg2.Sender != msg.Sender || msg2.Timestamp != msg.Timestamp {
		t.Error("Deserialized message doesn't match original")
	}
}

// Test that all message types serialize/deserialize correctly
func TestStreamingMessageSerialization(t *testing.T) {
	tests := []struct {
		name    string
		message interface {
			ToBytes() []byte
			FromBytes([]byte) error
		}
		verify func(interface{}) error
	}{
		{
			name: "ChatMessage",
			message: &streaming.ChatMessage{
				Text:      "Hello",
				Sender:    "Alice",
				Timestamp: uint64(time.Now().Unix()),
			},
			verify: func(m interface{}) error {
				msg := m.(*streaming.ChatMessage)
				if msg.Text != "Hello" || msg.Sender != "Alice" {
					t.Error("ChatMessage fields not preserved")
				}
				return nil
			},
		},
		{
			name: "ChatResponse",
			message: &streaming.ChatResponse{
				Status:    "ok",
				MessageID: 12345,
			},
			verify: func(m interface{}) error {
				resp := m.(*streaming.ChatResponse)
				if resp.Status != "ok" || resp.MessageID != 12345 {
					t.Error("ChatResponse fields not preserved")
				}
				return nil
			},
		},
		{
			name: "ServerNotification",
			message: &streaming.ServerNotification{
				Message: "Test notification",
				Type:    "info",
			},
			verify: func(m interface{}) error {
				notif := m.(*streaming.ServerNotification)
				if notif.Message != "Test notification" || notif.Type != "info" {
					t.Error("ServerNotification fields not preserved")
				}
				return nil
			},
		},
		{
			name:    "Empty",
			message: &streaming.Empty{},
			verify: func(m interface{}) error {
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize
			bytes := tt.message.ToBytes()
			if len(bytes) == 0 && tt.name != "Empty" {
				t.Errorf("%s: Serialization produced empty bytes", tt.name)
			}

			// Deserialize into a new instance
			var newMsg interface {
				FromBytes([]byte) error
			}

			switch tt.name {
			case "ChatMessage":
				newMsg = &streaming.ChatMessage{}
			case "ChatResponse":
				newMsg = &streaming.ChatResponse{}
			case "ServerNotification":
				newMsg = &streaming.ServerNotification{}
			case "Empty":
				newMsg = &streaming.Empty{}
			}

			err := newMsg.FromBytes(bytes)
			if err != nil {
				t.Fatalf("%s: Deserialization failed: %v", tt.name, err)
			}

			// Verify
			if err := tt.verify(newMsg); err != nil {
				t.Errorf("%s: Verification failed: %v", tt.name, err)
			}
		})
	}
}

// Test that stream type structures are properly generated and exported
func TestStreamingTypesExist(t *testing.T) {
	// These should compile if types are properly exported
	var _ interface{} = &streaming.ChatStreamStreamClient{}
	var _ interface{} = &streaming.ChatStreamStreamServer{}

	// Handler interface should exist
	var _ streaming.ChatStreamStreamHandler = nil

	t.Log("All streaming types are properly exported")
}
