package protocol

import (
	"encoding/json"
	"fmt"

	"github.com/roygabriel/crux/pkg/types"
)

// Marshal serializes a Message into JSON bytes.
func Marshal(msg types.Message) ([]byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal message: %w", err)
	}
	return data, nil
}

// Unmarshal deserializes JSON bytes into a Message.
func Unmarshal(data []byte) (types.Message, error) {
	var msg types.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return types.Message{}, fmt.Errorf("unmarshal message: %w", err)
	}
	return msg, nil
}
