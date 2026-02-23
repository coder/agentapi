package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kooshapari/agentapi/internal/routing"
	"github.com/kooshapari/agentapi/internal/server"
)

var (
	port        = flag.Int("port", 8318, "AgentAPI server port")
	cliproxyURL = flag.String("cliproxy", "http://127.0.0.1:8317", "cliproxy+bifrost backend URL")
)

func main() {
	flag.Parse()

	// Initialize agent routing layer (Bifrost extension)
	router, err := routing.NewAgentBifrost(*cliproxyURL)
	if err != nil {
		log.Fatalf("Failed to initialize agent bifrost: %v", err)
	}

	// Start the server
	srv := server.New(*port, router)
	
	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-quit
		log.Println("Shutting down agentapi...")
		srv.Shutdown()
	}()

	log.Printf("AgentAPI starting on port %d", *port)
	log.Printf("Connecting to cliproxy+bifrost at %s", *cliproxyURL)
	
	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
