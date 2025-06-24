package client

import (
	"FakeLPM/fakelpm"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

type Client struct {
	ServerAddr string
	conn       net.Conn
	timeout    time.Duration
}

func (c *Client) SendDownloadRequest(isTotal bool) (*fakelpm.Header, []*fakelpm.Measurement, error) {
	if c.conn == nil {
		return nil, nil, fmt.Errorf("not connected to server")
	}

	request := fakelpm.NewRequest()
	if isTotal {
		request.Command[1] = 0x54 // 'T'
	} else {
		request.Command[1] = 0x50 // 'P'
	}
	request.CalculateRequestChecksum()

	// Send request
	c.conn.SetWriteDeadline(time.Now().Add(c.timeout))
	_, err := c.conn.Write(request.Bytes())
	c.conn.SetWriteDeadline(time.Time{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to send request: %v", err)
	}
	log.Printf("Sent %s request", string(request.Command[:]))

	// Read ACK response
	c.conn.SetReadDeadline(time.Now().Add(c.timeout))
	ackBuf := make([]byte, 11)
	_, err = c.conn.Read(ackBuf)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read ACK: %v", err)
	}
	log.Printf("Received ACK response")

	// Read header block
	headerBuf := make([]byte, 35)
	_, err = c.conn.Read(headerBuf)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read header block: %v", err)
	}

	// Verify header checksum
	var calculatedSum uint16
	for _, b := range headerBuf[1:32] {
		calculatedSum += uint16(b)
	}
	receivedChecksum := binary.BigEndian.Uint16(headerBuf[32:34])
	log.Printf("Header checksum: calculated=%d, received=%d", calculatedSum, receivedChecksum)
	header, err := fakelpm.ParseHeader(headerBuf)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse header block: %v", err)
	}

	// Send ACK for header
	if _, err := c.conn.Write(fakelpm.BuildACKResponse()); err != nil {
		return header, nil, fmt.Errorf("failed to send header ACK: %v", err)
	}

	var measurements []*fakelpm.Measurement
    buf := make([]byte, 1) // For reading first byte

    for {
        // Read first byte to identify message type
        _, err := io.ReadFull(c.conn, buf)
        if err != nil {
            return header, measurements, fmt.Errorf("failed to read message start: %v", err)
        }

        // Handle STX
        if buf[0] != fakelpm.STX {
            return header, measurements, fmt.Errorf("expected STX, got %x", buf[0])
        }

        // Read next 4 bytes to identify message type
        typeBuf := make([]byte, 4)
        _, err = io.ReadFull(c.conn, typeBuf)
        if err != nil {
            return header, measurements, fmt.Errorf("failed to read message type: %v", err)
        }

        // Determine message type and length
        var (
            messageType string
            totalLength int
        )

        // Check for measurement (STX + 'PC' + 'D4')
        if bytes.Equal(typeBuf[:3], []byte{'P', 'C', 'D'}) && typeBuf[3] == '4' {
            messageType = "measurement"
            totalLength = 56
        } else if bytes.Equal(typeBuf, []byte{'P', 'C', 'D', '4'}) { // Check for final (STX + 'PC' + 'D4' + 'E')
            // Need to check next 3 bytes for 'EOD'
            eodBuf := make([]byte, 3)
            _, err = io.ReadFull(c.conn, eodBuf)
            if err != nil {
                return header, measurements, fmt.Errorf("failed to read EOD marker: %v", err)
            }
            
            if bytes.Equal(eodBuf, []byte{'E', 'O', 'D'}) {
                messageType = "final"
                totalLength = 10 // STX + PC + D4 + EOD + checksum + ETX
                typeBuf = append(typeBuf, eodBuf...)
            } else {
                return header, measurements, fmt.Errorf("unexpected message format")
            }
        } else {
            return header, measurements, fmt.Errorf("unknown message type: %x", typeBuf)
        }

        // Read remaining bytes
        remaining := totalLength - 1 - len(typeBuf) // Already read STX and typeBuf
        remainingBuf := make([]byte, remaining)
        _, err = io.ReadFull(c.conn, remainingBuf)
        if err != nil {
            return header, measurements, fmt.Errorf("failed to read message body: %v", err)
        }

        // Combine all parts
        fullMessage := append([]byte{fakelpm.STX}, typeBuf...)
        fullMessage = append(fullMessage, remainingBuf...)

        // Parse based on type
        switch messageType {
        case "measurement":
            measurement, err := fakelpm.ParseMeasurement(fullMessage)
            if err != nil {
                return header, measurements, fmt.Errorf("failed to parse measurement: %v", err)
            }
            measurements = append(measurements, measurement)
            log.Printf("Received measurement %d: %+v", len(measurements), measurement)

        case "final":
            final, err := fakelpm.ParseFinal(fullMessage)
            if err != nil {
                return header, measurements, fmt.Errorf("failed to parse final package: %v", err)
            }
            log.Printf("Received final package: %s", string(final.EndDownload[:]))
            return header, measurements, nil
        }

        // Send ACK
        if _, err := c.conn.Write(fakelpm.BuildACKResponse()); err != nil {
            return header, measurements, fmt.Errorf("failed to send ACK: %v", err)
        }
    }
}

// Helper function to parse measurement from byte slice
func parseMeasurement(data []byte) (*fakelpm.Measurement, error) {
	// find STX pos
	stxPos := bytes.IndexByte(data, fakelpm.STX)
	if stxPos == -1 {
		return nil, fmt.Errorf("STX not found")
	}

	// find ETB pos
	etbPos := bytes.IndexByte(data, fakelpm.ETB)
	if etbPos == -1 {
		return nil, fmt.Errorf("ETB not found")
	}

	// Extract the framed message
	framedData := data[stxPos : etbPos+1]

	if len(framedData) != 55 {
		return nil, fmt.Errorf("invalid measurement length (%d bytes)", len(framedData))
	}

	m := &fakelpm.Measurement{
		STX: framedData[0],
		ETB: framedData[54],
	}

	copy(m.Computer[:], framedData[1:3])
	copy(m.BlockType[:], framedData[3:5])
	copy(m.Data[:], framedData[5:53])
	copy(m.Checksum[:], framedData[53:55])

	// Verify checksum
	var sum uint16
	for _, b := range framedData[1:53] { // Sum bytes from Computer to end of Data
		sum += uint16(b)
	}
	if binary.BigEndian.Uint16(m.Checksum[:]) != sum {
		return nil, errors.New("invalid checksum")
	}

	return m, nil
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
