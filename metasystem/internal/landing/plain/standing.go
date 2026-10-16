package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type standingEvidence struct {
	Attempt     string       `json:"attempt"`
	Commit      string       `json:"commit"`
	Tree        string       `json:"tree"`
	Environment string       `json:"environment"`
	Failures    []FailedUnit `json:"failures"`
	Incidents   []string     `json:"incidents"`
	OwedSince   time.Time    `json:"owedSince"`
	At          time.Time    `json:"at"`
}

type testOutcome struct{ Unit, Test, Status string }

// selectStanding separates open identities from coverage. Only completed,
// classified evidence at unchanged code can suppress another execution.
func selectStanding(install, checkout string, running Running, results []Result, seams ProveSeams) (*standingEvidence, string) {
	seen := map[string]bool{}
	for i := len(results) - 1; i >= 0; i-- {
		red := results[i]
		if seen[red.Attempt] {
			continue
		}
		seen[red.Attempt] = red.Attempt != ""
		if red.didNotRun() || red.Commit == "" || red.Tree == "" || red.Result != Red && (red.Result != Green || red.Scope != "full" || red.FullTree == "") {
			continue
		}
		if _, err := seams.git(checkout, "merge-base", "--is-ancestor", red.Commit, running.Commit); err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) && exit.ExitCode() == 1 {
				continue
			}
			return nil, "the standing check's ancestry cannot be read"
		}
		if red.Result == Green {
			return nil, ""
		}
		if red.ClassificationPending || red.Cause == nil || red.Cause.Kind != "main" {
			return nil, ""
		}
		incidents, err := seams.incidents(install, checkout, running.Commit)
		if err != nil {
			return nil, "the incident register cannot be read: " + err.Error()
		}
		standing := &standingEvidence{Attempt: red.Attempt, Commit: red.Commit, Tree: red.Tree, Environment: red.Environment, OwedSince: red.OwedSince}
		standing.At, err = time.Parse(time.RFC3339, red.At)
		if err != nil {
			return nil, "the standing check time cannot be read"
		}
		if standing.OwedSince.IsZero() {
			origin := red.At
			if red.Scope == "scoped" && red.FullAt != "" {
				origin = red.FullAt
			}
			standing.OwedSince, err = time.Parse(time.RFC3339, origin)
			if err != nil {
				return nil, "the standing check's debt time cannot be read"
			}
		}
		open := false
		for _, unit := range red.Failed {
			names := unit.Tests
			if len(names) == 0 {
				names = []string{""}
			}
			retained := unit
			retained.Tests = nil
			kept := false
			for _, name := range names {
				identity := "red:" + unit.Unit
				if name != "" {
					identity += ":" + name
				}
				entry := slices.IndexFunc(incidents, func(e goal.TrunkRedEntry) bool {
					return e.Identity == identity && e.Closed == nil && e.EntryClass() == goal.TrunkRedClassTrunkRed
				})
				if entry < 0 {
					continue
				}
				open = true
				passed := false
				for j := i + 1; j < len(results); j++ {
					later := results[j]
					if later.didNotRun() || later.Environment != red.Environment {
						continue
					}
					moved, err := codeMoved(install, checkout, red.Commit, later.Commit, seams)
					if err != nil {
						return nil, "the later check's changed paths cannot be read"
					}
					if moved {
						continue
					}
					for _, outcome := range later.Outcomes {
						if outcome.Unit == unit.Unit && outcome.Test == name {
							passed = outcome.Status == "pass"
						}
					}
				}
				if passed {
					continue
				}
				if name != "" {
					retained.Tests = append(retained.Tests, name)
				}
				kept = true
				standing.Incidents = append(standing.Incidents, incidents[entry].ID)
			}
			if kept {
				standing.Failures = append(standing.Failures, retained)
			}
		}
		if open {
			moved, err := codeMoved(install, checkout, red.Commit, running.Commit, seams)
			if err != nil {
				return nil, "the standing check's changed paths cannot be read: " + err.Error()
			}
			if moved {
				return nil, "an executable input changed since the standing red"
			}
			return standing, ""
		}
		return nil, ""
	}
	return nil, ""
}

