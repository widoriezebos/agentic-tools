package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
)

// runUtilTokenHex prints a random hex token of the requested byte length
// (the hex string is twice that many characters). Used for nonces, ids, and
// name suffixes.
func runUtilTokenHex(args []string) int {
	flags := flag.NewFlagSet("util token-hex", flag.ContinueOnError)
	n := flags.Int("bytes", 16, "number of random bytes (hex output is 2x this)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *n < 1 {
		fmt.Fprintln(os.Stderr, "util token-hex: --bytes must be positive")
		return 2
	}
	buf := make([]byte, *n)
	if _, err := rand.Read(buf); err != nil {
		fmt.Fprintln(os.Stderr, "util token-hex:", err)
		return 1
	}
	fmt.Println(hex.EncodeToString(buf))
	return 0
}
