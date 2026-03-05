package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
)

// gateway holds the state for the managed gateway process.
var gateway = struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	logs   *LogBuffer
	events *EventBroadcaster
}{
	logs:   NewLogBuffer(200),
	events: NewEventBroadcaster(),
}

// registerGatewayRoutes binds gateway lifecycle endpoints to the ServeMux.
func (h *Handler) registerGatewayRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/gateway/status", h.handleGatewayStatus)
	mux.HandleFunc("GET /api/gateway/events", h.handleGatewayEvents)
	mux.HandleFunc("POST /api/gateway/start", h.handleGatewayStart)
	mux.HandleFunc("POST /api/gateway/stop", h.handleGatewayStop)
	mux.HandleFunc("POST /api/gateway/restart", h.handleGatewayRestart)
}

// handleGatewayStart starts the picoclaw gateway subprocess.
//
//	POST /api/gateway/start
func (h *Handler) handleGatewayStart(w http.ResponseWriter, r *http.Request) {
	gateway.mu.Lock()
	defer gateway.mu.Unlock()

	// Prevent duplicate starts
	if gateway.cmd != nil && gateway.cmd.Process != nil {
		// Check if process is still alive (signal 0 doesn't kill, just checks)
		if err := gateway.cmd.Process.Signal(syscall.Signal(0)); err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]any{
				"status": "already_running",
				"pid":    gateway.cmd.Process.Pid,
			})
			return
		}
		// Process is dead, clean up
		gateway.cmd = nil
	}

	// Locate the picoclaw executable
	execPath := findPicoclawBinary()

	cmd := exec.Command(execPath, "gateway")

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create stdout pipe: %v", err), http.StatusInternalServerError)
		return
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create stderr pipe: %v", err), http.StatusInternalServerError)
		return
	}

	// Clear old logs for this new run
	gateway.logs.Reset()

	// Ensure Pico Channel is configured before starting gateway
	if _, err := h.ensurePicoChannel(); err != nil {
		log.Printf("Warning: failed to ensure pico channel: %v", err)
		// Non-fatal: gateway can still start without pico channel
	}

	if err := cmd.Start(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to start gateway: %v", err), http.StatusInternalServerError)
		return
	}

	gateway.cmd = cmd
	pid := cmd.Process.Pid
	log.Printf("Started picoclaw gateway (PID: %d) from %s", pid, execPath)

	// Broadcast starting event
	gateway.events.Broadcast(GatewayEvent{Status: "starting", PID: pid})

	// Capture stdout/stderr in background
	go scanPipe(stdoutPipe, gateway.logs)
	go scanPipe(stderrPipe, gateway.logs)

	// Wait for exit in background and clean up
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("Gateway process exited: %v", err)
		} else {
			log.Printf("Gateway process exited normally")
		}

		gateway.mu.Lock()
		if gateway.cmd == cmd {
			gateway.cmd = nil
		}
		gateway.mu.Unlock()

		// Broadcast stopped event
		gateway.events.Broadcast(GatewayEvent{Status: "stopped"})
	}()

	// Start a goroutine to probe health and broadcast "running" once ready
	go func() {
		for i := 0; i < 30; i++ { // try for up to 15 seconds
			time.Sleep(500 * time.Millisecond)
			gateway.mu.Lock()
			stillOurs := gateway.cmd == cmd
			gateway.mu.Unlock()
			if !stillOurs {
				return
			}
			cfg, err := config.LoadConfig(h.configPath)
			if err != nil {
				continue
			}
			healthHost := "127.0.0.1"
			if cfg.Gateway.Host != "" && cfg.Gateway.Host != "0.0.0.0" {
				healthHost = cfg.Gateway.Host
			}
			healthPort := cfg.Gateway.Port
			if healthPort == 0 {
				healthPort = 18790
			}
			healthURL := fmt.Sprintf("http://%s/health", net.JoinHostPort(healthHost, strconv.Itoa(healthPort)))
			client := http.Client{Timeout: 1 * time.Second}
			resp, err := client.Get(healthURL)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					gateway.events.Broadcast(GatewayEvent{Status: "running", PID: pid})
					return
				}
			}
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"pid":    cmd.Process.Pid,
	})
}

// handleGatewayStop stops the running gateway subprocess gracefully.
//
//	POST /api/gateway/stop
func (h *Handler) handleGatewayStop(w http.ResponseWriter, r *http.Request) {
	gateway.mu.Lock()
	defer gateway.mu.Unlock()

	if gateway.cmd == nil || gateway.cmd.Process == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": "not_running",
		})
		return
	}

	pid := gateway.cmd.Process.Pid

	// Send SIGTERM for graceful shutdown (SIGKILL on Windows)
	var sigErr error
	if runtime.GOOS == "windows" {
		sigErr = gateway.cmd.Process.Kill()
	} else {
		sigErr = gateway.cmd.Process.Signal(syscall.SIGTERM)
	}

	if sigErr != nil {
		http.Error(w, fmt.Sprintf("Failed to stop gateway (PID %d): %v", pid, sigErr), http.StatusInternalServerError)
		return
	}

	log.Printf("Sent stop signal to gateway (PID: %d)", pid)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"pid":    pid,
	})
}

