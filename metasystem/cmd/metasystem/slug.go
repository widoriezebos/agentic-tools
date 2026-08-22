package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// runUtilSlug prints a stable slug of its positional argument. The sanitize
// rule's one home is lease.Slug: instance tags derived here must match the
// announcement filenames the lease writes.
func runUtilSlug(args []string) int {
	input := ""
	if len(args) > 0 {
		input = args[0]
	}
	fmt.Println(lease.Slug(input))
	return 0
}

// runUtilJSONValidate exits 0 when the --file or --value content is valid JSON,
// else 1.
func runUtilJSONValidate(args []string) int {
	flags := flag.NewFlagSet("util json-validate", flag.ContinueOnError)
	file := flags.String("file", "", "JSON file to validate")
	value := flags.String("value", "", "JSON string to validate")
	if flags.Parse(args) != nil {
		return 2
	}
	content := []byte(*value)
	if *file != "" {
		read, err := os.ReadFile(*file)
		if err != nil {
			return 1
		}
		content = read
	}
	var parsed any
	if json.Unmarshal(content, &parsed) != nil {
		return 1
	}
	return 0
}

// runUtilNowNs prints the current wall-clock time in nanoseconds, for coarse
// elapsed-time measurement across two calls.
func runUtilNowNs(args []string) int {
	fmt.Println(time.Now().UnixNano())
	return 0
}

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

// runUtilSHA256 prints the hex SHA-256 of --file, or of stdin when --file
// is absent. It exists so production scripts need no shasum — a perl tool
// on Linux — when the engine already owns hashing: owning the
// capability beats documenting the dependency.
func runUtilSHA256(args []string) int {
	flags := flag.NewFlagSet("util sha256", flag.ContinueOnError)
	file := flags.String("file", "", "file to hash; stdin when absent")
	if flags.Parse(args) != nil {
		return 2
	}
	var source io.Reader = os.Stdin
	if *file != "" {
		handle, err := os.Open(*file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "util sha256: %v\n", err)
			return 1
		}
		defer handle.Close()
		source = handle
	}
	digest := sha256.New()
	if _, err := io.Copy(digest, source); err != nil {
		fmt.Fprintf(os.Stderr, "util sha256: %v\n", err)
		return 1
	}
	fmt.Printf("%x\n", digest.Sum(nil))
	return 0
}
