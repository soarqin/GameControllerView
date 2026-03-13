package hub

import (
	"time"

	"github.com/soar/inputview/internal/gamepad"
	"github.com/soar/inputview/internal/input"
)

// WSMessage represents a WebSocket message sent from server to client.
type WSMessage struct {
	Type        string                `json:"type"`                  // Message type: "full", "delta", "player_selected", "km_full", "km_delta"
	Seq         int64                 `json:"seq"`                   // Sequence number for ordering
	Timestamp   int64                 `json:"timestamp"`             // Unix timestamp in milliseconds
	Data        *gamepad.GamepadState `json:"data,omitempty"`        // Full gamepad state for type "full"
	Changes     *gamepad.DeltaChanges `json:"changes,omitempty"`     // Delta changes for type "delta"
	PlayerIndex int                   `json:"playerIndex,omitempty"` // Player index for type "player_selected"
	KMState     *input.KeyMouseState  `json:"kmState,omitempty"`     // Full keyboard/mouse state for type "km_full"
	KMDelta     *input.KeyMouseDelta  `json:"kmDelta,omitempty"`     // Keyboard/mouse delta for type "km_delta"
}

// NewFullMessage creates a "full" type message containing complete gamepad state.
func NewFullMessage(seq int64, state *gamepad.GamepadState) *WSMessage {
	return &WSMessage{
		Type:      "full",
		Seq:       seq,
		Timestamp: time.Now().UnixMilli(),
		Data:      state,
	}
}

// NewDeltaMessage creates a "delta" type message containing only changed fields.
func NewDeltaMessage(seq int64, changes *gamepad.DeltaChanges) *WSMessage {
	return &WSMessage{
		Type:      "delta",
		Seq:       seq,
		Timestamp: time.Now().UnixMilli(),
		Changes:   changes,
	}
}

// NewPlayerSelectedMessage creates a "player_selected" confirmation message.
func NewPlayerSelectedMessage(playerIndex int) *WSMessage {
	return &WSMessage{
		Type:        "player_selected",
		Seq:         0,
		Timestamp:   time.Now().UnixMilli(),
		PlayerIndex: playerIndex,
	}
}

// NewKMFullMessage creates a "km_full" message with the complete keyboard/mouse state.
func NewKMFullMessage(seq int64, state *input.KeyMouseState) *WSMessage {
	return &WSMessage{
		Type:      "km_full",
		Seq:       seq,
		Timestamp: time.Now().UnixMilli(),
		KMState:   state,
	}
}

// NewKMDeltaMessage creates a "km_delta" message with only the changed keyboard/mouse fields.
func NewKMDeltaMessage(seq int64, delta *input.KeyMouseDelta) *WSMessage {
	return &WSMessage{
		Type:      "km_delta",
		Seq:       seq,
		Timestamp: time.Now().UnixMilli(),
		KMDelta:   delta,
	}
}

// ClientMessage represents a message sent from the client to the server.
type ClientMessage struct {
	Type        string  `json:"type"`
	PlayerIndex int     `json:"playerIndex,omitempty"`
	Value       float64 `json:"value,omitempty"` // Generic numeric value (e.g. mouse sensitivity)
}
