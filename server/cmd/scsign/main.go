// Command scsign generates release keys and signs/verifies agent binaries.
//
//	scsign keygen
//	scsign sign  -key <privB64> <file>
//	scsign verify -pub <pubB64> -sig <sigB64> <file>
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/southcorner/systemcheck/server/internal/release"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "keygen":
		pub, priv, err := release.GenerateKey()
		must(err)
		fmt.Printf("public (pin in agent):  %s\n", pub)
		fmt.Printf("private (keep offline): %s\n", priv)
	case "sign":
		fs := flag.NewFlagSet("sign", flag.ExitOnError)
		key := fs.String("key", "", "base64 private key")
		_ = fs.Parse(os.Args[2:])
		if *key == "" || fs.NArg() != 1 {
			usage()
		}
		data, err := os.ReadFile(fs.Arg(0))
		must(err)
		sig, err := release.Sign(*key, data)
		must(err)
		fmt.Println(sig)
	case "verify":
		fs := flag.NewFlagSet("verify", flag.ExitOnError)
		pub := fs.String("pub", "", "base64 public key")
		sig := fs.String("sig", "", "base64 signature")
		_ = fs.Parse(os.Args[2:])
		if *pub == "" || *sig == "" || fs.NArg() != 1 {
			usage()
		}
		data, err := os.ReadFile(fs.Arg(0))
		must(err)
		if release.Verify(*pub, *sig, data) {
			fmt.Println("OK: signature valid")
		} else {
			fmt.Println("FAIL: signature invalid")
			os.Exit(1)
		}
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: scsign keygen | sign -key <privB64> <file> | verify -pub <pubB64> -sig <sigB64> <file>")
	os.Exit(2)
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
