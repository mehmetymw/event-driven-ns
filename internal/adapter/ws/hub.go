package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/coder/websocket"
)

type StatusUpdate struct {
	NotificationID string `json:"notification_id"`
	Status         string `json:"status"`
	Timestamp      string `json:"timestamp"`
}

type Hub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]struct{}
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[*websocket.Conn]struct{}),
	}
}

func (h *Hub) Accept(w http.ResponseWriter, r *http.Request) error {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return err
	}

	h.mu.Lock()
	h.clients[conn] = struct{}{}
	h.mu.Unlock()

	go h.readPump(conn)
	return nil
}

func (h *Hub) Broadcast(notificationID string, status string, timestamp string) {
	data, err := json.Marshal(StatusUpdate{
		NotificationID: notificationID,
		Status:         status,
		Timestamp:      timestamp,
	})
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for conn := range h.clients {
		go func(c *websocket.Conn) {
			if err := c.Write(context.Background(), websocket.MessageText, data); err != nil {
				h.removeClient(c)
			}
		}(conn)
	}
}

func (h *Hub) readPump(conn *websocket.Conn) {
	defer h.removeClient(conn)
	for {
		_, _, err := conn.Read(context.Background())
		if err != nil {
			return
		}
	}
}

func (h *Hub) removeClient(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
	_ = conn.Close(websocket.StatusNormalClosure, "")
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
