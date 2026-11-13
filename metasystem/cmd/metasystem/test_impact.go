package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch/goadapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathpattern"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

func runTestImpact(args []string, stdout, stderr io.Writer) (code int) {
	only, _ := landingOnly()
	return runTestImpactSelection(args, stdout, stderr, only)
}

func runTestImpactSelection(args []string, stdout, stderr io.Writer, only string) int {
	return (impactSources{plain.Git, goadapter.UnitImpact, os.Environ(), config.Get, os.Getenv}).run(args, stdout, stderr, only)
}

func (sources impactSources) run(args []string, stdout, stderr io.Writer, only string) (code int) {
	invocation := testRunInvocation{stdout: stdout, stderr: stderr, name: "test impact"}
	finish := invocation.envelope(invocation.name, args)
	defer func() { finish(code) }()
	stdout = invocation.stdout
	flags := newFlagSet("test impact", stdout, stderr)
	flags.Bool("json", false, "print the plan as a JSON result")
	output := plain.ImpactPlan{Selections: []string{}}
	base := flags.String("base", sources.getenv("LANDING_PROOF_BASE"), "the unit's comparison base")
	check := flags.Bool("check", false, "check a unit at impact depth only when cheap")
	publicationOnly := flags.Bool("publication-only", false, "run only the publication requirements")
	plan := flags.Bool("plan", false, "print the selection without running it")
	root := pathFlag(flags, "root", ".", "the Go module installation")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	if *publicationOnly {
		only = ""
	}
	replay := strings.TrimSpace(only) != ""
	if !replay && !*publicationOnly && *base == "" {
		fmt.Fprintln(stderr, "the unit check needs its comparison base\nrun: metasystem test impact --base COMMIT # or set LANDING_PROOF_BASE")
		return 2
	}
	selectionPath := sources.getenv("METASYSTEM_CHECK_SELECTION")
	selection := launch.CheckSelection{Mode: "impact", Settings: map[string]string{}}
	var results []proofrun.NamedGroupResult
	var description string
	var environment []string
	installation, contract, _, err := testrun.LoadContract(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		fmt.Fprintln(stdout, "LANDING-NOT-RUN\tenvironment")
		return 1
	}
	fullContract := contract
	units := map[string]string{}
	if selectionPath != "" {
		if err := os.Remove(selectionPath); err != nil && !os.IsNotExist(err) {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer func() {
			if code == 0 && results != nil {
				selection.Base, selection.NotRunnable = output.Base, output.NotRunnable
				if output.Check != "" {
					selection.Mode = output.Check
				}
				if err := writeImpactSelection(selectionPath, installation, description, selection, contract, fullContract, results, units, environment); err != nil {
					fmt.Fprintln(stderr, err)
					code = 1
				}
			}
		}()
	}
	started := time.Now()
	identity := repairCheck{Reason: "impact"}
	cacheable := selectionPath == "" && *check && !*plan && !replay && !*publicationOnly
	timed := false
	finishTiming := func() {
		if !*plan && !timed {
			fmt.Fprintf(stdout, "impact check: %s; wall time %s\n", identity.Reason, time.Since(started).Round(time.Millisecond))
			timed = true
		}
	}
	defer finishTiming()
	if !*plan {
		defer func() {
			fix, err := plain.ReadFix(installation)
			if err == nil && fix != nil {
				fix.CheckMinutes = time.Since(started).Minutes()
				fix.CheckResult = plain.Green
				if output.Check == "static-only" {
					fix.CheckResult = "static-only"
				}
				if code != 0 {
					fix.CheckResult = plain.Red
				}
				err = plain.WriteFix(installation, fix)
			}
			if err != nil {
				fmt.Fprintln(stderr, err)
				code = 1
			}
		}()
	}

	if cacheable {
		identity, err = repairCheckIdentity(installation, *base, sources.git)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer func() {
			if code != 0 || identity.Evidence == nil {
				return
			}
			after, err := repairCheckIdentity(installation, *base, sources.git)
			if err != nil {
				fmt.Fprintln(stderr, err)
				code = 1
				return
			}
			if after.Tree != identity.Tree {
				fmt.Fprintln(stdout, "check not saved: the staged tree changed during the check")
				return
			}
			identity.DurationMS = time.Since(started).Milliseconds()
			identity.Minutes = time.Since(started).Minutes()
			identity.Evidence.Content.Before, identity.Evidence.Content.After = identity.Tree, after.Tree
			identity.Evidence.Strength.Cheap, identity.Evidence.Strength.Complete = true, true
			if _, err := repairCheckCache(installation, identity, true); err != nil {
				fmt.Fprintln(stderr, err)
				code = 1
			}
		}()
	}
	var required []string
	if !replay {
		required, err = publicationRequirements(contract)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	prefix := ""
	if config.TemplateMode(installation) {
		prefix = "metasystem/"
	}
	ids := []string{}
	selections := strings.Fields(only)
	if replay {
		selection.BaseReason = "replay"
		fmt.Fprintln(stdout, "note: replay selections ignore the comparison base from flags and environment")
	} else if !*publicationOnly {
		sha, err := sources.git(installation, "rev-parse", "--verify", *base+"^{commit}")
		if err != nil {
			fmt.Fprintf(stderr, "the unit check's base %s is not a commit in this repository\nrun: metasystem test impact --base COMMIT  (a commit the branch contains)\n%v\n", *base, err)
			return 2
		}
		if _, err := sources.git(installation, "merge-base", "--is-ancestor", sha, "HEAD"); err != nil {
			fmt.Fprintf(stderr, "the unit check's base %s is not an ancestor of HEAD; nothing was selected\nrun: metasystem test impact --base <the round's base>\n", sha)
			return 2
		}
		subject, err := sources.git(installation, "show", "-s", "--format=%s", sha)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		output.Base, output.Subject = sha, subject
		if selectionPath != "" || cacheable {
			selection.BaseTree, err = sources.git(installation, "rev-parse", sha+"^{tree}")
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
		}
		fmt.Fprintf(stdout, "plan: base %s (%s)\n", sha, subject)
		impact, err := sources.impact(installation, sha, sources.getenv("LANDING_PROOF_SCOPE") == "impact")
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		output.NotRunnable = impact.NotRunnable
		for i := range impact.Paths {
			impact.Paths[i] = prefix + impact.Paths[i]
		}
		affected, err := testpolicy.Affected(contract, impact.Paths)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		wanted := map[string]bool{}
		reads := map[string]bool{}
		for _, group := range affected.Groups {
			reads[group.Group.ID] = len(group.Paths) > 0
			for _, path := range group.Paths {
				if !slices.Contains(affected.TemplateCovered, path) {
					wanted[group.Group.ID] = true
					break
				}
			}
		}
	groups:
		for _, group := range contract.Groups {
			if slices.Contains(required, group.ID) {
				continue
			}
			canary := slices.Contains(contract.Always.Canary, group.ID)
			audit := auditGroup(contract, group) && reads[group.ID]
			if strings.HasPrefix(group.ID, "section/") || !canary && !audit && (group.Phase == "acceptance" || !wanted[group.ID]) {
				continue
			}
			for _, input := range group.Inputs {
				pattern, _ := pathpattern.Parse(input)
				if !audit && pattern.Covers(prefix+"cmd") {
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
		if impact.NotRunnable > 0 {
			fmt.Fprintf(stdout, "not runnable here: %d selected tests are in files Go does not build for %s/%s; not counted missing\n", impact.NotRunnable, impact.GOOS, impact.GOARCH)
		}
	}
	if *publicationOnly && selectionPath != "" {
		output.Base, err = sources.git(installation, "rev-parse", "--verify", *base+"^{commit}")
		if err == nil {
			selection.BaseTree, err = sources.git(installation, "rev-parse", output.Base+"^{tree}")
		}
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
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
			contract.Groups = append(contract.Groups, group)
			ids = append(ids, id)
		}
	}
	if (*check || *plan) && !replay && !*publicationOnly {
		effective := &selection
		var selection strings.Builder
		for _, item := range selections {
			fmt.Fprintln(&selection, "selection: "+item)
		}
		share, cheap, full, _, err := impactCost(installation, installation, selection.String(), fullContract, func(p config.GetParams) (string, int, error) {
			value, code, err := sources.get(p)
			// Keep the value from the same read that decides the selection.
			effective.Settings[p.Key] = value
			return value, code, err
		})
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if !cheap {
			audits := slices.DeleteFunc(slices.Clone(ids), func(id string) bool {
				index := slices.IndexFunc(contract.Groups, func(group testpolicy.Group) bool { return group.ID == id })
				return index < 0 || !auditGroup(contract, contract.Groups[index])
			})
			ids = append(slices.Clone(required), audits...)
			output.Check = "static-only"
			identity.Form = "static-only: selection not cheap"
			identity.Reason = fmt.Sprintf("check: fast-static-build only; impact would cover %d%% of %d packages; not cheap: static gate only; the batch proof covers the tree", share, full)
			if len(audits) > 0 {
				suffix := " + audit groups " + strings.Join(audits, ",")
				identity.Form, identity.Reason = identity.Form+suffix, identity.Reason+suffix
			}
		} else {
			output.Check = "impact"
			identity.Form, err = impactSelectionForm(installation, contract, ids, units)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			identity.Reason = fmt.Sprintf("check: impact; covers %d%% of %d packages", share, full)
		}
		if *check {
			output.CheckReason = identity.Reason
			fmt.Fprintln(stdout, identity.Reason)
		}
	}
	if *publicationOnly {
		output.Check, identity.Reason = "publication-only", "publication-only"
	}
	if *plan && !replay {
		fmt.Fprintln(stdout, "required: "+strings.Join(required, " "))
	}
	if *plan && (!replay || invocation.outcome != nil) {
		if invocation.outcome != nil {
			output.Selections = selections
			invocation.outcome.data = output
		}
		return 0
	}
	environment = slices.DeleteFunc(slices.Clone(sources.environment), func(entry string) bool {
		return strings.HasPrefix(entry, "METASYSTEM_CHECK_SELECTION=") || strings.HasPrefix(entry, "LANDING_ONLY=") || !replay && strings.HasPrefix(entry, "LANDING_PROOF_BASE=")
	})
	environment = append(environment, "LANDING_ONLY="+only)
	if !replay {
		environment = append(environment, "LANDING_PROOF_BASE="+output.Base)
	}
	if err := proofrun.WriteLandingEnvironment(stdout, func() (string, error) {
		var err error
		description, err = proofrun.LandingEnvironment(context.Background(), installation, environment)
		return description, err
	}); err != nil {
		fmt.Fprintln(stderr, err)
		finishTiming()
		fmt.Fprintln(stdout, "LANDING-NOT-RUN\tenvironment")
		return 1
	}
	if !replay {
		ids = append(slices.Clone(required), slices.DeleteFunc(ids, func(id string) bool { return slices.Contains(required, id) })...)
	}
	if len(ids) == 0 {
		finishTiming()
		fmt.Fprintln(stdout, "LANDING-CHECKED\t0")
		return 0
	}
	if cacheable {
		closure, problem := testpolicy.WithPrerequisiteClosure(contract, testpolicy.Plan{SelectedGroups: ids, Stages: []testpolicy.Stage{{ID: "impact", Groups: ids}}})
		if problem != nil {
			fmt.Fprintln(stderr, problem)
			return 1
		}
		_, _, workers := publicationCheckEnvironment(launch.UnitCheck{Environment: environment}, output.Base)
		for _, id := range closure.SelectedGroups {
			results = append(results, proofrun.NamedGroupResult{ID: id, Workers: workers})
		}
		identity.Evidence = &launch.CheckEvidence{Version: 1, Manual: true, Source: "test impact " + started.UTC().Format(time.RFC3339Nano)}
		selection.Base, selection.NotRunnable, selection.Mode = output.Base, output.NotRunnable, output.Check
		if err := writeImpactSelection("", installation, description, selection, contract, fullContract, results, units, environment, &identity.Evidence.Selection); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		decision, _ := repairCheckCache(installation, identity, false)
		if decision.Reuse != nil {
			fmt.Fprintf(stdout, "check reused: %s (%s)\n", decision.Reuse.Source, decision.Reuse.Path)
			identity.Evidence = nil
			return 0
		}
		fmt.Fprintln(stdout, "check runs: "+decision.Field+" differs")
	}
	code, results = runNamedTestGroups(installation, contract, ids, environment, units, stdout, stderr, finishTiming)
	if cacheable && code == 0 {
		if err := writeImpactSelection("", installation, description, selection, contract, fullContract, results, units, environment, &identity.Evidence.Selection); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	return code
}

// landingOnly reads the replay selection: the lane's gate always sets
// LANDING_ONLY, empty when it is not replaying, so only a non-empty value
// is a replay.
func landingOnly() (string, bool) {
	only := os.Getenv("LANDING_ONLY")
	return only, strings.TrimSpace(only) != ""
}

func auditGroup(contract testpolicy.Contract, group testpolicy.Group) bool {
	return group.Phase == "acceptance" && slices.Contains(contract.Always.Standard, group.ID) && !slices.ContainsFunc(group.Packages, func(pkg string) bool {
		return strings.TrimPrefix(strings.TrimPrefix(pkg, "./"), "metasystem/") == "cmd/metasystem"
	})
}

func impactDepth(plan string, contract testpolicy.Contract, full map[string]bool, limit int) (int, bool) {
	selected := map[string]bool{}
	whole := false
	addPackage := func(pkg string, named bool) {
		pkg = strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(pkg, "metasystem/"), "unit/"), "./")
		prefix, recursive := strings.CutSuffix(pkg, "/...")
		for key := range full {
			if !strings.HasPrefix(key, "group/") && (key == pkg || pkg == "..." || recursive && (key == prefix || strings.HasPrefix(key, prefix+"/"))) {
				selected[key] = true
				if !named && key == "cmd/metasystem" {
					whole = true
				}
			}
		}
	}
	for _, line := range strings.Split(plan, "\n") {
		selection, ok := strings.CutPrefix(line, "selection: ")
		if !ok || slices.Contains(contract.Always.Canary, selection) {
			continue
		}
		unit, names, named := strings.Cut(selection, "=")
		if named && names == "" {
			continue
		}
		index := slices.IndexFunc(contract.Groups, func(group testpolicy.Group) bool { return selection == group.ID })
		if index < 0 {
			addPackage(unit, named)
			continue
		}
		group := contract.Groups[index]
		if auditGroup(contract, group) {
			continue
		}
		if group.Adapter != "go" {
			selected["group/"+group.ID] = true
			continue
		}
		if group.PackageSelection != "" {
			addPackage("...", false)
		}
		for _, pkg := range group.Packages {
			addPackage(pkg, false)
		}
	}
	if len(full) == 0 {
		return 100, false
	}
	share := len(selected) * 100 / len(full)
	return share, !whole && len(selected)*100 < limit*len(full)
}

func depthSetting(install, key string, readers ...func(config.GetParams) (string, int, error)) (int, error) {
	get := config.Get
	if len(readers) > 0 {
		get = readers[0]
	}
	value, _, err := get(config.GetParams{ConfPath: filepath.Join(install, "metasystem.conf"), Key: key})
	if n, numberErr := strconv.Atoi(value); err == nil && numberErr == nil && n >= 1 {
		return n, nil
	}
	return 0, fmt.Errorf("%s must be a positive integer", key)
}

func fullPackageMeasure(install string, contract testpolicy.Contract) (map[string]bool, error) {
	packages := map[string]bool{}
	tags := map[string][]string{"": nil}
	for _, group := range contract.Groups {
		if group.Adapter != "go" {
			if !slices.Contains(contract.Always.Canary, group.ID) {
				packages["group/"+group.ID] = true
			}
		} else {
			tags[strings.Join(group.BuildTags, ",")] = group.BuildTags
		}
	}
	for _, buildTags := range tags {
		paths, err := goadapter.TestPackagePaths(install, buildTags)
		if err != nil {
			return nil, err
		}
		for _, pkg := range paths {
			packages[pkg] = true
		}
	}
	return packages, nil
}

func impactCost(install, module, plan string, contract testpolicy.Contract, readers ...func(config.GetParams) (string, int, error)) (int, bool, int, string, error) {
	packages, err := fullPackageMeasure(module, contract)
	full := len(packages)
	if err != nil {
		return 0, false, 0, "", err
	}
	limit, err := depthSetting(install, "landing.impact-max-share", readers...)
	share, cheap := impactDepth(plan, contract, packages, limit)
	reason := ""
	if err == nil && full > 0 && !cheap && share < limit {
		reason = "change selects cmd/metasystem whole; the batch check covers it"
	}
	return share, cheap, full, reason, err
}

func batchDepthSeams(seams plain.ProveSeams) plain.ProveSeams {
	if seams.BatchDepth == nil {
		seams.BatchDepth = func(install, checkout string, running plain.Running) (string, string) {
			impact, reason := batchDepth(install, checkout, running.Commit, seams)
			if reason == "" {
				return "", ""
			}
			if impact {
				return "impact", reason
			}
			return "full", reason
		}
	}
	return seams
}

func batchDepth(install, checkout, commit string, seams plain.ProveSeams) (bool, string) {
	batch, err := plain.ReadBatch(install)
	if err != nil || batch == nil || batch.State == plain.BatchClosed {
		return false, ""
	}
	fallback := func(err error) (bool, string) {
		if os.IsNotExist(err) {
			return false, ""
		}
		return false, "depth cannot be selected: " + err.Error() + "; full"
	}
	threshold, err := depthSetting(install, "landing.full-from-tier")
	if err != nil {
		return fallback(err)
	}
	tiers := []string{}
	highest := 0
	unset := false
	for _, member := range batch.Members {
		data, err := os.ReadFile(filepath.Join(install, "plans", "goals", member.Goal+".md"))
		if err != nil {
			return fallback(err)
		}
		file, problems := goal.ParseFile(data)
		if len(problems) > 0 {
			return fallback(fmt.Errorf("goal %s is invalid: %v", member.Goal, problems))
		}
		tier := int(goal.GateTier(file))
		tiers = append(tiers, strconv.Itoa(tier))
		highest = max(highest, tier)
		unset = unset || file.Tier == 0
	}
	if highest >= threshold {
		if unset && highest == 3 {
			return false, "tier 3 (unset): full"
		}
		return false, fmt.Sprintf("tier %d: full", highest)
	}
	git := seams.Git
	if git == nil {
		git = plain.Git
	}
	scratch, err := diskstore.ProcessScratch()
	if err != nil {
		return fallback(err)
	}
	tree, err := os.MkdirTemp(scratch, "metasystem-depth-")
	if err != nil {
		return fallback(err)
	}
	defer os.RemoveAll(tree)
	if _, err = git(checkout, "worktree", "add", "--detach", tree, commit); err != nil {
		return fallback(err)
	}
	defer git(checkout, "worktree", "remove", "--force", tree)
	relative, err := filepath.Rel(checkout, install)
	if err != nil {
		return fallback(err)
	}
	dir := filepath.Join(tree, relative)
	_, contract, _, err := testrun.LoadContract(dir)
	if err != nil {
		return fallback(err)
	}
	plan, err := plain.ReadImpactPlan(seams, dir, batch.Base, "impact")
	if err != nil {
		return fallback(err)
	}
	share, cheap, full, reason, err := impactCost(install, dir, plan, contract)
	if err != nil {
		return fallback(err)
	}
	if !cheap {
		if reason != "" {
			return false, reason
		}
		return false, fmt.Sprintf("impact would cover %d%% of %d packages: full", share, full)
	}
	return true, fmt.Sprintf("tiers %s; %d%% of %d packages", strings.Join(tiers, ","), share, full)
}

type repairCheck struct {
	Tree, Base, Reason string
	Form               string
	DurationMS         int64
	Minutes            float64
	AuditMinutes       float64
	Evidence           *launch.CheckEvidence
}

func repairCheckIdentity(install, base string, runners ...func(string, ...string) (string, error)) (repairCheck, error) {
	git := plain.Git
	if len(runners) != 0 {
		git = runners[0]
	}
	base, err := git(install, "rev-parse", "--verify", base+"^{commit}")
	if err != nil {
		return repairCheck{}, err
	}
	tree, err := git(install, "write-tree")
	if err != nil {
		return repairCheck{}, err
	}
	return repairCheck{Tree: tree, Base: base}, nil
}

func repairCheckCache(install string, check repairCheck, save bool) (launch.CheckDecision, error) {
	if check.Tree == "" {
		return launch.CheckDecision{Field: "tree"}, nil
	}
	path := filepath.Join(plain.Dir(install), "repair-check.json")
	if save {
		data, _ := json.Marshal(check)
		err := os.MkdirAll(filepath.Dir(path), 0700)
		if err == nil {
			_, err = atomicfile.WriteFile(path, data, 0600, plain.Dir(install))
		}
		return launch.CheckDecision{}, err
	}
	data, err := os.ReadFile(path)
	var previous repairCheck
	if err == nil {
		err = json.Unmarshal(data, &previous)
	}
	var current launch.CheckSelection
	if check.Evidence != nil {
		current = check.Evidence.Selection
	}
	return launch.ApplicableCheck(previous.Evidence, path, err, launch.CheckNeed{Tree: check.Tree, Base: check.Base, Strength: "cheap", Context: current, Requirements: current.Groups}), nil
}

// committedImpactCheck retains the impact decision made for this unit's candidate.
func committedImpactCheck(directory, base string, stdout, stderr io.Writer, flags ...string) (repairCheck, error) {
	if code := runTestImpactSelection(append(flags, "--root", directory, "--base", base, "--check"), stdout, stderr, ""); code != 0 {
		return repairCheck{}, fmt.Errorf("the unit impact check exited %d", code)
	}
	data, err := os.ReadFile(filepath.Join(plain.Dir(directory), "repair-check.json"))
	var checked repairCheck
	if err == nil {
		err = json.Unmarshal(data, &checked)
	}
	return checked, err
}

func impactSelectionForm(directory string, contract testpolicy.Contract, ids []string, units map[string]string, discovered ...map[string][]string) (string, error) {
	tests, packages := 0, 0
	var groups []string
	for _, group := range contract.Groups {
		if !slices.Contains(ids, group.ID) {
			continue
		}
		if units[group.ID] == "" {
			groups = append(groups, group.ID)
			continue
		}
		packages += len(group.Packages)
		var names []string
		if json.Unmarshal(group.Tests, &names) != nil {
			for _, path := range group.Packages {
				dir := filepath.Join(directory, path)
				pkg, err := build.Default.ImportDir(dir, 0)
				if err != nil {
					return "", err
				}
				for _, file := range append(pkg.TestGoFiles, pkg.XTestGoFiles...) {
					parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, file), nil, 0)
					if err != nil {
						return "", err
					}
					for _, decl := range parsed.Decls {
						if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && testpolicy.GoTestName(fn.Name.Name) {
							names = append(names, fn.Name.Name)
						}
					}
				}
			}
		}
		tests += len(names)
		slices.Sort(names)
		if len(discovered) > 0 {
			discovered[0][units[group.ID]] = names
		}
	}
	form := fmt.Sprintf("impact: %d tests of %d packages", tests, packages)
	if len(groups) > 0 {
		form += " + groups " + strings.Join(groups, ", ")
	}
	return form, nil
}

type impactSources struct {
	git         func(string, ...string) (string, error)
	impact      func(string, string, bool) (goadapter.Impact, error)
	environment []string
	get         func(config.GetParams) (string, int, error)
	getenv      func(string) string
}

func writeImpactSelection(path, installation, description string, selection launch.CheckSelection, contract, full testpolicy.Contract, results []proofrun.NamedGroupResult, units map[string]string, environment []string, capture ...*launch.CheckSelection) error {
	selection.Version, selection.VerdictPolicyVersion = 1, 1
	selection.Groups, selection.GroupDigests = []string{}, map[string]string{}
	selection.PackageTests, selection.BuildTags = map[string][]string{}, map[string][]string{}
	data, _ := json.Marshal(full)
	selection.ContractDigest = bytesSHA256(data)
	environment = slices.DeleteFunc(slices.Clone(environment), func(entry string) bool {
		value, found := strings.CutPrefix(entry, launch.CheckEnvironmentVariable+"=")
		if found {
			selection.EnvironmentSHA256 = value
		}
		return found
	})
	if selection.EnvironmentSHA256 == "" {
		selection.EnvironmentSHA256 = bytesSHA256([]byte(strings.Join(environment, "\x00")))
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	selection.EngineDigest, err = fileSHA256(executable)
	if err != nil {
		return err
	}
	selection.GoVersion, _, _ = strings.Cut(description, "; ")
	if _, err := fmt.Sscanf(strings.TrimPrefix(description, selection.GoVersion+"; "), "GOOS=%q; GOARCH=%q;", &selection.GOOS, &selection.GOARCH); err != nil {
		return err
	}
	for _, result := range results {
		selection.Groups = append(selection.Groups, result.ID)
		selection.Workers = result.Workers
	}
	for _, group := range contract.Groups {
		if !slices.Contains(selection.Groups, group.ID) {
			continue
		}
		data, _ := json.Marshal(group)
		selection.GroupDigests[group.ID] = bytesSHA256(data)
		selection.BuildTags[group.ID] = group.BuildTags
	}
	if _, err := impactSelectionForm(installation, contract, selection.Groups, units, selection.PackageTests); err != nil {
		return err
	}
	if len(capture) != 0 {
		*capture[0] = selection
	}
	if path != "" {
		return selection.Write(path)
	}
	return nil
}
