package lib

import (
	"errors"
	"strings"
	"testing"
)

func TestPasswordHashRoundTripAndUniqueSalt(t *testing.T) {
	password := "correct horse battery staple"
	first, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || strings.Contains(first, password) {
		t.Fatal("password hashes must be salted and must not contain plaintext")
	}
	matched, err := VerifyPassword(first, password)
	if err != nil || !matched {
		t.Fatalf("correct password matched=%v err=%v", matched, err)
	}
	matched, err = VerifyPassword(first, "incorrect password")
	if err != nil || matched {
		t.Fatalf("incorrect password matched=%v err=%v", matched, err)
	}
}

func TestPasswordValidationAndMalformedHash(t *testing.T) {
	if err := ValidateNewPassword("too short"); err == nil {
		t.Fatal("short password was accepted")
	}
	if err := ValidateNewPassword(strings.Repeat("a", PasswordMaxLength+1)); err == nil {
		t.Fatal("long password was accepted")
	}
	if _, err := VerifyPassword("$argon2id$v=19$m=999999,t=2,p=1$AA$AA", "password"); !errors.Is(err, ErrInvalidPasswordHash) {
		t.Fatalf("malformed hash error = %v", err)
	}
}
