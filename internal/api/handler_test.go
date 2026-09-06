package api

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/taiseimiyaji/echo-show-tools/internal/orca"
)

type sourceFunc func(context.Context) (orca.Snapshot, error)

func (f sourceFunc) Sessions(c context.Context) (orca.Snapshot, error) { return f(c) }

func TestHTTP(t *testing.T) {
	for _, tc := range []struct {
		code   string
		status int
	}{{"orca_unavailable", 503}, {"timeout", 504}, {"invalid_response", 502}, {"upstream_error", 502}, {"", 200}} {
		t.Run(tc.code, func(t *testing.T) {
			h := Handler(sourceFunc(func(context.Context) (orca.Snapshot, error) {
				if tc.code != "" {
					return orca.Snapshot{}, &orca.Error{Code: tc.code}
				}
				return orca.Snapshot{}, nil
			}), nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", "/api/sessions", nil))
			if w.Code != tc.status || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
			if tc.code == "" && !strings.Contains(w.Body.String(), `"sessions":[]`) {
				t.Fatal(w.Body)
			}
		})
	}
	h := Handler(sourceFunc(func(context.Context) (orca.Snapshot, error) {
		return orca.Snapshot{}, errors.New("/private/secret prompt")
	}), nil)
	for _, tc := range []struct {
		method, path string
		status       int
	}{{"POST", "/api/sessions", 405}, {"HEAD", "/api/sessions", 405}, {"POST", "/api/sessions/term/send", 404}, {"GET", "/api/other", 404}, {"GET", "/api/sessions", 500}} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.status || strings.Contains(w.Body.String(), "secret") {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
	}
}

func TestCacheSharesFetchAndDoesNotMaskFailure(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	cache := &Cache{TTL: time.Minute, Source: sourceFunc(func(context.Context) (orca.Snapshot, error) {
		calls.Add(1)
		<-release
		return orca.Snapshot{Sessions: []orca.Session{{ID: "first"}}}, nil
	})}
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, e := cache.Sessions(context.Background())
			if e != nil || len(s.Sessions) != 1 {
				t.Errorf("%+v %v", s, e)
			}
		}()
	}
	close(release)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("calls=%d", calls.Load())
	}
	cache.mu.Lock()
	cache.until = time.Time{}
	cache.Source = sourceFunc(func(context.Context) (orca.Snapshot, error) {
		return orca.Snapshot{}, &orca.Error{Code: "upstream_error"}
	})
	cache.mu.Unlock()
	s, e := cache.Sessions(context.Background())
	if e == nil || len(s.Sessions) != 0 {
		t.Fatalf("stale success returned: %+v %v", s, e)
	}
}

func TestCancelledCallerDoesNotCancelSharedRead(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	c := &Cache{Source: sourceFunc(func(ctx context.Context) (orca.Snapshot, error) {
		close(started)
		<-release
		return orca.Snapshot{}, ctx.Err()
	})}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, e := c.Sessions(ctx); done <- e }()
	<-started
	cancel()
	if !errors.Is(<-done, context.Canceled) {
		t.Fatal("caller was not cancelled")
	}
	close(release)
	if _, e := c.Sessions(context.Background()); e != nil {
		t.Fatal(e)
	}
}
