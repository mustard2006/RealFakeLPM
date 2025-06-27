package fakelpm

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"time"
)

const (
	STX      = 0x02
	ETX      = 0x03
	ETB      = 0x17
	Protocol = "C0"
)

const (
	HeaderMsgType      = "D0"
	MeasurementMsgType = "D4"
	FinalMsgType       = "D4EOD"
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

// 48 byte
type Data struct {
	Status      byte // [0] - Contains year high bits (5 bits) and status flags
	YearLow     byte // [1] - Year low bits
	Month       byte // [2] - Month in BCD format
	Day         byte // [3] - Day in BCD format
	PoleLow     byte // [4] - Pole address low byte
	PoleHigh    byte // [5] - Pole address high byte
	MeasureType byte // [6] - 0x00 or 0x07
	// Measurement 1 (AE - first measurement)
	AELampStatus  byte // [7]
	AETensionLow  byte // [8] - Voltage low byte
	AETensionHigh byte // [9] - Voltage high byte
	AECurrentLow  byte // [10] - Current low byte
	AECurrentHigh byte // [11] - Current high byte
	AEPoweredLow  byte // [12] - Powered duration low byte
	AEPoweredHigh byte // [13] - Powered duration high byte
	AELitLow      byte // [14] - Lit duration low byte
	AELitHigh     byte // [15] - Lit duration high byte
	AECosfiValue  byte // [16] - Power factor value
	AECosfiSign   byte // [17] - Power factor sign
	// Measurement 2 (M1 - second measurement)
	M1LampStatus  byte // [18]
	M1TensionLow  byte // [19]
	M1TensionHigh byte // [20]
	M1CurrentLow  byte // [21]
	M1CurrentHigh byte // [22]
	M1PoweredLow  byte // [23]
	M1PoweredHigh byte // [24]
	M1LitLow      byte // [25]
	M1LitHigh     byte // [26]
	M1CosfiValue  byte // [27]
	M1CosfiSign   byte // [28]
	// Measurement 3 (M2 - third measurement)
	M2LampStatus  byte // [29]
	M2TensionLow  byte // [30]
	M2TensionHigh byte // [31]
	M2CurrentLow  byte // [32]
	M2CurrentHigh byte // [33]
	M2PoweredLow  byte // [34]
	M2PoweredHigh byte // [35]
	M2LitLow      byte // [36]
	M2LitHigh     byte // [37]
	M2CosfiValue  byte // [38]
	M2CosfiSign   byte // [39]
	// Harvest times (for each measurement)
	AEHarvestTimeLow  byte // [40]
	AEHarvestTimeHigh byte // [41]
	M1HarvestTimeLow  byte // [42]
	M1HarvestTimeHigh byte // [43]
	M2HarvestTimeLow  byte // [44]
	M2HarvestTimeHigh byte // [45]
	ConversionType    byte // [46] - Time scale factor (0-3)
	Reserved          byte // [47]
}

// Measurement represents a single measurement record (D4 type)
// 1 + 2 + 2 + 48 + 2 + 1 = 56 byte
type Measurement struct {
	STX       byte     // [0] Start of transmission (always STX - 0x02)
	Computer  [2]byte  // [1-2] always 'P' 'C'
	BlockType [2]byte  // [3-4] 'D' '4' for measurements
	Data      [48]byte // [5-52] Measurement data
	Checksum  [2]byte  // [53-54] Checksum
	ETB       byte     // [55] End of transmission block (0x17)
}

// 1 + 2 + 2 + 3 + 2 + 1 = 11 byte
type Final struct {
	STX         byte    // [0]
	Computer    [2]byte // [1-2] always 'P' 'C'
	EndBlock    [2]byte // [3-4] Always 'D' '4'
	EndDownload [3]byte // [5-7] always 'E' 'O' 'D'
	Checksum    [2]byte // [8-9]
	ETX         byte    // [11]
}

func detectTimezone() (*time.Location, error) {
	// First try the local timezone
	if loc, err := time.LoadLocation("Local"); err == nil {
		return loc, nil
	}

	// Fallback to reading from /etc/localtime (Unix systems)
	if loc, err := time.LoadLocation(""); err == nil {
		return loc, nil
	}

	// Final fallback to UTC
	return time.UTC, nil
}

// Add these constants to your protocol.go file
const (
	LPM_lamp_measure_lamp_power_on                = "lamp_power_on"
	LPM_lamp_measure_power_supply_undervoltage    = "power_supply_undervoltage"
	LPM_lamp_measure_power_supply_overvoltage     = "power_supply_overvoltage"
	LPM_lamp_measure_power_supply_output_limiter  = "power_supply_output_limiter"
	LPM_lamp_measure_power_supply_termal_derating = "power_supply_termal_derating"
	LPM_lamp_measure_led_plate_open_circuit       = "led_plate_open_circuit"
	LPM_lamp_measure_led_plate_thermal_derating   = "led_plate_thermal_derating"
	LPM_lamp_measure_led_plate_thermal_shutdown   = "led_plate_thermal_shutdown"
	LPM_lamp_measure_voltage                      = "voltage"
	LPM_lamp_measure_current                      = "current"
	LPM_lamp_measure_cosfi                        = "cosfi"
	LPM_lamp_measure_active_power                 = "active_power"
	LPM_lamp_measure_energy                       = "energy"
	LPM_lamp_measure_time_lamp_powered            = "time_lamp_powered"
	LPM_lamp_measure_time_lamp_poweron            = "time_lamp_poweron"
	LPM_lamp_measure_state_not_responding         = "state_not_responding"
	LPM_lamp_address_tag                          = "lamp_address"
)

// DecodeHistoricalMeasures decodes the base64 encoded historical measures
// <---DECODE BASE64--->
func (s *Server) DecodeMeasures() ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	for _, sample := range SampleMeasurements {
		// Decode from base64
		data, err := base64.StdEncoding.DecodeString(sample)
		if err != nil {
			return nil, fmt.Errorf("base64 decode failed: %v", err)
		}

		// Verify and remove D4 header
		if len(data) < 2 || string(data[:2]) != "D4" {
			continue
		}
		measurementData := data[2:] // Remove D4 header

		// Verify length is multiple of 48
		if len(measurementData)%48 != 0 {
			return nil, fmt.Errorf("invalid data length: %d bytes (not divisible by 48)", len(measurementData))
		}

		// Process each 48-byte block
		for i := 0; i < len(measurementData)/48; i++ {
			block := measurementData[i*48 : (i*48)+48]

			// Parse the 48-byte block into measurements
			blockResults, err := parseMeasurementBlock(block, s.Location)
			if err != nil {
				return nil, err
			}
			results = append(results, blockResults...)
		}
	}

	return results, nil
}

