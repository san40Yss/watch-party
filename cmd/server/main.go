package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"watchparty/internal/auth"
	"watchparty/internal/db"
	"watchparty/internal/handler"
	"watchparty/internal/processing"
	"watchparty/internal/room"
	"watchparty/internal/upload"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://wp:wp@localhost:5432/watchparty?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	// Wait for Postgres to be ready (useful in docker compose startup)
	for i := range 15 {
		if err = pool.Ping(ctx); err == nil {
			break
		}
		log.Printf("waiting for DB (%d/15)...", i+1)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("DB not ready: %v", err)
	}
	log.Println("DB connected")

	// Any video stuck in 'processing' must be from a previous crash/restart —
	// its goroutine is gone. Reset so it can be retried.
	if n, err := db.ResetStuckProcessing(ctx, pool); err != nil {
		log.Printf("reset stuck processing: %v", err)
	} else if n > 0 {
		log.Printf("reset %d stuck processing video(s)", n)
	}

	// Expired sessions and stale rooms otherwise accumulate forever; purge on
	// startup and then daily.
	cleanup := func() {
		if n, err := db.DeleteExpiredSessions(ctx, pool); err != nil {
			log.Printf("purge sessions: %v", err)
		} else if n > 0 {
			log.Printf("purged %d expired session(s)", n)
		}
		if n, err := db.DeleteStaleRooms(ctx, pool, 48*time.Hour); err != nil {
			log.Printf("purge rooms: %v", err)
		} else if n > 0 {
			log.Printf("purged %d stale room(s)", n)
		}
	}
	cleanup()
	go func() {
		for range time.Tick(24 * time.Hour) {
			cleanup()
		}
	}()

	processedRoot := os.Getenv("PROCESSED_ROOT")
	if processedRoot == "" {
		processedRoot = "/processed"
	}
	mediaRoot := os.Getenv("MEDIA_ROOT")
	if mediaRoot == "" {
		mediaRoot = "/media"
	}
	proc := processing.New(pool, processedRoot)

	uploadHandler, err := upload.New(pool, mediaRoot)
	if err != nil {
		log.Fatalf("upload handler: %v", err)
	}

	// Auth seam. DEV_AUTO_LOGIN keeps the app open (anonymous → default user)
	// until a real login UI exists; flip it off to require authentication.
	devAutoLogin := os.Getenv("DEV_AUTO_LOGIN") != "false"
	authsvc := auth.New(pool, devAutoLogin)
	if err := authsvc.EnsureDefaultUser(ctx); err != nil {
		log.Fatalf("ensure default user: %v", err)
	}

	// Secret for signing direct stream links (VR headsets and other external
	// players can't send the session cookie). Generated once and persisted so
	// issued links survive restarts.
	candidate := make([]byte, 32)
	if _, err := rand.Read(candidate); err != nil {
		log.Fatalf("stream secret: %v", err)
	}
	streamSecret, err := db.GetOrCreateSetting(ctx, pool, "stream_secret", hex.EncodeToString(candidate))
	if err != nil {
		log.Fatalf("stream secret: %v", err)
	}

	hub := room.NewHub()
	h := handler.New(pool, proc, authsvc, hub, []byte(streamSecret))

	r := chi.NewRouter()
	r.Use(middleware.RealIP) // nginx fronts us; trust its X-Real-IP for rate limiting
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(authsvc.Middleware) // attaches current user to every request

	// Public: authentication endpoints. /me returns null when anonymous.
	// Credential endpoints are rate-limited per IP against brute force.
	authLimit := auth.RateLimit(10, time.Minute)
	r.With(authLimit).Post("/api/login", h.Login)
	r.With(authLimit).Post("/api/register", h.Register)
	r.Post("/api/logout", h.Logout)
	r.Get("/api/me", h.Me)

	// Streaming sits outside RequireAuth: it accepts a session cookie OR a
	// signed token (VR headsets and other external players can't log in).
	// HEAD too: some players probe with HEAD before streaming.
	r.Get("/api/videos/{id}/stream", h.StreamVideo)
	r.Head("/api/videos/{id}/stream", h.StreamVideo)

	// Protected: everything else requires a logged-in user. Same-origin player
	// requests (HLS playlists/segments, stream) carry the session cookie, so
	// they pass through fine.
	r.Group(func(r chi.Router) {
		r.Use(authsvc.RequireAuth)

		r.Post("/api/password", h.ChangePassword)

		// Watch-party rooms: create/lookup + the WebSocket (presence + sync).
		r.Post("/api/rooms", h.CreateRoom)
		r.Get("/api/rooms/{id}", h.GetRoom)
		r.Get("/api/rooms/{id}/ws", h.RoomWS)

		// Viewing is open to any authenticated user.
		r.Get("/api/videos", h.ListVideos)
		r.Get("/api/videos/{id}", h.GetVideo)
		// Mint a signed stream link for external players (48h validity).
		r.Post("/api/videos/{id}/streamlink", h.StreamLink)
		// HLS package (master playlist, per-stream playlists, segments).
		r.Get("/api/videos/{id}/hls/*", h.HLSFile)

		// Admin-only: mutating the library (so guests can't reprocess/delete/upload).
		r.Group(func(r chi.Router) {
			r.Use(authsvc.RequireAdmin)
			r.Post("/api/videos/{id}/process", h.ProcessVideo)
			r.Delete("/api/videos/{id}", h.DeleteVideo)
			// tus resumable uploads (POST/PATCH/HEAD/DELETE under this prefix).
			// tusd's router matches on the path with its base stripped, so strip
			// the prefix here (BasePath stays /api/upload/ for its Location URLs).
			r.Handle("/api/upload/*", http.StripPrefix("/api/upload", uploadHandler))
		})
	})

	// Timeouts guard against slow/idle clients holding connections. Responses
	// here are tiny (JSON + X-Accel headers); nginx serves the actual bytes.
	srv := &http.Server{
		Addr:              ":8080",
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	log.Printf("listening on %s", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}
