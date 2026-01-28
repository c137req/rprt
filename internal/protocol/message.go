package protocol

import (
	"encoding/binary"
	"fmt"
	"sync/atomic"
)

// message types for the tunnel wire protocol.
const (
	TypeHTTPRequest    uint8 = 1
	TypeHTTPResponse   uint8 = 2
	TypeBodyChunk      uint8 = 3
	TypeStreamClose    uint8 = 4
	TypePing           uint8 = 5
	TypePong           uint8 = 6
	TypeAuthChallenge  uint8 = 7
	TypeAuthResponse   uint8 = 8
)

// header size: 1 byte type + 4 byte stream id + 4 byte payload length.
const HeaderSize = 9

// maximum payload size per frame (64 KiB).
const MaxPayloadSize = 64 * 1024

// global stream id counter for unique identification.
var _stream_counter atomic.Uint64

// Frame represents a single wire-protocol frame.
type Frame struct {
	Type      uint8
	StreamID  uint32
	Payload   []byte
}

// NextStreamID returns a monotonically increasing stream identifier.
func NextStreamID() uint32 {
	return uint32(_stream_counter.Add(1))
}

// _encode_header writes the frame header into a 9-byte buffer.
func _encode_header(buf []byte, f *Frame) {
	buf[0] = f.Type
	binary.BigEndian.PutUint32(buf[1:5], f.StreamID)
	binary.BigEndian.PutUint32(buf[5:9], uint32(len(f.Payload)))
}

// _decode_header reads a frame header from a 9-byte buffer.
func _decode_header(buf []byte) (msgType uint8, streamID uint32, payloadLen uint32, err error) {
	if len(buf) < HeaderSize {
		return 0, 0, 0, fmt.Errorf("buffer too small for header: %d bytes", len(buf))
	}
	msgType = buf[0]
	streamID = binary.BigEndian.Uint32(buf[1:5])
	payloadLen = binary.BigEndian.Uint32(buf[5:9])
	if payloadLen > MaxPayloadSize {
		return 0, 0, 0, fmt.Errorf("payload size %d exceeds maximum %d", payloadLen, MaxPayloadSize)
	}
	return msgType, streamID, payloadLen, nil
}

// MarshalFrame serialises a frame into bytes (header + payload).
func MarshalFrame(f *Frame) ([]byte, error) {
	if len(f.Payload) > MaxPayloadSize {
		return nil, fmt.Errorf("payload size %d exceeds maximum %d", len(f.Payload), MaxPayloadSize)
	}
	buf := make([]byte, HeaderSize+len(f.Payload))
	_encode_header(buf, f)
	copy(buf[HeaderSize:], f.Payload)
	return buf, nil
}

// UnmarshalFrame deserialises bytes into a frame.
func UnmarshalFrame(data []byte) (*Frame, error) {
	msgType, streamID, payloadLen, err := _decode_header(data)
	if err != nil {
		return nil, err
	}
	totalLen := HeaderSize + int(payloadLen)
	if len(data) < totalLen {
		return nil, fmt.Errorf("data too short: have %d, need %d", len(data), totalLen)
	}
	payload := make([]byte, payloadLen)
	copy(payload, data[HeaderSize:totalLen])
	return &Frame{
		Type:     msgType,
		StreamID: streamID,
		Payload:  payload,
	}, nil
}