func parseMeasurementBlock(block []byte, loc *time.Location) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	// Extract basic information
	status := block[0]
	year := int(block[1])
	month := int(block[2])
	day := int(block[3])
	pole := binary.LittleEndian.Uint16(block[4:6])
	measureType := block[6]

	// Process the 3 measurements in each block
	for idx := 0; idx < 3; idx++ {
		result := make(map[string]interface{})
		offset := idx * 11
		timeOffset := idx * 2

		// Create timestamp (using server's location)
		measureTime := time.Date(
			2000+year,
			time.Month(month),
			day,
			12, 0, 0, 0, // Noon as base time
			loc,
		)

		if hrs := int(binary.LittleEndian.Uint16(block[40+timeOffset : 42+timeOffset])); hrs != 0xFFFF {
			measureTime = measureTime.Add(time.Minute * time.Duration(hrs))
		}

		// Electrical measurements
		voltage := binary.LittleEndian.Uint16(block[8+offset : 10+offset])
		current := float64(binary.LittleEndian.Uint16(block[10+offset:12+offset])) * 3.57 / 1000
		cosfi := float64(block[16+offset]) / 100
		if block[17+offset]&1 == 1 {
			cosfi *= -1
		}
		power := float64(voltage) * current * cosfi

		// Lamp state
		state := block[7+offset]
		lampOn := state&1 == 1

		// Build result map
		result["timestamp"] = measureTime
		result["pole"] = pole
		result["voltage"] = float64(voltage)
		result["current"] = current
		result["power"] = power
		result["cosfi"] = cosfi
		result["lamp_on"] = lampOn
		result["measure_type"] = measureType
		result["status"] = status

		results = append(results, result)
	}

	return results, nil
}

