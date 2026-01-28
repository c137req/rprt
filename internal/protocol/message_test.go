package protocol

import (
	"bytes"
	"testing"
)

func Test_marshal_unmarshal_round_trip(t *testing.T) {
	original := &Frame{
		Type:     TypeHTTPRequest,
		StreamID: 42,
		Payload:  []byte("hello world"),
	}

	data, err := MarshalFrame(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	decoded, err := UnmarshalFrame(data)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Type != original.Type {
		t.Errorf("type mismatch: got %d, want %d", decoded.Type, original.Type)
	}
	if decoded.StreamID != original.StreamID {
		t.Errorf("stream id mismatch: got %d, want %d", decoded.StreamID, original.StreamID)
	}
	if !bytes.Equal(decoded.Payload, original.Payload) {
		t.Errorf("payload mismatch: got %q, want %q", decoded.Payload, original.Payload)
	}
}

func Test_marshal_empty_payload(t *testing.T) {
	original := &Frame{
		Type:     TypePing,
		StreamID: 0,
		Payload:  nil,
	}

	data, err := MarshalFrame(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	if len(data) != HeaderSize {
		t.Errorf("expected %d bytes for empty payload, got %d", HeaderSize, len(data))
	}

	decoded, err := UnmarshalFrame(data)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Type != TypePing {
		t.Errorf("type mismatch: got %d, want %d", decoded.Type, TypePing)
	}
	if len(decoded.Payload) != 0 {
		t.Errorf("expected empty payload, got %d bytes", len(decoded.Payload))
	}
}

func Test_marshal_rejects_oversized_payload(t *testing.T) {
	oversized := &Frame{
		Type:     TypeHTTPRequest,
		StreamID: 1,
		Payload:  make([]byte, MaxPayloadSize+1),
	}

	_, err := MarshalFrame(oversized)
	if err == nil {
		t.Fatal("expected error for oversized payload")
	}
}

func Test_unmarshal_rejects_truncated_data(t *testing.T) {
	_, err := UnmarshalFrame([]byte{0x01, 0x02})
	if err == nil {
		t.Fatal("expected error for truncated data")
	}
}

func Test_all_message_types_round_trip(t *testing.T) {
	types := []uint8{
		TypeHTTPRequest, TypeHTTPResponse, TypeBodyChunk,
		TypeStreamClose, TypePing, TypePong,
		TypeAuthChallenge, TypeAuthResponse,
	}

	for _, msgType := range types {
		original := &Frame{
			Type:     msgType,
			StreamID: 100,
			Payload:  []byte("test"),
		}

		data, err := MarshalFrame(original)
		if err != nil {
			t.Fatalf("type %d: marshal failed: %v", msgType, err)
		}

		decoded, err := UnmarshalFrame(data)
		if err != nil {
			t.Fatalf("type %d: unmarshal failed: %v", msgType, err)
		}

		if decoded.Type != msgType {
			t.Errorf("type %d: got %d", msgType, decoded.Type)
		}
	}
}

func Test_next_stream_id_is_unique(t *testing.T) {
	id1 := NextStreamID()
	id2 := NextStreamID()
	if id1 == id2 {
		t.Errorf("expected unique stream ids, got %d twice", id1)
	}
	if id2 <= id1 {
		t.Errorf("expected monotonically increasing ids, got %d then %d", id1, id2)
	}
}
