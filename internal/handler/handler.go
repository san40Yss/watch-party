package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"watchparty/internal/auth"
	"watchparty/internal/db"
	"watchparty/internal/processing"
	"watchparty/internal/room"
)

type Handler struct {
	pool          *pgxpool.Pool
	proc          *processing.Processor
	authsvc       *auth.Service
	hub           *room.Hub
	streamSecret  []byte // HMAC key for signed (cookie-free) stream links
	mediaRoot     string // container path to source media, e.g. /media
	processedRoot string // container path to processed output, e.g. /processed
}

func New(pool *pgxpool.Pool, proc *processing.Processor, authsvc *auth.Service, hub *room.Hub, streamSecret []byte) *Handler {
	return &Handler{
		pool:          pool,
		proc:          proc,
		authsvc:       authsvc,
		hub:           hub,
		streamSecret:  streamSecret,
		mediaRoot:     envOr("MEDIA_ROOT", "/media"),
		processedRoot: envOr("PROCESSED_ROOT", "/processed"),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (h *Handler) ListVideos(w http.ResponseWriter, r *http.Request) {
	videos, err := db.ListVideos(r.Context(), h.pool)
	if err != nil {
		log.Printf("list videos: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if videos == nil {
		videos = []db.Video{}
	}
	writeJSON(w, http.StatusOK, videos)
}

func (h *Handler) GetVideo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	video, err := db.GetVideo(r.Context(), h.pool, id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	tracks, _ := db.ListAudioTracks(r.Context(), h.pool, id)
	if tracks == nil {
		tracks = []db.AudioTrack{}
	}
	subs, _ := db.ListSubtitleTracks(r.Context(), h.pool, id)
	if subs == nil {
		subs = []db.SubtitleTrack{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"video":           video,
		"audio_tracks":    tracks,
		"subtitle_tracks": subs,
	})
}

// ProcessVideo kicks off probe-and-branch + remux/transcode in the background.
func (h *Handler) ProcessVideo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if _, err := db.GetVideo(r.Context(), h.pool, id); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	// Target height for the H.264 transcode (HEVC sources). Default 1440p (2K);
	// ignored when the video is copied/direct-play.
	height := 1440
	if q := r.URL.Query().Get("height"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n >= 360 && n <= 4320 {
			height = n
		}
	}
	started := h.proc.Enqueue(id, height)
	writeJSON(w, http.StatusAccepted, map[string]any{
		"started": started, // false = already processing
	})
}

// DeleteVideo removes a video from the library: its DB row (audio/subtitle
// tracks cascade) and its processed output directory. The original source file
// in the media library is left untouched.
func (h *Handler) DeleteVideo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if _, err := db.GetVideo(r.Context(), h.pool, id); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := db.DeleteVideo(r.Context(), h.pool, id); err != nil {
		log.Printf("delete video %d: %v", id, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Remove only the per-video processed dir; never the source media.
	_ = os.RemoveAll(filepath.Join(h.processedRoot, strconv.Itoa(id)))
	w.WriteHeader(http.StatusNoContent)
}

// streamSig computes the signature for a stream link: HMAC over the video id
// and the expiry, so a leaked link grants exactly one video until it expires.
func (h *Handler) streamSig(videoID int, exp int64) string {
	mac := hmac.New(sha256.New, h.streamSecret)
	fmt.Fprintf(mac, "%d:%d", videoID, exp)
	return hex.EncodeToString(mac.Sum(nil))
}

// validStreamToken reports whether the request carries a valid, unexpired
// signed token for this video (exp + sig query params).
func (h *Handler) validStreamToken(r *http.Request, videoID int) bool {
	expStr := r.URL.Query().Get("exp")
	sig := r.URL.Query().Get("sig")
	if expStr == "" || sig == "" {
		return false
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return false
	}
	return hmac.Equal([]byte(h.streamSig(videoID, exp)), []byte(sig))
}

// StreamLink issues a signed, cookie-free stream URL for external players
// (VR headsets, VLC): they can't log in, so the link itself is the credential.
func (h *Handler) StreamLink(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if _, err := db.GetVideo(r.Context(), h.pool, id); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	exp := time.Now().Add(48 * time.Hour).Unix()
	writeJSON(w, http.StatusOK, map[string]any{
		"url":        fmt.Sprintf("/api/videos/%d/stream?exp=%d&sig=%s", id, exp, h.streamSig(id, exp)),
		"expires_at": exp,
	})
}

// StreamVideo serves the processed MP4 when ready, otherwise falls back to the
// raw source. nginx does the actual byte serving via X-Accel-Redirect.
//
// Registered outside RequireAuth: it accepts either a session cookie or a
// signed token (external players can't log in).
func (h *Handler) StreamVideo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if auth.UserFrom(r.Context()) == nil && !h.validStreamToken(r, id) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	video, err := db.GetVideo(r.Context(), h.pool, id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	target := video.FilePath
	if video.Status == "ready" && video.ProcessedPath != nil {
		target = *video.ProcessedPath
	}

	w.Header().Set("X-Accel-Redirect", h.accelFor(target))
	w.Header().Set("X-Accel-Buffering", "no")
}

// HLSFile serves any file within a video's HLS package (master.m3u8, per-stream
// playlists, segments) via X-Accel-Redirect. The player requests these relative
// to /api/videos/{id}/hls/, e.g. /api/videos/2/hls/stream_aud0/seg_001.m4s.
func (h *Handler) HLSFile(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	rel := chi.URLParam(r, "*")
	// Reject path traversal — rel is attacker-controlled.
	if rel == "" || strings.Contains(rel, "..") {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	w.Header().Set("X-Accel-Redirect", "/internal-processed/"+strconv.Itoa(id)+"/"+rel)
}

// accelFor maps an absolute container path to the matching nginx internal
// location. Processed output lives under processedRoot; source files (and
// skip-if-ready direct play) live under mediaRoot.
func (h *Handler) accelFor(path string) string {
	if strings.HasPrefix(path, h.processedRoot) {
		return "/internal-processed" + relUnder(path, h.processedRoot)
	}
	return "/internal-files" + relUnder(path, h.mediaRoot)
}

// relUnder strips the given root prefix and returns a leading-slash path.
func relUnder(path, root string) string {
	rel := strings.TrimPrefix(path, root)
	if !strings.HasPrefix(rel, "/") {
		rel = "/" + rel
	}
	return rel
}

// --- Auth ---

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	user, err := h.authsvc.Login(w, r, body.Username, body.Password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, user)
}

// Register creates a new account and logs it in (sets the session cookie).
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	user, err := h.authsvc.Register(w, r, body.Username, body.Password)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	h.authsvc.Logout(w, r)
	w.WriteHeader(http.StatusNoContent)
}

// ChangePassword updates the current user's password (verifies the old one).
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r.Context())
	if user == nil { // RequireAuth should prevent this, but guard anyway.
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := h.authsvc.ChangePassword(r, user.ID, body.Current, body.New); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Me returns the current user (or null when anonymous).
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, auth.UserFrom(r.Context()))
}
