package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterRoutes(t *testing.T) {
	handler := NewHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Verify that registered routes respond correctly
	req := httptest.NewRequest("GET", "/api/status", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("RegisterRoutes: /api/status returned status %d, want %d", status, http.StatusOK)
	}
}
