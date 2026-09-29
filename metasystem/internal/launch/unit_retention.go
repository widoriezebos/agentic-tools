package launch

// Unit record retention (design engine-owns-disk-lifetimes Part B, 3.1
// "unit", 3.5, U5d; DL3B-03). A unit record's owner is its goal: over
// disk.unit-target-mib the machine pass removes, oldest first, a unit whose
// goal has concluded, whose run lock and named lock are both free, and
// whose last step is older than disk.unit-keep-days, with the named entry
// that reached it. A unit of an open goal is a live owner whatever its age;
// an unknown goal state keeps it; a held lock is pending. Units are visited before
// launches, so a launch a released unit named stops being a retention root.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"golang.org/x/sys/unix"
)

// UnitRetention is the unit store's class in the machine pass.
type UnitRetention struct {
	Root string
	// Target is disk.unit-target-mib in bytes; Keep disk.unit-keep-days.
	Target int64
	Keep   time.Duration
	// GoalEnded reads whether the goal a unit belongs to has concluded, and
	// whether that is known, from the checkout its worktree belongs to.
	GoalEnded func(goal, worktree string) (ended, known bool)

	count int
	bytes int64
}

func (*UnitRetention) Name() string { return "unit records" }

// Totals are every unit record and the store's bytes.
func (r *UnitRetention) Totals() (int, int64) { return r.count, r.bytes }

type unitEntry struct {
	id     string
	path   string
	bytes  int64
	ended  time.Time
	record UnitRunRecord
}

// Plan reads every unit record and, over the target, names the oldest
// units whose proof holds. Its lock probes create nothing.
func (r *UnitRetention) Plan(ctx context.Context, pass *diskstore.Pass) ([]diskstore.Item, error) {
	r.count, r.bytes = 0, 0
	dirEntries, err := os.ReadDir(r.Root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.bytes, _, _ = diskstore.Measure(ctx, r.Root)
	var items []diskstore.Item
	var units []unitEntry
	for _, entry := range dirEntries {
		name, path := entry.Name(), filepath.Join(r.Root, entry.Name())
		if !entry.IsDir() {
			continue
		}
		if id, ok := strings.CutPrefix(name, releasedPrefix); ok && idPattern.MatchString(id) {
			items = append(items, diskstore.Item{Class: r.Name(), Key: name, Path: path,
				Verdict: diskstore.Verdict{Decision: diskstore.Release, Reason: "a unit release the budget cut short is finished"}})
			continue
		}
		if !idPattern.MatchString(name) {
			continue
		}
		r.count++
		record, found, err := readUnitRecord(path)
		switch {
		case err != nil:
			items = append(items, diskstore.Item{Class: r.Name(), Key: name, Path: path,
				Verdict: diskstore.Verdict{Decision: diskstore.Pending, Reason: "unit record unreadable: " + err.Error(), Command: "metasystem work status"}})
			continue
		case !found:
			continue
		}
		bytes, _, _ := diskstore.Measure(ctx, path)
		units = append(units, unitEntry{id: name, path: path, bytes: bytes, ended: unitEnded(record, path), record: record})
	}
	if r.bytes <= r.Target {
		return items, nil
	}
	if _, err := r.namedEntries(""); err != nil {
		return append(items, diskstore.Item{Class: r.Name(), Key: "~named", Path: filepath.Join(r.Root, ".named"), Verdict: diskstore.Verdict{Decision: diskstore.Pending,
			Reason: "the named entries cannot all be read (" + err.Error() + "); no unit is released this pass", Command: "metasystem work status"}}), nil
	}
	sort.Slice(units, func(i, j int) bool {
		if !units[i].ended.Equal(units[j].ended) {
			return units[i].ended.Before(units[j].ended)
		}
		return units[i].id < units[j].id
	})
	over := r.bytes - r.Target
	for _, unit := range units {
		if over <= 0 || ctx.Err() != nil {
			break
		}
		verdict := r.judge(unit, pass.Now, false)
		if verdict.Decision == "" {
			continue
		}
		items = append(items, diskstore.Item{Class: r.Name(), Key: unit.id, Path: unit.path, Bytes: unit.bytes, Verdict: verdict})
		if verdict.Decision == diskstore.Release {
			over -= unit.bytes
		}
	}
	return items, nil
}

// judge is the unit proof; the empty decision is a young unit, which has
// no line.
func (r *UnitRetention) judge(unit unitEntry, now time.Time, holdingRun bool) diskstore.Verdict {
	if unit.ended.IsZero() || now.Sub(unit.ended) < r.Keep {
		return diskstore.Verdict{}
	}
	goal := unit.record.Goal
	ended, known := false, false
	if goal != "" && r.GoalEnded != nil {
		ended, known = r.GoalEnded(goal, unit.record.Worktree)
	}
	switch {
	case !known:
		return diskstore.Verdict{Decision: diskstore.Keep, Reason: "whether goal " + goal + " has concluded cannot be read in the checkout the unit names; it is kept",
			Command: "metasystem goal show " + goal}
	case !ended:
		return diskstore.Verdict{Decision: diskstore.Keep, Reason: "its goal is open; a unit of an open goal stays whatever its age",
			Command: "metasystem goal show " + goal}
	}
	if held := r.heldLock(unit.id, holdingRun); held != "" {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: held, Command: "metasystem disk clean, once the review or revision has ended"}
	}
	return diskstore.Verdict{Decision: diskstore.Release, Reason: "goal " + goal + " concluded, past disk.unit-keep-days, the store over its target"}
}

