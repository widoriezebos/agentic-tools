package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

var nonSlugChar = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// runUtilSlug prints a stable slug of its positional argument: runs of
// characters outside [A-Za-z0-9._-] collapse to a single dash, leading and
// trailing dashes and dots are trimmed, the result is lowercased, and an empty
// result becomes "session". Used to derive instance tags that must match
// across the arming and hook paths.
func runUtilSlug(args []string) int {
	input := ""
	if len(args) > 0 {
		input = args[0]
	}
	slug := strings.ToLower(strings.Trim(nonSlugChar.ReplaceAllString(input, "-"), "-."))
	if slug == "" {
		slug = "session"
	}
	fmt.Println(slug)
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
