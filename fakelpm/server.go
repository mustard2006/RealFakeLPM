package fakelpm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

type Server struct {
	Addr        string
	Connections map[net.Conn]bool
	mu          sync.Mutex
	stopChan    chan struct{}
	StartTime   time.Time
	Location    *time.Location
}

func New(addr string) (*Server, error) {
	loc, err := detectTimezone()
	if err != nil {
		return nil, fmt.Errorf("timezone detection failed: %v", err)
	}

	return &Server{
		Addr:        addr,
		Connections: make(map[net.Conn]bool),
		stopChan:    make(chan struct{}),
		StartTime:   time.Now().In(loc),
		Location:    loc,
	}, nil
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	log.Printf("Server started at %s", s.StartTime.Format(time.RFC3339))
	log.Printf("Server listening on %s", s.Addr)

	for {
		select {
		case <-s.stopChan:
			return nil
		default:
			conn, err := ln.Accept()
			if err != nil {
				log.Printf("Accept error: %v", err)
				continue
			}

			s.mu.Lock()
			s.Connections[conn] = true
			s.mu.Unlock()

			// Send initial ACK on connection (Requirement 3)
			if _, err := conn.Write(BuildACKResponse()); err != nil {
				log.Printf("Failed to send initial ACK: %v", err)
				conn.Close()
				continue
			}

			go s.handleConnection(conn)
		}
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer func() {
		s.mu.Lock()
		delete(s.Connections, conn)
		s.mu.Unlock()
		conn.Close()

		loc, _ := time.LoadLocation("Europe/Rome")
		results, err := DecodeHistoricalMeasures(SampleMeasurements[0], loc)
		if err != nil {
			log.Fatal(err)
		}

		for _, result := range results {
			fmt.Printf("Measurement: %+v\n", result)
		}

		// Encode back to base64
		encoded, err := EncodeHistoricalMeasures(results)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println("Original base64 string:", SampleMeasurements[0])
		fmt.Println("Encoded data:", encoded)
	}()

	log.Printf("New connection from %s", conn.RemoteAddr())

	buf := make([]byte, 2048)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			log.Printf("Read error: %v", err)
			return
		}

		req, err := ParseRequest(buf[:n])
		if err != nil {
			log.Printf("Invalid request: %v", err)
			if bytes.Contains(buf[:n], []byte{STX}) && bytes.Contains(buf[:n], []byte{ETX}) {
				if _, err := conn.Write(BuildNAKResponse()); err != nil {
					log.Printf("Failed to send NAK: %v", err)
				}
			}
			continue
		}

		switch string(req.Command[:]) {
		case "DT", "DP":
			log.Printf("Received %s request - Measures download", string(req.Command[:]))
			headerBytes := BuildHeaderResponse(s, req)
			if _, err := conn.Write(headerBytes); err != nil {
				log.Printf("Failed to send header: %v", err)
				return
			}
			log.Printf("Sent Header block for %s request", string(req.Command[:]))

			// Wait for client to acknowledge header
			ackBuf := make([]byte, 11)
			if _, err := conn.Read(ackBuf); err != nil {
				log.Printf("Failed to read header ACK: %v", err)
				return
			}

			// Send measurements
			numMeasurements := 3 + rand.Intn(8)
			for i := 0; i < numMeasurements; i++ {
				measurement := NewRandomMeasurement()
				measurementBytes := measurementToBytes(measurement)
				if _, err := conn.Write(measurementBytes); err != nil {
					log.Printf("Failed to send measurement: %v", err)
					return
				}
				log.Printf("Sent measurement %d/%d", i+1, numMeasurements)

				// Wait for ACK with timeout
				ackBuf := make([]byte, 11)
				conn.SetReadDeadline(time.Now().Add(5 * time.Second))
				n, err := conn.Read(ackBuf)
				conn.SetReadDeadline(time.Time{})
				if err != nil {
					log.Printf("Failed to read measurement ACK: %v", err)
					return
				}
				log.Printf("received session ACK: %q", ackBuf[:n])
			}

			// Send final package
			final := NewFinal()
			final.CalculateFinalChecksum()
			finalBytes := final.Bytes()
			if _, err := conn.Write(finalBytes); err != nil {
				log.Printf("Failed to send final package: %v", err)
				return
			}
			log.Printf("Sent final package")

		default:
			log.Printf("Unknown command: %s", req.Command[:])
			if _, err := conn.Write(BuildNAKResponse()); err != nil {
				log.Printf("Failed to send NAK: %v", err)
			}
		}
	}
}

func BuildHeaderResponse(s *Server, req *Request) []byte {
	header := NewHeader()

	// Copy relevant fields from request
	copy(header.UserCode[:], req.UserCode[:])
	copy(header.PlantCode[:], req.PlantCode[:])

	// Set current date and time
	now := time.Now().In(s.Location)
	copy(header.Day[:], intToBCD(now.Day()))
	copy(header.Month[:], intToBCD(int(now.Month())))
	copy(header.Year[:], intToBCD(now.Year()%100))
	copy(header.Hour[:], intToBCD(now.Hour()))
	copy(header.Minute[:], intToBCD(now.Minute()))

	// Set default values
	header.RAM = 0x01
	copy(header.SWVersion[:], []byte{0x01, 0x02, 0x03, 0x04})

	// Calculate checksum
	header.CalculateHeaderChecksum()

	// Convert to bytes with correct ordering
	b := make([]byte, 35)
	b[0] = header.STX
	copy(b[1:3], header.Computer[:])
	copy(b[3:5], header.IntestationBlock[:])
	copy(b[5:7], header.Model[:])
	copy(b[7:11], header.UserCode[:])
	copy(b[11:15], header.PlantCode[:])
	copy(b[15:17], header.Day[:])
	copy(b[17:19], header.Month[:])
	copy(b[19:23], header.Year[:])
	copy(b[23:25], header.Hour[:])
	copy(b[25:27], header.Minute[:])
	b[27] = header.RAM
	copy(b[28:32], header.SWVersion[:])
	binary.BigEndian.PutUint16(b[32:34], binary.BigEndian.Uint16(header.Checksum[:]))
	b[34] = header.ETB

	// Verification (for debugging)
	var sentSum uint16
	for _, bt := range b[1:32] { // Changed from 33 to 32 - checksum covers bytes 1-31
		sentSum += uint16(bt)
	}
	sentChecksum := binary.BigEndian.Uint16(b[32:34])
	log.Printf("Server checksum verification: calculated=%d, sent=%d", sentSum, sentChecksum)

	return b
}

// intToBCD converts an integer to BCD format (2 digits per byte)
func intToBCD(n int) []byte {
	// For numbers 0-99
	return []byte{byte((n/10)<<4 | (n % 10))}
}

func (s *Server) Stop() {
	close(s.stopChan)
	s.mu.Lock()
	defer s.mu.Unlock()
	for conn := range s.Connections {
		conn.Close()
	}
}
