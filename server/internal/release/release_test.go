package release

import "testing"

func TestSignVerifyRoundTrip(t *testing.T) {
	pub, priv, err := GenerateKey()
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	data := []byte("agent-0.2.0.exe contents")
	sig, err := Sign(priv, data)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if !Verify(pub, sig, data) {
		t.Fatal("expected valid signature to verify")
	}
	// Tampered data must fail.
	if Verify(pub, sig, []byte("tampered")) {
		t.Fatal("tampered data should not verify")
	}
	// Wrong key must fail.
	pub2, _, _ := GenerateKey()
	if Verify(pub2, sig, data) {
		t.Fatal("wrong public key should not verify")
	}
}