// resolveFailure maps native package/test identities to concrete selections,
// falling back to the package template only for a package present in the tree.
func resolveFailure(contract testpolicy.Contract, unit FailedUnit, packages []string) ([]string, bool) {
	if len(unit.Tests) > 1 {
		var ids []string
		for _, name := range unit.Tests {
			one := unit
			one.Tests = []string{name}
			resolved, ok := resolveFailure(contract, one, packages)
			if !ok {
				return nil, false
			}
			for _, id := range resolved {
				if !slices.Contains(ids, id) {
					ids = append(ids, id)
				}
			}
		}
		return ids, true
	}

	for _, group := range contract.Groups {
		if group.ID == unit.Unit && group.PackageSelection == "" {
			return []string{group.ID}, true
		}
	}
	pkg, ok := strings.CutPrefix(unit.Unit, "metasystem/")
	if !ok || !slices.Contains(packages, pkg) {
		return nil, false
	}
	var ids []string
	for _, group := range contract.Groups {
		if group.Adapter != "go" || group.PackageSelection != "" {
			continue
		}
		matches := slices.ContainsFunc(group.Packages, func(pattern string) bool {
			cwd := strings.TrimPrefix(strings.TrimPrefix(group.CWD, "metasystem"), "/")
			pattern = path.Join(cwd, strings.TrimPrefix(pattern, "./"))
			return pattern == pkg || pattern == "..." || strings.HasSuffix(pattern, "/...") && (pkg == strings.TrimSuffix(pattern, "/...") || strings.HasPrefix(pkg, strings.TrimSuffix(pattern, "...")))
		})
		all, names, err := testpolicy.GoTests(group)
		if matches && err == nil && (all || len(unit.Tests) == 0 || slices.ContainsFunc(unit.Tests, func(name string) bool { top, _, _ := strings.Cut(name, "/"); return slices.Contains(names, top) })) {
			ids = append(ids, group.ID)
		}
	}
	if len(ids) > 0 {
		return ids, true
	}
	for _, group := range contract.Groups {
		if group.PackageSelection != "" {
			return []string{"unit/" + pkg}, true
		}
	}
	return nil, false
}

func finishStandingScope(install, checkout string, running Running, d scopeDecision, seams ProveSeams) scopeDecision {
	full := func(reason string) scopeDecision {
		return scopeDecision{scopeRecord: scopeRecord{Scope: "full", ScopeReason: reason}}
	}
	files, err := seams.git(checkout, "ls-tree", "-r", "--name-only", running.Tree, "--", "metasystem")
	if err != nil {
		return full("the standing check's packages cannot be read")
	}
	var packages []string
	for _, file := range strings.Fields(files) {
		if strings.HasSuffix(file, ".go") {
			packages = append(packages, strings.TrimPrefix(path.Dir(file), "metasystem/"))
		}
	}
	for _, failure := range d.standing.Failures {
		if _, ok := resolveFailure(d.contract, failure, packages); !ok {
			return full("a standing failure cannot be resolved to a check: " + failure.Unit)
		}
	}
	affected := d.groupIDs()
	plan, err := testpolicy.WithPrerequisiteClosure(d.contract, testpolicy.Plan{SelectedGroups: affected, Stages: []testpolicy.Stage{{ID: "affected", Groups: affected}}})
	if err != nil {
		return full("the required checks cannot be read")
	}
	var prerequisites []string
	for _, id := range plan.SelectedGroups {
		if !slices.Contains(affected, id) {
			prerequisites = append(prerequisites, id)
			for _, g := range d.contract.Groups {
				if g.ID == id {
					d.affected.Groups = append(d.affected.Groups, testpolicy.AffectedGroup{Group: g, Paths: []string{"prerequisite of an affected check"}})
				}
			}
		}
	}
	// A green's environment and changed inputs govern only its retained coverage.
	covered := map[string]bool{}
	if d.base.FullTree != "" && d.base.Environment != "" && d.base.Environment == d.standing.Environment {
		changed, err := seams.git(checkout, "diff", "--name-only", "--no-renames", d.base.FullTree, running.Tree)
		if err == nil {
			invalid, err := testpolicy.Affected(d.contract, strings.Fields(changed))
			if err == nil && len(invalid.Uncovered) == 0 {
				for _, g := range d.contract.Groups {
					covered[g.ID] = !slices.ContainsFunc(invalid.Groups, func(a testpolicy.AffectedGroup) bool { return a.Group.ID == g.ID })
				}
			}
		}
	}
	for _, g := range d.contract.Groups {
		if g.PackageSelection == "" {
			entry := scopeGroup{ID: g.ID, State: "unproved"}
			if covered[g.ID] {
				entry.State, entry.From = "kept", d.base.FullTree
			}
			d.Groups = append(d.Groups, entry)
		}
	}
	// Native packages with no concrete selector still have explicit provenance.
	for _, failure := range d.standing.Failures {
		ids, _ := resolveFailure(d.contract, failure, packages)
		for _, id := range ids {
			if !slices.ContainsFunc(d.Groups, func(g scopeGroup) bool { return g.ID == id }) {
				d.Groups = append(d.Groups, scopeGroup{ID: id, State: "unproved"})
			}
		}
	}
	at := d.standing.At
	d.ScopeReason = fmt.Sprintf("the standing red of tree %s (%s) is inherited; ", Short(d.standing.Tree), at.Local().Format("15:04"))
	if len(affected) == 0 {
		d.ScopeReason += "nothing to run"
		return d
	}
	repeats := "none"
	if len(prerequisites) > 0 {
		repeats = strings.Join(prerequisites, ", ")
	}
	d.ScopeReason += "affected checks run: " + strings.Join(affected, ", ") + "; prerequisites repeated: " + repeats
	if d.base.FullTree != "" {
		at, _ := time.Parse(time.RFC3339, d.base.FullAt)
		d.ScopeReason += "; the rest holds from the " + at.Local().Format("15:04") + " check"
	} else {
		d.ScopeReason += "; the rest is unproved"
	}
	return d
}

