package utils

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// Encodes two []byte slices into a single []byte with length prefixes
func Encode(a, b []byte) []byte {
	buf := new(bytes.Buffer)

	// Write length of 'a' as 4-byte big-endian
	binary.Write(buf, binary.BigEndian, uint32(len(a)))
	buf.Write(a)

	// Write length of 'b' as 4-byte big-endian
	binary.Write(buf, binary.BigEndian, uint32(len(b)))
	buf.Write(b)

	return buf.Bytes()
}

// Decodes a []byte into the original two []byte slices
func Decode(data []byte) ([]byte, []byte, error) {
	if len(data) < 8 {
		return nil, nil, fmt.Errorf("invalid data: too short")
	}

	offset := 0

	// Read first 4 bytes as length of 'a'
	aLen := binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	// Check if there's enough data for 'a' + b's header
	if offset+int(aLen) > len(data)-4 {
		return nil, nil, fmt.Errorf("invalid a length")
	}

	a := data[offset : offset+int(aLen)]
	offset += int(aLen)

	// Read next 4 bytes as length of 'b'
	bLen := binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	// Check if there's enough data for 'b'
	if offset+int(bLen) > len(data) {
		return nil, nil, fmt.Errorf("invalid b length")
	}

	b := data[offset : offset+int(bLen)]
	offset += int(bLen)

	// Verify no extra data remains
	if offset != len(data) {
		return nil, nil, fmt.Errorf("extra data at end")
	}

	return a, b, nil
}
