// Package buildhelpers verifies and updates supported upstream source releases.
package buildhelpers

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"strings"
)

func Verify(data []byte, algorithm, expected string) error {
	return VerifyReader(bytes.NewReader(data), algorithm, expected)
}

func VerifyReader(source io.Reader, algorithm, expected string) error {
	var digest hash.Hash
	switch strings.ReplaceAll(strings.ToLower(algorithm), "-", "") {
	case "sha256":
		digest = sha256.New()
	case "sha384":
		digest = sha512.New384()
	case "sha512":
		digest = sha512.New()
	default:
		return fmt.Errorf("unsupported or weak checksum algorithm %q", algorithm)
	}
	wanted, err := hex.DecodeString(strings.TrimSpace(expected))
	if err != nil || len(wanted) != digest.Size() {
		return fmt.Errorf("malformed %s checksum", algorithm)
	}
	if _, err := io.Copy(digest, source); err != nil {
		return err
	}
	if subtle.ConstantTimeCompare(digest.Sum(nil), wanted) != 1 {
		return fmt.Errorf("%s checksum mismatch; source pins were not updated", algorithm)
	}
	return nil
}
