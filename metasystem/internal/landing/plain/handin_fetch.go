package plain

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// A failed fetch is retained as a queue-shaped line for its handed-in commit.
// Keeper ticks and status reads do not retry that same hand-in indefinitely.
func handInFetchPath(install string) string { return Dir(install) + "/hand-in-fetches.jsonl" }

// handInCommits fetches only at selection; status reads retained failures.
// Fix claims stay local: their delivery contract forbids fetching the branch.
func handInCommits(install, checkout string, entries []Entry, seams ProveSeams, fetch bool) error {
	if fetch && !slices.ContainsFunc(entries, func(entry Entry) bool {
		if entry.State != StateWaiting || entry.Fix != "" {
			return false
		}
		_, err := seams.git(checkout, "cat-file", "-e", entry.SHA+"^{commit}")
		return err != nil
	}) {
		return nil
	}
	if fetch {
		held, err := lock.File(handInFetchPath(install)+".lock", 0600, lock.Exclusive)
		if err != nil {
			return err
		}
		defer held.Release()
	}
	attempts, err := readLines[Line](handInFetchPath(install))
	if err != nil {
		return err
	}
	var problems []error
	for _, entry := range entries {
		if entry.State != StateWaiting || entry.Fix != "" {
			continue
		}
		if _, err := seams.git(checkout, "cat-file", "-e", entry.SHA+"^{commit}"); err == nil {
			continue
		}
		problem := ""
		for _, attempt := range attempts {
			if attempt.Goal == entry.Goal && attempt.SHA == entry.SHA {
				problem = attempt.Reason
			}
		}
		if problem == "" && !fetch {
			continue
		}
		if problem == "" {
			args := []string{"fetch", "origin", "refs/heads/goal/" + entry.Goal}
			output, fetchErr := "", error(nil)
			if seams.Git == nil || seams.FetchCommand != nil {
				output, fetchErr = boundedFetch(checkout, seams.FetchTimeout, seams.FetchCommand, seams.FetchDeadline, args...)
			} else {
				output, fetchErr = seams.git(checkout, args...)
			}
			if _, err := seams.git(checkout, "cat-file", "-e", entry.SHA+"^{commit}"); err == nil {
				continue
			}
			problem = fmt.Sprintf("hand-in %s at %s is not on origin", entry.Goal, entry.SHA)
			if fetchErr != nil {
				detail := strings.SplitN(strings.TrimSpace(output+"\n"+fetchErr.Error()), "\n", 2)[0]
				if strings.Contains(detail, "couldn't find remote ref") {
					problem += ": " + detail
				} else {
					problem = fmt.Sprintf("hand-in %s at %s: origin unreachable: %s", entry.Goal, entry.SHA, detail)
				}
			}
			if err := appendLine(handInFetchPath(install), Line{Goal: entry.Goal, SHA: entry.SHA, Reason: problem}); err != nil {
				return err
			}
		}
		problems = append(problems, errors.New(problem))
	}
	return errors.Join(problems...)
}
