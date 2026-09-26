// The cross-family helpers every verb file may use: flag parsing,
// JSON read/write/print. Family-named files hold only their own verbs.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

type pathValue struct {
	target *string
}

func (value pathValue) String() string {
	if value.target == nil {
		return ""
	}
	return *value.target
}

func (value pathValue) Set(raw string) error {
	resolved, err := resolvePathFlag(raw)
	if err != nil {
		return err
	}
	*value.target = resolved
	return nil
}

func resolvePathFlag(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	absolute, err := filepath.Abs(raw)
	if err != nil {
		return "", err
	}
	return filepath.Clean(absolute), nil
}

func pathFlag(flags *flag.FlagSet, name, value, usage string) *string {
	target := new(string)
	pathFlagVar(flags, target, name, value, usage)
	return target
}

func pathFlagVar(flags *flag.FlagSet, target *string, name, value, usage string) {
	resolved, err := resolvePathFlag(value)
	if err != nil {
		panic(fmt.Sprintf("invalid default for -%s: %v", name, err))
	}
	*target = resolved
	flags.Var(pathValue{target: target}, name, usage)
}

func printJSON(value any) { writeJSONLine(os.Stdout, os.Stderr, value) }

// writeJSONLine is printJSON onto a caller's own streams.
func writeJSONLine(stdout, stderr io.Writer, value any) {
	encoded, err := json.Marshal(value)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return
	}
	fmt.Fprintln(stdout, string(encoded))
}

// writeIdentityJSON writes indented, key-sorted JSON atomically: temp in the
// target directory, fsync, rename, directory fsync.
func writeIdentityJSON(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	// Through the durable-write owner; the empty anchor syncs only the
	// target's own directory, and the durable outcome is dropped,
	// because this writer's callers have not adopted the two-outcome
	// contract.
	_, writeErr := atomicfile.WriteText(path, string(encoded), "")
	return writeErr
}

// strictBool registers a string boolean flag that accepts exactly the two
// spellings its shell callers already pass and refuses anything else: a
// typo must be a usage error (exit 2), never a silent false — a
// mistyped --signal value would quietly disable the session-handshake
// deadline. Existing wire spellings are preserved ("true"/"false" and
// "1"/"0" families); NEW verbs use flags.Bool instead.
func strictBool(flags *flag.FlagSet, name, trueWord, falseWord, usage string) *bool {
	value := false
	flags.Func(name, usage, func(raw string) error {
		switch raw {
		case trueWord:
			value = true
		case falseWord:
			value = false
		default:
			return fmt.Errorf("must be %s or %s", trueWord, falseWord)
		}
		return nil
	})
	return &value
}
