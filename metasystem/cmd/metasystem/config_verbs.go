package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// runConfigGet resolves one configuration key through the full precedence order
// (explicit flag, environment, .local override, mode-scoped key, committed key,
// default) and prints the value. Exit 2 marks an invalid key or mode; exit 1 a
// missing value or a malformed source.
func runConfigGet(args []string) int {
	flags := flag.NewFlagSet("config get", flag.ContinueOnError)
	key := flags.String("key", "", "configuration key to resolve")
	mode := flags.String("mode", "", "mode scope for role runtime/model keys")
	role := flags.String("role", "", "reserved role scope (overrides are scoped by mode only)")
	flagVal := flags.String("flag", "", "explicit override value; wins over every source")
	def := flags.String("default", "", "value to use when no source holds the key")
	conf := flags.String("conf", "metasystem.conf", "path to metasystem.conf")
	if flags.Parse(args) != nil {
		return 2
	}
	params := config.GetParams{
		Key:      *key,
		Mode:     *mode,
		Role:     *role,
		Flag:     *flagVal,
		Default:  *def,
		ConfPath: *conf,
	}
	flags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "flag":
			params.FlagSet = true
		case "default":
			params.DefaultSet = true
		}
	})
	value, code, err := config.Get(params)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return code
	}
	fmt.Println(value)
	return code
}

// runConfigValidate validates the whole metasystem.conf domain against the
// repository, printing each problem and exiting non-zero when any is found.
func runConfigValidate(args []string) int {
	flags := flag.NewFlagSet("config validate", flag.ContinueOnError)
	conf := flags.String("conf", "metasystem.conf", "path to metasystem.conf")
	repo := flags.String("repo", ".", "repository root the configuration is validated against")
	if flags.Parse(args) != nil {
		return 2
	}
	tiersAbsent, problems, err := config.Validate(*conf, *repo)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, problem := range problems {
		fmt.Fprintf(os.Stderr, "invalid metasystem configuration: %s\n", problem)
	}
	if tiersAbsent {
		fmt.Println("INFO: model tiers are absent; dispatch overrides therefore always escalate")
	}
	if len(problems) > 0 {
		return 1
	}
	return 0
}

// runConfigKeys enumerates configured keys under the --matching prefix, one per
// line, unioning the committed file, its .local sibling, and numeric-suffix
// members named only in the environment.
func runConfigKeys(args []string) int {
	flags := flag.NewFlagSet("config keys", flag.ContinueOnError)
	conf := flags.String("conf", "metasystem.conf", "path to metasystem.conf")
	matching := flags.String("matching", "", "enumerate keys starting with this prefix")
	if flags.Parse(args) != nil {
		return 2
	}
	for _, key := range config.Keys(*conf, *matching, os.Environ()) {
		fmt.Println(key)
	}
	return 0
}

// runConfigConfValue reads a single value from a metasystem.conf-format file.
// Exit 0 prints the value; exit 3 means the key is absent; exit 1 means the
// file is unreadable or the key appears more than once.
func runConfigConfValue(args []string) int {
	flags := flag.NewFlagSet("config conf-value", flag.ContinueOnError)
	file := flags.String("file", "", "path to a metasystem.conf-format file")
	key := flags.String("key", "", "key to read")
	if flags.Parse(args) != nil {
		return 2
	}
	if *file == "" || *key == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem config conf-value --file F --key K")
		return 2
	}
	value, found, err := config.ConfLookup(*file, *key)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !found {
		return 3
	}
	fmt.Println(value)
	return 0
}
