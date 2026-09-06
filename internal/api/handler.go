package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/taiseimiyaji/echo-show-tools/internal/orca"
)

type Source interface {
	Sessions(context.Context) (orca.Snapshot, error)
}

// Cache shares one in-flight read and caches both success and failure briefly.
// A disconnected HTTP caller cannot cancel a read used by other callers.
type Cache struct {
	Source   Source
	TTL      time.Duration
	Timeout  time.Duration
	mu       sync.Mutex
	pending  chan struct{}
	until    time.Time
	snapshot orca.Snapshot
	err      error
}

func (c *Cache) Sessions(ctx context.Context) (orca.Snapshot, error) {
	for {
		c.mu.Lock()
		if time.Now().Before(c.until) {
			s, e := c.snapshot, c.err
			c.mu.Unlock()
			return s, e
		}
		if c.pending == nil {
			c.pending = make(chan struct{})
			go c.fetch()
		}
		done := c.pending
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return orca.Snapshot{}, ctx.Err()
		case <-done:
		}
	}
}

func (c *Cache) fetch() {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 6 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	s, e := c.Source.Sessions(ctx)
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		e = &orca.Error{Code: "timeout"}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snapshot, c.err = s, e
	ttl := c.TTL
	if ttl <= 0 {
		ttl = time.Second
	}
	c.until = time.Now().Add(ttl)
	close(c.pending)
	c.pending = nil
}

func Handler(source Source, static http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; frame-ancestors 'none'")
		if r.URL.Path == "/api/sessions" {
			w.Header().Set("Cache-Control", "no-store")
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", "GET")
				failure(w, 405, "method_not_allowed")
				return
			}
			s, err := source.Sessions(r.Context())
			if err != nil {
				code, status := "internal_error", 500
				var upstream *orca.Error
				if errors.As(err, &upstream) {
					code = upstream.Code
					switch code {
					case "orca_unavailable":
						status = 503
					case "timeout":
						status = 504
					default:
						status = 502
					}
				}
				if errors.Is(err, context.DeadlineExceeded) {
					code, status = "timeout", 504
				}
				failure(w, status, code)
				return
			}
			if s.Sessions == nil {
				s.Sessions = []orca.Session{}
			}
			write(w, 200, s)
			return
		}
		if r.URL.Path == "/api" || len(r.URL.Path) >= 5 && r.URL.Path[:5] == "/api/" {
			failure(w, 404, "not_found")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			failure(w, 405, "method_not_allowed")
			return
		}
		if static == nil {
			http.NotFound(w, r)
			return
		}
		static.ServeHTTP(w, r)
	})
}

func failure(w http.ResponseWriter, status int, code string) {
	write(w, status, map[string]any{"error": map[string]string{"code": code}})
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
