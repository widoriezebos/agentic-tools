package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// repeatedFlag collects every occurrence of a flag that may be given
// more than once.
type repeatedFlag []string

func (r *repeatedFlag) String() string { return strings.Join(*r, ",") }

func (r *repeatedFlag) Set(value string) error {
	*r = append(*r, value)
	return nil
}

// runConfigGet resolves one configuration key through the full precedence order
// (explicit flag, environment, .local override, mode-scoped key, committed key,
// default) and prints the value. Exit 2 marks an invalid key or mode; exit 1 a
// missing value or a malformed source.
func runConfigGet(args []string) int {
	flags := newFlagSet("config get")
	key := flags.String("key", "", "configuration key to resolve")
	mode := flags.String("mode", "", "mode scope for role runtime/model keys")
	role := flags.String("role", "", "reserved role scope (overrides are scoped by mode only)")
	flagVal := flags.String("flag", "", "explicit override value; wins over every source")
	def := flags.String("default", "", "value to use when no source holds the key")
	conf := flags.String("conf", "metasystem.conf", "path to metasystem.conf")
	if flags.Parse(args) != nil || !requireFlags(flags, nil, "key") {
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
	flags := newFlagSet("config validate")
	conf := flags.String("conf", "metasystem.conf", "path to metasystem.conf")
	repo := pathFlag(flags, "repo", "", "repository root the configuration is validated against (default: the Git toplevel holding --conf, else its directory)")
	if flags.Parse(args) != nil {
		return 2
	}
	return configValidateTo(os.Stdout, os.Stderr, *conf, *repo)
}

// configValidateTo validates the configuration domain on the caller's
// streams. An empty repo scopes to the configuration's own repository.
func configValidateTo(stdout, stderr io.Writer, conf, repo string) int {
	if repo == "" {
		repo = configRepositoryScope(conf)
	}
	tiersAbsent, problems, err := config.Validate(conf, repo)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	proofRunProblems, err := proofRunConfigProblems(conf)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	problems = append(problems, proofRunProblems...)
	for _, problem := range problems {
		fmt.Fprintf(stderr, "invalid metasystem configuration: %s\n", problem)
	}
	if tiersAbsent {
		fmt.Fprintln(stdout, "INFO: model tiers are absent; dispatch overrides therefore always escalate")
	}
	if len(problems) > 0 {
		return 1
	}
	return 0
}

// configKeysTo enumerates configured keys onto the caller's stream; the
// environment is the one whose numeric-suffix members count.
func configKeysTo(stdout io.Writer, conf, matching string, environment []string) int {
	for _, key := range config.Keys(conf, matching, environment) {
		fmt.Fprintln(stdout, key)
	}
	return 0
}

// configRepositoryScope is the repository a configuration file is validated
// against when none is named: the Git toplevel holding it, resolved, or the
// file's own directory outside Git.
func configRepositoryScope(conf string) string {
	directory := filepath.Dir(conf)
	if absolute, err := filepath.Abs(directory); err == nil {
		directory = absolute
	}
	if top, err := stateroot.RepositoryTop(directory); err == nil && top != "" {
		directory = top
	}
	if resolved, err := filepath.EvalSymlinks(directory); err == nil {
		return resolved
	}
	return directory
}
