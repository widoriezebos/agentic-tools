package main

import (
	"flag"
	"fmt"
)

// strictBool registers a string boolean flag that accepts exactly the two
// spellings its shell callers already pass and refuses anything else: a
// typo must be a usage error (exit 2), never a silent false (D16/cli-7 —
// a mistyped --signal value used to quietly disable the session-handshake
// deadline). Existing wire spellings are preserved ("true"/"false" and
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
