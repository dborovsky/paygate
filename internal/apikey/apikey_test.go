package apikey

import (
	"bytes"
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	key, prefix, hash, err := Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if !strings.HasPrefix(key, "sk_live_") {
		t.Errorf("key %q must start with sk_live_", key)
	}
	if len(key) != 51 {
		t.Errorf("key length = %d, want 51", len(key))
	}
	if prefix != key[:12] {
		t.Errorf("prefix = %q, want %q", prefix, key[:12])
	}
	if !bytes.Equal(hash, Hash(key)) {
		t.Error("hash must be equal to Hash(key)")
	}
}

func TestGenerateIsRandom(t *testing.T) {
	key1, _, _, _ := Generate()
	key2, _, _, _ := Generate()
	if key1 == key2 {
		t.Error("two generated keys must be different")
	}
}

func TestHashIsDeterministic(t *testing.T) {
	if !bytes.Equal(Hash("sk_live_abc"), Hash("sk_live_abc")) {
		t.Error("same key must give the same hash")
	}
	if bytes.Equal(Hash("sk_live_abc"), Hash("sk_live_abd")) {
		t.Error("different keys must give different hashes")
	}
	if len(Hash("sk_live_abc")) != 32 {
		t.Error("sha256 hash must be 32 bytes")
	}
}
