package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"FakeLPM/fakelpm"
)

// Client
func main() {
	port := flag.Int("port", 5001, "Server port")
	flag.Parse()

	// Setup client
	cl := fakelpm.NewClient(fmt.Sprintf("localhost:%d", *port))
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
	// Print measurements
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
	// Print measurements
	// for i, m := range measurements {
	// 	log.Printf("Measurement %d: %+v", i+1, m)
	// }
}
