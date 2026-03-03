package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/sipeed/picoclaw/web/backend/model"
)

// startTime records when the server was started, used to calculate uptime.
var startTime = time.Now()

// Version is set at build time via -ldflags.
var Version = "dev"

// handleStatus returns the current server status, version, and uptime.
//
//	GET /api/status
//	Response: 200 OK
//	{
//	  "status": "online",
//	  "version": "dev",
//	  "uptime": "2h30m15s"
//	}
func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	resp := model.StatusResponse{
		Status:  "online",
		Version: Version,
		Uptime:  time.Since(startTime).Round(time.Second).String(),
	}
	json.NewEncoder(w).Encode(resp)
}
