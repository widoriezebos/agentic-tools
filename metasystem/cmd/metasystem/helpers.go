// The cross-family helpers every verb file may use: flag parsing,
// JSON read/write/print. Family-named files hold only their own verbs.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

func printJSON(value any) {
	encoded, err := json.Marshal(value)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	fmt.Println(string(encoded))
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

func readJSONObject(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func jsonIntField(v any) (int64, bool) {
	f, ok := v.(float64)
	if !ok || f != float64(int64(f)) {
		return 0, false
	}
	return int64(f), true
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
