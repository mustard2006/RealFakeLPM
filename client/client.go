package client

import (
	"FakeLPM/fakelpm"
	"fmt"
	"log"
	"net"
)

type Client struct {
	ServerAddr string
	conn       net.Conn
}

func (c *Client) SendDownloadRequest(isTotal bool) error {
	if c.conn == nil {
		return fmt.Errorf("not connected to server")
	}

	request := fakelpm.NewRequest()

	// 13-th byte to change for T 0x54 or P 0x50
	if isTotal {
		request.Command[1] = 0x54 // T
	} else {
		request.Command[1] = 0x50 // P
	}

	// Calculate checksum
	// Calculate checksum
	request.CalculateChecksum()

	// Convert to bytes and send
	if _, err := c.conn.Write(request.Bytes()); err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}

	log.Printf("Sent %s request (measures only)", string(request.Command[:]))
	return nil
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

	log.Printf("Connected to %s, received ACK: %q", c.ServerAddr, ack[:n])
	return nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