// handleGatewayRestart stops the gateway (if running) and starts a new instance.
//
//	POST /api/gateway/restart
func (h *Handler) handleGatewayRestart(w http.ResponseWriter, r *http.Request) {
	gateway.mu.Lock()

	// Stop existing process if running
	if gateway.cmd != nil && gateway.cmd.Process != nil {
		if err := gateway.cmd.Process.Signal(syscall.Signal(0)); err == nil {
			// Process is alive, send SIGTERM
			if runtime.GOOS == "windows" {
				gateway.cmd.Process.Kill()
			} else {
				gateway.cmd.Process.Signal(syscall.SIGTERM)
			}

			// Wait briefly for it to exit
			gateway.mu.Unlock()
			time.Sleep(2 * time.Second)
			gateway.mu.Lock()
		}
		gateway.cmd = nil
	}

	gateway.mu.Unlock()

	// Start fresh via the existing handler
	h.handleGatewayStart(w, r)
}

// handleGatewayStatus returns the gateway run status, health info, and logs.
//
//	GET /api/gateway/status
func (h *Handler) handleGatewayStatus(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{}

	// Check process state
	gateway.mu.Lock()
	processAlive := false
	if gateway.cmd != nil && gateway.cmd.Process != nil {
		if err := gateway.cmd.Process.Signal(syscall.Signal(0)); err == nil {
			processAlive = true
			data["pid"] = gateway.cmd.Process.Pid
		}
	}
	gateway.mu.Unlock()

	if !processAlive {
		data["gateway_status"] = "stopped"
	} else {
		// Process is alive — probe its health endpoint
		cfg, err := config.LoadConfig(h.configPath)
		host := "127.0.0.1"
		port := 18790
		if err == nil && cfg != nil {
			if cfg.Gateway.Host != "" && cfg.Gateway.Host != "0.0.0.0" {
				host = cfg.Gateway.Host
			}
			if cfg.Gateway.Port != 0 {
				port = cfg.Gateway.Port
			}
		}

		url := fmt.Sprintf("http://%s/health", net.JoinHostPort(host, strconv.Itoa(port)))
		client := http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(url)

		if err != nil {
			data["gateway_status"] = "starting"
		} else {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				data["gateway_status"] = "error"
				data["status_code"] = resp.StatusCode
			} else {
				var healthData map[string]any
				if decErr := json.NewDecoder(resp.Body).Decode(&healthData); decErr != nil {
					data["gateway_status"] = "error"
				} else {
					for k, v := range healthData {
						data[k] = v
					}
					data["gateway_status"] = "running"
				}
			}
		}
	}

	// Append incremental log data
	appendGatewayLogs(r, data)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// appendGatewayLogs reads log_offset and log_run_id query params from the request
// and populates the response data map with incremental log lines.
func appendGatewayLogs(r *http.Request, data map[string]any) {
	clientOffset := 0
	clientRunID := -1

	if v := r.URL.Query().Get("log_offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			clientOffset = n
		}
	}

	if v := r.URL.Query().Get("log_run_id"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			clientRunID = n
		}
	}

	runID := gateway.logs.RunID()

	if runID == 0 {
		data["logs"] = []string{}
		data["log_total"] = 0
		data["log_run_id"] = 0
		return
	}

	// If runID changed, reset offset to get all logs from new run
	offset := clientOffset
	if clientRunID != runID {
		offset = 0
	}

	lines, total, runID := gateway.logs.LinesSince(offset)
	if lines == nil {
		lines = []string{}
	}

	data["logs"] = lines
	data["log_total"] = total
	data["log_run_id"] = runID
}

// handleGatewayEvents serves an SSE stream of gateway state change events.
//
//	GET /api/gateway/events
func (h *Handler) handleGatewayEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Subscribe to gateway events
	ch := gateway.events.Subscribe()
	defer gateway.events.Unsubscribe(ch)

	// Send initial status so the client doesn't start blank
	initial := h.currentGatewayStatus()
	fmt.Fprintf(w, "data: %s\n\n", initial)
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case data, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// currentGatewayStatus returns the current gateway status as a JSON string.
func (h *Handler) currentGatewayStatus() string {
	gateway.mu.Lock()
	defer gateway.mu.Unlock()

	event := GatewayEvent{Status: "stopped"}
	if gateway.cmd != nil && gateway.cmd.Process != nil {
		if err := gateway.cmd.Process.Signal(syscall.Signal(0)); err == nil {
			event.Status = "running"
			event.PID = gateway.cmd.Process.Pid
		}
	}

	data, _ := json.Marshal(event)
	return string(data)
}

// findPicoclawBinary locates the picoclaw executable.
// Tries the same directory as the current executable first, then falls back to $PATH.
func findPicoclawBinary() string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidate := filepath.Join(dir, "picoclaw")
		if runtime.GOOS == "windows" {
			candidate += ".exe"
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return "picoclaw"
}

// scanPipe reads lines from r and appends them to buf. Returns when r reaches EOF.
func scanPipe(r io.Reader, buf *LogBuffer) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		buf.Append(scanner.Text())
	}
}
