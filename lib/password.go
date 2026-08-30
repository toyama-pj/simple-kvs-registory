package lib

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	PasswordMinLength = 12
	PasswordMaxLength = 128

	argon2Memory      = 19 * 1024
	argon2Iterations  = 2
	argon2Parallelism = 1
	argon2SaltLength  = 16
	argon2KeyLength   = 32
)

var ErrInvalidPasswordHash = errors.New("invalid password hash")

// ValidateNewPassword applies length limits without modifying the supplied
// value. Passwords may contain spaces and arbitrary valid Unicode.
func ValidateNewPassword(password string) error {
	if !utf8.ValidString(password) {
		return errors.New("password must be valid UTF-8")
	}
	length := utf8.RuneCountInString(password)
	if length < PasswordMinLength || length > PasswordMaxLength || len(password) > PasswordMaxLength*4 {
		return fmt.Errorf("password must contain %d to %d characters", PasswordMinLength, PasswordMaxLength)
	}
	return nil
}

// HashPassword returns an Argon2id PHC string with a unique random salt.
func HashPassword(password string) (string, error) {
	if err := ValidateNewPassword(password); err != nil {
		return "", err
	}
	salt := make([]byte, argon2SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argon2Iterations, argon2Memory, argon2Parallelism, argon2KeyLength)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argon2Memory,
		argon2Iterations,
		argon2Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// VerifyPassword parses bounded Argon2id parameters and compares in constant
// time. Bounds prevent a corrupted database value from forcing huge resource
// allocations during login.
func VerifyPassword(encoded, password string) (bool, error) {
	if !utf8.ValidString(password) || len(password) > PasswordMaxLength*4 {
		return false, nil
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return false, ErrInvalidPasswordHash
	}
	var version int
	if count, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || count != 1 || version != argon2.Version || parts[2] != fmt.Sprintf("v=%d", version) {
		return false, ErrInvalidPasswordHash
	}
	var memory, iterations uint32
	var parallelism uint8
	if count, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil || count != 3 || parts[3] != fmt.Sprintf("m=%d,t=%d,p=%d", memory, iterations, parallelism) {
		return false, ErrInvalidPasswordHash
	}
	if memory < 7*1024 || memory > 64*1024 || iterations < 1 || iterations > 10 || parallelism < 1 || parallelism > 8 {
		return false, ErrInvalidPasswordHash
	}
	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil || len(salt) < 8 || len(salt) > 64 {
		return false, ErrInvalidPasswordHash
	}
	want, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil || len(want) != argon2KeyLength {
		return false, ErrInvalidPasswordHash
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(want)))
	return subtle.ConstantTimeCompare(actual, want) == 1, nil
}
