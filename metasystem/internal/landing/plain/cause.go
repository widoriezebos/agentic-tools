package plain

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Cause records what a red check or return demonstrates and where its evidence lives.
type Cause struct {
	Kind     string   `json:"kind"`
	Goal     string   `json:"goal,omitempty"`
	SHA      string   `json:"sha,omitempty"`
	Name     string   `json:"name,omitempty"`
	Tests    []string `json:"tests,omitempty"`
	Evidence string   `json:"evidence,omitempty"`
}

func (c Cause) Valid() bool {
	return slices.Contains([]string{"own", "main", "other", "flake", "environment", "unclassified"}, c.Kind)
}

// GoalSHA identifies a hand-in contained in the checked commit.
type GoalSHA struct {
	Goal string `json:"goal"`
	SHA  string `json:"sha"`
}

func goalsInCommit(install, checkout, commit string, git func(string, ...string) (string, error)) ([]GoalSHA, error) {
	queue, err := Entries(install)
	if err != nil {
		return nil, err
	}
	return proofGoals(queue, commit, checkoutGit(checkout, ProveSeams{Git: git}).contains)
}

func failingTests(failed []FailedUnit) []string {
	var tests []string
	for _, unit := range failed {
		for _, test := range unit.Tests {
			tests = append(tests, unit.Unit+" "+test)
		}
	}
	return tests
}

// ReturnRefused explains why the newest proof cannot return this hand-in.
type ReturnRefused struct {
	Goal  string
	Proof Result
}

func (r *ReturnRefused) Error() string {
	why := "no own defect of goal " + r.Goal + " at its waiting commit"
	if c := r.Proof.Cause; c != nil {
		switch c.Kind {
		case "main":
			why = "main itself is red"
		case "own":
			why = "an own defect of goal " + c.Goal + " at " + Short(c.SHA)
		default:
			why = "a cause of " + c.Kind
		}
		if len(c.Tests) > 0 {
			why += " (" + strings.Join(c.Tests, ", ") + ")"
		}
	}
	return "goal " + r.Goal + " was not returned: the lane's last check found " + why + ". Only a goal's own defect sends it back."
}

// ReturnProven checks and records a return under the same queue lock. A person's
// proven act may return any cause; an agent needs the newest proof of this hand-in.
func ReturnProven(install, goal, kind, reason string, person bool, by string, now time.Time) (entry Entry, changed bool, err error) {
	if !(Cause{Kind: kind}).Valid() {
		return Entry{}, false, fmt.Errorf("unknown return cause %q", kind)
	}
	err = withLock(install, func() error {
		latest, ok, readErr := Latest(install, goal)
		if readErr != nil {
			return readErr
		}
		if !ok {
			return fmt.Errorf("%w: %s was never handed in", ErrNotWaiting, goal)
		}
		if latest.State == StateReturned {
			entry = latest
			return nil
		}
		proof, readErr := lastGoalProof(install, goal)
		if readErr != nil && !person {
			return readErr
		}
		cause := &Cause{Kind: kind, Goal: goal, SHA: latest.SHA}
		if !person {
			c := proof.Cause
			if kind != "own" || proof.Result != Red || c == nil || c.Kind != "own" || c.Goal != goal || c.SHA != latest.SHA {
				return &ReturnRefused{Goal: goal, Proof: proof}
			}
			copy := *c
			cause = &copy
		} else {
			cause.Name = by
			if proof.Cause != nil {
				cause.Tests, cause.Evidence = proof.Cause.Tests, proof.Cause.Evidence
			}
		}
		if len(cause.Tests) == 0 {
			cause.Tests = failingTests(proof.Failed)
		}
		if cause.Evidence == "" {
			cause.Evidence = proof.Log
		}
		if reason == "" {
			reason = strings.Join(cause.Tests, ", ")
			if cause.Evidence != "" {
				reason += "; evidence: " + cause.Evidence
			}
			if strings.TrimSpace(reason) == "" {
				reason = proof.Reason
			}
			if reason == "" {
				reason = "cause: " + kind
			}
		}
		entry, changed, readErr = returnLocked(install, goal, reason, cause, nil, now)
		return readErr
	})
	return
}

func lastGoalProof(install, goal string) (Result, error) {
	var newest Result
	for _, path := range []string{resultsPath(install), filepath.Join(Dir(install), "gates.jsonl")} {
		results, err := readLines[Result](path)
		if err != nil {
			return Result{}, err
		}
		var last Result
		for _, result := range results {
			names := slices.ContainsFunc(result.Goals, func(g GoalSHA) bool { return g.Goal == goal }) || result.Cause != nil && result.Cause.Goal == goal
			if names {
				last = result
			}
		}
		if last.Result != "" && (newest.Result == "" || !resultTime(last).Before(resultTime(newest))) {
			newest = last
		}
	}
	return newest, nil
}

func resultTime(r Result) time.Time { at, _ := time.Parse(time.RFC3339Nano, r.At); return at }
