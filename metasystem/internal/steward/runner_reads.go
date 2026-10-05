package steward

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Only resident runners keep snapshots. Manual readers remain fresh, and a
// runner releases its snapshots with its repository lock.
var runnerReads sync.Map

type runnerReadState struct {
	sync.Mutex
	tick         int
	dirs         map[string]any
	scans, reads int
}

type recordRead[T any] struct {
	name  string
	info  os.FileInfo
	value T
	err   error
}

type directoryRead[T any] struct {
	info    os.FileInfo
	tick    int
	byName  map[string]recordRead[T]
	ordered []recordRead[T]
}

// Records are published by atomic replacement, which changes directory mtime.
// Every tenth tick also checks individual mtimes to reconcile in-place edits.
const runnerReadAuditTicks = 10

func readRunnerRecords[T any](root, dir string, load func(string) (T, error)) ([]recordRead[T], error) {
	var state *runnerReadState
	if value, ok := runnerReads.Load(canonicalPath(root)); ok {
		state = value.(*runnerReadState)
		state.Lock()
		defer state.Unlock()
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	var previous *directoryRead[T]
	if state != nil {
		previous, _ = state.dirs[dir].(*directoryRead[T])
		if previous != nil && os.SameFile(info, previous.info) && info.ModTime() == previous.info.ModTime() && state.tick-previous.tick < runnerReadAuditTicks {
			return previous.ordered, nil
		}
		state.scans++
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	next := &directoryRead[T]{info: info, byName: map[string]recordRead[T]{}}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		fileInfo, statErr := entry.Info()
		var record recordRead[T]
		if previous != nil {
			record = previous.byName[entry.Name()]
		}
		if statErr != nil {
			record = recordRead[T]{err: statErr}
		} else if record.info == nil || !os.SameFile(fileInfo, record.info) || fileInfo.ModTime() != record.info.ModTime() || fileInfo.Size() != record.info.Size() || record.err != nil {
			record.info = fileInfo
			record.value, record.err = load(filepath.Join(dir, entry.Name()))
			if state != nil {
				state.reads++
			}
		}
		record.name = entry.Name()
		next.byName[entry.Name()] = record
		next.ordered = append(next.ordered, record)
	}
	if state != nil {
		next.tick = state.tick
		state.dirs[dir] = next
		for _, record := range next.ordered {
			if record.err != nil {
				delete(state.dirs, dir)
				break
			}
		}
	}
	return next.ordered, nil
}

type timedCensus struct {
	WorkerCensus
	clock   func() time.Time
	elapsed time.Duration
}

func (c *timedCensus) Workers(root string) (Workers, error) {
	start := c.clock()
	defer func() { c.elapsed += c.clock().Sub(start) }()
	return c.WorkerCensus.Workers(root)
}
