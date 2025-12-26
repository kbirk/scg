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
)

const (
	ErrorResponse   = uint8(0x01)
	MessageResponse = uint8(0x02)
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
