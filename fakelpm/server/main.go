package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"FakeLPM/fakelpm"
)

// Server
func main() {
	port := flag.Int("port", 5001, "Server port")
	flag.Parse()

	// Start server
	server, _ := fakelpm.New(fmt.Sprintf(":%d", *port))
	log.Printf("Server starting on port %d", *port)

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	<-sigChan
	log.Println("Shutting down server...")
	server.Stop()
}
