package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathpattern"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

func runTestImpact(args []string, stdout, stderr io.Writer) (code int) {
	invocation := testRunInvocation{stdout: stdout, stderr: stderr, name: "test impact"}
	finish := invocation.envelope(invocation.name, args)
	defer func() { finish(code) }()
	stdout = invocation.stdout
	flags := newFlagSet("test impact", stdout, stderr)
	flags.Bool("json", false, "print the plan as a JSON result")
	output := plain.ImpactPlan{Selections: []string{}}
	base := flags.String("base", os.Getenv("LANDING_PROOF_BASE"), "the unit's comparison base")
	check := flags.Bool("check", false, "check a repair at impact depth only when cheap; reuse its unchanged staged tree")
	plan := flags.Bool("plan", false, "print the selection without running it")
	root := pathFlag(flags, "root", ".", "the Go module installation")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	only, replay := landingOnly()
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
	started := time.Now()
	identity := repairCheck{Reason: "impact"}
	timed := false
	reusedCheck := false
	finishTiming := func() {
		if !*plan && !timed {
			fmt.Fprintf(stdout, "impact check: %s; wall time %s\n", identity.Reason, time.Since(started).Round(time.Millisecond))
			timed = true
		}
	}
	defer finishTiming()
	if !*plan {
		defer func() {
			if reusedCheck {
				return
			}
			fix, err := plain.ReadFix(installation)
			if err == nil && fix != nil {
				fix.CheckMinutes = time.Since(started).Minutes()
				fix.CheckResult = plain.Green
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

	if *check && !*plan && !replay {
		identity, err = repairCheckIdentity(installation, *base)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if reused, _ := repairCheckCache(installation, identity, false); reused {
			reusedCheck = true
			identity.Reason = "check reused from the job"
			fmt.Fprintln(stdout, identity.Reason)
			return 0
		}
		defer func() {
			if code != 0 {
				return
			}
			after, err := repairCheckIdentity(installation, *base)
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
			if _, err := repairCheckCache(installation, identity, true); err != nil {
				fmt.Fprintln(stderr, err)
				code = 1
			}
		}()
	}
	prefix := ""
	if config.TemplateMode(installation) {
		prefix = "metasystem/"
	}
	fullContract := contract
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
		output.Base, output.Subject = sha, subject
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
			contract.Groups = append(contract.Groups, group)
			ids = append(ids, id)
		}
	}
	if *check && !replay {
		var selection strings.Builder
		for _, item := range selections {
			fmt.Fprintln(&selection, "selection: "+item)
		}
		share, cheap, full, _, err := impactCost(installation, installation, selection.String(), fullContract)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if !cheap {
			ids = []string{"fast-static-build"}
			identity.Reason = fmt.Sprintf("check: fast-static-build only; impact would cover %d%% of %d packages", share, full)
		} else {
			identity.Reason = fmt.Sprintf("check: impact; covers %d%% of %d packages", share, full)
		}
		fmt.Fprintln(stdout, identity.Reason)
	}
	if *plan && (!replay || invocation.outcome != nil) {
		if invocation.outcome != nil {
			output.Selections = selections
			invocation.outcome.data = output
		}
		return 0
	}
	environment := os.Environ()
	if err := proofrun.WriteLandingEnvironment(stdout, func() (string, error) {
		return proofrun.LandingEnvironment(context.Background(), installation, environment)
	}); err != nil {
		fmt.Fprintln(stderr, err)
		finishTiming()
		fmt.Fprintln(stdout, "LANDING-NOT-RUN\tenvironment")
		return 1
	}
	if !replay && os.Getenv("LANDING_PROOF_SCOPE") == "impact" {
		ids = append([]string{"fast-static-build"}, slices.DeleteFunc(ids, func(id string) bool { return id == "fast-static-build" })...)
	}
	if len(ids) == 0 {
		finishTiming()
		fmt.Fprintln(stdout, "LANDING-CHECKED\t0")
		return 0
	}
	return runNamedTestGroups(installation, contract, ids, environment, units, stdout, stderr, finishTiming)
}

// landingOnly reads the replay selection: the lane's gate always sets
// LANDING_ONLY, empty when it is not replaying, so only a non-empty value
// is a replay.
func landingOnly() (string, bool) {
	only := os.Getenv("LANDING_ONLY")
	return only, strings.TrimSpace(only) != ""
}

func impactDepth(plan string, contract testpolicy.Contract, full map[string]bool, limit int) (int, bool) {
	selected := map[string]bool{}
	addPackage := func(pkg string) {
		pkg = strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(pkg, "metasystem/"), "unit/"), "./")
		prefix, recursive := strings.CutSuffix(pkg, "/...")
		for key := range full {
			if !strings.HasPrefix(key, "group/") && (key == pkg || pkg == "..." || recursive && (key == prefix || strings.HasPrefix(key, prefix+"/"))) {
				selected[key] = true
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
			addPackage(unit)
			continue
		}
		group := contract.Groups[index]
		if group.Adapter != "go" {
			selected["group/"+group.ID] = true
			continue
		}
		if group.PackageSelection != "" {
			addPackage("...")
		}
		for _, pkg := range group.Packages {
			addPackage(pkg)
		}
	}
	if len(full) == 0 {
		return 100, false
	}
	share := len(selected) * 100 / len(full)
	return share, !selected["cmd/metasystem"] && len(selected)*100 < limit*len(full)
}

func depthSetting(install, key string) (int, error) {
	value, _, err := config.Get(config.GetParams{ConfPath: filepath.Join(install, "metasystem.conf"), Key: key})
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

func impactCost(install, module, plan string, contract testpolicy.Contract) (int, bool, int, string, error) {
	packages, err := fullPackageMeasure(module, contract)
	full := len(packages)
	if err != nil {
		return 0, false, 0, "", err
	}
	limit, err := depthSetting(install, "landing.impact-max-share")
	share, cheap := impactDepth(plan, contract, packages, limit)
	reason := ""
	if err == nil && full > 0 && !cheap && share < limit {
		reason = "change selects cmd/metasystem: full"
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
	plan, err := plain.ReadImpactPlan(seams, dir, batch.Base)
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
	DurationMS         int64
}

func repairCheckIdentity(install, base string) (repairCheck, error) {
	base, err := plain.Git(install, "rev-parse", "--verify", base+"^{commit}")
	if err != nil {
		return repairCheck{}, err
	}
	tree, err := plain.Git(install, "write-tree")
	if err != nil {
		return repairCheck{}, err
	}
	return repairCheck{Tree: tree, Base: base}, nil
}

func repairCheckCache(install string, check repairCheck, save bool) (bool, error) {
	if check.Tree == "" {
		return false, nil
	}
	path := filepath.Join(plain.Dir(install), "repair-check.json")
	if save {
		data, _ := json.Marshal(check)
		err := os.MkdirAll(filepath.Dir(path), 0700)
		if err == nil {
			_, err = atomicfile.WriteFile(path, data, 0600, plain.Dir(install))
		}
		return false, err
	}
	data, err := os.ReadFile(path)
	var previous repairCheck
	return err == nil && json.Unmarshal(data, &previous) == nil && previous.Tree == check.Tree && previous.Base == check.Base, nil
}
