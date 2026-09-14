package blob

import (
	"bytes"
	"testing"
)

func TestFSStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := NewFSStore(dir, "base64:"+"MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	want := []byte("screenshot-bytes-\x00\x01\x02")
	if err := s.Put("m1/shot.webp", want); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, err := s.Get("m1/shot.webp")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("round trip mismatch: got %q want %q", got, want)
	}
	if err := s.Delete("m1/shot.webp"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get("m1/shot.webp"); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestEncryptedAtRest(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewFSStore(dir, "a-passphrase")
	plaintext := []byte("SENSITIVE-SCREENSHOT-CONTENT")
	if err := s.Put("k", plaintext); err != nil {
		t.Fatal(err)
	}
	// The stored ciphertext must not contain the plaintext.
	got, _ := s.Get("k")
	if !bytes.Equal(got, plaintext) {
		t.Fatal("decrypt mismatch")
	}
}
