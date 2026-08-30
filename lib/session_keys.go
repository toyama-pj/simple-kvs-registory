package lib

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const deviceSessionKeySize = 16

func ValidateSessionKeyEncryptionKey(value string) error {
	_, err := decodeMasterKey(value)
	return err
}

func decodeMasterKey(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("DEVICE_SESSION_KEY_ENCRYPTION_KEY is not configured")
	}
	if decoded, err := hex.DecodeString(value); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding} {
		if decoded, err := encoding.DecodeString(value); err == nil && len(decoded) == 32 {
			return decoded, nil
		}
	}
	return nil, errors.New("DEVICE_SESSION_KEY_ENCRYPTION_KEY must be 32 bytes encoded as hex or base64")
}

func ParseSessionKey(value string) ([deviceSessionKeySize]byte, error) {
	var key [deviceSessionKeySize]byte
	decoded, err := hex.DecodeString(strings.TrimSpace(value))
	if err != nil || len(decoded) != deviceSessionKeySize {
		return key, errors.New("LoRaWAN session key must be 32 hexadecimal characters")
	}
	copy(key[:], decoded)
	return key, nil
}

func EncryptSessionKey(masterKey, sessionKey string) (string, error) {
	plaintext, err := ParseSessionKey(sessionKey)
	if err != nil {
		return "", err
	}
	key, err := decodeMasterKey(masterKey)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, plaintext[:], nil)
	return base64.RawStdEncoding.EncodeToString(sealed), nil
}

func DecryptSessionKey(masterKey, encrypted string) ([deviceSessionKeySize]byte, error) {
	var result [deviceSessionKeySize]byte
	key, err := decodeMasterKey(masterKey)
	if err != nil {
		return result, err
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(encrypted)
	if err != nil {
		return result, fmt.Errorf("decode encrypted session key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return result, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return result, err
	}
	if len(ciphertext) < aead.NonceSize() {
		return result, errors.New("encrypted session key is too short")
	}
	plaintext, err := aead.Open(nil, ciphertext[:aead.NonceSize()], ciphertext[aead.NonceSize():], nil)
	if err != nil {
		return result, errors.New("decrypt session key: authentication failed")
	}
	if len(plaintext) != deviceSessionKeySize {
		return result, errors.New("decrypted session key has an invalid length")
	}
	copy(result[:], plaintext)
	return result, nil
}
