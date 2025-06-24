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
	cl := client.New(fmt.Sprintf("localhost:%d", *port))
	cl.SetTimeout(15 * time.Second) // Set reasonable timeout

	if err := cl.Connect(); err != nil {
		log.Fatalf("Client failed to connect: %v", err)
	}
	defer cl.Close()

	// Send DT request (total download)
	log.Println("Sending DT request...")
	header, measurements, err := cl.SendDownloadRequest(true)
	if err != nil {
		log.Fatalf("DT request failed: %v", err)
	}
	log.Printf("Received header block:\n%+v", header)
	log.Printf("Received %d measurements:", len(measurements))
	for i, m := range measurements {
		log.Printf("Measurement %d: %+v", i+1, m)
	}

	// Wait a moment before next request
	time.Sleep(1 * time.Second)

	// Send DP request (partial download)
	log.Println("Sending DP request...")
	header, measurements, err = cl.SendDownloadRequest(false)
	if err != nil {
		log.Fatalf("DP request failed: %v", err)
	}
	log.Printf("Received header block:\n%+v", header)
	log.Printf("Received %d measurements:", len(measurements))
	for i, m := range measurements {
		log.Printf("Measurement %d: %+v", i+1, m)
	}

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("Shutting down...")
	server.Stop()
}
