package p2p

import (
	"bufio"
	"encoding/json"
	"io"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// StreamImpl is the actual libp2p stream type
type StreamImpl = network.Stream

// Stream wraps libp2p stream with convenience methods
type Stream interface {
	io.ReadWriteCloser

	// Peer returns the remote peer ID
	Peer() peer.ID

	// SendJSON sends a JSON-encoded message
	SendJSON(v interface{}) error

	// ReceiveJSON receives a JSON-encoded message
	ReceiveJSON(v interface{}) error

	// SendBytes sends raw bytes with length prefix
	SendBytes(data []byte) error

	// ReceiveBytes receives raw bytes with length prefix
	ReceiveBytes() ([]byte, error)
}

// streamWrapper implements Stream interface
type streamWrapper struct {
	stream StreamImpl
}

// WrapStream wraps a libp2p stream with convenience methods
func WrapStream(s StreamImpl) Stream {
	return &streamWrapper{stream: s}
}

func (s *streamWrapper) Read(p []byte) (n int, err error) {
	return s.stream.Read(p)
}

func (s *streamWrapper) Write(p []byte) (n int, err error) {
	return s.stream.Write(p)
}

func (s *streamWrapper) Close() error {
	return s.stream.Close()
}

func (s *streamWrapper) Peer() peer.ID {
	return s.stream.Conn().RemotePeer()
}

func (s *streamWrapper) SendJSON(v interface{}) error {
	encoder := json.NewEncoder(s.stream)
	if err := encoder.Encode(v); err != nil {
		return err
	}
	return nil
}

func (s *streamWrapper) ReceiveJSON(v interface{}) error {
	decoder := json.NewDecoder(s.stream)
	return decoder.Decode(v)
}

func (s *streamWrapper) SendBytes(data []byte) error {
	writer := bufio.NewWriter(s.stream)

	// Write length prefix (4 bytes)
	length := uint32(len(data))
	if err := writeUint32(writer, length); err != nil {
		return err
	}

	// Write data
	if _, err := writer.Write(data); err != nil {
		return err
	}

	return writer.Flush()
}

func (s *streamWrapper) ReceiveBytes() ([]byte, error) {
	reader := bufio.NewReader(s.stream)

	// Read length prefix
	length, err := readUint32(reader)
	if err != nil {
		return nil, err
	}

	// Read data
	data := make([]byte, length)
	if _, err := io.ReadFull(reader, data); err != nil {
		return nil, err
	}

	return data, nil
}

// Helper functions for binary encoding
func writeUint32(w io.Writer, v uint32) error {
	buf := []byte{
		byte(v >> 24),
		byte(v >> 16),
		byte(v >> 8),
		byte(v),
	}
	_, err := w.Write(buf)
	return err
}

func readUint32(r io.Reader) (uint32, error) {
	buf := make([]byte, 4)
	if _, err := io.ReadFull(r, buf); err != nil {
		return 0, err
	}
	return uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3]), nil
}
