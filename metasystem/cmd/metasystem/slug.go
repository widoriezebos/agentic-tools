package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

// runUtilNowNs prints the current wall-clock time in nanoseconds, for coarse
// elapsed-time measurement across two calls.
func runUtilNowNs(args []string) int {
	fmt.Println(time.Now().UnixNano())
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
