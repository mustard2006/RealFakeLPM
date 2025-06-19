package client

import (
	"FakeLPM/fakelpm"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"
)

type Client struct {
	ServerAddr string
	conn       net.Conn
	timeout    time.Duration
}

func (c *Client) SendDownloadRequest(isTotal bool) (*fakelpm.Header, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("not connected to server")
	}

	request := fakelpm.NewRequest()
	if isTotal {
		request.Command[1] = 0x54 // 'T'
	} else {
		request.Command[1] = 0x50 // 'P'
	}
	request.CalculateChecksum()

	// Send request
	c.conn.SetWriteDeadline(time.Now().Add(c.timeout))
	_, err := c.conn.Write(request.Bytes())
	c.conn.SetWriteDeadline(time.Time{})
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	log.Printf("Sent %s request", string(request.Command[:]))

	// Read ACK response (fixed 11 bytes)
	c.conn.SetReadDeadline(time.Now().Add(c.timeout))
	ackBuf := make([]byte, 11)
	_, err = c.conn.Read(ackBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to read ACK: %v", err)
	}
	log.Printf("Received ACK response")

	// Read header block (fixed 35 bytes)
	headerBuf := make([]byte, 35)
	_, err = c.conn.Read(headerBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to read header block: %v", err)
	}
	c.conn.SetReadDeadline(time.Time{})

	var calculatedSum uint16
	for _, b := range headerBuf[1:32] { // Changed from 33 to 32 - checksum covers bytes 1-31
		calculatedSum += uint16(b)
		// log.Printf("Byte %d: 0x%02x (%d) - running sum: %d", i+1, b, b, calculatedSum) // for debugging arrived header bytes
	}
	receivedChecksum := binary.BigEndian.Uint16(headerBuf[32:34])
	log.Printf("Final checksum: calculated=%d, received=%d", calculatedSum, receivedChecksum)
	header, err := fakelpm.ParseHeader(headerBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to parse header block: %v", err)
	}

	return header, nil
}

func New(serverAddr string) *Client {
	return &Client{ServerAddr: serverAddr}
}

func (c *Client) Connect() error {
	conn, err := net.Dial("tcp", c.ServerAddr)
	if err != nil {
		return fmt.Errorf("connection failed: %v", err)
	}
	c.conn = conn

	// Read ACK
	ack := make([]byte, 2048)
	n, err := conn.Read(ack)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to read ACK: %v", err)
	}

	// Reset timeout
	conn.SetReadDeadline(time.Time{})

	log.Printf("Connected to %s, received ACK: %q", c.ServerAddr, ack[:n])
	return nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}
