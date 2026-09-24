package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// sessionStopAcceptedRepository supplies the two committed goal files read by
// the turn verdict. It has no mutation authority.
type sessionStopAcceptedRepository struct {
	tip      string
	files    map[string][]byte
	mu       sync.Mutex
	captured map[string]bool
	fetches  int
	releases int
}

func (r *sessionStopAcceptedRepository) Capture(opid string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if opid == "" || r.captured[opid] {
		return "", fmt.Errorf("session-stop fixture refuses capture %q", opid)
	}
	if r.captured == nil {
		r.captured = make(map[string]bool)
	}
	r.captured[opid] = true
	r.fetches++
	return r.tip, nil
}

func (r *sessionStopAcceptedRepository) Accepted() (string, bool, error) {
	return r.tip, true, nil
}

func (r *sessionStopAcceptedRepository) Files(commit string, prefixes ...string) (map[string][]byte, error) {
	if commit != r.tip {
		return nil, fmt.Errorf("session-stop fixture refuses commit %q", commit)
	}
	rootRead := len(prefixes) == 1 && prefixes[0] == "plans/goals/backlog.md"
	treeRead := len(prefixes) == 2 && prefixes[0] == "plans/goals/" && prefixes[1] == "records/goals/"
	if !rootRead && !treeRead {
		return nil, fmt.Errorf("session-stop fixture refuses file prefixes %q", prefixes)
	}
	files := make(map[string][]byte)
	for path, data := range r.files {
		if (rootRead && path == prefixes[0]) || (treeRead && strings.HasPrefix(path, prefixes[0])) {
			files[path] = append([]byte(nil), data...)
		}
	}
	return files, nil
}

func (r *sessionStopAcceptedRepository) CommitTime(commit string) (time.Time, error) {
	if commit != r.tip {
		return time.Time{}, fmt.Errorf("session-stop fixture refuses commit time for %q", commit)
	}
	return time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC), nil
}

func (r *sessionStopAcceptedRepository) Release(opid string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.captured[opid] {
		return fmt.Errorf("session-stop fixture refuses release %q without capture", opid)
	}
	delete(r.captured, opid)
	r.releases++
	return nil
}

func (*sessionStopAcceptedRepository) Build(string, string, []goal.Change, string) (string, error) {
	return "", fmt.Errorf("session-stop fixture refuses Build")
}
func (*sessionStopAcceptedRepository) Publish(string, string) (goal.CASOutcome, error) {
	return "", fmt.Errorf("session-stop fixture refuses Publish")
}
func (*sessionStopAcceptedRepository) AcceptedCAS(string, string) error {
	return fmt.Errorf("session-stop fixture refuses AcceptedCAS")
}
func (*sessionStopAcceptedRepository) IsAncestor(string, string) (bool, error) {
	return false, fmt.Errorf("session-stop fixture refuses IsAncestor")
}
func (*sessionStopAcceptedRepository) TrailerPresent(string, string) (bool, error) {
	return false, fmt.Errorf("session-stop fixture refuses TrailerPresent")
}
func (*sessionStopAcceptedRepository) CommitWithTrailer(string, string, string) (string, error) {
	return "", fmt.Errorf("session-stop fixture refuses CommitWithTrailer")
}
