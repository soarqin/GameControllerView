package hub

import (
	"log"
	"sync"
)

// Hub manages WebSocket clients and broadcasts messages.
type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Register adds a new client to the hub.
func (h *Hub) Register(c *Client) {
	h.register <- c
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(c *Client) {
	h.unregister <- c
}

// BroadcastToPlayer sends a message to all clients with matching player index.
func (h *Hub) BroadcastToPlayer(msg []byte, playerIndex int) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.playerIndex == playerIndex {
			select {
			case client.send <- msg:
			default:
				// Client send buffer full, disconnect
				go func(c *Client) {
					h.unregister <- c
				}(client)
			}
		}
	}
}

// Run starts the hub's main loop. Should be run in a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("Client connected (total: %d)", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("Client disconnected (total: %d)", len(h.clients))
		}
	}
}
