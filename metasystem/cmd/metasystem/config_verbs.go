package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	runtimereg "github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// repeatedFlag collects every occurrence of a flag that may be given
// more than once.
type repeatedFlag []string

func (r *repeatedFlag) String() string { return strings.Join(*r, ",") }

func (r *repeatedFlag) Set(value string) error {
	*r = append(*r, value)
	return nil
}

// runConfigTailor rewrites a metasystem.conf in place for the selected
// runtime set: the runtime list becomes durable state, unselected
// runtimes lose their role and mode bindings, per-runtime model keys,
// and model-tier members, and the default runtime is set. --set
// key=value overrides (applied after tailoring, so they win) replace or
// append individual keys. Exit 2 marks bad flags; exit 1 a failed
// rewrite.
func runConfigTailor(args []string) int {
	flags := flag.NewFlagSet("config tailor", flag.ContinueOnError)
	conf := flags.String("conf", "", "path to the metasystem.conf to rewrite")
	runtimes := flags.String("runtimes", "", "comma-separated selected runtimes, or none")
	testingContract := flags.String("testing-contract", "", "write an explicit incomplete first-adoption testing contract")
	var sets repeatedFlag
	flags.Var(&sets, "set", "key=value to set after tailoring (repeatable)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *conf == "" || *runtimes == "" {
		fmt.Fprintf(os.Stderr, "usage: metasystem internal config tailor --conf F --runtimes %s|none [--set key=value ...]\n",
			strings.Join(runtimereg.Names(), ","))
		return 2
	}
	selected := strings.Split(*runtimes, ",")
	seen := map[string]bool{}
	for _, runtime := range selected {
		switch {
		case runtime == "none":
		case runtimereg.Supported(runtime):
		default:
			fmt.Fprintf(os.Stderr, "unknown runtime: %s (%s, or none)\n",
				runtime, strings.Join(runtimereg.Names(), ", "))
			return 2
		}
		if seen[runtime] {
			fmt.Fprintln(os.Stderr, "--runtimes contains a duplicate runtime")
			return 2
		}
		seen[runtime] = true
	}
	if seen["none"] && len(selected) > 1 {
		fmt.Fprintln(os.Stderr, "--runtimes none cannot be combined with other runtimes")
		return 2
	}
	var settings []validate.ConfSetting
	for _, assignment := range sets {
		key, value, found := strings.Cut(assignment, "=")
		if !found || strings.TrimSpace(key) == "" {
			fmt.Fprintf(os.Stderr, "--set needs key=value, got: %s\n", assignment)
			return 2
		}
		key = strings.TrimSpace(key)
		settings = append(settings, validate.ConfSetting{Key: key, Value: value})
	}
	if err := validate.TailorConf(*conf, selected); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if len(settings) > 0 {
		if err := validate.SetConfKeys(*conf, settings); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	if *testingContract != "" {
		data, err := testpolicy.IncompleteTemplate()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if err := os.WriteFile(*testingContract, data, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write incomplete testing contract: %v\n", err)
			return 1
		}
	}
	return 0
}

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
