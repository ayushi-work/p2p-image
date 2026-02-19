package protocol

import (
	"encoding/json"
	"time"
)

// MessageType defines the type of P2P message
type MessageType uint8

const (
	MsgConsensusRequest  MessageType = 1
	MsgConsensusResponse MessageType = 2
	MsgProcessRequest    MessageType = 3
	MsgProcessResponse   MessageType = 4
	MsgHashStore         MessageType = 5
)

// ConsensusRequest is sent by gateway to all peers to check if hash exists
type ConsensusRequest struct {
	Hash      string    `json:"hash"`      // SHA-256 hex string
	Operation string    `json:"operation"` // "dither" or "remove_bg"
	Timestamp time.Time `json:"timestamp"`
}

// ConsensusResponse is sent by each peer back to gateway
type ConsensusResponse struct {
	PeerID string `json:"peer_id"`
	Hash   string `json:"hash"`
	Match  bool   `json:"match"` // true = hash exists locally, false = new
}

// HashStoreCommand tells all peers to store a hash after consensus
type HashStoreCommand struct {
	Hash      string    `json:"hash"`
	Timestamp time.Time `json:"timestamp"`
}

// ProcessRequest contains image data and processing parameters
type ProcessRequest struct {
	ImageData []byte                 `json:"image_data"`
	Operation string                 `json:"operation"` // "dither" or "remove_bg"
	Params    map[string]interface{} `json:"params"`    // operation-specific parameters
}

// ProcessResponse contains the processed image or error
type ProcessResponse struct {
	Success   bool   `json:"success"`
	ImageData []byte `json:"image_data,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Message wraps all message types with a type identifier
type Message struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// NewMessage creates a new message with the given type and payload
func NewMessage(msgType MessageType, payload interface{}) (*Message, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &Message{
		Type:    msgType,
		Payload: data,
	}, nil
}

// Decode decodes the message payload into the given interface
func (m *Message) Decode(v interface{}) error {
	return json.Unmarshal(m.Payload, v)
}
