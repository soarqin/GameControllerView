package hub

import (
	"encoding/json"
	"log/slog"
	"sync/atomic"

	"github.com/lxzan/gws"
)

// PlayerSwitcher defines the interface for switching active player index.
type PlayerSwitcher interface {
	SetActiveByPlayerIndex(int) bool
}

// KMStateProvider can send the current keyboard/mouse state to a client on demand.
type KMStateProvider interface {
	SendInitialKMState(c *Client)
}

// MouseSensitivitySetter can update the mouse movement sensitivity divisor.
type MouseSensitivitySetter interface {
	SetMouseSensitivity(float32)
}

// Client represents a connected WebSocket client.
type Client struct {
	hub           *Hub
	conn          *gws.Conn
	playerIndex   atomic.Int32 // 1-based player index this client is listening to
	wantsKeyMouse atomic.Int32 // 1 when client has subscribed to keyboard/mouse events; 0 otherwise
}

// NewClient creates a new Client attached to the hub.
func NewClient(hub *Hub, conn *gws.Conn) *Client {
	c := &Client{
		hub:  hub,
		conn: conn,
	}
	c.playerIndex.Store(1) // Default to player 1
	return c
}

// SetPlayerIndex sets the player index for this client.
// Safe to call from any goroutine.
func (c *Client) SetPlayerIndex(index int) {
	c.playerIndex.Store(int32(index))
}

// Send queues a text message for asynchronous delivery to the client.
// It is goroutine-safe and non-blocking; gws manages the internal write queue.
func (c *Client) Send(data []byte) {
	c.conn.WriteAsync(gws.OpcodeText, data, nil)
}

// HandleMessage parses and dispatches a client command message.
// Called from the gws OnMessage event handler.
func (c *Client) HandleMessage(reader PlayerSwitcher, kmProvider KMStateProvider, sensSetter MouseSensitivitySetter, message []byte) {
	var clientMsg ClientMessage
	if err := json.Unmarshal(message, &clientMsg); err != nil {
		slog.Error("error parsing client message", "error", err)
		return
	}

	switch clientMsg.Type {
	case "select_player":
		if reader.SetActiveByPlayerIndex(clientMsg.PlayerIndex) {
			c.SetPlayerIndex(clientMsg.PlayerIndex)
			msg := NewPlayerSelectedMessage(clientMsg.PlayerIndex)
			data, err := json.Marshal(msg)
			if err != nil {
				slog.Error("error marshaling player_selected message", "error", err)
				return
			}
			c.Send(data)
			slog.Info("client switched player", "player", clientMsg.PlayerIndex)
		} else {
			slog.Warn("failed to switch player: invalid index", "player", clientMsg.PlayerIndex)
		}
	case "subscribe_km":
		c.wantsKeyMouse.Store(1)
		slog.Info("client subscribed to keyboard/mouse events")
		if kmProvider != nil {
			kmProvider.SendInitialKMState(c)
		}
	case "set_mouse_sens":
		if sensSetter != nil && clientMsg.Value > 0 {
			sensSetter.SetMouseSensitivity(float32(clientMsg.Value))
			slog.Info("mouse sensitivity set", "value", clientMsg.Value)
		}
	}
}
