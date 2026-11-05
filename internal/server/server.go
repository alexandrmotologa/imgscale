package server

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexandrmotologa/imgscale/internal/cache"
	"github.com/alexandrmotologa/imgscale/internal/config"
	"github.com/alexandrmotologa/imgscale/internal/storage"
)

// Server wraps the HTTP server lifecycle.
type Server struct {
	httpServer *http.Server
	cfg        *config.Config
	cache      *cache.TwoTierCache
}

// NewServer initializes dependencies and compiles the HTTP router.
func NewServer(cfg *config.Config, store storage.StorageSource, tc *cache.TwoTierCache, uiFS fs.FS) *Server {
	handler := NewGatewayHandler(cfg, store, tc)
	router := SetupRouter(handler, uiFS)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	return &Server{
		httpServer: srv,
		cfg:        cfg,
		cache:      tc,
	}
}

// Run boots the server and blocks until an interrupt signal is received.
func (s *Server) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start background cache cleaner
	s.cache.StartBackgroundSweeper(ctx)

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("imgscale gateway listening on http://localhost:%d (UI at /ui)", s.cfg.Server.Port)
		serverErrors <- s.httpServer.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server error: %w", err)
		}
	case sig := <-shutdown:
		log.Printf("shutdown signal received: %v, closing listener...", sig)
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			_ = s.httpServer.Close()
			return fmt.Errorf("could not stop server gracefully: %w", err)
		}
	}

	return nil
}
