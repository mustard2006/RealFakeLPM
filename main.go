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

func main() {
	// Parse command line flags
	port := flag.Int("port", 5001, "Port to listen on")
	flag.Parse()

	// Create and start the server
	server := fakelpm.New(fmt.Sprintf(":%d", *port))

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down server...")
		server.Stop()
		os.Exit(0)
	}()

	// Start the server
	log.Printf("Starting fake LPM server on port %d", *port)
	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

