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
	STX       byte    // [0] Start of transmission (0x02)
	Protocol  [2]byte // [1-2] Protocol "C0"
	UserCode  [4]byte // [3-6] User code
	PlantCode [4]byte // [7-10] Plant code
	Command   [2]byte // [11-12] Command (D + T/P)
	BlockSel  byte    // [13] Block select
	Reserved  byte    // [14] Reserved
	MaxRec    [4]byte // [15-18] Max records
	Checksum  [2]byte // [19-20] Checksum
	ETX       byte    // [21] End of transmission (0x03)
}

func NewRequest() Request {
	return Request{
		STX:       STX,
		Protocol:  [2]byte{'C', '0'},
		UserCode:  [4]byte{'0', '0', '0', '0'},
		PlantCode: [4]byte{'0', '0', '0', '0'},
		Command:   [2]byte{'D', 'T'}, // Default to DT
		BlockSel:  0x08,              // Measures only (00001000)
		Reserved:  0x00,
		MaxRec:    [4]byte{0x00, 0x00, 0x00, 0x00}, // All records
		ETX:       ETX,
	}
}

func (r *Request) Bytes() []byte {
	b := make([]byte, 22)
	b[0] = r.STX
	copy(b[1:3], r.Protocol[:])
	copy(b[3:7], r.UserCode[:])
	copy(b[7:11], r.PlantCode[:])
	copy(b[11:13], r.Command[:])
	b[13] = r.BlockSel
	b[14] = r.Reserved
	copy(b[15:19], r.MaxRec[:])
	copy(b[19:21], r.Checksum[:])
	b[21] = r.ETX
	return b
}

func (r *Request) CalculateChecksum() {
	var sum uint16
	data := r.Bytes()
	for _, b := range data[1:19] { // Sum bytes 1-18
		sum += uint16(b)
	}
	binary.BigEndian.PutUint16(r.Checksum[:], sum)
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

	if len(framedData) != 22 {
		return nil, fmt.Errorf("invalid message length (%d bytes)", len(framedData))
	}

	req := &Request{
		STX: framedData[0],
		ETX: framedData[21],
	}

	copy(req.Protocol[:], framedData[1:3])
	copy(req.UserCode[:], framedData[3:7])
	copy(req.PlantCode[:], framedData[7:11])
	copy(req.Command[:], framedData[11:13])
	req.BlockSel = framedData[13]
	req.Reserved = framedData[14]
	copy(req.MaxRec[:], framedData[15:19])
	copy(req.Checksum[:], framedData[19:21])

	// Verify checksum
	var sum uint16
	for _, b := range framedData[1:19] {
		sum += uint16(b)
	}
	if binary.BigEndian.Uint16(req.Checksum[:]) != sum {
		return nil, errors.New("invalid checksum")
	}

	return req, nil
}
