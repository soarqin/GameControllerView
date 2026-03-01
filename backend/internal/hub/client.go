package hub

import (
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"
)

// PlayerSwitcher defines the interface for switching active player index.
type PlayerSwitcher interface {
	SetActiveByPlayerIndex(int) bool
}

// Client represents a connected WebSocket client.
type Client struct {
	hub         *Hub
	conn        *websocket.Conn
	send        chan []byte
	playerIndex int // 1-based player index this client is listening to
}

// NewClient creates a new Client attached to the hub.
func NewClient(hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		hub:         hub,
		conn:        conn,
		send:        make(chan []byte, 256),
		playerIndex: 1, // Default to player 1
	}
}

// SetPlayerIndex sets the player index for this client.
func (c *Client) SetPlayerIndex(index int) {
	c.playerIndex = index
}

// WritePump sends messages from the send channel to the WebSocket connection.
func (c *Client) WritePump() {
	defer func() {
		c.conn.Close()
	}()

	for msg := range c.send {
		err := c.conn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			break
		}
	}
}

// ReadPumpWithHandler reads messages from the WebSocket and handles client commands.
func (c *Client) ReadPumpWithHandler(reader PlayerSwitcher) {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		// Parse client message
		var clientMsg ClientMessage
		if err := json.Unmarshal(message, &clientMsg); err != nil {
			log.Printf("Error parsing client message: %v", err)
			continue
		}

		switch clientMsg.Type {
		case "select_player":
			// Handle player selection
			if reader.SetActiveByPlayerIndex(clientMsg.PlayerIndex) {
				c.SetPlayerIndex(clientMsg.PlayerIndex)
				// Send confirmation
				msg := NewPlayerSelectedMessage(clientMsg.PlayerIndex)
				data, _ := json.Marshal(msg)
				c.send <- data
				log.Printf("Client switched to player %d", clientMsg.PlayerIndex)
			} else {
				log.Printf("Failed to switch to player %d: invalid index", clientMsg.PlayerIndex)
			}
		}
	}
}