// heldLock probes the unit's run lock (unless the caller holds it) and the
// named lock of every entry that reaches it, without creating either; it
// names a held one.
func (r *UnitRetention) heldLock(id string, holdingRun bool) string {
	var locks []string
	if !holdingRun {
		locks = append(locks, filepath.Join(r.Root, id, ".lock"))
	}
	named, err := r.namedEntries(id)
	if err != nil {
		return "the named entries cannot all be read (" + err.Error() + ")"
	}
	for _, entry := range named {
		locks = append(locks, strings.TrimSuffix(entry, ".json")+".lock")
	}
	for _, path := range locks {
		file, err := os.OpenFile(path, os.O_RDONLY, 0)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "unit lock " + path + " unreadable: " + err.Error()
		}
		err = unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
		}
		_ = file.Close()
		if err != nil {
			return "a review or revision holds " + path
		}
	}
	return ""
}

// namedEntries are the named entries whose run is id; a named entry that
// cannot be read is an error, because which run it reaches is unknown.
func (r *UnitRetention) namedEntries(id string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(r.Root, ".named"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var found []string
	for _, dirEntry := range entries {
		if !strings.HasSuffix(dirEntry.Name(), ".json") {
			continue
		}
		path := filepath.Join(r.Root, ".named", dirEntry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var entry namedUnitEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			return nil, fmt.Errorf("named entry %s: %w", path, err)
		}
		if entry.Run == id {
			found = append(found, path)
		}
	}
	return found, nil
}

// Apply holds the unit's run lock for the removal, so a Continue or Revise
// that arrives meanwhile reads UNIT_RUN_BUSY; it rereads and rejudges, sets
// the directory aside, removes each named entry that reached it under its
// own lock, then removes the directory.
func (r *UnitRetention) Apply(ctx context.Context, pass *diskstore.Pass, item diskstore.Item) diskstore.Verdict {
	if strings.HasPrefix(item.Key, releasedPrefix) {
		return removeReleased(ctx, item.Path)
	}
	path := filepath.Join(r.Root, item.Key)
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return diskstore.Verdict{Decision: diskstore.Release, Reason: "already gone"}
	}
	held, err := lock.File(filepath.Join(path, ".lock"), 0o600, lock.TryExclusive)
	if err != nil {
		if lock.Busy(err) {
			return diskstore.Verdict{Decision: diskstore.Pending, Reason: "a review or revision holds the unit", Command: "metasystem disk clean, once it has ended"}
		}
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "unit lock: " + err.Error(), Command: "metasystem disk show"}
	}
	defer held.Release()
	record, found, err := readUnitRecord(path)
	if err != nil || !found {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: fmt.Sprintf("unit record unreadable: %v", err), Command: "metasystem work status"}
	}
	unit := unitEntry{id: item.Key, path: path, ended: unitEnded(record, path), record: record}
	verdict := r.judge(unit, pass.Now, true)
	if verdict.Decision != diskstore.Release {
		if verdict.Decision == "" {
			verdict = diskstore.Verdict{Decision: diskstore.Keep, Reason: "the unit changed since the plan", Command: "metasystem disk show"}
		}
		return verdict
	}
	named, err := r.namedEntries(item.Key)
	if err != nil {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "the named entries cannot all be read: " + err.Error(), Command: "metasystem work status"}
	}
	for _, entry := range named {
		named, err := lock.File(strings.TrimSuffix(entry, ".json")+".lock", 0o600, lock.TryExclusive)
		if err != nil {
			return diskstore.Verdict{Decision: diskstore.Pending, Reason: "the unit's name is being advanced", Command: "metasystem disk clean, once that has ended"}
		}
		err = os.Remove(entry)
		_ = named.Release()
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return diskstore.Verdict{Decision: diskstore.Pending, Reason: "named entry " + entry + ": " + err.Error(), Command: "metasystem disk show"}
		}
	}
	aside := filepath.Join(r.Root, releasedPrefix+item.Key)
	if err := os.Rename(path, aside); err != nil {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "the unit could not be set aside: " + err.Error(), Command: "metasystem disk show"}
	}
	if verdict := removeReleased(ctx, aside); verdict.Decision != diskstore.Release {
		return verdict
	}
	return diskstore.Verdict{Decision: diskstore.Release, Reason: "unit of a concluded goal past its window; removed"}
}

func readUnitRecord(dir string) (UnitRunRecord, bool, error) {
	data, err := os.ReadFile(filepath.Join(dir, "run.json"))
	if errors.Is(err, os.ErrNotExist) {
		return UnitRunRecord{}, false, nil
	}
	if err != nil {
		return UnitRunRecord{}, false, err
	}
	var record UnitRunRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return UnitRunRecord{}, false, err
	}
	return record, true, nil
}

// unitEnded is the latest time any step of the unit started or finished;
// a unit with no step time reads its record file's modification time.
func unitEnded(record UnitRunRecord, dir string) time.Time {
	var latest time.Time
	for _, round := range record.Rounds {
		for _, step := range round.Steps {
			for _, stamp := range []string{step.StartedAt, step.FinishedAt} {
				if parsed, err := time.Parse(time.RFC3339Nano, stamp); err == nil && parsed.After(latest) {
					latest = parsed
				}
			}
		}
	}
	if latest.IsZero() {
		if info, err := os.Stat(filepath.Join(dir, "run.json")); err == nil {
			latest = info.ModTime()
		}
	}
	return latest
}
