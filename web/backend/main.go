package main

import (
	"log"

	"github.com/sipeed/picoclaw/web/backend/api"
	"github.com/sipeed/picoclaw/web/backend/ws"
)

func main() {
	log.Println("Starting picoclaw Web Console...")

	// Initialize Server components
	srv := NewServer(
		api.NewHandler(),
		ws.NewHandler(),
	)

	// Start the Server
	if err := srv.Start(":8080"); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
