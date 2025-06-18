package client

import (
	"fmt"
	"log"
	"net"
)

type Client struct {
	ServerAddr string
	conn       net.Conn
}

func New(serverAddr string) *Client {
	return &Client{
		ServerAddr: serverAddr,
	}
}

func (c *Client) Connect() error {
	conn, err := net.Dial("tcp", c.ServerAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %v", err)
	}
	c.conn = conn

	log.Printf("Connected to server at %s", c.ServerAddr)

	// Read the initial ACK from server
	ack := make([]byte, 2048)
	n, err := c.conn.Read(ack)
	if err != nil {
		c.conn.Close()
		return fmt.Errorf("failed to read ACK from server: %v", err)
	}

	log.Printf("Received ACK from server: %q", ack[:n])
	return nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
