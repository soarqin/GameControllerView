package hub

import (
	"time"

	"github.com/soar/GameControllerView/backend/internal/gamepad"
)

type WSMessage struct {
	Type      string                `json:"type"`
	Seq       int64                 `json:"seq"`
	Timestamp int64                 `json:"timestamp"`
	Event     string                `json:"event,omitempty"`
	Data      *gamepad.GamepadState `json:"data,omitempty"`
	Changes   *gamepad.DeltaChanges `json:"changes,omitempty"`
}

func NewFullMessage(seq int64, state *gamepad.GamepadState) *WSMessage {
	return &WSMessage{
		Type:      "full",
		Seq:       seq,
		Timestamp: time.Now().UnixMilli(),
		Data:      state,
	}
}

func NewDeltaMessage(seq int64, changes *gamepad.DeltaChanges) *WSMessage {
	return &WSMessage{
		Type:      "delta",
		Seq:       seq,
		Timestamp: time.Now().UnixMilli(),
		Changes:   changes,
	}
}

func NewEventMessage(seq int64, event string, state *gamepad.GamepadState) *WSMessage {
	return &WSMessage{
		Type:      "event",
		Seq:       seq,
		Timestamp: time.Now().UnixMilli(),
		Event:     event,
		Data:      state,
	}
}
