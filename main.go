package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"FakeLPM/client"
	"FakeLPM/fakelpm"
)

func main() {
	// Parse command line flags
	port := flag.Int("port", 5001, "Port to listen on")
	flag.Parse()

	// Create and start the server
	serverAddr := fmt.Sprintf(":%d", *port)
	server := fakelpm.New(serverAddr)

	// Start the server in a goroutine so it doesn't block
	go func() {
		log.Printf("Starting fake LPM server on port %d", *port)
		if err := server.Start(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Create and connect client to the same port (not port+2000)
	cl := client.New(fmt.Sprintf("localhost:%d", *port))
	err := cl.Connect()
	if err != nil {
		log.Fatalf("Client failed: %v", err)
	}
	defer cl.Close()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutting down server...")
	server.Stop()
	os.Exit(0)
}
