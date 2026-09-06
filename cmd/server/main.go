package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/taiseimiyaji/echo-show-tools/internal/api"
	"github.com/taiseimiyaji/echo-show-tools/internal/orca"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "HTTP listen address (set explicitly for LAN access)")
	binary := flag.String("orca", "orca", "Orca CLI executable path")
	timeout := flag.Duration("timeout", 5*time.Second, "CLI timeout")
	web := flag.String("web", "web/dist", "Built UI directory")
	demo := flag.Bool("demo", false, "Use synthetic demo data; never connect to Orca")
	flag.Parse()
	if *timeout <= 0 {
		log.Fatal("timeout must be positive")
	}
	var source api.Source = &orca.Client{Binary: *binary, Timeout: *timeout}
	if *demo {
		source = orca.Demo{}
	}
	cache := &api.Cache{Source: source, TTL: time.Second, Timeout: *timeout + time.Second}
	if _, err := os.Stat(*web + "/index.html"); err != nil {
		log.Print("UI build not found; API is available. Run npm --prefix web ci && npm --prefix web run build.")
	}
	server := &http.Server{Addr: *addr, Handler: api.Handler(cache, http.FileServer(http.Dir(*web))), ReadHeaderTimeout: 5 * time.Second, WriteTimeout: *timeout + 10*time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("Agent Deck listening at http://%s (demo=%t)", *addr, *demo)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
