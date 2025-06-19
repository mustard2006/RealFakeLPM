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
	port := flag.Int("port", 5001, "Server port")
	flag.Parse()

	// Start server
	server := fakelpm.New(fmt.Sprintf(":%d", *port))
	go func() {
		log.Printf("Server starting on port %d", *port)
		if err := server.Start(); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Let server start
	time.Sleep(200 * time.Millisecond)

	// Setup client
	cl := client.New(fmt.Sprintf("localhost:%d", *port)) // Match server port
	if err := cl.Connect(); err != nil {
		log.Fatalf("Client failed: %v", err)
	}
	defer cl.Close()

	// Send requests
	if err := cl.SendDownloadRequest(true); err != nil {
		log.Printf("DT error: %v", err)
	}
	time.Sleep(1 * time.Second)
	if err := cl.SendDownloadRequest(false); err != nil {
		log.Printf("DP error: %v", err)
	}

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("Shutting down...")
	server.Stop()
}
