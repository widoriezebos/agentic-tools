package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostsetup"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func runRuntimeSetup(args []string) int {
	flags := flag.NewFlagSet("runtime setup", flag.ContinueOnError)
	repo := flags.String("repo", "", "repository root or a path inside the target installation")
	runtimeCSV := flags.String("runtimes", "", "comma-separated adoptable hosts (default: all)")
	copySkills := flags.Bool("copy-skills", false, "copy skill trees instead of creating relative links")
	check := flags.Bool("check", false, "validate readiness without writing")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" || len(flags.Args()) != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem runtime setup --repo PATH [--runtimes CSV] [--copy-skills] [--check]")
		return 2
	}
	var selected []string
	provided := false
	flags.Visit(func(item *flag.Flag) {
		if item.Name == "runtimes" {
			provided = true
		}
	})
	if provided {
		if *runtimeCSV == "" {
			fmt.Fprintln(os.Stderr, "runtime setup: --runtimes cannot be empty")
			return 2
		}
		selected = strings.Split(*runtimeCSV, ",")
	}
	result, err := hostsetup.Setup(hostsetup.Options{RepositoryPath: *repo, Runtimes: selected, CopySkills: *copySkills, Check: *check})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, runtime := range result.Runtimes {
		fmt.Printf("CONFIG_READY %s repository=%s installation=%s\n", runtime, result.Layout.RepositoryRoot, result.Layout.InstallationRoot)
		fmt.Printf("PROVIDER_TRUST %s configuration is installed; provider trust or restart remains provider-owned\n", runtime)
		fmt.Printf("LIFECYCLE_OBSERVED %s not observed by configuration setup\n", runtime)
	}
	for _, changed := range result.Changed {
		absolute := filepath.Join(result.Layout.RepositoryRoot, filepath.FromSlash(changed))
		fmt.Printf("INSTALLED %s\n", absolute)
	}
	confPath := filepath.Join(result.Layout.InstallationRoot, "metasystem.conf")
	contractRel, present, lookupErr := config.ConfLookup(confPath, "testing.contract")
	switch {
	case lookupErr != nil || !present:
		// Registration remains independently checkable while an older
		// installation is still outside the testing-contract migration.
	case strings.TrimSpace(contractRel) == "":
		fmt.Printf("TEST_CONTRACT_INVALID configuration=%s reason=testing.contract-is-unavailable\n", confPath)
	case filepath.IsAbs(contractRel) || filepath.ToSlash(filepath.Clean(contractRel)) != contractRel || strings.HasPrefix(contractRel, "../"):
		fmt.Printf("TEST_CONTRACT_INVALID configuration=%s reason=testing.contract-is-not-relative-and-normalized\n", confPath)
	default:
		contractPath := filepath.Join(result.Layout.InstallationRoot, filepath.FromSlash(contractRel))
		if _, contractErr := testpolicy.Load(contractPath); contractErr == nil {
			fmt.Printf("TEST_CONTRACT_READY contract=%s\n", contractPath)
		} else if strings.Contains(contractErr.Error(), "TEST_CONTRACT_REQUIRED:") {
			fmt.Printf("TEST_CONTRACT_REQUIRED contract=%s\n", contractPath)
		} else {
			fmt.Printf("TEST_CONTRACT_INVALID contract=%s reason=%s\n", contractPath, strings.ReplaceAll(contractErr.Error(), " ", "-"))
		}
	}
	return 0
}
