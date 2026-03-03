package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sipeed/picoclaw/web/backend/model"
)

func TestHandleStatus(t *testing.T) {
	handler := NewHandler()

	req := httptest.NewRequest("GET", "/api/status", nil)
	rr := httptest.NewRecorder()
	handler.handleStatus(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handleStatus returned status %d, want %d", status, http.StatusOK)
	}

	var resp model.StatusResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to decode response JSON: %v", err)
	}

	if resp.Status != "online" {
		t.Errorf("Expected status 'online', got %q", resp.Status)
	}
	if resp.Version == "" {
		t.Error("Expected non-empty version")
	}
	if resp.Uptime == "" {
		t.Error("Expected non-empty uptime")
	}
}
