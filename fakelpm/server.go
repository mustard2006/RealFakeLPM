package fakelpm

import (
	"bytes"
	"encoding/binary"
	"log"
	"net"
	"sync"
	"time"
)

type Server struct {
	Addr        string
	Connections map[net.Conn]bool
	mu          sync.Mutex
	stopChan    chan struct{}
}

func New(addr string) *Server {
	return &Server{
		Addr:        addr,
		Connections: make(map[net.Conn]bool),
		stopChan:    make(chan struct{}),
	}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	defer ln.Close()

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

func (s *Server) Stop() {
	close(s.stopChan)
	s.mu.Lock()
	defer s.mu.Unlock()
	for conn := range s.Connections {
		conn.Close()
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer func() {
		s.mu.Lock()
		delete(s.Connections, conn)
		s.mu.Unlock()
		conn.Close()
	}()

	log.Printf("New connection from %s", conn.RemoteAddr())

	buf := make([]byte, 2048)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			log.Printf("Read error: %v", err)
			return
		}

		// Validate and parse the request
		req, err := ParseRequest(buf[:n])
		if err != nil {
			log.Printf("Invalid request: %v", err)

			// Check if basic framing exists
			if bytes.Contains(buf[:n], []byte{STX}) && bytes.Contains(buf[:n], []byte{ETX}) {
				if _, err := conn.Write(BuildNAKResponse()); err != nil {
					log.Printf("Failed to send NAK: %v", err)
				}
			}
			continue
		}

		// Process valid request
		switch string(req.Command[:]) {
		case "DT":
			log.Printf("Received DT request - Measures download")
			// Send ACK first
			if _, err := conn.Write(BuildACKResponse()); err != nil {
				log.Printf("Failed to send ACK: %v", err)
				return
			}
			// Then send intestation block
			intestation := BuildIntestationResponse(req)
			if _, err := conn.Write(intestation); err != nil {
				log.Printf("Failed to send intestation block: %v", err)
				return
			}
			log.Printf("Sent Header block for DT request")

		case "DP":
			log.Printf("Received DP request - Partial measures download")
			// Send ACK first
			if _, err := conn.Write(BuildACKResponse()); err != nil {
				log.Printf("Failed to send ACK: %v", err)
				return
			}
			// Then send intestation block
			intestation := BuildIntestationResponse(req)
			if _, err := conn.Write(intestation); err != nil {
				log.Printf("Failed to send intestation block: %v", err)
				return
			}
			log.Printf("Sent Header block for DP request")

		default:
			log.Printf("Unknown command: %s", req.Command[:])
			if _, err := conn.Write(BuildNAKResponse()); err != nil {
				log.Printf("Failed to send NAK: %v", err)
			}
		}
	}
}

func BuildIntestationResponse(req *Request) []byte {
	header := NewIntestation()

	// Copy relevant fields from request
	copy(header.UserCode[:], req.UserCode[:])
	copy(header.PlantCode[:], req.PlantCode[:])

	// Set current date and time
	now := time.Now()
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
