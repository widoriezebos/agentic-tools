package main

import (
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// personClassifyAt is the classifier of the person-act checks that decide by
// class rather than by proof: lease.ClassifyPersonAt at the command's clock,
// so the main session a live general power of attorney admits is a person
// there. A clock that cannot be read admits no grant.
func personClassifyAt(repo, metasystemRoot string, pid int64) (lease.Classification, error) {
	now, err := goalCommandNow(metasystemRoot)
	if err != nil {
		now = time.Time{}
	}
	return lease.ClassifyPersonAt(repo, metasystemRoot, pid, now)
}

// personClassify is personClassifyAt for one root.
func personClassify(root string, pid int64) (lease.Classification, error) {
	return personClassifyAt(root, root, pid)
}

// personVerbCaller is classifyVerbCaller for a person-act check: a caller a
// live general power of attorney admits answers HUMAN.
func personVerbCaller(root string, pid int64) (lease.ClassifyResult, error) {
	return personVerbCallerWith(root, pid, classifyVerbCaller, goalCommandNow, humanauthority.AtAttorney)
}

func personVerbCallerWith(root string, pid int64, classify func(string, int64) (lease.ClassifyResult, error),
	clock func(string) (time.Time, error), admit func(string, int64, time.Time) (humanauthority.HelmGrant, bool)) (lease.ClassifyResult, error) {
	result, err := classify(root, pid)
	if err != nil || result.Class == lease.ClassHuman || admit == nil {
		return result, err
	}
	now, clockErr := clock(root)
	if clockErr != nil || now.IsZero() {
		return result, nil
	}
	if grant, admitted := admit(root, pid, now); admitted && grant.Grant != "" {
		result.Class = lease.ClassHuman
	}
	return result, nil
}
