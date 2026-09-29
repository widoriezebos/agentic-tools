package launch

// The launch store's retention (design engine-owns-disk-lifetimes Part B,
// 3.1 "launch", 3.5 "Launch store and unit records", U5d). A launch record
// is its own record and its directory's .lock its own record lock, so the
// sweeper adopts nothing: this class observes the store in the machine
// pass and removes, oldest first, only what the launch proof releases and
// only while the store is over disk.launch-target-mib. The window alone
// never removes a launch.

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// releasedPrefix names a launch directory whose release began: renamed
// under its record lock after the proof held, so no listing reads it and
// the next pass finishes a removal the budget cut short.
const releasedPrefix = ".released-"

// Retention is the launch store's class in the machine pass.
type Retention struct {
	// Manager supplies the store, the prober and the process groups.
	Manager *Manager
	// UnitRoot is the unit store beside the launch store: a launch any unit
	// record's round names is a retention root while that record exists.
	UnitRoot string
	// Target is disk.launch-target-mib in bytes; Keep is
	// disk.launch-keep-days.
	Target int64
	Keep   time.Duration

	count int
	bytes int64
	named map[string]bool
	// mentions is every launch-id-shaped token in the JSON files under the
	// unit store and the design store, read at the plan, with each file's
	// modification time, so the apply rereads only what changed.
	mentions map[string]bool
	seen     map[string]time.Time
}

// Name names the class.
func (*Retention) Name() string { return "launch store" }

// Totals are every launch and its bytes, for the class line.
func (r *Retention) Totals() (int, int64) { return r.count, r.bytes }

type launchEntry struct {
	id       string
	path     string
	bytes    int64
	finished time.Time
	record   Record
}

// Plan observes the store: it reads every record and the unit records,
// measures, and names the oldest launches whose proof holds until the
// store would be under its target. It writes nothing.
func (r *Retention) Plan(ctx context.Context, pass *diskstore.Pass) ([]diskstore.Item, error) {
	r.count, r.bytes, r.named = 0, 0, nil
	root, err := r.Manager.Store.root()
	if err != nil {
		return nil, err
	}
	dirEntries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var items []diskstore.Item
	var launches []launchEntry
	for _, entry := range dirEntries {
		if ctx.Err() != nil {
			break
		}
		name, path := entry.Name(), filepath.Join(root, entry.Name())
		if !entry.IsDir() {
			continue
		}
		if id, ok := strings.CutPrefix(name, releasedPrefix); ok && idPattern.MatchString(id) {
			bytes, _, _ := diskstore.Measure(ctx, path)
			r.bytes += bytes
			items = append(items, diskstore.Item{Class: r.Name(), Key: name, Path: path, Bytes: bytes,
				Verdict: diskstore.Verdict{Decision: diskstore.Release, Reason: "a launch release the budget cut short is finished"}})
			continue
		}
		if !idPattern.MatchString(name) || name == "declared-output-locks" {
			continue
		}
		bytes, _, _ := diskstore.Measure(ctx, path)
		r.count++
		r.bytes += bytes
		record, readErr := r.Manager.Store.Read(name)
		if readErr != nil {
			items = append(items, diskstore.Item{Class: r.Name(), Key: name, Path: path, Bytes: bytes,
				Verdict: diskstore.Verdict{Decision: diskstore.Pending, Reason: "launch record unreadable: " + readErr.Error(), Command: "metasystem work status"}})
			continue
		}
		launches = append(launches, launchEntry{id: name, path: path, bytes: bytes, finished: endedAt(record), record: record})
	}
	if r.bytes <= r.Target {
		return items, nil
	}
	named, unitErr := namedLaunches(r.UnitRoot, root)
	if unitErr == nil {
		r.mentions, r.seen = map[string]bool{}, map[string]time.Time{}
		unitErr = r.readMentions(root, false)
	}
	if unitErr != nil {
		return append(items, diskstore.Item{Class: r.Name(), Key: "~units", Path: r.UnitRoot,
			Verdict: diskstore.Verdict{Decision: diskstore.Pending, Reason: "which launches the unit records name is unknown (" + unitErr.Error() + "); no launch is released",
				Command: "metasystem work status"}}), nil
	}
	r.named = named
	sort.Slice(launches, func(i, j int) bool {
		if !launches[i].finished.Equal(launches[j].finished) {
			return launches[i].finished.Before(launches[j].finished)
		}
		return launches[i].id < launches[j].id
	})
	over, protected := r.bytes-r.Target, int64(0)
	for _, launch := range launches {
		if over <= 0 || ctx.Err() != nil {
			break
		}
		verdict := r.judge(launch.record, pass.Now)
		if verdict.Decision == "" {
			protected += launch.bytes
			continue
		}
		item := diskstore.Item{Class: r.Name(), Key: launch.id, Path: launch.path, Bytes: launch.bytes, Verdict: verdict}
		items = append(items, item)
		if verdict.Decision == diskstore.Release {
			over -= launch.bytes
		}
	}
	if over > 0 {
		items = append(items, diskstore.Item{Class: r.Name(), Key: "~target", Path: root, Verdict: diskstore.Verdict{Decision: diskstore.Keep,
			Reason:  fmt.Sprintf("the launch store is over its target of %d MiB after every release its proofs allow; what remains is younger than disk.launch-keep-days, running, or named below", r.Target>>20),
			Command: "metasystem disk show"}})
	}
	return items, nil
}

