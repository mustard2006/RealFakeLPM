package fakelpm

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	STX      = 0x02
	ETX      = 0x03
	Protocol = "C0"
)

type Request struct {
	STX         byte
	Protocol    [2]byte
	UserCode    [4]byte
	PlantCode   [4]byte
	Command     [2]byte
	BlockSelect byte
	MaxRecords  uint16
	Checksum    byte
	ETX         byte
}

func ParseRequest(data []byte) (*Request, error) {
	// Find STX and ETX positions
	stxPos := bytes.IndexByte(data, STX)
	if stxPos == -1 {
		return nil, fmt.Errorf("STX not found")
	}

	etxPos := bytes.IndexByte(data, ETX)
	if etxPos == -1 {
		return nil, fmt.Errorf("ETX not found")
	}

	// Extract the framed message
	framedData := data[stxPos : etxPos+1]

	// Minimum message length check (17 bytes for basic message)
	if len(framedData) < 17 {
		return nil, fmt.Errorf("message too short (%d bytes)", len(framedData))
	}

	req := &Request{
		STX:      framedData[0],
		ETX:      framedData[len(framedData)-1],
		Checksum: framedData[len(framedData)-2],
	}

	// Verify protocol
	copy(req.Protocol[:], framedData[1:3])
	if string(req.Protocol[:]) != Protocol {
		return nil, fmt.Errorf("invalid protocol: %s", req.Protocol)
	}

	// Copy other fields
	copy(req.UserCode[:], framedData[3:7])
	copy(req.PlantCode[:], framedData[7:11])
	copy(req.Command[:], framedData[11:13])
	req.BlockSelect = framedData[13]
	req.MaxRecords = binary.BigEndian.Uint16(framedData[14:16])

	// Verify checksum (sum of all bytes between STX and Checksum)
	var sum byte
	for _, b := range framedData[1 : len(framedData)-2] {
		sum += b
	}
	if sum != req.Checksum {
		return nil, errors.New("invalid checksum")
	}

	return req, nil
}
