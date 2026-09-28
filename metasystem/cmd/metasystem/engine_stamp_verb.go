package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
)

// runUtilEngineStamp prints the build stamp read from an engine file's bytes,
// the one Go reader the steward and seat launch use, so shell callers never
// parse `go version -m` and a -trimpath engine still answers. The file is read,
// never executed.
func runUtilEngineStamp(args []string) int {
	return engineStamp(args, os.Stdout, os.Stderr)
}

func engineStamp(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("util engine-stamp", flag.ContinueOnError)
	flags.SetOutput(stderr)
	path := flags.String("file", "", "engine file whose build stamp to read")
	if flags.Parse(args) != nil || *path == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: metasystem internal util engine-stamp --file PATH")
		return 2
	}
	file, err := os.Open(*path)
	if err != nil {
		fmt.Fprintf(stderr, "util engine-stamp: %v\n", err)
		return 1
	}
	defer file.Close()
	stamp, err := enginebuild.ReadStamp(file)
	if errors.Is(err, enginebuild.ErrStampDisagreement) || err == nil && stamp == "" {
		fmt.Fprintf(stderr, "util engine-stamp: no stamp in %s\n", *path)
		return 1
	}
	if err != nil {
		fmt.Fprintf(stderr, "util engine-stamp: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, stamp)
	return 0
}
