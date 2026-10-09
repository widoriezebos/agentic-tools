package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch/goadapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
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
	only, replay := os.LookupEnv("LANDING_ONLY")
	if !replay && *base == "" {
		fmt.Fprintln(stderr, "the unit check needs its comparison base\nrun: metasystem test impact --base COMMIT # or set LANDING_PROOF_BASE")
		return 2
	}
	installation, contract, _, err := testrun.LoadContract(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		fmt.Fprintln(stdout, "LANDING-NOT-RUN\tenvironment")
		return 1
	}
	prefix := ""
	if config.TemplateMode(installation) {
		prefix = "metasystem/"
	}
	ids := []string{}
	units := map[string]string{}
	selections := strings.Fields(only)
	if replay {
		fmt.Fprintln(stdout, "note: replay selections ignore the comparison base from flags and environment")
	} else {
		sha, err := plain.Git(installation, "rev-parse", "--verify", *base+"^{commit}")
		if err != nil {
			fmt.Fprintf(stderr, "the unit check's base %s is not a commit in this repository\nrun: metasystem test impact --base COMMIT  (a commit the branch contains)\n%v\n", *base, err)
			return 2
		}
		if _, err := plain.Git(installation, "merge-base", "--is-ancestor", sha, "HEAD"); err != nil {
			fmt.Fprintf(stderr, "the unit check's base %s is not an ancestor of HEAD; nothing was selected\nrun: metasystem test impact --base <the round's base>\n", sha)
			return 2
		}
		subject, err := plain.Git(installation, "show", "-s", "--format=%s", sha)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "plan: base %s (%s)\n", sha, subject)
		impact, err := goadapter.UnitImpact(installation, sha)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		for i := range impact.Paths {
			impact.Paths[i] = prefix + impact.Paths[i]
		}
		affected, err := testpolicy.Affected(contract, impact.Paths)
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
			selections = append(selections, group.ID)
		}
		for _, group := range impact.Packages {
			selection := prefix + strings.TrimPrefix(group.ID, "unit/")
			if string(group.Tests) != `"all"` {
				var names []string
				_ = json.Unmarshal(group.Tests, &names)
				selection += "=" + strings.Join(names, ",")
			}
			selections = append(selections, selection)
			ids = append(ids, group.ID)
			units[group.ID] = prefix + strings.TrimPrefix(group.ID, "unit/")
			// The adapter names packages from the module; template contracts run from its parent.
			group.CWD = filepath.Clean(prefix)
			contract.Groups = append(contract.Groups, group)
		}
		for _, selection := range selections {
			fmt.Fprintln(stdout, "selection: "+selection)
		}
	}
	if replay {
		for _, selection := range selections {
			unit, names, named := strings.Cut(selection, "=")
			if slices.ContainsFunc(contract.Groups, func(group testpolicy.Group) bool { return group.ID == selection }) {
				ids = append(ids, selection)
				continue
			}
			pkg := strings.TrimPrefix(unit, "metasystem/")
			tests := json.RawMessage(`"all"`)
			if named {
				tests, _ = json.Marshal(strings.Split(names, ","))
			}
			id := "unit/" + pkg
			units[id] = unit
			group := testpolicy.Group{ID: id, Adapter: "go", CWD: filepath.Clean(prefix), Packages: []string{"./" + pkg}, Tests: tests}
			if pkg == "cmd/metasystem" {
				group.BuildTags = []string{"batchtest"}
			}
			contract.Groups = append(contract.Groups, group)
			ids = append(ids, id)
		}
	}
	if !replay && *plan {
		return 0
	}
	if len(ids) == 0 {
		fmt.Fprintln(stdout, "LANDING-CHECKED\t0")
		return 0
	}
	return runNamedTestGroups(installation, contract, ids, os.Environ(), units, stdout, stderr)
}
