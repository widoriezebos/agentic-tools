package testgoal

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// New starts a private accepted goal ledger from copied seed files.
func New(files map[string][]byte, at time.Time, identity string) *Repository {
	return &Repository{commits: map[string]mutationCommit{identity: {files: copyMutationFiles(files), at: at}}, canonical: identity, accepted: identity, refs: make(map[string]string), serial: 1}
}

type mutationCommit struct {
	parent  string
	trailer string
	files   map[string][]byte
	at      time.Time
}

// Every fixture owns its commits and refs. Commit files are copied on both
// ingress and egress so a later goal transaction cannot rewrite an old tip.
type Repository struct {
	mu        sync.Mutex
	commits   map[string]mutationCommit
	canonical string
	accepted  string
	refs      map[string]string
	serial    uint64
}

func copyMutationFiles(files map[string][]byte) map[string][]byte {
	copy := make(map[string][]byte, len(files))
	for path, data := range files {
		copy[path] = append([]byte(nil), data...)
	}
	return copy
}

func (r *Repository) commit(id string) (mutationCommit, error) {
	commit, ok := r.commits[id]
	if !ok {
		return mutationCommit{}, fmt.Errorf("undeclared goal commit %q", id)
	}
	return commit, nil
}

func (r *Repository) Capture(opid string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if opid == "" {
		return "", fmt.Errorf("empty goal operation capture")
	}
	if _, err := r.commit(r.canonical); err != nil {
		return "", err
	}
	r.refs[opid] = r.canonical
	return r.canonical, nil
}

func (r *Repository) Accepted() (string, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := r.commit(r.accepted); err != nil {
		return "", false, err
	}
	return r.accepted, true, nil
}

func (r *Repository) Files(id string, prefixes ...string) (map[string][]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	commit, err := r.commit(id)
	if err != nil {
		return nil, err
	}
	if len(prefixes) == 0 {
		return nil, fmt.Errorf("goal files request has no prefixes")
	}
	files := make(map[string][]byte)
	for path, data := range commit.files {
		for _, prefix := range prefixes {
			if path == prefix || strings.HasPrefix(path, prefix) {
				files[path] = append([]byte(nil), data...)
				break
			}
		}
	}
	return files, nil
}

func (r *Repository) Build(opid, parent string, changes []goal.Change, message string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	base, err := r.commit(parent)
	if err != nil {
		return "", err
	}
	if opid == "" || message == "" || len(changes) == 0 || r.refs[opid] != parent {
		return "", fmt.Errorf("undeclared goal build op=%q parent=%q", opid, parent)
	}
	files := copyMutationFiles(base.files)
	for _, change := range changes {
		if change.Path == "" || change.Path == "plans/goals/backlog.md" && change.Delete {
			return "", fmt.Errorf("invalid goal change path %q", change.Path)
		}
		if change.Delete {
			delete(files, change.Path)
		} else {
			files[change.Path] = append([]byte(nil), change.Content...)
		}
	}
	r.serial++
	id := fmt.Sprintf("%040x", r.serial)
	r.commits[id] = mutationCommit{parent: parent, trailer: opid, files: files, at: base.at.Add(time.Minute)}
	r.refs[opid] = id
	return id, nil
}

func (r *Repository) Publish(parent, id string) (goal.CASOutcome, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	commit, err := r.commit(id)
	if err != nil {
		return goal.CASUnknown, err
	}
	if commit.parent != parent {
		return goal.CASUnknown, fmt.Errorf("goal commit %s has parent %s, not %s", id, commit.parent, parent)
	}
	if r.refs[commit.trailer] != id {
		return goal.CASUnknown, fmt.Errorf("goal operation %s does not own commit %s", commit.trailer, id)
	}
	if r.canonical != parent {
		return goal.CASRefused, fmt.Errorf("stale canonical compare: %s != %s", r.canonical, parent)
	}
	r.canonical = id
	return goal.CASLanded, nil
}

func (r *Repository) AcceptedCAS(old, next string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.accepted != old {
		return fmt.Errorf("stale accepted compare: %s != %s", r.accepted, old)
	}
	if _, err := r.commit(next); err != nil {
		return err
	}
	r.accepted = next
	return nil
}

func (r *Repository) IsAncestor(ancestor, descendant string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := r.commit(ancestor); err != nil {
		return false, err
	}
	for descendant != "" {
		if descendant == ancestor {
			return true, nil
		}
		commit, err := r.commit(descendant)
		if err != nil {
			return false, err
		}
		descendant = commit.parent
	}
	return false, nil
}

func (r *Repository) TrailerPresent(tip, opid string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if opid == "" {
		return false, fmt.Errorf("empty goal trailer query")
	}
	for tip != "" {
		commit, err := r.commit(tip)
		if err != nil {
			return false, err
		}
		if commit.trailer == opid {
			return true, nil
		}
		tip = commit.parent
	}
	return false, nil
}

func (r *Repository) CommitWithTrailer(revision, key, value string) (string, error) {
	return "", fmt.Errorf("undeclared goal history query revision=%q key=%q value=%q", revision, key, value)
}

func (r *Repository) CommitTime(id string) (time.Time, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	commit, err := r.commit(id)
	return commit.at, err
}

func (r *Repository) Release(opid string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.refs[opid]; !ok {
		return fmt.Errorf("undeclared goal operation release %q", opid)
	}
	delete(r.refs, opid)
	return nil
}
