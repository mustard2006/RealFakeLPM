package fakelpm

import (
	"bytes"
	"log"
	"net"
	"sync"
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
			if _, err := conn.Write(BuildACKResponse()); err != nil {
				log.Printf("Failed to send ACK: %v", err)
				return
			}
			// Add measure download logic here

		case "DP":
			log.Printf("Received DP request - Partial measures download")
			if _, err := conn.Write(BuildACKResponse()); err != nil {
				log.Printf("Failed to send ACK: %v", err)
				return
			}
			// Add partial download logic here

		default:
			log.Printf("Unknown command: %s", req.Command[:])
			if _, err := conn.Write(BuildNAKResponse()); err != nil {
				log.Printf("Failed to send NAK: %v", err)
			}
		}
	}
}
