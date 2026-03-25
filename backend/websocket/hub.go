package websocket

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type Message struct {
	Type      string      `json:"type"`
	From      string      `json:"from"`
	FromName  string      `json:"from_name"`
	To        string      `json:"to,omitempty"`
	Content   string      `json:"content,omitempty"`
	RoomID    string      `json:"room_id"`
	MsgType   string      `json:"msg_type,omitempty"`
	FileURL   string      `json:"file_url,omitempty"`
	FileName  string      `json:"file_name,omitempty"`
	FileSize  int64       `json:"file_size,omitempty"`
	Timestamp string      `json:"timestamp"`
	Data      interface{} `json:"data,omitempty"`
}

type Client struct {
	ID     string
	Name   string
	Conn   *websocket.Conn
	Send   chan []byte
	RoomID string
}

type Hub struct {
	Clients    map[string]*Client
	Rooms      map[string]map[string]*Client
	Broadcast  chan *Message
	Register   chan *Client
	Unregister chan *Client
	OnMessage  func(msg *Message)
	mu         sync.RWMutex
}

var GlobalHub *Hub

func NewHub() *Hub {
	return &Hub{
		Clients:    make(map[string]*Client),
		Rooms:      make(map[string]map[string]*Client),
		Broadcast:  make(chan *Message, 256),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.Clients[client.ID] = client
			if h.Rooms[client.RoomID] == nil {
				h.Rooms[client.RoomID] = make(map[string]*Client)
			}
			h.Rooms[client.RoomID][client.ID] = client
			h.mu.Unlock()
			log.Printf("User %s (%s) joined room %s", client.Name, client.ID, client.RoomID)
			h.BroadcastUserList(client.RoomID)

		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.Clients[client.ID]; ok {
				delete(h.Clients, client.ID)
				if room, ok := h.Rooms[client.RoomID]; ok {
					delete(room, client.ID)
				}
				close(client.Send)
			}
			h.mu.Unlock()
			log.Printf("User %s (%s) left", client.Name, client.ID)
			h.BroadcastUserList(client.RoomID)

		case message := <-h.Broadcast:
			if h.OnMessage != nil {
				h.OnMessage(message)
			}
			h.mu.RLock()
			if room, ok := h.Rooms[message.RoomID]; ok {
				data, _ := json.Marshal(message)
				for _, client := range room {
					select {
					case client.Send <- data:
					default:
						close(client.Send)
						delete(h.Clients, client.ID)
						delete(room, client.ID)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) BroadcastUserList(roomID string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	users := make([]map[string]string, 0)
	if room, ok := h.Rooms[roomID]; ok {
		for _, client := range room {
			users = append(users, map[string]string{"id": client.ID, "name": client.Name})
		}
	}

	msg := &Message{Type: "user_list", RoomID: roomID, Data: users}
	if room, ok := h.Rooms[roomID]; ok {
		data, _ := json.Marshal(msg)
		for _, client := range room {
			select {
			case client.Send <- data:
			default:
			}
		}
	}
}

func (c *Client) ReadPump(hub *Hub) {
	defer func() {
		hub.Unregister <- c
		c.Conn.Close()
	}()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}
		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}
		msg.From = c.ID
		msg.FromName = c.Name

		switch msg.Type {
		case "chat", "file", "typing":
			hub.Broadcast <- &msg
		}
	}
}

func (c *Client) WritePump() {
	defer c.Conn.Close()
	for message := range c.Send {
		if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
			break
		}
	}
}