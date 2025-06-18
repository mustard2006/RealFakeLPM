package fakelpm

import (
	"encoding/base64"
	"encoding/hex"
)

var (
	SampleMeasurements = []string{
		"RDQ4N0U4MTExODI1MDYwNzAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwODlGMjAwMTQwMDk0NkU2MTZENjEwMUZFRkZGRkZGMTgwMzAxMDA4N0U4MTExODI2MDYwNzAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwODlGMzAwMTUwMDI3NDYyNzQ2NTkwMUZFRkZGRkZGMTgwMzAxMDA=",
		// ... other samples ...
	}
	currentSampleIndex = 0
)

func GetNextMeasurement() ([]byte, error) {
	if len(SampleMeasurements) == 0 {
		return nil, nil
	}

	sample := SampleMeasurements[currentSampleIndex]
	currentSampleIndex = (currentSampleIndex + 1) % len(SampleMeasurements)
	return base64.StdEncoding.DecodeString(sample)
}

func BuildACKResponse() []byte {
	return []byte{STX, 'P', 'C', 'R', '0', 'A', 'C', 'K', '1', 'A', ETX}
}

func BuildNAKResponse() []byte {
	return []byte{STX, 'P', 'C', 'R', '0', 'N', 'A', 'K', '0', 'F', ETX}
}

func BuildEndFrameResponse() []byte {
	// Example end frame from documentation
	endFrame := "02 50 43 52 31 45 33 38 37 31 30 30 37 32 33 33 31 37 31 03"
	data, _ := hex.DecodeString(endFrame)
	return data
}
