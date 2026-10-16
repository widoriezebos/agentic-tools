package main

import (
	"slices"
	"strings"
	"testing"
)

// landingFixtureParents answers only the shared ancestry read, using the bed's
// commit graph. Unknown roots stay missing, including an unknown main.
func landingFixtureParents(args []string, parents map[string][]string) (string, bool) {
	if len(args) < 4 || !slices.Equal(args[:3], []string{"rev-list", "--parents", "--ignore-missing"}) {
		return "", false
	}
	seen := map[string]bool{}
	var lines []string
	var visit func(string)
	visit = func(commit string) {
		ancestors, known := parents[commit]
		if !known || seen[commit] {
			return
		}
		seen[commit] = true
		lines = append(lines, strings.Join(append([]string{commit}, ancestors...), " "))
		for _, parent := range ancestors {
			visit(parent)
		}
	}
	for _, root := range args[3:] {
		visit(root)
	}
	return strings.Join(lines, "\n"), true
}

func drainFixtureParents(landed map[string]bool) map[string][]string {
	parents := map[string][]string{"main": nil}
	for commit, onMain := range landed {
		if onMain {
			parents["main"] = append(parents["main"], commit)
			parents[commit] = nil
		}
	}
	slices.Sort(parents["main"])
	return parents
}

func TestLandingFixtureParentListingUsesCurrentFacts(t *testing.T) {
	t.Parallel()
	parents := map[string][]string{"main": {"base"}, "base": nil, "pushed": {"old", "goal"}, "old": {"base"}, "goal": {"base"}}
	args := []string{"rev-list", "--parents", "--ignore-missing", "main", "pushed", "old", "missing"}
	listed, ok := landingFixtureParents(args, parents)
	if !ok || listed != "main base\nbase\npushed old goal\nold base\ngoal base" {
		t.Fatalf("parent listing: %q, handled=%v", listed, ok)
	}
	parents["main"] = []string{"new"}
	parents["new"] = []string{"base"}
	listed, ok = landingFixtureParents(args, parents)
	if !ok || !strings.HasPrefix(listed, "main new\nnew base\nbase\n") {
		t.Fatalf("listing lost current ancestry: %q", listed)
	}
	if listed, ok := landingFixtureParents([]string{"rev-list", "--parents", "--ignore-missing", "missing"}, parents); !ok || listed != "" {
		t.Fatalf("invented a requested root: %q, handled=%v", listed, ok)
	}
	for _, other := range [][]string{{"rev-list", "main..pushed"}, {"rev-list", "--parents", "main"}, {"push", "origin", "main"}} {
		if _, ok := landingFixtureParents(other, parents); ok {
			t.Fatalf("accepted another Git command: %v", other)
		}
	}
}
