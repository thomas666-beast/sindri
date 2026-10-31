package chat

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Hub struct {
	mu        sync.RWMutex
	rooms     map[string]map[*Client]struct{}
	userRooms map[string]map[*Client]struct{}
	svc       *Service
}

type Client struct {
	UserID string
	Conn   *websocket.Conn
	subs   map[string]struct{}
	mu     sync.Mutex
	closed bool
}

type inbound struct {
	Type           string `json:"type"`
	ConversationID string `json:"conversation_id"`
	Body           string `json:"body"`
	IsTyping       bool   `json:"is_typing"`
}

type outbound struct {
	Type           string      `json:"type"`
	ConversationID string      `json:"conversation_id,omitempty"`
	Message        *Message    `json:"message,omitempty"`
	UserID         string      `json:"user_id,omitempty"`
	IsTyping       *bool       `json:"is_typing,omitempty"`
	Notification   interface{} `json:"notification,omitempty"`
	Error          string      `json:"error,omitempty"`
}

func NewHub(svc *Service) *Hub {
	return &Hub{
		rooms:     make(map[string]map[*Client]struct{}),
		userRooms: make(map[string]map[*Client]struct{}),
		svc:       svc,
	}
}

func (h *Hub) add(c *Client, convID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[convID] == nil {
		h.rooms[convID] = make(map[*Client]struct{})
	}
	h.rooms[convID][c] = struct{}{}
	c.subs[convID] = struct{}{}
}

func (h *Hub) remove(c *Client, convID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[convID] != nil {
		delete(h.rooms[convID], c)
		if len(h.rooms[convID]) == 0 {
			delete(h.rooms, convID)
		}
	}
	delete(c.subs, convID)
}

func (h *Hub) dropClient(c *Client) {
	h.mu.Lock()
	for convID := range c.subs {
		if h.rooms[convID] != nil {
			delete(h.rooms[convID], c)
			if len(h.rooms[convID]) == 0 {
				delete(h.rooms, convID)
			}
		}
	}
	c.subs = make(map[string]struct{})
	h.mu.Unlock()
	_ = c.Conn.Close()
}

func (h *Hub) registerUser(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.userRooms[c.UserID] == nil {
		h.userRooms[c.UserID] = make(map[*Client]struct{})
	}
	h.userRooms[c.UserID][c] = struct{}{}
}

func (h *Hub) unregisterUser(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.userRooms[c.UserID] != nil {
		delete(h.userRooms[c.UserID], c)
		if len(h.userRooms[c.UserID]) == 0 {
			delete(h.userRooms, c.UserID)
		}
	}
}

func (h *Hub) PushNotification(userID string, payload outbound) {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.userRooms[userID]))
	for c := range h.userRooms[userID] {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	data, _ := json.Marshal(payload)
	var dead []*Client
	for _, c := range clients {
		c.mu.Lock()
		if !c.closed {
			_ = c.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := c.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
				c.closed = true
				dead = append(dead, c)
			}
		}
		c.mu.Unlock()
	}
	for _, c := range dead {
		h.dropClient(c)
	}
}

func NewOutboundNotification(n interface{}) outbound {
	return outbound{
		Type:         "notification",
		Notification: n,
	}
}

func (h *Hub) broadcast(convID string, payload outbound) {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.rooms[convID]))
	for c := range h.rooms[convID] {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	data, _ := json.Marshal(payload)
	var dead []*Client
	for _, c := range clients {
		c.mu.Lock()
		if !c.closed {
			_ = c.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := c.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
				c.closed = true
				dead = append(dead, c)
			}
		}
		c.mu.Unlock()
	}
	for _, c := range dead {
		h.dropClient(c)
	}
}

func (h *Hub) broadcastExcept(convID string, sender *Client, payload outbound) {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.rooms[convID]))
	for c := range h.rooms[convID] {
		if c != sender {
			clients = append(clients, c)
		}
	}
	h.mu.RUnlock()

	data, _ := json.Marshal(payload)
	var dead []*Client
	for _, c := range clients {
		c.mu.Lock()
		if !c.closed {
			_ = c.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := c.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
				c.closed = true
				dead = append(dead, c)
			}
		}
		c.mu.Unlock()
	}
	for _, c := range dead {
		h.dropClient(c)
	}
}

