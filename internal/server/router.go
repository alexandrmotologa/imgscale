package server

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// SetupRouter constructs the HTTP handler tree and mounts API, image transformation, and visual studio routes.
func SetupRouter(h *GatewayHandler, uiFS fs.FS) http.Handler {
	r := chi.NewRouter()

	// Base Middleware
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RealIP)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS", "HEAD"},
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   []string{"ETag", "X-Cache", "X-Transform-Duration-Ms", "Content-Length"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// System & Telemetry Endpoints
	r.Get("/healthz", h.HandleHealth)
	r.Get("/api/stats", h.HandleStats)
	r.Post("/api/sign", h.HandleSign)
	r.Get("/api/presets", h.HandlePresets)

	// Image Transformation Routes (with optional HMAC signature guard)
	r.Group(func(ig chi.Router) {
		ig.Use(HMACMiddleware(h.cfg.Security.Secret, h.cfg.Security.EnforceSignature))
		ig.Get("/v1/images/*", h.HandleImage)
		ig.Get("/v1/entropy/*", h.HandleEntropy)
	})

	// Embedded React 19 Studio UI
	if uiFS != nil {
		fileServer := http.FileServer(http.FS(uiFS))

		// Redirect root to /ui/
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/ui/", http.StatusFound)
		})

		// Serve static files under /ui/
		r.Get("/ui*", func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/ui")
			if path == "" || path == "/" {
				// Serve index.html
				indexData, err := fs.ReadFile(uiFS, "index.html")
				if err == nil {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(indexData)
					return
				}
			}

			// Try file server
			http.StripPrefix("/ui", fileServer).ServeHTTP(w, r)
		})
	}

	return r
}
