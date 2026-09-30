package main

import "strings"

// typedArgvLess is this command as the person typed it, less every named
// option: a value option drops its value with it (--name VALUE or
// --name=VALUE), a switch drops only itself. It is the retry a refusal of a
// misplaced option names ("Messages a Person Reads"); retryWith is the one
// mechanism behind it.
func (inv *intentInvocation) typedArgvLess(names ...string) []string {
	return inv.retryWith(names)
}

// typedArgvWith is the typed command with words added at its end: the retry
// a refusal of a missing option names.
func (inv *intentInvocation) typedArgvWith(words ...string) []string {
	return inv.retryWith(nil, words...)
}

// typedArgvFor is the typed command with target placed right after the
// command's words: the retry of a command that named no target.
func (inv *intentInvocation) typedArgvFor(target string) []string {
	return append(append(append([]string{"metasystem"}, inv.command.words()...), target), inv.raw...)
}

// withoutSwitch is argv less every --name (or --name=VALUE) switch.
func withoutSwitch(argv []string, name string) []string {
	var kept []string
	for _, word := range argv {
		if word == "--"+name || strings.HasPrefix(word, "--"+name+"=") {
			continue
		}
		kept = append(kept, word)
	}
	return kept
}
