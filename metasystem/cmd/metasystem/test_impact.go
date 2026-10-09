package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch/goadapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathpattern"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

func runTestImpact(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("test impact", stdout, stderr)
	base := flags.String("base", os.Getenv("LANDING_PROOF_BASE"), "the unit's comparison base")
	plan := flags.Bool("plan", false, "print the selection without running it")
	root := flags.String("root", ".", "the Go module installation")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	if *base == "" {
		fmt.Fprintln(stderr, "the unit check needs its comparison base\nrun: metasystem test impact --base COMMIT # or set LANDING_PROOF_BASE")
		return 2
	}
	installation, contract, _, err := testrun.LoadContract(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	impact, err := goadapter.UnitImpact(installation, *base, "HEAD")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	prefix := ""
	if config.TemplateMode(installation) {
		prefix = "metasystem/"
	}
	paths := make([]string, len(impact.Paths))
	for i, path := range impact.Paths {
		paths[i] = prefix + path
	}
	affected, err := testpolicy.Affected(contract, paths)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	wanted := map[string]bool{}
	for _, group := range affected.Groups {
		for _, path := range group.Paths {
			if !slices.Contains(affected.TemplateCovered, path) {
				wanted[group.Group.ID] = true
				break
			}
		}
	}
	ids := []string{}
groups:
	for _, group := range contract.Groups {
		canary := slices.Contains(contract.Always.Canary, group.ID)
		if strings.HasPrefix(group.ID, "section/") || !canary && (group.Phase == "acceptance" || !wanted[group.ID]) {
			continue
		}
		for _, input := range group.Inputs {
			pattern, _ := pathpattern.Parse(input)
			if pattern.Covers(prefix + "cmd") {
				continue groups
			}
		}
		ids = append(ids, group.ID)
	}
	fmt.Fprintln(stdout, "groups: "+strings.Join(ids, ", "))
	for _, group := range impact.Packages {
		group.CWD = filepath.Clean(prefix)
		all, names, _ := testpolicy.GoTests(group)
		if all {
			fmt.Fprintf(stdout, "whole: %s\n", group.Packages[0])
		} else {
			fmt.Fprintf(stdout, "by name: %s %d tests (%s)\n", group.Packages[0], len(names), strings.Join(names, ", "))
		}
		contract.Groups = append(contract.Groups, group)
		if !slices.Contains(ids, group.ID) {
			ids = append(ids, group.ID)
		}
	}
	if *plan || len(ids) == 0 {
		return 0
	}
	return runNamedTestGroups(installation, contract, ids, os.Environ(), stdout, stderr)
}
