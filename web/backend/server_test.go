package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sipeed/picoclaw/web/backend/api"
	"github.com/sipeed/picoclaw/web/backend/ws"
)

func TestNewServer(t *testing.T) {
	apiHandler := api.NewHandler()
	wsHandler := ws.NewHandler()

	srv := NewServer(apiHandler, wsHandler)

	if srv == nil {
		t.Fatal("Expected NewServer to return a valid instance, got nil")
	}
	if srv.apiHandler == nil || srv.wsHandler == nil {
		t.Error("Not all server components were correctly initialized")
	}
}

func TestEmbedRoutes(t *testing.T) {
	mux := http.NewServeMux()
	registerEmbedRoutes(mux)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}
