package websocket

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Разрешаем подключения с любого origin (в продакшене нужно настроить правильно)
		return true
	},
}

type Hub struct {
	connections map[int64]map[*Connection]bool

	register   chan *Connection
	unregister chan *Connection

	broadcast chan *Message

	mu sync.RWMutex
}

type Connection struct {
	ws     *websocket.Conn
	userID int64
	send   chan *Message
	hub    *Hub
}

type Message struct {
	UserID  int64       `json:"user_id,omitempty"`
	Type    string      `json:"type"`
	Channel string      `json:"channel,omitempty"`
	Data    interface{} `json:"data"`
}

func NewHub() *Hub {
	return &Hub{
		connections: make(map[int64]map[*Connection]bool),
		register:    make(chan *Connection),
		unregister:  make(chan *Connection),
		broadcast:   make(chan *Message, 256),
	}
}

func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// On shutdown: collect connections and close underlying websocket connections
			// so read/write pumps receive errors and unregister themselves.
			h.mu.RLock()
			var conns []*Connection
			for _, m := range h.connections {
				for c := range m {
					conns = append(conns, c)
				}
			}
			h.mu.RUnlock()

			// Close websockets outside lock so unregister logic can acquire mu.
			for _, c := range conns {
				// best-effort close; ignore errors
				_ = c.ws.Close()
			}

			return
		case conn := <-h.register:
			h.mu.Lock()
			if h.connections[conn.userID] == nil {
				h.connections[conn.userID] = make(map[*Connection]bool)
			}
			h.connections[conn.userID][conn] = true
			h.mu.Unlock()

		case conn := <-h.unregister:
			h.mu.Lock()
			if connections, ok := h.connections[conn.userID]; ok {
				if _, exists := connections[conn]; exists {
					delete(connections, conn)
					close(conn.send)
					if len(connections) == 0 {
						delete(h.connections, conn.userID)
					}
				}
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			if connections, ok := h.connections[message.UserID]; ok {
				for conn := range connections {
					select {
					case conn.send <- message:
					default:
						close(conn.send)
						delete(connections, conn)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) Broadcast(userID int64, message *Message) {
	message.UserID = userID
	select {
	case h.broadcast <- message:
	default:
		log.Printf("Hub broadcast channel is full, dropping message for user %d", userID)
	}
}

func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request, userID int64) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	conn := &Connection{
		ws:     ws,
		userID: userID,
		send:   make(chan *Message, 256),
		hub:    h,
	}

	h.register <- conn

	go conn.writePump()
	go conn.readPump()
}

const (
	writeWait = 10 * time.Second

	pongWait = 60 * time.Second

	pingPeriod = (pongWait * 9) / 10
)

func (c *Connection) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.ws.Close()
	}()

	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
	}
}

func (c *Connection) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.ws.WriteJSON(message); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}

		case <-ticker.C:
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
