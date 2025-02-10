package utils

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/edgelesssys/ego/attestation"
	"github.com/edgelesssys/ego/attestation/tcbstatus"
	"github.com/edgelesssys/ego/eclient"
)

// VerifyReport verifies the enclave report.
func VerifyReport(reportBytes, certBytes, signer []byte) error {
	start := time.Now()
	report, err := eclient.VerifyRemoteReport(reportBytes)
	if err == attestation.ErrTCBLevelInvalid {
		fmt.Printf("Warning: TCB level is invalid: %v\n%v\n", report.TCBStatus, tcbstatus.Explain(report.TCBStatus))
		fmt.Println("We'll ignore this issue in this sample. For an app that should run in production, you must decide which of the different TCBStatus values are acceptable for you to continue.")
	} else if err != nil {
		return err
	}

	hash := sha256.Sum256(certBytes)
	if !bytes.Equal(report.Data[:len(hash)], hash[:]) {
		return errors.New("report data does not match the certificate's hash")
	}

	// You can either verify the UniqueID or the tuple (SignerID, ProductID, SecurityVersion, Debug).

	if report.SecurityVersion < 1 {
		return errors.New("invalid security version")
	}
	if binary.LittleEndian.Uint16(report.ProductID) != 1 {
		return errors.New("invalid product")
	}
	if !bytes.Equal(report.SignerID, signer) {
		return errors.New("invalid signer")
	}

	fmt.Println("Verification took:", time.Since(start))
	// For production, you must also verify that report.Debug == false

	return nil
}

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
