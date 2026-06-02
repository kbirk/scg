package rpc

import "github.com/kbirk/scg/pkg/serialize"

var (
	RequestPrefix = [16]byte{
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x73, 0x63, 0x67,
		0x2D, 0x72, 0x65, 0x71,
		0x75, 0x65, 0x73, 0x74}
	ResponsePrefix = [16]byte{
		0x00, 0x00, 0x00, 0x00,
		0x73, 0x63, 0x67, 0x2D,
		0x72, 0x65, 0x73, 0x70,
		0x6F, 0x6E, 0x73, 0x65}
	// StreamPrefix tags every streaming frame (OPEN/MSG/HALF_CLOSE/CLOSE). It is
	// distinct from the unary prefixes so the unary fast path is unchanged and a
	// streaming-capable peer interoperates with it. "scg-stream"
	StreamPrefix = [16]byte{
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x73, 0x63,
		0x67, 0x2D, 0x73, 0x74,
		0x72, 0x65, 0x61, 0x6D}
)

const (
	ErrorResponse   = uint8(0x01)
	MessageResponse = uint8(0x02)
)

// Streaming frame kinds. Carried as a uint8 immediately after the stream id.
const (
	StreamFrameOpen      = uint8(0x01) // client -> server: open a stream (ctx, serviceID, methodID)
	StreamFrameMessage   = uint8(0x02) // bidirectional: a single serialized message
	StreamFrameHalfClose = uint8(0x03) // sender is done sending, still receiving
	StreamFrameClose     = uint8(0x04) // terminal: status + message
	StreamFramePing      = uint8(0x05) // connection-level keepalive probe (stream id ignored)
	StreamFramePong      = uint8(0x06) // connection-level keepalive reply (stream id ignored)
	// StreamFrameWindowUpdate grants the sender `increment` more bytes of credit
	// for a stream (or the whole connection when streamID == 0). Flow control.
	StreamFrameWindowUpdate = uint8(0x07)
	// StreamFrameSettings carries the server-dictated flow-control parameters. It
	// is sent by the server only, as the first frame on every accepted connection
	// (streamID == 0). The client obeys it; a client that sends SETTINGS is a
	// protocol violation and the server closes the connection.
	StreamFrameSettings = uint8(0x08)
)

// Stream close statuses, carried in a CLOSE frame.
const (
	StreamStatusOK    = uint8(0x00)
	StreamStatusError = uint8(0x01)
)

type Message interface {
	BitSize() int
	ToJSON() ([]byte, error)
	FromJSON([]byte) error
	ToBytes() []byte
	FromBytes([]byte) error
	Serialize(*serialize.Writer)
	Deserialize(*serialize.Reader) error
}

func BitSizePrefix() int {
	return 16 * 8
}

func SerializePrefix(writer *serialize.Writer, data [16]byte) {
	for _, b := range data {
		writer.WriteBits(b, 8)
	}
}

func DeserializePrefix(data *[16]byte, reader *serialize.Reader) error {
	for i := 0; i < 16; i++ {
		var b byte
		if err := reader.ReadBits(&b, 8); err != nil {
			return err
		}
		data[i] = b
	}
	return nil
}
