package main

import (
	"fmt"
	"net/http"

	"github.com/sipeed/picoclaw/web/backend/api"
	"github.com/sipeed/picoclaw/web/backend/middleware"
	"github.com/sipeed/picoclaw/web/backend/ws"
)

// Server holds the components necessary to run the web UI backend.
type Server struct {
	apiHandler *api.Handler
	wsHandler  *ws.Handler
}

// NewServer initializes a new Server instance.
func NewServer(apiHandler *api.Handler, wsHandler *ws.Handler) *Server {
	return &Server{
		apiHandler: apiHandler,
		wsHandler:  wsHandler,
	}
}

// Start attaches the routes and begins listening on the specified address.
func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()

	// API Routes (e.g. /api/status)
	s.apiHandler.RegisterRoutes(mux)

	// WebSocket Routes
	s.wsHandler.RegisterRoutes(mux)

	// Frontend Embedded Assets
	registerEmbedRoutes(mux)

	// Apply middleware stack
	handler := middleware.Recoverer(
		middleware.Logger(
			middleware.JSONContentType(mux),
		),
	)

	fmt.Printf("WebUI listening on %s\n", addr)
	return http.ListenAndServe(addr, handler)
}
