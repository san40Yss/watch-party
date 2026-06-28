// Package room is the live layer for watch-party rooms: a hub that tracks who's
// connected to each room (presence) and fans out messages. Room *existence* and
// playback anchor live in the DB; the hub only relays and tracks connections.
package room

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Member is a connected participant, as exposed in presence updates.
type Member struct {
	UserID   int    `json:"-"`
	Username string `json:"username"`
	IsHost   bool   `json:"isHost"`
}

// Command is a host control message received over the socket.
type Command struct {
	Type   string  `json:"type"` // "play" | "pause" | "seek"
	Time   float64 `json:"time"` // playback position (seconds)
	Paused bool    `json:"paused"`
}

type client struct {
	member Member
	send   chan []byte
}

type roomConns struct {
	mu      sync.Mutex
	clients map[*client]bool
}

func (r *roomConns) add(c *client) {
	r.mu.Lock()
	r.clients[c] = true
	r.mu.Unlock()
}

// remove drops the client and closes its send channel. Holding the lock while
// closing guarantees no concurrent broadcast sends to a closed channel (both
// take r.mu, and remove deletes from the map before closing).
func (r *roomConns) remove(c *client) {
	r.mu.Lock()
	if r.clients[c] {
		delete(r.clients, c)
		close(c.send)
	}
	r.mu.Unlock()
}

func (r *roomConns) empty() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.clients) == 0
}

func (r *roomConns) members() []Member {
	r.mu.Lock()
	defer r.mu.Unlock()
	ms := make([]Member, 0, len(r.clients))
	for c := range r.clients {
		ms = append(ms, c.member)
	}
	return ms
}

// broadcast queues a message to every client. A non-blocking send drops the
// message for a client whose buffer is full (it'll resync on the next state).
func (r *roomConns) broadcast(msg []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for c := range r.clients {
		select {
		case c.send <- msg:
		default:
		}
	}
}

func (r *roomConns) broadcastPresence() {
	msg, _ := json.Marshal(struct {
		Type    string   `json:"type"`
		Members []Member `json:"members"`
	}{"presence", r.members()})
	r.broadcast(msg)
}

// Hub owns all live rooms.
type Hub struct {
	mu    sync.Mutex
	rooms map[string]*roomConns
}

func NewHub() *Hub {
	return &Hub{rooms: map[string]*roomConns{}}
}

func (h *Hub) get(id string) *roomConns {
	h.mu.Lock()
	defer h.mu.Unlock()
	r := h.rooms[id]
	if r == nil {
		r = &roomConns{clients: map[*client]bool{}}
		h.rooms[id] = r
	}
	return r
}

func (h *Hub) drop(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r, ok := h.rooms[id]; ok && r.empty() {
		delete(h.rooms, id)
	}
}

// Broadcast sends a raw JSON message to everyone in a room (used for playback
// state). No-op if the room has no live connections.
func (h *Hub) Broadcast(roomID string, msg []byte) {
	h.mu.Lock()
	r := h.rooms[roomID]
	h.mu.Unlock()
	if r != nil {
		r.broadcast(msg)
	}
}

// Serve registers conn in the room, sends it the current roster + an optional
// hello (the current playback state), then pumps until the socket closes.
// onCommand handles host control messages; pass nil for guests (their messages
// are read-and-ignored, which still lets us detect disconnects).
func (h *Hub) Serve(ctx context.Context, conn *websocket.Conn, roomID string, m Member, hello []byte, onCommand func(Command)) {
	c := &client{member: m, send: make(chan []byte, 16)}
	r := h.get(roomID)
	r.add(c)

	r.broadcastPresence()
	if hello != nil {
		c.send <- hello // buffered, room just created the client
	}

	ctx, cancel := context.WithCancel(ctx)
	go writePump(ctx, conn, c)
	readPump(ctx, conn, onCommand) // blocks until disconnect

	cancel()
	r.remove(c)
	r.broadcastPresence()
	h.drop(roomID)
	conn.Close(websocket.StatusNormalClosure, "")
}

func writePump(ctx context.Context, conn *websocket.Conn, c *client) {
	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Write(wctx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				return
			}
		case <-ping.C:
			pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Ping(pctx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func readPump(ctx context.Context, conn *websocket.Conn, onCommand func(Command)) {
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if onCommand == nil {
			continue
		}
		var cmd Command
		if json.Unmarshal(data, &cmd) == nil {
			onCommand(cmd)
		}
	}
}
