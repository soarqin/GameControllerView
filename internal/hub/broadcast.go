package hub

import (
	"encoding/json"
	"log/slog"
	"sync"
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
	mu          sync.Mutex // protects lastState, lastKMState, seq, kmSeq
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

			b.mu.Lock()
			delta := gamepad.ComputeDelta(b.lastState, state)
			b.lastState = state

			if delta.IsEmpty() {
				b.mu.Unlock()
				continue
			}

			b.seq++
			seq := b.seq
			playerIndex := state.PlayerIndex
			b.mu.Unlock()

			deltaCount++

			// Send full sync periodically
			if deltaCount >= deltaCountSync {
				b.broadcastFull(seq, state, playerIndex)
				deltaCount = 0
			} else {
				b.broadcastDelta(seq, delta, playerIndex)
			}

		case kmState, ok := <-b.kmChanges:
			if !ok {
				// km channel closed; nil it out so we stop selecting it
				b.kmChanges = nil
				continue
			}
			b.handleKMState(kmState)

		case <-ticker.C:
			b.mu.Lock()
			if b.lastState.Connected {
				b.seq++
				seq := b.seq
				stateCopy := b.lastState
				b.mu.Unlock()
				b.broadcastFull(seq, stateCopy, stateCopy.PlayerIndex)
			} else {
				b.mu.Unlock()
			}
		}
	}
}

// handleKMState computes the delta from the previous keyboard/mouse state and broadcasts it.
func (b *Broadcaster) handleKMState(curr input.KeyMouseState) {
	b.mu.Lock()
	delta := input.ComputeKeyMouseDelta(b.lastKMState, curr)
	b.lastKMState = curr

	if delta.IsEmpty() {
		b.mu.Unlock()
		return
	}

	b.kmSeq++
	seq := b.kmSeq
	b.mu.Unlock()
	b.broadcastKMDelta(seq, delta)
}

// SendInitialState sends the current full state to a newly connected client.
// Safe to call from any goroutine (e.g. gws OnOpen handler).
func (b *Broadcaster) SendInitialState(c *Client) {
	b.mu.Lock()
	b.seq++
	stateCopy := b.lastState
	seq := b.seq
	b.mu.Unlock()

	msg := NewFullMessage(seq, &stateCopy)
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("error marshaling initial state", "error", err)
		return
	}
	c.Send(data)
}

// SendInitialKMState sends the current full keyboard/mouse state to a newly subscribed client.
// Safe to call from any goroutine (e.g. gws OnMessage handler).
func (b *Broadcaster) SendInitialKMState(c *Client) {
	b.mu.Lock()
	b.kmSeq++
	// Deep-copy map fields to avoid racing with Run() goroutine.
	kmCopy := b.lastKMState
	kmCopy.Keys = make(map[uint16]bool, len(b.lastKMState.Keys))
	for k, v := range b.lastKMState.Keys {
		kmCopy.Keys[k] = v
	}
	kmCopy.MouseButtons = make(map[uint16]bool, len(b.lastKMState.MouseButtons))
	for k, v := range b.lastKMState.MouseButtons {
		kmCopy.MouseButtons[k] = v
	}
	seq := b.kmSeq
	b.mu.Unlock()

	msg := NewKMFullMessage(seq, &kmCopy)
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("error marshaling initial km state", "error", err)
		return
	}
	c.Send(data)
}

// broadcastFull marshals and broadcasts a full state message.
// All state is passed by value — no lock needed.
func (b *Broadcaster) broadcastFull(seq int64, state gamepad.GamepadState, playerIndex int) {
	msg := NewFullMessage(seq, &state)
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("error marshaling full message", "error", err)
		return
	}
	b.hub.BroadcastToPlayer(data, playerIndex)
}

// broadcastDelta marshals and broadcasts a delta message.
func (b *Broadcaster) broadcastDelta(seq int64, delta *gamepad.DeltaChanges, playerIndex int) {
	msg := NewDeltaMessage(seq, delta)
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("error marshaling delta message", "error", err)
		return
	}
	b.hub.BroadcastToPlayer(data, playerIndex)
}

// broadcastKMDelta marshals and broadcasts a keyboard/mouse delta message.
func (b *Broadcaster) broadcastKMDelta(seq int64, delta *input.KeyMouseDelta) {
	msg := NewKMDeltaMessage(seq, delta)
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("error marshaling km delta message", "error", err)
		return
	}
	b.hub.BroadcastKeyMouse(data)
}
