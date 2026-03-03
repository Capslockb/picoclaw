package ws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleWebSocket(t *testing.T) {
	handler := NewHandler()

	req := httptest.NewRequest("GET", "/ws/chat", nil)
	rr := httptest.NewRecorder()
	handler.handleWebSocket(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handleWebSocket returned status %d, want %d", status, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "WebSocket chat functionality placeholder") {
		t.Errorf("Response body did not contain placeholder text, got: %s", body)
	}
}
