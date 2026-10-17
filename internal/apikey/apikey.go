package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

const (
	keyPrefix = "sk_live_"
	keyBytes  = 32
	prefixLen = 12
)

func Generate() (key, prefix string, hash []byte, err error) {
	// create a byte slice of length keyBytes and fill it with crypto/rand.Read.
	b := make([]byte, keyBytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", nil, fmt.Errorf("generate random bytes: %w", err)
	}

	// encode the bytes to text with base64.RawURLEncoding.EncodeToString
	s := base64.RawURLEncoding.EncodeToString(b)
	key = keyPrefix + s

	// prefix = the first prefixLen characters of the key (use slicing: key[:prefixLen]).
	prefix = key[:prefixLen]

	// hash = Hash(key)
	hash = Hash(key)

	// return key, prefix, hash, nil
	return key, prefix, hash, nil
}

// Hash returns sha256 of the key as a byte slice.
// The same key always gives the same hash — that's why we can search by it in the DB.
func Hash(key string) []byte {
	// compute sha256.Sum256 of []byte(key).
	// Sum256 returns an ARRAY [32]byte → convert it to a slice with [:] and return it.
	sum := sha256.Sum256([]byte(key))

	return sum[:]
}
