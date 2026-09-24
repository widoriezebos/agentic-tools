package goal

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeGoalCommit struct {
	parent  string
	files   map[string][]byte
	trailer string
	at      time.Time
}

// Each fixture owns a store. Clients share only its canonical tip and commits.
type fakeGoalStore struct {
	mu        sync.Mutex
	commits   map[string]fakeGoalCommit
	canonical string
	serial    uint64
}

type fakeGoalRepository struct {
	store          *fakeGoalStore
	accepted       string
	brokenAccepted error
	unknownNext    bool
	released       []string
	captures       []string
	trailerChecks  []fakeTrailerCheck
	historyQueries map[fakeHistoryQuery]*fakeHistoryReply
	historyT       *testing.T
}

type fakeTrailerCheck struct{ tip, opid string }

type fakeHistoryQuery struct{ revision, key, value string }
type fakeHistoryReply struct {
	commit string
	err    error
	used   int
}

func newFakeGoalStore() *fakeGoalStore {
	s := &fakeGoalStore{commits: make(map[string]fakeGoalCommit)}
	s.serial++
	id := fmt.Sprintf("%040x", s.serial)
	s.commits[id] = fakeGoalCommit{files: map[string][]byte{}, at: time.Date(2026, 8, 20, 20, 0, 0, 0, time.UTC)}
	s.canonical = id
	return s
}

func (s *fakeGoalStore) client() *fakeGoalRepository { return &fakeGoalRepository{store: s} }

func copyFakeFiles(files map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(files))
	for path, data := range files {
		out[path] = append([]byte(nil), data...)
	}
	return out
}

func (f *fakeGoalRepository) Capture(string) (string, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	if _, ok := f.store.commits[f.store.canonical]; !ok {
		return "", errors.New("canonical commit missing")
	}
	f.captures = append(f.captures, f.store.canonical)
	return f.store.canonical, nil
}

func (f *fakeGoalRepository) Accepted() (string, bool, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	if f.brokenAccepted != nil {
		return "", false, f.brokenAccepted
	}
	if f.accepted == "" {
		return "", false, nil
	}
	if _, ok := f.store.commits[f.accepted]; !ok {
		return "", false, errors.New("accepted commit missing")
	}
	return f.accepted, true, nil
}

func (f *fakeGoalRepository) Files(commit string, prefixes ...string) (map[string][]byte, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	c, ok := f.store.commits[commit]
	if !ok {
		return nil, fmt.Errorf("commit %s missing", commit)
	}
	if len(prefixes) == 0 {
		return nil, errors.New("file read needs a prefix")
	}
	out := make(map[string][]byte)
	for path, data := range c.files {
		for _, prefix := range prefixes {
			if path == prefix || strings.HasPrefix(path, prefix) {
				out[path] = append([]byte(nil), data...)
				break
			}
		}
	}
	return out, nil
}

func (f *fakeGoalRepository) Build(opid, parent string, changes []Change, message string) (string, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	c, ok := f.store.commits[parent]
	if !ok {
		return "", fmt.Errorf("parent %s missing", parent)
	}
	if len(changes) == 0 || opid == "" || message == "" {
		return "", errors.New("incomplete commit request")
	}
	files := copyFakeFiles(c.files)
	for _, change := range changes {
		if change.Path == "" {
			return "", errors.New("empty change path")
		}
		if change.Delete {
			delete(files, change.Path)
		} else {
			files[change.Path] = append([]byte(nil), change.Content...)
		}
	}
	f.store.serial++
	id := fmt.Sprintf("%040x", f.store.serial)
	f.store.commits[id] = fakeGoalCommit{parent: parent, files: files, trailer: opid, at: c.at.Add(time.Minute)}
	return id, nil
}

func (f *fakeGoalRepository) Publish(parent, commit string) (CASOutcome, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	c, ok := f.store.commits[commit]
	if !ok || c.parent != parent {
		return CASUnknown, errors.New("commit does not descend directly from expected parent")
	}
	if f.unknownNext {
		f.unknownNext = false
		return CASUnknown, errors.New("transport outcome unknown")
	}
	if f.store.canonical != parent {
		return CASRefused, errors.New("stale canonical compare")
	}
	f.store.canonical = commit
	return CASLanded, nil
}

func (f *fakeGoalRepository) AcceptedCAS(old, next string) error {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	if f.brokenAccepted != nil {
		return f.brokenAccepted
	}
	if f.accepted != old {
		return errors.New("stale accepted compare")
	}
	if _, ok := f.store.commits[next]; !ok {
		return fmt.Errorf("accepted target %s missing", next)
	}
	f.accepted = next
	return nil
}

func (f *fakeGoalRepository) IsAncestor(ancestor, descendant string) (bool, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	if _, ok := f.store.commits[ancestor]; !ok {
		return false, fmt.Errorf("ancestor %s missing", ancestor)
	}
	for descendant != "" {
		if descendant == ancestor {
			return true, nil
		}
		c, ok := f.store.commits[descendant]
		if !ok {
			return false, fmt.Errorf("descendant %s missing", descendant)
		}
		descendant = c.parent
	}
	return false, nil
}

func (f *fakeGoalRepository) TrailerPresent(tip, opid string) (bool, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.trailerChecks = append(f.trailerChecks, fakeTrailerCheck{tip, opid})
	for tip != "" {
		c, ok := f.store.commits[tip]
		if !ok {
			return false, fmt.Errorf("history commit %s missing", tip)
		}
		if c.trailer == opid {
			return true, nil
		}
		tip = c.parent
	}
	return false, nil
}

func (f *fakeGoalRepository) CommitWithTrailer(revision, key, value string) (string, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	query := fakeHistoryQuery{revision, key, value}
	reply, ok := f.historyQueries[query]
	if !ok {
		err := fmt.Errorf("undeclared carry history query: revision=%q key=%q value=%q", revision, key, value)
		if f.historyT == nil {
			panic(err)
		}
		f.historyT.Error(err)
		return "", err
	}
	reply.used++
	return reply.commit, reply.err
}

func (f *fakeGoalRepository) CommitTime(commit string) (time.Time, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	c, ok := f.store.commits[commit]
	if !ok {
		return time.Time{}, fmt.Errorf("commit %s missing", commit)
	}
	return c.at, nil
}

func (f *fakeGoalRepository) Release(opid string) error {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	if opid == "" {
		return errors.New("empty operation ref")
	}
	f.released = append(f.released, opid)
	return nil
}
