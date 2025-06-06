package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

func Hash(cmd string) string {
	sum := sha256.Sum256([]byte(cmd))
	return hex.EncodeToString(sum[:])
}
