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
	PrefixSize         = 16
	RequestIDSize      = 8
	ServiceIDSize      = 8
	MethodIDSize       = 8
	ResponseTypeSize   = 1
	RequestHeaderSize  = PrefixSize + RequestIDSize + ServiceIDSize + MethodIDSize
	ResponseHeaderSize = PrefixSize + RequestIDSize + ResponseTypeSize
)

type Message interface {
	ByteSize() int
	ToJSON() ([]byte, error)
	FromJSON([]byte) error
	ToBytes() []byte
	FromBytes([]byte) error
	Serialize(*serialize.FixedSizeWriter)
	Deserialize(*serialize.Reader) error
}

const (
	ErrorResponse   = uint8(0x01)
	MessageResponse = uint8(0x02)
)

func SerializePrefix(writer *serialize.FixedSizeWriter, data [16]byte) {
	bs := writer.Next(16)
	copy(bs, data[:])
}

func DeserializePrefix(data *[16]byte, reader *serialize.Reader) error {
	bs, err := reader.Read(16)
	if err != nil {
		return err
	}
	copy((*data)[:], bs)
	return nil
}
