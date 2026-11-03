package loopstop

import (
	"slices"
	"strings"
)

type RedUnit struct {
	Unit              string
	Tests, Registered []string
}

type RedRun struct {
	Attempted, Exited, Complete, Passed, Wrapper bool
	Launch                                       string
	Selectors                                    []string
	Whole                                        bool
	Failed                                       []RedUnit
}

type RedReplay struct {
	Role         string
	Goal, SHA    string
	Named, Whole RedRun
}

type RedEvidence struct {
	Goal      string
	Failed    []RedUnit
	Alone     RedRun
	Replays   []RedReplay
	CannotRun string
}

type RedVerdict struct {
	Next      string
	Selectors []string
	Replay    int
	Whole     bool
	Cause     *Cause
	Flaked    []string
	Words     string
}

func (r RedRun) complete() bool { return r.Attempted && r.Exited && (r.Complete || r.Wrapper) }

func (r RedRun) unfinished() bool {
	return r.Attempted && (!r.Exited || !r.Passed && !r.complete())
}

func redContains(units []RedUnit, unit, test string) bool {
	return slices.ContainsFunc(units, func(u RedUnit) bool {
		if u.Unit != unit {
			return false
		}
		return len(u.Tests) == 0 ||
			slices.Contains(u.Tests, test) ||
			slices.Contains(u.Tests, test+"(did not report)")
	})
}

func (r RedReplay) run(alone RedRun) (RedRun, bool) {
	named := r.Named
	like := alone.complete() && !alone.Passed && slices.Equal(alone.Selectors, named.Selectors)
	if named.Attempted && len(named.Selectors) > 0 && (named.Passed || !named.complete()) && !like {
		return r.Whole, true
	}
	return named, false
}

// ClassifyRed decides only from retained facts. Callers declare replay trees
// oldest first and perform the next execution the verdict asks for.
func ClassifyRed(e RedEvidence) (v RedVerdict) {
	var remaining []RedUnit
	defer func() {
		if v.Cause != nil && v.Cause.Kind != "flake" {
			v.Words = redWords(remaining, v.Cause, e.CannotRun)
		}
	}()
	var tests, flakeWords []string
	unnamed := false
	for _, unit := range e.Failed {
		if len(unit.Tests) == 0 {
			unnamed = true
			remaining = append(remaining, unit)
			tests = append(tests, unit.Unit+" (package)")
			continue
		}
		left := RedUnit{Unit: unit.Unit}
		for i, test := range unit.Tests {
			if e.Alone.complete() && (e.Alone.Passed || len(e.Alone.Failed) > 0 && !redContains(e.Alone.Failed, unit.Unit, test)) {
				v.Flaked = append(v.Flaked, unit.Unit+" "+test)
				flakeWords = append(flakeWords, test+" ("+unit.Unit+") failed in the check and passed alone on this tree")
			} else {
				left.Tests = append(left.Tests, test)
				registration := ""
				if i < len(unit.Registered) {
					registration = unit.Registered[i]
				}
				left.Registered = append(left.Registered, registration)
				tests = append(tests, unit.Unit+" "+test)
			}
		}
		if len(left.Tests) > 0 {
			remaining = append(remaining, left)
		}
	}
	answer := func(kind string) RedVerdict {
		v.Cause = &Cause{Kind: kind, Tests: tests}
		return v
	}
	if e.Alone.unfinished() {
		return answer("environment")
	}
	for _, replay := range e.Replays {
		run, _ := replay.run(e.Alone)
		if run.unfinished() || replay.Whole.unfinished() {
			return answer("environment")
		}
	}
	if e.CannotRun != "" {
		return answer("unclassified")
	}
	if !e.Alone.Attempted && len(RedSelectors(e.Failed)) > 0 {
		v.Next, v.Selectors = "alone", RedSelectors(e.Failed)
		return v
	}
	if e.Alone.complete() && e.Alone.Passed && !unnamed {
		v = answer("flake")
		v.Cause.Tests = slices.Clone(v.Flaked)
		v.Words = "checks passed after a flake rerun: " + strings.Join(flakeWords, "; ")
		return v
	}
	if len(e.Replays) == 0 {
		return answer("unclassified")
	}
	greenReplays := true
	for i, replay := range e.Replays {
		run, fallback := replay.run(e.Alone)
		if !run.Attempted {
			v.Next, v.Replay, v.Whole, v.Selectors = "replay", i, fallback, slices.Clone(run.Selectors)
			return v
		}
		if !run.complete() {
			return answer("unclassified")
		}
		for _, unit := range remaining {
			identity := "red:" + unit.Unit
			reproduced := !run.Passed && (fallback || run.Whole || run.Wrapper)
			for _, test := range unit.Tests {
				if !run.Passed && (fallback || run.Whole || run.Wrapper || redContains(run.Failed, unit.Unit, test)) {
					identity, reproduced = identity+":"+test, true
					break
				}
			}
			if !reproduced {
				continue
			}
			v = answer("other")
			switch replay.Role {
			case "main":
				v.Cause.Kind, v.Cause.Name = "main", identity
			case "goal":
				v.Cause.Kind, v.Cause.Goal, v.Cause.SHA = "own", replay.Goal, replay.SHA
			}
			return v
		}
		greenReplays = greenReplays && run.Passed
	}
	if !greenReplays || len(RedSelectors(remaining)) > 0 && (!e.Alone.complete() || e.Alone.Passed) {
		return answer("unclassified")
	}
	v = answer("own")
	v.Cause.Goal = e.Goal
	return v
}

