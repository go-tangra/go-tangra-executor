package service

import (
	"crypto/sha256"
	"encoding/hex"
)

// ComputeContentHash returns the SHA-256 hex digest of the given content.
func ComputeContentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}