// <---DECODE BASE64--->

// <---REQUEST PACKAGE--->

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

func (r *Request) CalculateRequestChecksum() {
	var sum uint16
	data := r.Bytes()
	for _, b := range data[1:19] { // Sum bytes 1-18
		sum += uint16(b)
	}
	binary.BigEndian.PutUint16(r.Checksum[:], sum)
}

// <---REQUEST PACKAGE--->

// <---HEADER PACKAGE--->

func NewHeader() *Header {
	return &Header{
		STX:              STX,                         // [0] Start of transmission (0x02)
		Computer:         [2]byte{'P', 'C'},           // [1-2] always "PC"
		IntestationBlock: [2]byte{'D', '0'},           // [3-4] always "D0"
		Model:            [2]byte{'L', '0'},           // [5-6] always "L0"
		UserCode:         [4]byte{'0', '0', '0', '0'}, // [7-10] User code
		PlantCode:        [4]byte{'0', '0', '0', '0'}, // [11-14] Plant code
		ETB:              ETB,                         // [21] End of transmission (0x03)
	}
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

// <---HEADER PACKAGE--->

// <---MEASUREMENT PACKAGE--->

func NewMeasurement() *Measurement {
	return &Measurement{
		STX:       STX,
		Computer:  [2]byte{'P', 'C'},
		BlockType: [2]byte{'D', '4'},
		ETB:       ETB,
	}
}

func NewRandomMeasurement() *Measurement {
	m := NewMeasurement()
	now := time.Now()
	randData := generateRandomData(now)
	copy(m.Data[:], randData[:])
	m.CalculateMeasurementChecksum()
	return m
}

func (m *Measurement) Bytes() []byte {
	b := make([]byte, 56)
	b[0] = m.STX
	copy(b[1:3], m.Computer[:])
	copy(b[3:5], m.BlockType[:])
	copy(b[5:53], m.Data[:])
	copy(b[53:55], m.Checksum[:])
	b[55] = m.ETB
	return b
}

// Add this function to protocol.go, with the other parsing functions
func ParseMeasurement(data []byte) (*Measurement, error) {
	if len(data) != 56 {
		return nil, fmt.Errorf("invalid measurement length (%d bytes), expected 56", len(data))
	}

	if data[0] != STX || data[55] != ETB {
		return nil, fmt.Errorf("invalid frame markers")
	}

	m := &Measurement{
		STX: data[0],
		ETB: data[55],
	}

	copy(m.Computer[:], data[1:3])
	copy(m.BlockType[:], data[3:5])
	copy(m.Data[:], data[5:53])
	copy(m.Checksum[:], data[53:55])

	// Verify checksum
	var sum uint16
	for _, b := range data[1:53] { // Sum from Computer to end of Data
		sum += uint16(b)
	}
	if binary.BigEndian.Uint16(m.Checksum[:]) != sum {
		return nil, errors.New("invalid checksum")
	}

	return m, nil
}

func generateRandomData(t time.Time) [48]byte {
	var d [48]byte

	// Status and date
	d[0] = generateStatusByte()
	d[1] = byte(t.Year() % 100)
	d[2] = byteToBCD(byte(t.Month()))
	d[3] = byteToBCD(byte(t.Day()))

	// Pole number
	pole := uint16(rand.Intn(10000))
	binary.LittleEndian.PutUint16(d[4:6], pole)

	// Measurement type
	d[6] = byte(rand.Intn(2)) * 7 // 0 or 7

	// Generate 3 measurements
	for i := 0; i < 3; i++ {
		offset := 7 + i*11

		// Lamp state
		d[offset] = generateLampStatus()

		// Voltage (180-250V)
		voltage := 18000 + rand.Intn(7000)
		binary.LittleEndian.PutUint16(d[offset+1:offset+3], uint16(voltage))

		// Current (0-5000 units of 3.47mA)
		current := rand.Intn(5000)
		binary.LittleEndian.PutUint16(d[offset+3:offset+5], uint16(current))

		// Durations
		binary.LittleEndian.PutUint16(d[offset+5:offset+7], uint16(rand.Intn(65536)))
		binary.LittleEndian.PutUint16(d[offset+7:offset+9], uint16(rand.Intn(65536)))

		// Power factor
		pf := byte(rand.Intn(101))
		sign := byte(0)
		if rand.Float32() < 0.1 {
			sign = 1
		}
		d[offset+9] = pf
		d[offset+10] = sign

		// Harvest time
		if i < 3 {
			harvestOffset := 40 + i*2
			minutes := rand.Intn(1440)
			binary.LittleEndian.PutUint16(d[harvestOffset:harvestOffset+2], uint16(minutes))
		}
	}

	// Conversion type
	d[46] = byte(rand.Intn(4))
	d[47] = 0

	return d
}

// generateStatusByte creates random status byte
func generateStatusByte() byte {
	var status byte
	if rand.Float32() < 0.8 { // 80% chance of being acquired
		status |= 0x80
	}
	if rand.Float32() < 0.7 { // 70% chance of being complete
		status |= 0x40
	}
	if rand.Float32() < 0.9 { // 90% chance of being final
		status |= 0x20
	}
	// Year bits (0-4) are handled separately
	return status
}

// generateLampStatus creates random lamp status byte
func generateLampStatus() byte {
	var status byte
	if rand.Float32() < 0.8 { // 80% chance of being on
		status |= 0x01
	}
	// Random faults (each has 10% chance)
	if rand.Float32() < 0.1 {
		status |= 0x02 // Undervoltage
	}
	if rand.Float32() < 0.1 {
		status |= 0x04 // Overvoltage
	}
	if rand.Float32() < 0.1 {
		status |= 0x08 // Output limiter
	}
	if rand.Float32() < 0.1 {
		status |= 0x10 // Thermal derating
	}
	if rand.Float32() < 0.1 {
		status |= 0x20 // LED open circuit
	}
	if rand.Float32() < 0.1 {
		status |= 0x40 // LED thermal derating
	}
	if rand.Float32() < 0.1 {
		status |= 0x80 // LED thermal shutdown
	}
	return status
}

// generateAEMeasurements fills AE measurement data (10 bytes)
func generateAEMeasurements(d []byte) {
	// Voltage (8-9) - 180-250V
	voltage := 18000 + rand.Intn(7000) // 180.00V-250.00V
	binary.LittleEndian.PutUint16(d[0:2], uint16(voltage))

	// Current (10-11) - 0-5000 (units of 3.47mA)
	current := rand.Intn(5000)
	binary.LittleEndian.PutUint16(d[2:4], uint16(current))

	// Powered duration (12-13) and lit duration (14-15) - 0-65535
	binary.LittleEndian.PutUint16(d[4:6], uint16(rand.Intn(65536)))
	binary.LittleEndian.PutUint16(d[6:8], uint16(rand.Intn(65536)))

	// Power factor (16-17) - 0-100 with sign
	pf := byte(rand.Intn(101))
	sign := byte(0)
	if rand.Float32() < 0.1 { // 10% chance of negative
		sign = 1
	}
	d[8] = pf
	d[9] = sign
}

// generateSingleMeasurement fills a single measurement (11 bytes)
func generateSingleMeasurement(d []byte) {
	// Lamp status (0)
	d[0] = generateLampStatus()

	// Voltage (1-2) - 180-250V
	voltage := 18000 + rand.Intn(7000) // 180.00V-250.00V
	binary.LittleEndian.PutUint16(d[1:3], uint16(voltage))

	// Current (3-4) - 0-5000 (units of 3.47mA)
	current := rand.Intn(5000)
	binary.LittleEndian.PutUint16(d[3:5], uint16(current))

	// Powered duration (5-6) and lit duration (7-8) - 0-65535
	binary.LittleEndian.PutUint16(d[5:7], uint16(rand.Intn(65536)))
	binary.LittleEndian.PutUint16(d[7:9], uint16(rand.Intn(65536)))

	// Power factor (9-10) - 0-100 with sign
	pf := byte(rand.Intn(101))
	sign := byte(0)
	if rand.Float32() < 0.1 { // 10% chance of negative
		sign = 1
	}
	d[9] = pf
	d[10] = sign
}

// generateHarvestTimes fills harvest times (6 bytes)
func generateHarvestTimes(d []byte) {
	// Each harvest time is 2 bytes
	for i := 0; i < 6; i += 2 {
		// 10% chance of no measurement
		if rand.Float32() < 0.1 {
			d[i] = 0xFF
			d[i+1] = 0xFF
		} else {
			// Random time in minutes from noon (0-1439)
			minutes := rand.Intn(1440)
			binary.LittleEndian.PutUint16(d[i:i+2], uint16(minutes))
		}
	}
}

// Helper function to convert Measurement to byte slice
func measurementToBytes(m *Measurement) []byte {
	b := make([]byte, 56) // STX(1) + Computer(2) + BlockType(2) + Data(48) + Checksum(2) + ETB(1) = 56 bytes
	b[0] = m.STX
	copy(b[1:3], m.Computer[:])
	copy(b[3:5], m.BlockType[:])
	copy(b[5:53], m.Data[:])
	binary.BigEndian.PutUint16(b[53:55], binary.BigEndian.Uint16(m.Checksum[:]))
	b[55] = m.ETB
	return b
}

// byteToBCD converts a byte to BCD format
func byteToBCD(value byte) byte {
	return ((value / 10) << 4) | (value % 10)
}

// CalculateChecksum calculates and sets the checksum for the Measurement
func (m *Measurement) CalculateMeasurementChecksum() {
	var sum uint16

	// Sum all bytes from Computer to end of Data
	for _, b := range m.Computer[:] {
		sum += uint16(b)
	}
	for _, b := range m.BlockType[:] {
		sum += uint16(b)
	}
	for _, b := range m.Data[:] {
		sum += uint16(b)
	}

	// Store checksum in big-endian format
	binary.BigEndian.PutUint16(m.Checksum[:], sum)
}

// <---MEASUREMENT PACKAGE--->

// <---FINAL PACKAGE--->

func NewFinal() *Final {
	return &Final{
		STX:         STX,
		Computer:    [2]byte{'P', 'C'},
		EndBlock:    [2]byte{'D', '4'},
		EndDownload: [3]byte{'E', 'O', 'D'},
		Checksum:    [2]byte{0, 0}, // Will be calculated
		ETX:         ETX,
	}
}

// Bytes converts the Final package to a byte slice
func (f *Final) Bytes() []byte {
	b := make([]byte, 11) // STX(1) + Computer(2) + EndBlock(2) + EndDownload(3) + Checksum(2) + ETX(1)
	b[0] = f.STX
	copy(b[1:3], f.Computer[:])
	copy(b[3:5], f.EndBlock[:])
	copy(b[5:8], f.EndDownload[:])
	copy(b[8:10], f.Checksum[:])
	b[10] = f.ETX
	return b
}

// ParseFinal parses a byte slice into a Final package
func ParseFinal(data []byte) (*Final, error) {
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

	if len(framedData) != 11 { // Changed from 10 to 11 (STX + PC + D4 + EOD + checksum(2) + ETX)
		return nil, fmt.Errorf("invalid final message length (%d bytes), expected 11", len(framedData))
	}

	f := &Final{
		STX: framedData[0],
		ETX: framedData[10], // Changed from 9 to 10
	}

	copy(f.Computer[:], framedData[1:3])
	copy(f.EndBlock[:], framedData[3:5])
	copy(f.EndDownload[:], framedData[5:8])
	copy(f.Checksum[:], framedData[8:10]) // Checksum is 2 bytes at positions 8-9

	// Verify checksum
	var sum uint16
	// Sum bytes from Computer (1) to EndDownload (7)
	for _, b := range framedData[1:8] {
		sum += uint16(b)
	}

	receivedChecksum := binary.BigEndian.Uint16(f.Checksum[:])
	if sum != receivedChecksum {
		return nil, fmt.Errorf("invalid checksum (calculated: %d, received: %d)", sum, receivedChecksum)
	}

	return f, nil
}

func (f *Final) CalculateFinalChecksum() {
	var sum uint16

	// Sum all bytes from Computer to EndDownload
	for _, b := range f.Computer[:] {
		sum += uint16(b)
	}
	for _, b := range f.EndBlock[:] {
		sum += uint16(b)
	}
	for _, b := range f.EndDownload[:] {
		sum += uint16(b)
	}

	// Store checksum in big-endian format
	binary.BigEndian.PutUint16(f.Checksum[:], sum)
}

// <---FINAL PACKAGE--->