// judge is the launch proof (3.1): terminal, ended before the keep window,
// named by no unit record, outputs not unproven, the process group proven
// ended, and every recorded process proven dead. A launch the window or a
// running state protects has no line (the empty decision); every other
// kept launch names its reason and command.
func (r *Retention) judge(record Record, now time.Time) diskstore.Verdict {
	ended := endedAt(record)
	switch {
	case !record.State.Terminal(), ended.IsZero(), now.Sub(ended) < r.Keep:
		return diskstore.Verdict{}
	case r.named[record.ID] || r.mentions[record.ID]:
		return diskstore.Verdict{Decision: diskstore.Keep, Reason: "a unit record's round names this launch; it stays while that unit record exists",
			Command: "metasystem work status"}
	case record.OutputOwnerUnproven || strings.Contains(record.Reason, "process-group-unproven"):
		return diskstore.Verdict{Decision: diskstore.Keep, Reason: "its process group was never proven ended, so its declared outputs stay protected",
			Command: "metasystem work stop j1:" + record.ID}
	case !r.Manager.provenDead(record):
		return diskstore.Verdict{Decision: diskstore.Keep, Reason: "a recorded process or its process group is still alive or cannot be read",
			Command: "metasystem work stop j1:" + record.ID}
	}
	return diskstore.Verdict{Decision: diskstore.Release, Reason: "ended, past disk.launch-keep-days, the store over its target"}
}

// Apply is one launch's critical section: its own record lock taken
// without waiting, the record reloaded and the proof re-run, the
// directory renamed aside under the lock, then removed; a removal the
// budget cuts short is finished by the next pass.
func (r *Retention) Apply(ctx context.Context, pass *diskstore.Pass, item diskstore.Item) diskstore.Verdict {
	if strings.HasPrefix(item.Key, releasedPrefix) {
		return removeReleased(ctx, item.Path)
	}
	dir, err := r.Manager.Store.StateDir(item.Key)
	if err != nil {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: err.Error(), Command: "metasystem disk show"}
	}
	if _, err := os.Lstat(dir); errors.Is(err, os.ErrNotExist) {
		return diskstore.Verdict{Decision: diskstore.Release, Reason: "already gone"}
	}
	held, err := lock.File(filepath.Join(dir, ".lock"), 0o600, lock.TryExclusive)
	if err != nil {
		if lock.Busy(err) {
			return diskstore.Verdict{Decision: diskstore.Pending, Reason: "a launch verb holds the launch's record lock", Command: "metasystem disk clean, once that verb has ended"}
		}
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "launch record lock: " + err.Error(), Command: "metasystem disk show"}
	}
	defer held.Release()
	record, err := r.Manager.Store.Read(item.Key)
	if err != nil {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "launch record unreadable: " + err.Error(), Command: "metasystem work status"}
	}
	if root, err := r.Manager.Store.root(); err != nil || r.readMentions(root, true) != nil {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "the records that may name this launch cannot all be read", Command: "metasystem work status"}
	}
	if verdict := r.judge(record, pass.Now); verdict.Decision != diskstore.Release {
		if verdict.Decision == "" {
			verdict = diskstore.Verdict{Decision: diskstore.Keep, Reason: "the launch changed since the plan", Command: "metasystem disk show"}
		}
		return verdict
	}
	aside := filepath.Join(filepath.Dir(dir), releasedPrefix+item.Key)
	if err := os.Rename(dir, aside); err != nil {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "launch directory could not be set aside: " + err.Error(), Command: "metasystem disk show"}
	}
	if verdict := removeReleased(ctx, aside); verdict.Decision != diskstore.Release {
		return verdict
	}
	return diskstore.Verdict{Decision: diskstore.Release, Reason: "launch ended past its window; removed"}
}

func removeReleased(ctx context.Context, path string) diskstore.Verdict {
	if err := diskstore.RemoveTree(ctx, path); err != nil {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "removal cut short (" + err.Error() + "); the next pass finishes it", Command: "metasystem disk clean"}
	}
	return diskstore.Verdict{Decision: diskstore.Release, Reason: "an interrupted launch release finished"}
}

