package hub

import (
	"encoding/json"
	"log"
	"time"

	"github.com/soar/inputview/internal/gamepad"
	"github.com/soar/inputview/internal/input"
)

const (
	fullSyncInterval = 5 * time.Second
	deltaCountSync   = 100
)

// Broadcaster listens for gamepad state changes and broadcasts them to the hub.
type Broadcaster struct {
	hub         *Hub
	changes     <-chan gamepad.GamepadState
	kmChanges   <-chan input.KeyMouseState
	lastState   gamepad.GamepadState
	lastKMState input.KeyMouseState
	seq         int64
	kmSeq       int64
}

func NewBroadcaster(h *Hub, changes <-chan gamepad.GamepadState, kmChanges <-chan input.KeyMouseState) *Broadcaster {
	return &Broadcaster{
		hub:       h,
		changes:   changes,
		kmChanges: kmChanges,
		lastKMState: input.KeyMouseState{
			Keys:         make(map[uint16]bool),
			MouseButtons: make(map[uint16]bool),
		},
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

		case kmState, ok := <-b.kmChanges:
			if !ok {
				// km channel closed; nil it out so we stop selecting it
				b.kmChanges = nil
				continue
			}
			b.handleKMState(kmState)

		case <-ticker.C:
			if b.lastState.Connected {
				b.seq++
				b.sendFull(b.lastState)
			}
		}
	}
}

// handleKMState computes the delta from the previous keyboard/mouse state and broadcasts it.
func (b *Broadcaster) handleKMState(curr input.KeyMouseState) {
	delta := input.ComputeKeyMouseDelta(b.lastKMState, curr)
	b.lastKMState = curr

	if delta.IsEmpty() {
		return
	}

	b.kmSeq++
	b.sendKMDelta(delta)
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
	c.Send(data)
}

// SendInitialKMState sends the current full keyboard/mouse state to a newly subscribed client.
func (b *Broadcaster) SendInitialKMState(c *Client) {
	b.kmSeq++
	msg := NewKMFullMessage(b.kmSeq, &b.lastKMState)
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling initial km state: %v", err)
		return
	}
	c.Send(data)
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

func (b *Broadcaster) sendKMDelta(delta *input.KeyMouseDelta) {
	msg := NewKMDeltaMessage(b.kmSeq, delta)
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling km delta message: %v", err)
		return
	}
	b.hub.BroadcastKeyMouse(data)
}
