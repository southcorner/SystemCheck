// Command agenttool is a release helper for the signed auto-update mechanism.
//
//	agenttool keygen [keyfile]     generate an ed25519 keypair: prints the PUBLIC
//	                               key (base64, to pin into the agent build) and
//	                               writes the PRIVATE key to keyfile (keep secret)
//	agenttool sign <bin> <keyfile> print the base64 ed25519 signature of <bin>
//	                               using the private key in <keyfile>
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "keygen":
		out := "agent-signing.key"
		if len(os.Args) > 2 {
			out = os.Args[2]
		}
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		must(err)
		must(os.WriteFile(out, []byte(base64.StdEncoding.EncodeToString(priv)), 0o600))
		fmt.Println("PUBLIC_KEY_B64=" + base64.StdEncoding.EncodeToString(pub))
		fmt.Fprintln(os.Stderr, "private key written to "+out+" (keep this secret; do not commit)")
	case "sign":
		if len(os.Args) < 4 {
			usage()
		}
		kb, err := os.ReadFile(os.Args[3])
		must(err)
		priv, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(kb)))
		must(err)
		if len(priv) != ed25519.PrivateKeySize {
			fail("private key is not a valid ed25519 key")
		}
		data, err := os.ReadFile(os.Args[2])
		must(err)
		sig := ed25519.Sign(ed25519.PrivateKey(priv), data)
		fmt.Println(base64.StdEncoding.EncodeToString(sig))
	case "verify":
		if len(os.Args) < 5 {
			usage()
		}
		pub, err := base64.StdEncoding.DecodeString(os.Args[3])
		must(err)
		sig, err := base64.StdEncoding.DecodeString(os.Args[4])
		must(err)
		data, err := os.ReadFile(os.Args[2])
		must(err)
		if ed25519.Verify(ed25519.PublicKey(pub), data, sig) {
			fmt.Println("OK: signature valid")
		} else {
			fail("signature INVALID")
		}
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: agenttool keygen [keyfile] | agenttool sign <bin> <keyfile>")
	os.Exit(2)
}
func must(err error) {
	if err != nil {
		fail(err.Error())
	}
}
func fail(msg string) {
	fmt.Fprintln(os.Stderr, "error: "+msg)
	os.Exit(1)
}