// endedAt is when a launch ended: FinishedAt, else StartedAt, else zero
// (a launch with neither is never old enough to release).
func endedAt(record Record) time.Time {
	for _, stamp := range []string{record.FinishedAt, record.StartedAt} {
		if parsed, err := time.Parse(time.RFC3339Nano, stamp); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

// namedLaunches reads the records that name launches, each by its type:
// every unit record's rounds, every standalone read attempt's round
// (unit/.reads/<ref>/attempt-N/attempt.json) and every retained design
// request's attempts (launch/.design/<key>/request.json). A unit directory
// without its run.json names none yet (its launches are newer than
// itself); any other record that cannot be read is an error, because which
// launches it names is then unknown.
func namedLaunches(unitRoot, launchRoot string) (map[string]bool, error) {
	named := map[string]bool{}
	steps := func(round UnitRound) {
		for _, step := range round.Steps {
			if step.LaunchID != "" {
				named[step.LaunchID] = true
			}
		}
	}
	entries, err := os.ReadDir(unitRoot)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || !idPattern.MatchString(entry.Name()) {
			continue
		}
		path := filepath.Join(unitRoot, entry.Name(), "run.json")
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var record UnitRunRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("unit record %s: %w", path, err)
		}
		for _, round := range record.Rounds {
			steps(round)
		}
	}
	reads, err := filepath.Glob(filepath.Join(unitRoot, ".reads", "*", "*", "attempt.json"))
	if err != nil {
		return nil, err
	}
	for _, path := range reads {
		var attempt ReadAttempt
		if err := readJSONFile(path, &attempt); err != nil {
			return nil, err
		}
		steps(attempt.Round)
	}
	designs, err := filepath.Glob(filepath.Join(launchRoot, ".design", "*", "request.json"))
	if err != nil {
		return nil, err
	}
	for _, path := range designs {
		var entry designEntry
		if err := readJSONFile(path, &entry); err != nil {
			return nil, err
		}
		for _, attempt := range entry.Attempts {
			if attempt.LaunchID != "" {
				named[attempt.LaunchID] = true
			}
		}
	}
	return named, nil
}

func readJSONFile(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, value); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// launchToken is a launch id's shape inside any text.
var launchToken = regexp.MustCompile(`[a-z0-9][a-z0-9-]{0,63}`)

// readMentions is the backstop behind the typed readers (Round B3-4): it
// collects every launch-id-shaped token from every JSON file under the
// unit store and the design store, so a launch any of them mentions, in
// any field, is kept. With changedOnly it rereads only files new or
// changed since the plan. A file or directory that cannot be read is an
// error, which holds the class.
func (r *Retention) readMentions(launchRoot string, changedOnly bool) error {
	for _, dir := range []string{r.UnitRoot, filepath.Join(launchRoot, ".design")} {
		err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, os.ErrNotExist) && path == dir {
					return filepath.SkipDir
				}
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".json") {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if seen, ok := r.seen[path]; changedOnly && ok && seen.Equal(info.ModTime()) {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			r.seen[path] = info.ModTime()
			for _, token := range launchToken.FindAll(data, -1) {
				r.mentions[string(token)] = true
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// compressFinishedLog compresses the log of a launch that ended with its
// process group proven ended; a launch whose group or outputs are unproven
// keeps its log exactly as written.
func compressFinishedLog(record Record, path string, threshold int64) {
	if !record.State.Terminal() || record.OutputOwnerUnproven || strings.Contains(record.Reason, "process-group-unproven") {
		return
	}
	compressLog(path, threshold)
}

// compressLog replaces a finished launch's log at or above threshold bytes
// by its gzip: the stream is staged beside it, synced, read back and
// compared with the original's digest, renamed into place, and only then
// is the original removed. Any failure leaves the original as it was; an
// existing .gz is never overwritten.
func compressLog(path string, threshold int64) {
	info, err := os.Lstat(path)
	if path == "" || threshold <= 0 || err != nil || !info.Mode().IsRegular() || info.Size() < threshold {
		return
	}
	final, stage := path+".gz", path+".gz.partial"
	if _, err := os.Lstat(final); err == nil {
		return
	}
	want, err := fileDigest(path, false)
	if err != nil {
		return
	}
	if err := writeGzip(path, stage); err != nil {
		_ = os.Remove(stage)
		return
	}
	if got, err := fileDigest(stage, true); err != nil || got != want {
		_ = os.Remove(stage)
		return
	}
	if err := os.Rename(stage, final); err != nil {
		_ = os.Remove(stage)
		return
	}
	if err := os.Remove(path); err == nil {
		syncDir(filepath.Dir(path))
	}
}

func writeGzip(source, stage string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(stage, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	writer := gzip.NewWriter(out)
	_, copyErr := io.Copy(writer, in)
	return errors.Join(copyErr, writer.Close(), out.Sync(), out.Close())
}

func fileDigest(path string, gzipped bool) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	var reader io.Reader = file
	if gzipped {
		inflated, err := gzip.NewReader(file)
		if err != nil {
			return "", err
		}
		defer inflated.Close()
		reader = inflated
	}
	sum := sha256.New()
	if _, err := io.Copy(sum, reader); err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

func syncDir(path string) {
	if dir, err := os.Open(path); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
}
