package models

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// NewID generates a CUID-like string compatible with the existing frontend expectations.
func NewID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return fmt.Sprintf("c%s%s", time.Now().UTC().Format("20060102150405"), hex.EncodeToString(b))
}
