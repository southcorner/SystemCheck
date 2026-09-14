package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func TestIsNewer(t *testing.T) {
	cases := []struct {
		cand, cur string
		want      bool
	}{
		{"0.2.0", "0.1.0", true},
		{"0.1.10", "0.1.9", true},
		{"1.0.0", "0.9.9", true},
		{"0.1.0", "0.1.0", false},
		{"0.1.0", "0.2.0", false},
	}
	for _, c := range cases {
		if got := isNewer(c.cand, c.cur); got != c.want {
			t.Errorf("isNewer(%q,%q)=%v want %v", c.cand, c.cur, got, c.want)
		}
	}
}

func TestCheckAndUpdateVerifiesSignature(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	PinnedPublicKey = base64.StdEncoding.EncodeToString(pub)
	defer func() { PinnedPublicKey = "" }()

	binary := []byte("new agent binary bytes")
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, binary))

	get := func(_ context.Context, _ string) ([]byte, error) { return binary, nil }
	var installed []byte
	install := func(data []byte) error { installed = data; return nil }

	// Newer + valid signature installs.
	ok, err := CheckAndUpdate(context.Background(), "0.1.0",
		VersionInfo{Version: "0.2.0", URL: "https://x/agent.exe", Signature: sig}, get, install)
	if err != nil || !ok {
		t.Fatalf("expected install, got ok=%v err=%v", ok, err)
	}
	if string(installed) != string(binary) {
		t.Fatal("installed bytes mismatch")
	}

	// Tampered download must be rejected and not installed.
	installed = nil
	badGet := func(_ context.Context, _ string) ([]byte, error) { return []byte("tampered"), nil }
	ok, err = CheckAndUpdate(context.Background(), "0.1.0",
		VersionInfo{Version: "0.2.0", URL: "https://x/agent.exe", Signature: sig}, badGet, install)
	if ok || err == nil {
		t.Fatal("expected signature failure")
	}
	if installed != nil {
		t.Fatal("must not install tampered binary")
	}

	// Not newer -> no-op.
	ok, err = CheckAndUpdate(context.Background(), "0.2.0",
		VersionInfo{Version: "0.2.0", URL: "https://x", Signature: sig}, get, install)
	if ok || err != nil {
		t.Fatalf("expected no-op for same version, got ok=%v err=%v", ok, err)
	}
}

func TestDisabledWhenNoKey(t *testing.T) {
	PinnedPublicKey = ""
	ok, err := CheckAndUpdate(context.Background(), "0.1.0",
		VersionInfo{Version: "0.2.0", URL: "https://x", Signature: "sig"},
		func(context.Context, string) ([]byte, error) { return nil, nil },
		func([]byte) error { return nil })
	if ok || err != nil {
		t.Fatalf("expected disabled no-op, got ok=%v err=%v", ok, err)
	}
}