func redWords(units []RedUnit, cause *Cause, why string) string {
	if cause.Kind == "unclassified" {
		if why == "" {
			why = "no completed, comparable base replay is available"
		}
		return "the red could not be classified: " + why
	}
	if len(units) == 0 {
		units = []RedUnit{{Unit: "the check"}}
	}
	var words []string
	for _, unit := range units {
		names := unit.Tests
		if len(names) == 0 {
			names = []string{""}
		}
		for i, test := range names {
			name := unit.Unit
			if test != "" {
				name = test + " in " + unit.Unit
			}
			if cause.Kind == "own" && i < len(unit.Registered) && unit.Registered[i] != "" {
				identity := unit.Registered[i]
				name += ", registered as flake:" + strings.TrimPrefix(identity, "flake:")[:min(8, len(strings.TrimPrefix(identity, "flake:")))] + ","
				if cause.Name == "" {
					cause.Name = identity
				}
			}
			suffix := " failed in the check, failed again alone on this tree, and passed on the unit's base: the change causes it"
			switch cause.Kind {
			case "own":
				if test == "" {
					suffix = " failed in the check and passed on the unit's base; no rerun alone is possible for a failure that names no test"
				}
			case "main":
				suffix = " fails on main too (main's incident " + cause.Name + "); this change is not the cause"
			case "other":
				suffix = " failed in the check and on the unit's base, which is not main"
			case "environment":
				suffix = ": the rerun or the base run could not finish (no completed exit evidence or complete report)"
			}
			words = append(words, name+suffix)
		}
	}
	return strings.Join(words, "; ")
}

// RedSelectors uses the lane's combined selector form, omitting unnamed units.
func RedSelectors(units []RedUnit) []string {
	var packages, selections, tests []string
	for _, unit := range units {
		if len(unit.Tests) > 0 {
			packages = append(packages, unit.Unit)
			selections = append(selections, unit.Unit+"="+strings.Join(unit.Tests, ","))
			tests = append(tests, unit.Tests...)
		}
	}
	if len(packages) == 0 {
		return nil
	}
	if len(packages) == 1 {
		selections = packages
	}
	return []string{"LANDING_ONLY=" + strings.Join(selections, " "), "LANDING_ONLY_TESTS=" + strings.Join(tests, " ")}
}
