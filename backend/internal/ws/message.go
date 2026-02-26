package ws

import "encoding/json"

// Message is the envelope sent over the WebSocket connection. Every message
// has a Type that identifies the payload kind and a Data field carrying the
// actual payload.
type Message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// Encode serialises a Message to JSON bytes.
func (m *Message) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// DecodeMessage parses raw JSON bytes into a Message.
func DecodeMessage(raw []byte) (*Message, error) {
	var m Message
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
