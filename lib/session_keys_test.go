package lib_test

import (
	"encoding/hex"
	"testing"

	"github.com/toyama-pj/simple-kvs-registory/lib"
)

func TestDeviceSessionKeyEncryptionRoundTrip(t *testing.T) {
	master := "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	secret := "00112233445566778899aabbccddeeff"
	encrypted, err := lib.EncryptSessionKey(master, secret)
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == secret {
		t.Fatal("session key was stored as plaintext")
	}
	decrypted, err := lib.DecryptSessionKey(master, encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(decrypted[:]); got != secret {
		t.Fatalf("decrypted key = %s", got)
	}
	if _, err := lib.DecryptSessionKey("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", encrypted); err == nil {
		t.Fatal("wrong master key was accepted")
	}
}
