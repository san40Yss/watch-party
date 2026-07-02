package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	"watchparty/internal/auth"
	"watchparty/internal/db"
	"watchparty/internal/room"
)

// Unambiguous alphabet for room codes (no 0/O/1/I).
const roomIDAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func newRoomID() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = roomIDAlphabet[int(b[i])%len(roomIDAlphabet)]
	}
	return string(b), nil
}

// newConnID returns a short random id for one WebSocket connection.
func newConnID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// CreateRoom makes a new room hosted by the current user, optionally bound to a
// video. Returns the room (including its shareable id).
func (h *Handler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r.Context())
	var body struct {
		VideoID *int `json:"video_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body) // body is optional

	// Retry on the (astronomically unlikely) id collision; any other DB error
	// won't be fixed by a new code, so bail immediately.
	var id string
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		if id, err = newRoomID(); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		err = db.CreateRoom(r.Context(), h.pool, id, user.ID, body.VideoID)
		if err == nil || !db.IsUniqueViolation(err) {
			break
		}
	}
	if err != nil {
		http.Error(w, "could not create room", http.StatusInternalServerError)
		return
	}
	rm, err := db.GetRoom(r.Context(), h.pool, id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, rm)
}

// GetRoom returns room metadata (existence check + which video it's bound to).
func (h *Handler) GetRoom(w http.ResponseWriter, r *http.Request) {
	rm, err := db.GetRoom(r.Context(), h.pool, chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, rm)
}

// RoomWS upgrades to a WebSocket and joins the room: presence for everyone,
// playback control for the host.
func (h *Handler) RoomWS(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user := auth.UserFrom(r.Context())

	rm, err := db.GetRoom(r.Context(), h.pool, id)
	if err != nil {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}

	// A WebSocket outlives the server's WriteTimeout; clear the deadlines on the
	// hijacked connection so reads/writes aren't killed mid-session.
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})
	_ = rc.SetReadDeadline(time.Time{})

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return // Accept already wrote the error
	}

	isHost := user.ID == rm.HostID
	// ConnID makes each connection unique in presence — the same account in two
	// tabs must not produce duplicate identities (the UI keys members by it).
	member := room.Member{UserID: user.ID, Username: user.Username, IsHost: isHost, ConnID: newConnID()}

	// Only the host drives playback; guests' messages are ignored.
	var onCommand func(room.Command)
	if isHost {
		onCommand = func(cmd room.Command) { h.handleRoomCommand(id, cmd) }
	}

	// Greet the joining client with the room's current playback state so it can
	// sync immediately rather than waiting for the next host action.
	h.hub.Serve(r.Context(), conn, id, member, stateMsg(rm), onCommand)
}

// handleRoomCommand persists the host's new playback anchor and broadcasts it.
func (h *Handler) handleRoomCommand(roomID string, cmd room.Command) {
	switch cmd.Type {
	case "play", "pause", "seek":
	default:
		return
	}
	// Detached from the WS request context on purpose (the anchor write must
	// survive the socket), but bounded so a hung DB can't leak goroutines.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.UpdateRoomState(ctx, h.pool, roomID, cmd.Time, cmd.Paused); err != nil {
		return
	}
	msg, _ := json.Marshal(map[string]any{
		"type":       "state",
		"position":   cmd.Time,
		"paused":     cmd.Paused,
		"serverTime": time.Now().UnixMilli(),
	})
	h.hub.Broadcast(roomID, msg)
}

// stateMsg builds the "state" message for a room from its DB anchor.
func stateMsg(rm *db.Room) []byte {
	msg, _ := json.Marshal(map[string]any{
		"type":       "state",
		"position":   rm.LivePosition(),
		"paused":     rm.Paused,
		"serverTime": time.Now().UnixMilli(),
		"videoId":    rm.VideoID,
	})
	return msg
}
