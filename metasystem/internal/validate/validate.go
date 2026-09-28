// Package validate holds whole-artifact validators and rewrites: each
// function checks one artifact shape end to end (an assembled turn
// prompt, a critique join, a quote block, a receipt claim, a wrapper
// token) or performs one entangled transform (the metasystem.conf
// runtime tailoring, the second-session isolation copy). The assert
// scripts exec into these through the binary's verbs.
package validate

import (
	"os"
	"strings"
)

// splitLines splits text into lines the way the validators read files:
// no trailing empty line for a trailing newline, and no carriage
// returns left on line content.
func splitLines(text string) []string {
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for index, line := range lines {
		lines[index] = strings.TrimSuffix(line, "\r")
	}
	return lines
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// readFileIfExists distinguishes a missing file from an unreadable one:
// the validators word those two failures differently.
func readFileIfExists(path string) (data []byte, exists bool, err error) {
	data, err = os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, true, err
	}
	return data, true, nil
}
