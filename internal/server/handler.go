package server

import (
	"log"
	"net/http"

	"github.com/lxzan/gws"
	"github.com/soar/inputview/internal/gamepad"
	"github.com/soar/inputview/internal/hub"
)

const sessionKeyClient = "client"

// wsHandler implements the gws.Event interface to handle WebSocket lifecycle events.
type wsHandler struct {
	gws.BuiltinEventHandler
	hub         *hub.Hub
	broadcaster *hub.Broadcaster
	reader      *gamepad.Reader
	sensSetter  hub.MouseSensitivitySetter
}

// OnOpen is called when a new WebSocket connection is established.
// It creates a Client, registers it with the Hub, and sends the initial gamepad state.
func (h *wsHandler) OnOpen(socket *gws.Conn) {
	client := hub.NewClient(h.hub, socket)
	socket.Session().Store(sessionKeyClient, client)
	h.hub.Register(client)
	h.broadcaster.SendInitialState(client)
}

// OnClose is called when a WebSocket connection is closed (gracefully or due to error).
// It unregisters the client from the Hub.
func (h *wsHandler) OnClose(socket *gws.Conn, err error) {
	v, ok := socket.Session().Load(sessionKeyClient)
	if !ok {
		return
	}
	client := v.(*hub.Client)
	h.hub.Unregister(client)
}

// OnMessage is called when a text or binary message is received from the client.
func (h *wsHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	v, ok := socket.Session().Load(sessionKeyClient)
	if !ok {
		return
	}
	client := v.(*hub.Client)
	client.HandleMessage(h.reader, h.broadcaster, h.sensSetter, message.Bytes())
}

func handleWebSocket(h *hub.Hub, b *hub.Broadcaster, reader *gamepad.Reader, sensSetter hub.MouseSensitivitySetter) http.HandlerFunc {
	handler := &wsHandler{
		hub:         h,
		broadcaster: b,
		reader:      reader,
		sensSetter:  sensSetter,
	}
	upgrader := gws.NewUpgrader(handler, &gws.ServerOption{
		// Allow all origins for local use
		Authorize: func(r *http.Request, session gws.SessionStorage) bool {
			return true
		},
	})

	return func(w http.ResponseWriter, r *http.Request) {
		socket, err := upgrader.Upgrade(w, r)
		if err != nil {
			log.Printf("WebSocket upgrade failed: %v", err)
			return
		}
		// ReadLoop drives the event loop (OnOpen, OnMessage, OnClose).
		// Run in a goroutine so the HTTP handler returns immediately.
		go socket.ReadLoop()
	}
}
