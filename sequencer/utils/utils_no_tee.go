//go:build no_tee
// +build no_tee

package utils

// VerifyReport verifies the enclave report, this is a No-op
func VerifyReport(reportBytes, certBytes, signer []byte) error {
	return nil
}