// BroadcastMessage sends a message to everyone in the conversation room.
// Used by the HTTP upload handler to push attachments in real-time.
func (h *Hub) BroadcastMessage(convID string, authorID string, msg *Message) {
	// 1. Send to the room
	h.broadcast(convID, outbound{
		Type: "message", ConversationID: convID, Message: msg,
	})
	
	// 2. Push unread badge to other members
	members, _ := h.svc.Members(context.Background(), convID)
	for _, memberID := range members {
		if memberID != authorID {
			h.PushNotification(memberID, outbound{
				Type:           "chat.unread",
				ConversationID: convID,
				UserID:         authorID,
			})
		}
	}
}

func (h *Hub) Serve(ctx context.Context, c *Client) {
	defer h.dropClient(c)
	defer h.unregisterUser(c)
	h.registerUser(c)

	c.subs = make(map[string]struct{})

	for {
		var in inbound
		if err := c.Conn.ReadJSON(&in); err != nil {
			return
		}
		switch in.Type {
		case "subscribe":
			if in.ConversationID == "" {
				h.sendError(c, "conversation_id required")
				continue
			}
			isMember, err := h.svc.isMember(ctx, in.ConversationID, c.UserID)
			if err != nil || !isMember {
				h.sendError(c, "not a member")
				continue
			}
			h.add(c, in.ConversationID)
			h.send(c, outbound{Type: "subscribed", ConversationID: in.ConversationID})
		case "unsubscribe":
			h.remove(c, in.ConversationID)
			h.send(c, outbound{Type: "ack", ConversationID: in.ConversationID})
		case "message":
			if in.ConversationID == "" || in.Body == "" {
				h.sendError(c, "conversation_id and body required")
				continue
			}
			msg, err := h.svc.SendMessage(ctx, in.ConversationID, c.UserID, in.Body)
			if err != nil {
				h.sendError(c, err.Error())
				continue
			}
			
			// 1. Broadcast the message to the room
			h.broadcast(in.ConversationID, outbound{
				Type: "message", ConversationID: in.ConversationID, Message: msg,
			})
			
			// 2. BROADCAST UNREAD TO OTHER MEMBERS
			members, _ := h.svc.Members(ctx, in.ConversationID)
			for _, memberID := range members {
				if memberID != c.UserID {
					h.PushNotification(memberID, outbound{
						Type:           "chat.unread",
						ConversationID: in.ConversationID,
						UserID:         c.UserID,
					})
				}
			}
		case "read":
			if in.ConversationID == "" {
				h.sendError(c, "conversation_id required")
				continue
			}
			err := h.svc.MarkRead(ctx, in.ConversationID, c.UserID)
			if err == nil {
				// FETCH MEMBERS DIRECTLY AND PUSH TO THEM
				members, _ := h.svc.Members(ctx, in.ConversationID)
				for _, memberID := range members {
					if memberID != c.UserID {
						h.PushNotification(memberID, outbound{
							Type:           "read",
							ConversationID: in.ConversationID,
							UserID:         c.UserID,
						})
					}
				}
			}
		case "typing":
			if in.ConversationID == "" {
				h.sendError(c, "conversation_id required")
				continue
			}
			isMember, err := h.svc.isMember(ctx, in.ConversationID, c.UserID)
			if err != nil || !isMember {
				h.sendError(c, "not a member")
				continue
			}
			isTyping := in.IsTyping
			h.broadcastExcept(in.ConversationID, c, outbound{
				Type:           "typing",
				ConversationID: in.ConversationID,
				UserID:         c.UserID,
				IsTyping:       &isTyping,
			})
		default:
			h.sendError(c, "unknown type")
		}
	}
}

func (h *Hub) send(c *Client, payload outbound) {
	data, _ := json.Marshal(payload)
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		_ = c.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := c.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
			c.closed = true
		}
	}
}

func (h *Hub) sendError(c *Client, msg string) {
	h.send(c, outbound{Type: "error", Error: msg})
	slog.Debug("chat ws error", "user", c.UserID, "error", msg)
}
