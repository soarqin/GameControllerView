package server

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/soar/GameControllerView/backend/internal/gamepad"
	"github.com/soar/GameControllerView/backend/internal/hub"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for local use
	},
}

func handleWebSocket(h *hub.Hub, b *hub.Broadcaster, reader *gamepad.Reader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade failed: %v", err)
			return
		}

		client := hub.NewClient(h, conn)
		h.Register(client)

		// Send current state to the new client
		b.SendInitialState(client)

		// Start write pump
		go client.WritePump()
		// Start read pump with reader and broadcaster for handling client messages
		go client.ReadPumpWithHandler(reader, b)
	}
}
