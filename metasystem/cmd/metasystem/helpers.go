// The cross-family helpers every verb file may use: flag parsing,
// JSON read/write/print. Family-named files hold only their own verbs.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/cliflags"
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

// writeJSONLine prints value as one JSON line on stdout; an encoding
// failure is printed on stderr.
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

// newFlagSet is the one constructor of the engine's flag sets (package
// cliflags): a parse error is answered in the public style on stderr and a
// help request prints the options on stdout, the invocation's own streams.
func newFlagSet(name string, stdout, stderr io.Writer) *flag.FlagSet {
	if publicCommand != nil {
		if command, ok := publicCommand(name); ok {
			shown := publicOptions(command)
			return cliflags.NewShown(name, "metasystem "+name, stdout, stderr, func(option string) bool { return shown[option] })
		}
	}
	return cliflags.New(name, commandLabel(name), stdout, stderr)
}

// command is an entrypoint's handler: its words and the invocation's own
// standard output and error, which everything it prints is written to.
type command func(args []string, stdout, stderr io.Writer) int

// publicCommand finds the public action a flag set is named after; bound in
// init, as internalEntrypoint is.
var publicCommand func(name string) (intentCommand, bool)

// publicOptions are the options a public action's help documents: its
// shown flags and every --NAME its usage lines spell.
func publicOptions(command intentCommand) map[string]bool {
	shown := map[string]bool{}
	for _, flag := range command.flags {
		if flag.hidden {
			continue
		}
		shown[flag.name] = true
		for _, alias := range flag.aliases {
			shown[alias] = true
		}
	}
	for _, line := range command.usage {
		for _, word := range strings.Fields(line) {
			word = strings.Trim(word, "[]|")
			if strings.HasPrefix(word, "--") {
				shown[strings.SplitN(strings.TrimPrefix(word, "--"), "=", 2)[0]] = true
			}
		}
	}
	return shown
}

// helpAware makes a lone --help or -h a successful request: a handler whose
// parser answered it with its usage exits 0 rather than as a refusal.
func helpAware(run command) command {
	return func(args []string, stdout, stderr io.Writer) int {
		if len(args) != 1 || !isHelpWord(args[0]) {
			return run(args, stdout, stderr)
		}
		before := cliflags.HelpAnswered()
		code := run(args, stdout, stderr)
		if code == 2 && cliflags.HelpAnswered() > before {
			return 0
		}
		return code
	}
}

// commandLabel is how a person reads a flag set's command: an internal
// entrypoint is "metasystem internal FAMILY VERB", anything else
// "metasystem NAME".
func commandLabel(name string) string {
	words := strings.Fields(name)
	if internalEntrypoint != nil && len(words) > 0 && internalEntrypoint(words) {
		return "metasystem internal " + name
	}
	return "metasystem " + name
}

// internalEntrypoint reports whether a flag set's words name an internal
// entrypoint. It is bound in init: the registry refers to every handler, and
// the handlers build flag sets.
var internalEntrypoint func(words []string) bool

func init() {
	publicCommand = func(name string) (intentCommand, bool) {
		command, ok := findIntentCommand(name)
		return command, ok && !command.hidden
	}
	internalEntrypoint = func(words []string) bool {
		for _, entry := range topLevelEntries() {
			if entry.name == words[0] {
				return true
			}
		}
		if len(words) < 2 {
			return false
		}
		for _, fam := range families() {
			if fam.name != words[0] {
				continue
			}
			for _, v := range fam.verbs {
				if v.name == words[1] {
					return true
				}
			}
		}
		return false
	}
}

// requireFlags answers a missing required option by name on errs before
// anything is done; it reports whether every named option was given.
func requireFlags(flags *flag.FlagSet, errs io.Writer, names ...string) bool {
	given := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { given[f.Name] = true })
	for _, name := range names {
		if !given[name] {
			fmt.Fprintf(errs, "%s: --%s is required; nothing was done\n", cliflags.Label(flags), name)
			return false
		}
	}
	return true
}

// refuseUnknownOption answers an option an entrypoint with its own parser
// does not take, in the same public style as newFlagSet: a help request's
// usage on stdout, a refusal on stderr.
func refuseUnknownOption(stdout, stderr io.Writer, name, option, takes string) int {
	if isHelpWord(option) {
		fmt.Fprintf(stdout, "usage: %s: %s\n", commandLabel(name), takes)
		return 0
	}
	fmt.Fprintf(stderr, "%s: does not take %s; %s; nothing was done\n", commandLabel(name), option, takes)
	return 2
}
