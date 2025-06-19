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
	ETB      = 0x17
	Protocol = "C0"
)

// 22 bytes
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

// 35 bytes
type Header struct {
	STX              byte    // [0] Start of transmission (0x02)
	Computer         [2]byte // [1-2] always "PC"
	IntestationBlock [2]byte // [3-4] always "D0"
	Model            [2]byte // [5-6] always "L0"
	UserCode         [4]byte // [7-10] User code
	PlantCode        [4]byte // [11-14] Plant code
	Day              [2]byte // [15-16]
	Month            [2]byte // [17-18]
	Year             [4]byte // [19-22]
	Hour             [2]byte // [23-24]
	Minute           [2]byte // [25-26]
	RAM              byte    // [27]
	SWVersion        [4]byte // [28-31] SW Version
	Checksum         [2]byte // [32-33] Checksum
	ETB              byte    // [34] End of transmission (0x03)
}

func NewIntestation() Header {
	return Header{
		STX:              STX,                         // [0] Start of transmission (0x02)
		Computer:         [2]byte{'P', 'C'},           // [1-2] always "PC"
		IntestationBlock: [2]byte{'D', '0'},           // [3-4] always "D0"
		Model:            [2]byte{'L', '0'},           // [5-6] always "L0"
		UserCode:         [4]byte{'0', '0', '0', '0'}, // [7-10] User code
		PlantCode:        [4]byte{'0', '0', '0', '0'}, // [11-14] Plant code
		ETB:              ETB,                         // [21] End of transmission (0x03)
	}
}

func (header *Header) CalculateHeaderChecksum() {
	// Create a temporary buffer containing all fields from Computer to SWVersion
	buf := make([]byte, 31) // Changed from 32 to 31
	pos := 0

	copy(buf[pos:pos+2], header.Computer[:])
	pos += 2
	copy(buf[pos:pos+2], header.IntestationBlock[:])
	pos += 2
	copy(buf[pos:pos+2], header.Model[:])
	pos += 2
	copy(buf[pos:pos+4], header.UserCode[:])
	pos += 4
	copy(buf[pos:pos+4], header.PlantCode[:])
	pos += 4
	copy(buf[pos:pos+2], header.Day[:])
	pos += 2
	copy(buf[pos:pos+2], header.Month[:])
	pos += 2
	copy(buf[pos:pos+4], header.Year[:])
	pos += 4
	copy(buf[pos:pos+2], header.Hour[:])
	pos += 2
	copy(buf[pos:pos+2], header.Minute[:])
	pos += 2
	buf[pos] = header.RAM
	pos += 1
	copy(buf[pos:pos+4], header.SWVersion[:])

	// Calculate checksum over all bytes in buffer
	var sum uint16
	for _, b := range buf {
		sum += uint16(b)
	}

	// Store the checksum
	binary.BigEndian.PutUint16(header.Checksum[:], sum)
}

func ParseHeader(data []byte) (*Header, error) {
	if len(data) != 35 {
		return nil, fmt.Errorf("header block must be exactly 35 bytes")
	}

	if data[0] != STX || data[34] != ETB {
		return nil, fmt.Errorf("invalid frame markers")
	}

	// Verify checksum - sum bytes 1-31 (Computer to SWVersion)
	var sum uint16
	for _, b := range data[1:32] { // Changed from 33 to 32
		sum += uint16(b)
	}

	receivedChecksum := binary.BigEndian.Uint16(data[32:34])
	if sum != receivedChecksum {
		return nil, fmt.Errorf("invalid checksum (calculated: %d, received: %d)", sum, receivedChecksum)
	}

	// Parse the header
	header := &Header{
		STX: data[0],
		ETB: data[34],
	}

	copy(header.Computer[:], data[1:3])
	copy(header.IntestationBlock[:], data[3:5])
	copy(header.Model[:], data[5:7])
	copy(header.UserCode[:], data[7:11])
	copy(header.PlantCode[:], data[11:15])
	copy(header.Day[:], data[15:17])
	copy(header.Month[:], data[17:19])
	copy(header.Year[:], data[19:23])
	copy(header.Hour[:], data[23:25])
	copy(header.Minute[:], data[25:27])
	header.RAM = data[27]
	copy(header.SWVersion[:], data[28:32])
	copy(header.Checksum[:], data[32:34])

	return header, nil
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
	// find STX pos
	stxPos := bytes.IndexByte(data, STX)
	if stxPos == -1 {
		return nil, fmt.Errorf("STX not found")
	}

	// find ETX pos
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
