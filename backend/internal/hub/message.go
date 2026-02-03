package hub

import (
	"time"

	"github.com/soar/GameControllerView/backend/internal/gamepad"
)

// WSMessage represents a WebSocket message sent from server to client.
type WSMessage struct {
	Type        string                `json:"type"`         // Message type: "full", "delta", "event", "player_selected"
	Seq         int64                 `json:"seq"`          // Sequence number for ordering
	Timestamp   int64                 `json:"timestamp"`    // Unix timestamp in milliseconds
	Event       string                `json:"event,omitempty"`      // Event name for type "event"
	Data        *gamepad.GamepadState `json:"data,omitempty"`       // Full gamepad state for type "full" or "event"
	Changes     *gamepad.DeltaChanges `json:"changes,omitempty"`    // Delta changes for type "delta"
	PlayerIndex int                   `json:"playerIndex,omitempty"` // Player index for type "player_selected"
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

// NewEventMessage creates an "event" type message for special events.
func NewEventMessage(seq int64, event string, state *gamepad.GamepadState) *WSMessage {
	return &WSMessage{
		Type:      "event",
		Seq:       seq,
		Timestamp: time.Now().UnixMilli(),
		Event:     event,
		Data:      state,
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

// ClientMessage represents a message sent from the client to the server.
type ClientMessage struct {
	Type        string `json:"type"`
	PlayerIndex int    `json:"playerIndex,omitempty"`
}
