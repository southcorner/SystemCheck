package auth

import (
	"encoding/base32"
	"strings"
	"testing"
	"time"
)

// currentCodeForTest derives the current TOTP code for a base32 secret using the
// package's own hotp primitive.
func currentCodeForTest(secret string) string {
	s := strings.ToUpper(strings.TrimSpace(secret))
	if pad := len(s) % 8; pad != 0 {
		s += strings.Repeat("=", 8-pad)
	}
	key, _ := base32.StdEncoding.DecodeString(s)
	return hotp(key, uint64(time.Now().Unix()/30))
}

func TestPasswordHashVerify(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	ok, err := VerifyPassword("correct horse battery staple", hash)
	if err != nil || !ok {
		t.Fatalf("expected match, got ok=%v err=%v", ok, err)
	}
	bad, _ := VerifyPassword("wrong password", hash)
	if bad {
		t.Fatal("expected mismatch for wrong password")
	}
}

func TestTOTPRoundTrip(t *testing.T) {
	secret, err := NewTOTPSecret()
	if err != nil {
		t.Fatalf("secret: %v", err)
	}
	// Generate the current code via the same primitive and verify it.
	// VerifyTOTP allows +/- one step, so the current step must pass.
	if VerifyTOTP(secret, "000000") {
		t.Log("unlikely but code happened to be 000000")
	}
	// Derive a valid code and check acceptance.
	code := currentCodeForTest(secret)
	if !VerifyTOTP(secret, code) {
		t.Fatalf("expected current TOTP code %q to verify", code)
	}
	if VerifyTOTP(secret, "123") {
		t.Fatal("short/invalid code should not verify")
	}
}
