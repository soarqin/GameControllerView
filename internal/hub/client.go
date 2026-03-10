package hub

import (
	"encoding/json"
	"log"
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

// Client represents a connected WebSocket client.
type Client struct {
	hub           *Hub
	conn          *gws.Conn
	playerIndex   int          // 1-based player index this client is listening to
	wantsKeyMouse atomic.Int32 // 1 when client has subscribed to keyboard/mouse events; 0 otherwise
}

// NewClient creates a new Client attached to the hub.
func NewClient(hub *Hub, conn *gws.Conn) *Client {
	return &Client{
		hub:         hub,
		conn:        conn,
		playerIndex: 1, // Default to player 1
	}
}

// SetPlayerIndex sets the player index for this client.
func (c *Client) SetPlayerIndex(index int) {
	c.playerIndex = index
}

// Send queues a text message for asynchronous delivery to the client.
// It is goroutine-safe and non-blocking; gws manages the internal write queue.
func (c *Client) Send(data []byte) {
	c.conn.WriteAsync(gws.OpcodeText, data, nil)
}

// HandleMessage parses and dispatches a client command message.
// Called from the gws OnMessage event handler.
func (c *Client) HandleMessage(reader PlayerSwitcher, kmProvider KMStateProvider, message []byte) {
	var clientMsg ClientMessage
	if err := json.Unmarshal(message, &clientMsg); err != nil {
		log.Printf("Error parsing client message: %v", err)
		return
	}

	switch clientMsg.Type {
	case "select_player":
		if reader.SetActiveByPlayerIndex(clientMsg.PlayerIndex) {
			c.SetPlayerIndex(clientMsg.PlayerIndex)
			msg := NewPlayerSelectedMessage(clientMsg.PlayerIndex)
			data, _ := json.Marshal(msg)
			c.Send(data)
			log.Printf("Client switched to player %d", clientMsg.PlayerIndex)
		} else {
			log.Printf("Failed to switch to player %d: invalid index", clientMsg.PlayerIndex)
		}
	case "subscribe_km":
		c.wantsKeyMouse.Store(1)
		log.Printf("Client subscribed to keyboard/mouse events")
		if kmProvider != nil {
			kmProvider.SendInitialKMState(c)
		}
	}
}
