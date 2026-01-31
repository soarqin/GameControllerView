package hub

import (
	"encoding/json"
	"log"
	"time"

	"github.com/soar/GameControllerView/backend/internal/gamepad"
)

const (
	fullSyncInterval = 5 * time.Second
	deltaCountSync   = 100
)

// Broadcaster listens for gamepad state changes and broadcasts them to the hub.
type Broadcaster struct {
	hub       *Hub
	changes   <-chan gamepad.GamepadState
	lastState gamepad.GamepadState
	seq       int64
}

func NewBroadcaster(h *Hub, changes <-chan gamepad.GamepadState) *Broadcaster {
	return &Broadcaster{
		hub:     h,
		changes: changes,
	}
}

// Run starts the broadcaster loop. Should be run in a goroutine.
func (b *Broadcaster) Run() {
	ticker := time.NewTicker(fullSyncInterval)
	defer ticker.Stop()

	var deltaCount int64

	for {
		select {
		case state, ok := <-b.changes:
			if !ok {
				return
			}

			delta := gamepad.ComputeDelta(b.lastState, state)
			b.lastState = state

			if delta.IsEmpty() {
				continue
			}

			b.seq++
			deltaCount++

			// Send full sync periodically
			if deltaCount >= deltaCountSync {
				b.sendFull(state)
				deltaCount = 0
			} else {
				b.sendDelta(delta)
			}

		case <-ticker.C:
			if b.lastState.Connected {
				b.seq++
				b.sendFull(b.lastState)
			}
		}
	}
}

// SendInitialState sends the current full state to a newly connected client.
func (b *Broadcaster) SendInitialState(c *Client) {
	b.seq++
	msg := NewFullMessage(b.seq, &b.lastState)
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling initial state: %v", err)
		return
	}
	select {
	case c.send <- data:
	default:
	}
}

func (b *Broadcaster) sendFull(state gamepad.GamepadState) {
	msg := NewFullMessage(b.seq, &state)
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling full message: %v", err)
		return
	}
	b.hub.BroadcastToPlayer(data, state.PlayerIndex)
}

func (b *Broadcaster) sendDelta(delta *gamepad.DeltaChanges) {
	msg := NewDeltaMessage(b.seq, delta)
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling delta message: %v", err)
		return
	}
	// For delta, we need to get player index from lastState
	b.hub.BroadcastToPlayer(data, b.lastState.PlayerIndex)
}
