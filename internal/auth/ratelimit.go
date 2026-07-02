package auth

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// RateLimit returns middleware allowing at most limit requests per window per
// client IP, answering 429 beyond that. Used on login/register to blunt
// credential brute-forcing and account spam. In-memory sliding window — fine
// for a single instance with a handful of users.
func RateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	var mu sync.Mutex
	hits := map[string][]time.Time{}

	// Janitor: drop IPs whose attempts have all expired, so one-off visitors
	// don't accumulate in the map forever.
	go func() {
		for range time.Tick(10 * time.Minute) {
			mu.Lock()
			for ip, ts := range hits {
				if len(ts) == 0 || time.Since(ts[len(ts)-1]) >= window {
					delete(hits, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr // already a bare IP (e.g. behind chi RealIP)
			}

			mu.Lock()
			now := time.Now()
			recent := hits[ip][:0]
			for _, t := range hits[ip] {
				if now.Sub(t) < window {
					recent = append(recent, t)
				}
			}
			blocked := len(recent) >= limit
			if !blocked {
				recent = append(recent, now)
			}
			hits[ip] = recent
			mu.Unlock()

			if blocked {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"too_many_attempts"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
