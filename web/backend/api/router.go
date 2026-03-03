package api

import "net/http"

// Handler serves HTTP API requests.
type Handler struct{}

// NewHandler creates an instance of the API handler.
func NewHandler() *Handler {
	return &Handler{}
}

// RegisterRoutes binds all API endpoint handlers to the ServeMux.
// All routes are registered under the /api/ prefix.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/status", h.handleStatus)
}
