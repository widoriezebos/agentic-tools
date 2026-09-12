package dispatch

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// ContinuationAfterCap marks a follow-up round that continues a predecessor
// the reaper cut off at its cap: the worktree is inherited, the packet
// carries the prior-worktree slot, and the round composes fresh context.
const ContinuationAfterCap = "after-cap"

// capContinuationListLimit bounds the paths the paragraph lists and
// capContinuationListBytes bounds their bytes (the packet has an inline
// input limit); the total is always named.
const capContinuationListLimit = 200
const capContinuationListBytes = 8 * 1024

// CapContinuationText writes the one paragraph a continuation round is told
// under the prior-worktree slot: the fact that its predecessor was cut off
// at its cap and wrote no return, what the worktree already holds against
// its head at composition, and the one rule the review needs (a
// diffBoundary that lists every changed path of the chain). It refuses a
// parent that is not an implementer round in timeout with budget-cap, and a
// missing worktree.
func CapContinuationText(parentRecordPath, worktree string) (string, error) {
	record, err := readObject(parentRecordPath)
	if err != nil {
		return "", fmt.Errorf("cannot read the parent record: %w", err)
	}
	role := asString(record["role"])
	status := asString(record["status"])
	errorCode := asString(record["error"])
	if role != "implementer" {
		return "", fmt.Errorf("a continuation after a cap is an implementer chain's; the parent round's role is %q", role)
	}
	if status != "timeout" || errorCode != "budget-cap" {
		return "", fmt.Errorf("a continuation after a cap needs a parent in timeout with budget-cap; the parent is %s with error %s", status, orNone(errorCode))
	}
	if worktree == "" {
		return "", fmt.Errorf("a continuation after a cap needs the chain's worktree")
	}
	if info, statErr := os.Stat(worktree); statErr != nil || !info.IsDir() {
		return "", fmt.Errorf("the chain's worktree %s is missing; nothing is left to continue, use a fresh dispatch", worktree)
	}
	head, err := gitOutput(worktree, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("the chain's worktree %s has no readable head: %w", worktree, err)
	}
	dirty, err := followUpDirtyPaths(worktree)
	if err != nil {
		return "", err
	}
	paths := make([]string, 0, len(dirty))
	for path := range dirty {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	round, _ := numInt(record["round"])
	capMinutes, _ := JobRecordOf(record).CapMinutes()
	deathAt := asString(record["groupDeathProvenAt"])
	if deathAt == "" {
		deathAt = asString(record["endedAt"])
	}
	var text strings.Builder
	fmt.Fprintf(&text, "Round %d of this chain was cut off at its %d-minute cap at %s and wrote no return. ", round, capMinutes, orNone(deathAt))
	fmt.Fprintf(&text, "The worktree already holds its work: %d path(s) changed against the worktree head %s at composition", len(paths), head)
	if len(paths) == 0 {
		text.WriteString(" (no changed path was found; the round left the tree as it found it).\n")
	} else {
		text.WriteString(":\n")
		listed, listedBytes := 0, 0
		for _, path := range paths {
			line := "- " + quotePath(path) + "\n"
			if listed >= capContinuationListLimit || listedBytes+len(line) > capContinuationListBytes {
				break
			}
			text.WriteString(line)
			listed++
			listedBytes += len(line)
		}
		if len(paths) > listed {
			fmt.Fprintf(&text, "- and %d more (the total is %d)\n", len(paths)-listed, len(paths))
		}
	}
	text.WriteString("Because the review diffs the whole worktree and the predecessor declared no boundary, your diffBoundary must list every changed path of the chain, the predecessor's included.\n")
	return text.String(), nil
}

// quotePath renders a path as one line of the paragraph: a path that is
// plain text goes as it is; one with whitespace, control characters or
// anything not printable is quoted, so a filename cannot carry a line or a
// heading into the engine-owned slot.
func quotePath(path string) string {
	for _, r := range path {
		if r <= ' ' || r == 0x7f || !strconv.IsPrint(r) {
			return strconv.Quote(path)
		}
	}
	return path
}

func orNone(value string) string {
	if value == "" {
		return "(none)"
	}
	return value
}