func (d scopeDecision) standingGroups(result Result, observed *proofOutput) []scopeGroup {
	groups := slices.Clone(d.Groups)
	for i := range groups {
		g := &groups[i]
		if duration, ran := observed.durations[g.ID]; ran {
			g.State, g.From, g.DurationMS = "ran", result.Tree, duration
			continue
		}
		for _, failure := range result.Failed {
			if !failure.Inherited {
				continue
			}
			ids, _ := resolveFailure(d.contract, failure, []string{strings.TrimPrefix(failure.Unit, "metasystem/")})
			if slices.Contains(ids, g.ID) {
				g.State, g.From = "standing", d.standing.Tree
			}
		}
	}
	return groups
}

func (d scopeDecision) carryStanding(result Result, observed *proofOutput) Result {
	if d.standing == nil || d.Scope != "scoped" {
		return result
	}
	for _, failure := range d.standing.Failures {
		carried := failure
		carried.Tests = nil
		carry := false
		names := failure.Tests
		if len(names) == 0 {
			names = []string{""}
		}
		for _, name := range names {
			executed := false
			for _, outcome := range observed.outcomes {
				if outcome.Unit == failure.Unit && outcome.Test == name && (outcome.Status == "pass" || outcome.Status == "fail") {
					executed = true
				}
			}
			if !executed {
				carry = true
				if name != "" {
					carried.Tests = append(carried.Tests, name)
				}
			}
		}
		if carry {
			carried.Inherited, carried.Attempt = true, d.standing.Attempt
			result.Failed = append(result.Failed, carried)
		}
	}
	if slices.ContainsFunc(result.Failed, func(f FailedUnit) bool { return f.Inherited }) {
		result.Result = Red
		if result.Cause == nil {
			result.Cause = &Cause{Kind: "main"}
		}
		if result.Reason == "" {
			result.Reason = d.ScopeReason
		}
	}
	return result
}

// Completed package terminals make their test outcomes usable. A partial log,
// a skipped test, or a blocked prerequisite never supplies passing evidence.
func (p *proofOutput) testLine(line string) {
	var event struct{ Action, Package, Test string }
	if !strings.HasPrefix(line, "{") || json.Unmarshal([]byte(line), &event) != nil || event.Package == "" {
		return
	}
	unit := strings.Replace(event.Package, "github.com/widoriezebos/agentic-tools/metasystem/", "metasystem/", 1)
	if p.events == nil {
		p.events = map[string][]testOutcome{}
	}
	if event.Test != "" && (event.Action == "pass" || event.Action == "fail" || event.Action == "skip") {
		p.events[unit] = append(p.events[unit], testOutcome{unit, event.Test, event.Action})
	}
	if event.Test == "" && (event.Action == "pass" || event.Action == "fail") {
		p.outcomes = append(p.outcomes, testOutcome{Unit: unit, Status: event.Action})
		p.outcomes = append(p.outcomes, p.events[unit]...)
		delete(p.events, unit)
	}
}

// retainOutcomes keeps only the latest executed evidence for open classified
// identities. Complete logs retain every other test's diagnostic output.
func (s ProveSeams) retainOutcomes(install, checkout string, result Result) Result {
	incidents, _ := s.incidents(install, checkout, "origin/main")
	open := map[string]bool{}
	for _, e := range incidents {
		if e.Closed == nil && e.EntryClass() == goal.TrunkRedClassTrunkRed {
			open[e.Identity] = true
		}
	}
	var retained []testOutcome
	for i := len(result.Outcomes) - 1; i >= 0; i-- {
		o := result.Outcomes[i]
		identity := "red:" + o.Unit
		if o.Test != "" {
			identity += ":" + o.Test
		}
		if open[identity] && (o.Status == "pass" || o.Status == "fail") {
			retained = append(retained, o)
			delete(open, identity)
		}
	}
	slices.Reverse(retained)
	result.Outcomes = retained
	return result
}
