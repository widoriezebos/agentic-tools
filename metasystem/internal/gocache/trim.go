package gocache

// The cache trimmer (disk-lifetimes Part A, 3.5 and unit A12). Go trims its
// build cache only of entries unused for five days and has no size bound;
// staticcheck's cache is a vendored copy of the same design. The steward
// trims each machine cache by last use to a hard cap:
//
//   - never inside the keep window: an entry whose mtime (Go's best-effort
//     record of last use, refreshed at most hourly) is younger than the
//     window is never deleted, so a build in flight keeps what it reads;
//   - handle-relative: the root and every shard are opened O_NOFOLLOW and
//     checked against the Lstat taken just before, and every entry
//     operation is relative to the shard handle (Fstatat, Unlinkat,
//     Renameat, Openat); a symlink at the root or a shard position refuses
//     the cache with nothing touched;
//   - whole or not at all: a directory-form -d entry (a cached executable)
//     is Known only when it holds exactly one regular child, is measured by
//     that child's bytes, and goes through an aside name <name>.trim that
//     every later pass finishes; anything else is Unknown, reported, and
//     never renamed or partly deleted;
//   - bounded and resumable: names are listed once per shard, sorted, and
//     stat'ed in batches with a checkpoint after every batch; eviction
//     candidates are persisted, so a later pass evicts without measuring
//     again; the budget and cancellation are checked between batches;
//   - one trimmer per cache: a nonblocking flock; a loser skips.
//
// The accepted failure: an entry deleted under a build that already looked
// it up fails that build with an error (Go's cache gives no availability
// guarantee after Get), never a wrong result; a rerun rebuilds it.

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const (
	// trimBatch is how many entries one batch stats or evicts between two
	// checkpoints and two budget checks.
	trimBatch = 256
	// maxUnknownReported bounds the Unknown list a report carries; the
	// count is always whole.
	maxUnknownReported = 200
	// asideSuffix names an entry being removed: no -a/-d suffix, so no Go
	// reader finds it and Go's own trim skips it.
	asideSuffix = ".trim"
)

// TrimConfig is one trim pass over one cache.
type TrimConfig struct {
	// Name names the cache in the state directory: its flock, report and
	// candidates files.
	Name string
	// Root is the cache directory.
	Root string
	// CapBytes is the hard cap the pass trims to.
	CapBytes int64
	// Keep is the window inside which no entry is ever deleted.
	Keep time.Duration
	// StateDir holds <Name>.flock, <Name>.json and <Name>.candidates.jsonl.
	StateDir string
	// Now is the pass's time: the keep window is measured from it.
	Now time.Time
	// Clock reads the time for the report's timestamp and duration; nil is
	// Now, unchanging.
	Clock func() time.Time
	// Stopped is asked between batches; true ends the pass as cancelled.
	Stopped func() bool

	hooks trimHooks
}

type trimHooks struct {
	afterMeasure func()
	beforeStat   func(shard, name string)
	beforeRemove func(shard, name string)
}

// TrimCheckpoint is where an unfinished measurement resumes: the shard, the
// last name stat'ed in it, and the Known bytes counted so far.
type TrimCheckpoint struct {
	Shard      string `json:"shard"`
	LastName   string `json:"lastName"`
	BytesSoFar int64  `json:"bytesSoFar"`
}

// UnknownEntry is an entry the trimmer could not classify and never
// touched, by shard/name, with the reason.
type UnknownEntry struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// TrimReport is one pass's report, persisted as <StateDir>/<Name>.json. The
// measurement fields (checkpoint, bytes before, keep window, Unknown) belong
// to the current measurement and carry across passes; the removal counters
// are this pass's.
type TrimReport struct {
	Cache      string  `json:"cache"`
	Root       string  `json:"root"`
	CapBytes   int64   `json:"capBytes"`
	KeepHours  float64 `json:"keepHours"`
	PassAt     string  `json:"passAt"`
	DurationMS int64   `json:"durationMs"`
	// EndedBy is complete, budget, cancelled, lock-held, refused or absent.
	EndedBy string `json:"endedBy"`
	Reason  string `json:"reason,omitempty"`
	// Phase is measure (a measurement is under way), evict (a measurement
	// found the cache over its cap and candidates wait) or idle.
	Phase             string         `json:"phase"`
	Checkpoint        TrimCheckpoint `json:"checkpoint"`
	MeasuredAt        string         `json:"measuredAt,omitempty"`
	BytesBefore       int64          `json:"bytesBefore"`
	BytesAfter        int64          `json:"bytesAfter"`
	KeepWindowEntries int            `json:"keepWindowEntries"`
	KeepWindowBytes   int64          `json:"keepWindowBytes"`
	EntriesRemoved    int            `json:"entriesRemoved"`
	BytesRemoved      int64          `json:"bytesRemoved"`
	AsidesFinished    int            `json:"asidesFinished"`
	MovedSkips        int            `json:"movedSkips"`
	Unknown           []UnknownEntry `json:"unknown"`
	UnknownCount      int            `json:"unknownCount"`
}

// trimCandidate is an entry older than the keep window, as measured.
type trimCandidate struct {
	Shard   string `json:"shard"`
	Name    string `json:"name"`
	MtimeNS int64  `json:"mtimeNs"`
	Size    int64  `json:"size"`
}

// trimRefusal ends a pass with nothing more touched.
type trimRefusal struct{ reason string }

func (r trimRefusal) Error() string { return r.reason }

var cacheNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

// Trim runs one pass over cfg.Root. It returns an error only when the pass
// could not run at all (a bad configuration, an unwritable state
// directory); a refused, skipped or cut-short pass is a report.
func Trim(ctx context.Context, cfg TrimConfig) (TrimReport, error) {
	clock := cfg.Clock
	if clock == nil {
		clock = func() time.Time { return cfg.Now }
	}
	started := clock()
	report := TrimReport{Cache: cfg.Name, Root: cfg.Root, CapBytes: cfg.CapBytes, KeepHours: cfg.Keep.Hours(), Unknown: []UnknownEntry{}}
	if !cacheNamePattern.MatchString(cfg.Name) || !filepath.IsAbs(cfg.Root) || !filepath.IsAbs(cfg.StateDir) || cfg.CapBytes < 1 || cfg.Keep <= 0 {
		return report, fmt.Errorf("trim %q: needs a cache name, an absolute root and state directory, a positive cap and a positive keep window", cfg.Name)
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		return report, err
	}
	lockFile, err := os.OpenFile(filepath.Join(cfg.StateDir, cfg.Name+".flock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return report, err
	}
	defer lockFile.Close()
	if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) {
			report.EndedBy, report.Reason, report.Phase = "lock-held", "another steward trims", "idle"
			report.PassAt = clock().UTC().Format(time.RFC3339Nano)
			return report, nil
		}
		return report, err
	}
	p := &trimPass{cfg: cfg, report: &report, cutoff: cfg.Now.Add(-cfg.Keep)}
	p.carry(p.readReport())
	p.run(ctx)
	ended := clock()
	report.PassAt = ended.UTC().Format(time.RFC3339Nano)
	report.DurationMS = ended.Sub(started).Milliseconds()
	if err := p.persist(); err != nil {
		return report, err
	}
	return report, nil
}

type trimPass struct {
	cfg    TrimConfig
	report *TrimReport
	cutoff time.Time
	root   *os.File
}

func (p *trimPass) statePath(suffix string) string {
	return filepath.Join(p.cfg.StateDir, p.cfg.Name+suffix)
}

func (p *trimPass) readReport() TrimReport {
	var previous TrimReport
	data, err := os.ReadFile(p.statePath(".json"))
	if err != nil || json.Unmarshal(data, &previous) != nil {
		return TrimReport{}
	}
	return previous
}

// carry continues the previous pass's measurement or eviction of the same
// cache, else starts a new measurement.
func (p *trimPass) carry(previous TrimReport) {
	r := p.report
	if previous.Cache == p.cfg.Name && previous.Root == p.cfg.Root && (previous.Phase == "measure" || previous.Phase == "evict") {
		r.Phase, r.Checkpoint, r.MeasuredAt = previous.Phase, previous.Checkpoint, previous.MeasuredAt
		r.BytesBefore, r.BytesAfter = previous.BytesBefore, previous.BytesAfter
		r.KeepWindowEntries, r.KeepWindowBytes = previous.KeepWindowEntries, previous.KeepWindowBytes
		r.Unknown, r.UnknownCount = previous.Unknown, previous.UnknownCount
		if r.Unknown == nil {
			r.Unknown = []UnknownEntry{}
		}
		if r.Phase == "measure" && r.Checkpoint.Shard == "" {
			r.Checkpoint.Shard = shardName(0)
		}
		return
	}
	p.startMeasure()
}

func (p *trimPass) startMeasure() {
	r := p.report
	r.Phase, r.Checkpoint, r.MeasuredAt = "measure", TrimCheckpoint{Shard: shardName(0)}, ""
	r.BytesBefore, r.BytesAfter, r.KeepWindowEntries, r.KeepWindowBytes = 0, 0, 0, 0
	r.Unknown, r.UnknownCount = []UnknownEntry{}, 0
	p.writeCandidates(nil)
}

func (p *trimPass) run(ctx context.Context) {
	root, absent, err := openVerifiedRoot(p.cfg.Root)
	if absent {
		p.report.EndedBy, p.report.Phase = "absent", "idle"
		p.report.Checkpoint = TrimCheckpoint{}
		return
	}
	if err == nil {
		defer root.Close()
		p.root = root
		err = p.checkShardPositions()
	}
	if err == nil && p.report.Phase == "measure" {
		var stop string
		stop, err = p.measure(ctx)
		if err == nil && stop != "" {
			p.report.EndedBy = stop
			return
		}
	}
	if err == nil && p.report.Phase == "evict" {
		var stop string
		stop, err = p.evict(ctx)
		if err == nil && stop != "" {
			p.report.EndedBy = stop
			return
		}
	}
	var refusal trimRefusal
	if errors.As(err, &refusal) {
		p.report.EndedBy, p.report.Reason = "refused", refusal.reason
		return
	}
	if err != nil {
		p.report.EndedBy, p.report.Reason = "refused", err.Error()
		return
	}
	p.report.EndedBy = "complete"
}

// interrupted names what ends the pass here, if anything: the budget (the
// context's deadline), a cancellation, or the stop question at a batch
// boundary.
func (p *trimPass) interrupted(ctx context.Context, batchBoundary bool) string {
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "budget"
		}
		return "cancelled"
	}
	if batchBoundary && p.cfg.Stopped != nil && p.cfg.Stopped() {
		return "cancelled"
	}
	return ""
}

func shardName(index int) string { return fmt.Sprintf("%02x", index) }

// checkShardPositions refuses the cache when any of the 256 shard
// positions holds anything but a directory, before anything is touched.
func (p *trimPass) checkShardPositions() error {
	for index := 0; index < 256; index++ {
		var st unix.Stat_t
		err := unix.Fstatat(int(p.root.Fd()), shardName(index), &st, unix.AT_SYMLINK_NOFOLLOW)
		if errors.Is(err, unix.ENOENT) {
			continue
		}
		if err != nil {
			return trimRefusal{fmt.Sprintf("shard %s cannot be read: %v", shardName(index), err)}
		}
		if kind := fileKind(st); kind != unix.S_IFDIR {
			return trimRefusal{fmt.Sprintf("shard %s is %s, not a directory: the cache is not Go's layout and nothing was touched", shardName(index), kindName(kind))}
		}
	}
	return nil
}

func fileKind(st unix.Stat_t) uint32 { return uint32(st.Mode) & unix.S_IFMT }

func kindName(kind uint32) string {
	switch kind {
	case unix.S_IFLNK:
		return "a symbolic link"
	case unix.S_IFREG:
		return "a regular file"
	case unix.S_IFDIR:
		return "a directory"
	}
	return "a special file"
}

func sameFile(a, b unix.Stat_t) bool {
	return uint64(a.Dev) == uint64(b.Dev) && uint64(a.Ino) == uint64(b.Ino)
}

// openVerifiedRoot opens the cache root without following a symlink and
// confirms the handle is the directory the path named a moment before.
func openVerifiedRoot(root string) (*os.File, bool, error) {
	var before unix.Stat_t
	if err := unix.Lstat(root, &before); err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil, true, nil
		}
		return nil, false, trimRefusal{fmt.Sprintf("cache root %s cannot be read: %v", root, err)}
	}
	if kind := fileKind(before); kind != unix.S_IFDIR {
		return nil, false, trimRefusal{fmt.Sprintf("cache root %s is %s, not a directory: nothing was touched", root, kindName(kind))}
	}
	fd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, false, trimRefusal{fmt.Sprintf("cache root %s cannot be opened without following links: %v", root, err)}
	}
	return verifiedHandle(fd, root, before)
}

// openShard opens one shard of the verified root through the root handle;
// exists is false when the shard is absent.
func (p *trimPass) openShard(shard string) (*os.File, bool, error) {
	var before unix.Stat_t
	rootFd := int(p.root.Fd())
	if err := unix.Fstatat(rootFd, shard, &before, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil, false, nil
		}
		return nil, false, trimRefusal{fmt.Sprintf("shard %s cannot be read: %v", shard, err)}
	}
	if kind := fileKind(before); kind != unix.S_IFDIR {
		return nil, false, trimRefusal{fmt.Sprintf("shard %s is %s, not a directory: nothing more was touched", shard, kindName(kind))}
	}
	fd, err := unix.Openat(rootFd, shard, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, false, trimRefusal{fmt.Sprintf("shard %s cannot be opened without following links: %v", shard, err)}
	}
	file, _, err := verifiedHandle(fd, shard, before)
	return file, err == nil, err
}

func verifiedHandle(fd int, name string, before unix.Stat_t) (*os.File, bool, error) {
	var after unix.Stat_t
	if err := unix.Fstat(fd, &after); err != nil || fileKind(after) != unix.S_IFDIR || !sameFile(before, after) {
		_ = unix.Close(fd)
		return nil, false, trimRefusal{fmt.Sprintf("%s changed between its stat and its open: nothing more was touched", name)}
	}
	return os.NewFile(uintptr(fd), name), false, nil
}

// shardNames lists a shard's entry and aside names, sorted.
func shardNames(shard *os.File) ([]string, error) {
	names, err := shard.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	var kept []string
	for _, name := range names {
		if isEntryName(name) || isAsideName(name) {
			kept = append(kept, name)
		}
	}
	sort.Strings(kept)
	return kept, nil
}

// isEntryName: Go's cache entries end in -a (action) or -d (output).
func isEntryName(name string) bool {
	return strings.HasSuffix(name, "-a") || strings.HasSuffix(name, "-d")
}

func isAsideName(name string) bool { return strings.HasSuffix(name, "-d"+asideSuffix) }

func (p *trimPass) unknown(shard, name, reason string) {
	p.report.UnknownCount++
	if len(p.report.Unknown) < maxUnknownReported {
		p.report.Unknown = append(p.report.Unknown, UnknownEntry{Name: shard + "/" + name, Reason: reason})
	}
}

// measure stats every entry from the checkpoint on, batch by batch, and
// ends in the evict phase when the cache is over its cap, else idle.
func (p *trimPass) measure(ctx context.Context) (string, error) {
	r := p.report
	start := 0
	if _, err := fmt.Sscanf(r.Checkpoint.Shard, "%02x", &start); err != nil || start < 0 || start > 255 {
		start = 0
		r.Checkpoint = TrimCheckpoint{Shard: shardName(0), BytesSoFar: r.Checkpoint.BytesSoFar}
	}
	for index := start; index < 256; index++ {
		if stop := p.interrupted(ctx, false); stop != "" {
			return stop, nil
		}
		shard := shardName(index)
		handle, exists, err := p.openShard(shard)
		if err != nil {
			return "", err
		}
		if exists {
			stop, err := p.measureShard(ctx, handle, shard)
			handle.Close()
			if err != nil || stop != "" {
				return stop, err
			}
		}
		r.Checkpoint = TrimCheckpoint{Shard: shardName(index + 1), BytesSoFar: r.Checkpoint.BytesSoFar}
	}
	r.BytesBefore, r.BytesAfter = r.Checkpoint.BytesSoFar, r.Checkpoint.BytesSoFar
	r.Checkpoint = TrimCheckpoint{}
	r.MeasuredAt = p.cfg.Now.UTC().Format(time.RFC3339Nano)
	if r.BytesBefore > p.cfg.CapBytes {
		r.Phase = "evict"
		if err := p.planEviction(); err != nil {
			return "", err
		}
	} else {
		r.Phase = "idle"
		p.writeCandidates(nil)
	}
	if err := p.persist(); err != nil {
		return "", err
	}
	if p.cfg.hooks.afterMeasure != nil {
		p.cfg.hooks.afterMeasure()
	}
	return p.interrupted(ctx, true), nil
}

func (p *trimPass) measureShard(ctx context.Context, handle *os.File, shard string) (string, error) {
	r := p.report
	names, err := shardNames(handle)
	if err != nil {
		p.unknown(shard, "", "the shard cannot be listed: "+err.Error())
		return "", nil
	}
	from := 0
	if shard == r.Checkpoint.Shard && r.Checkpoint.LastName != "" {
		from = sort.SearchStrings(names, r.Checkpoint.LastName)
		if from < len(names) && names[from] == r.Checkpoint.LastName {
			from++
		}
	}
	for begin := from; begin < len(names); begin += trimBatch {
		batch := names[begin:min(begin+trimBatch, len(names))]
		var found []trimCandidate
		for _, name := range batch {
			if candidate, ok := p.measureEntry(handle, shard, name); ok {
				found = append(found, candidate)
			}
		}
		if err := p.appendCandidates(found); err != nil {
			return "", err
		}
		r.Checkpoint = TrimCheckpoint{Shard: shard, LastName: batch[len(batch)-1], BytesSoFar: r.Checkpoint.BytesSoFar}
		if err := p.persist(); err != nil {
			return "", err
		}
		if stop := p.interrupted(ctx, true); stop != "" {
			return stop, nil
		}
	}
	return "", nil
}

// measureEntry classifies one name through the shard handle: an aside is
// finished; a Known entry counts its payload bytes and is a candidate when
// it is older than the keep window.
func (p *trimPass) measureEntry(handle *os.File, shard, name string) (trimCandidate, bool) {
	if p.cfg.hooks.beforeStat != nil {
		p.cfg.hooks.beforeStat(shard, name)
	}
	fd := int(handle.Fd())
	if isAsideName(name) {
		if _, finished := p.finishAside(fd, shard, name, nil); finished {
			p.report.AsidesFinished++
			p.report.EntriesRemoved++
		}
		return trimCandidate{}, false
	}
	size, mtime, reason := classifyEntry(fd, name)
	if reason != "" {
		p.unknown(shard, name, reason)
		return trimCandidate{}, false
	}
	p.report.Checkpoint.BytesSoFar += size
	if !time.Unix(0, mtime).Before(p.cutoff) {
		p.report.KeepWindowEntries++
		p.report.KeepWindowBytes += size
		return trimCandidate{}, false
	}
	return trimCandidate{Shard: shard, Name: name, MtimeNS: mtime, Size: size}, true
}

// classifyEntry stats one entry without following links: a regular file is
// Known at its size; a -d directory is Known only in Go's one-child layout,
// at its child's size; anything else has a reason.
func classifyEntry(fd int, name string) (int64, int64, string) {
	var st unix.Stat_t
	if err := unix.Fstatat(fd, name, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return 0, 0, "stat: " + err.Error()
	}
	switch kind := fileKind(st); kind {
	case unix.S_IFREG:
		return st.Size, st.Mtim.Nano(), ""
	case unix.S_IFDIR:
		if !strings.HasSuffix(name, "-d") {
			return 0, 0, "a directory whose name is not an output entry"
		}
		_, size, reason := classifyDirectory(fd, name)
		return size, st.Mtim.Nano(), reason
	default:
		return 0, 0, kindName(kind) + ", not Go's layout"
	}
}

// classifyDirectory reads a directory entry whole through a handle opened
// without following links: Known when it holds exactly one regular child.
// It returns the directory's identity and the child's size.
func classifyDirectory(parent int, name string) (unix.Stat_t, int64, string) {
	var identity unix.Stat_t
	fd, err := unix.Openat(parent, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return identity, 0, "open: " + err.Error()
	}
	dir := os.NewFile(uintptr(fd), name)
	defer dir.Close()
	if err := unix.Fstat(fd, &identity); err != nil {
		return identity, 0, "stat: " + err.Error()
	}
	children, err := dir.Readdirnames(-1)
	if err != nil {
		return identity, 0, "list: " + err.Error()
	}
	if len(children) != 1 {
		return identity, 0, fmt.Sprintf("holds %d entries, not Go's one executable", len(children))
	}
	var child unix.Stat_t
	if err := unix.Fstatat(fd, children[0], &child, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return identity, 0, "stat of its child: " + err.Error()
	}
	if kind := fileKind(child); kind != unix.S_IFREG {
		return identity, 0, "its child " + children[0] + " is " + kindName(kind)
	}
	return identity, child.Size, ""
}

// finishAside removes an aside directory whole: it must hold one regular
// child or nothing. expect, when given, is the identity the directory had
// before its rename. It reports the child's bytes and whether it finished.
func (p *trimPass) finishAside(fd int, shard, aside string, expect *unix.Stat_t) (int64, bool) {
	var before unix.Stat_t
	if err := unix.Fstatat(fd, aside, &before, unix.AT_SYMLINK_NOFOLLOW); err != nil || fileKind(before) != unix.S_IFDIR {
		p.unknown(shard, aside, "an aside that is not a directory")
		return 0, false
	}
	if expect != nil && !sameFile(before, *expect) {
		p.unknown(shard, aside, "the aside is not the directory that was renamed")
		return 0, false
	}
	asideFd, err := unix.Openat(fd, aside, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		p.unknown(shard, aside, "open: "+err.Error())
		return 0, false
	}
	dir := os.NewFile(uintptr(asideFd), aside)
	defer dir.Close()
	var opened unix.Stat_t
	if unix.Fstat(asideFd, &opened) != nil || !sameFile(before, opened) {
		p.unknown(shard, aside, "the aside changed between its stat and its open")
		return 0, false
	}
	children, err := dir.Readdirnames(-1)
	if err != nil || len(children) > 1 {
		p.unknown(shard, aside, fmt.Sprintf("an aside that is not Go's one-child layout (%d entries)", len(children)))
		return 0, false
	}
	var size int64
	if len(children) == 1 {
		var child unix.Stat_t
		if err := unix.Fstatat(asideFd, children[0], &child, unix.AT_SYMLINK_NOFOLLOW); err != nil || fileKind(child) != unix.S_IFREG {
			p.unknown(shard, aside, "an aside whose child is not a regular file")
			return 0, false
		}
		if err := unix.Unlinkat(asideFd, children[0], 0); err != nil {
			p.unknown(shard, aside, "unlink of its child: "+err.Error())
			return 0, false
		}
		size = child.Size
	}
	if err := unix.Unlinkat(fd, aside, unix.AT_REMOVEDIR); err != nil {
		p.unknown(shard, aside, "remove: "+err.Error())
		return 0, false
	}
	return size, true
}

// planEviction sorts the measured candidates oldest first and persists them
// atomically: a later pass evicts from this plan without measuring again.
func (p *trimPass) planEviction() error {
	candidates := p.readCandidates()
	seen := map[string]bool{}
	var unique []trimCandidate
	for _, candidate := range candidates {
		key := candidate.Shard + "/" + candidate.Name
		if !seen[key] {
			seen[key] = true
			unique = append(unique, candidate)
		}
	}
	sort.Slice(unique, func(i, j int) bool {
		if unique[i].MtimeNS != unique[j].MtimeNS {
			return unique[i].MtimeNS < unique[j].MtimeNS
		}
		if unique[i].Shard != unique[j].Shard {
			return unique[i].Shard < unique[j].Shard
		}
		return unique[i].Name < unique[j].Name
	})
	return p.writeCandidates(unique)
}

// evict removes candidates oldest first until the total is at or below the
// cap, each re-stat'ed through its shard handle immediately before removal.
func (p *trimPass) evict(ctx context.Context) (string, error) {
	r := p.report
	candidates := p.readCandidates()
	handles := map[string]*os.File{}
	defer func() {
		for _, handle := range handles {
			handle.Close()
		}
	}()
	next := 0
	for next < len(candidates) && r.BytesAfter > p.cfg.CapBytes {
		end := min(next+trimBatch, len(candidates))
		for ; next < end && r.BytesAfter > p.cfg.CapBytes; next++ {
			if err := p.evictOne(handles, candidates[next]); err != nil {
				return "", err
			}
		}
		if err := p.writeCandidates(candidates[next:]); err != nil {
			return "", err
		}
		if err := p.persist(); err != nil {
			return "", err
		}
		if stop := p.interrupted(ctx, true); stop != "" {
			return stop, nil
		}
	}
	r.Phase = "idle"
	return "", p.writeCandidates(nil)
}

func (p *trimPass) evictOne(handles map[string]*os.File, candidate trimCandidate) error {
	r := p.report
	handle, ok := handles[candidate.Shard]
	if !ok {
		opened, exists, err := p.openShard(candidate.Shard)
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}
		handles[candidate.Shard], handle = opened, opened
	}
	fd := int(handle.Fd())
	if p.cfg.hooks.beforeRemove != nil {
		p.cfg.hooks.beforeRemove(candidate.Shard, candidate.Name)
	}
	var st unix.Stat_t
	if err := unix.Fstatat(fd, candidate.Name, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		if errors.Is(err, unix.ENOENT) {
			// Gone already, or renamed aside by a pass that was killed:
			// finish the aside by name.
			aside := candidate.Name + asideSuffix
			var asideStat unix.Stat_t
			if unix.Fstatat(fd, aside, &asideStat, unix.AT_SYMLINK_NOFOLLOW) == nil {
				if size, finished := p.finishAside(fd, candidate.Shard, aside, nil); finished {
					p.removed(size)
				}
			}
			return nil
		}
		p.unknown(candidate.Shard, candidate.Name, "stat: "+err.Error())
		return nil
	}
	if st.Mtim.Nano() != candidate.MtimeNS || !time.Unix(0, st.Mtim.Nano()).Before(p.cutoff) {
		r.MovedSkips++
		return nil
	}
	switch kind := fileKind(st); kind {
	case unix.S_IFREG:
		if err := unix.Unlinkat(fd, candidate.Name, 0); err != nil {
			p.unknown(candidate.Shard, candidate.Name, "unlink: "+err.Error())
			return nil
		}
		p.removed(st.Size)
	case unix.S_IFDIR:
		identity, _, reason := classifyDirectory(fd, candidate.Name)
		if reason != "" || !strings.HasSuffix(candidate.Name, "-d") {
			p.unknown(candidate.Shard, candidate.Name, reason)
			return nil
		}
		aside := candidate.Name + asideSuffix
		var existing unix.Stat_t
		if unix.Fstatat(fd, aside, &existing, unix.AT_SYMLINK_NOFOLLOW) == nil {
			if size, finished := p.finishAside(fd, candidate.Shard, aside, nil); finished {
				p.removed(size)
			} else {
				return nil
			}
		}
		if err := unix.Renameat(fd, candidate.Name, fd, aside); err != nil {
			p.unknown(candidate.Shard, candidate.Name, "rename aside: "+err.Error())
			return nil
		}
		if size, finished := p.finishAside(fd, candidate.Shard, aside, &identity); finished {
			p.removed(size)
		}
	default:
		p.unknown(candidate.Shard, candidate.Name, kindName(kind)+", not Go's layout")
	}
	return nil
}

func (p *trimPass) removed(size int64) {
	p.report.EntriesRemoved++
	p.report.BytesRemoved += size
	p.report.BytesAfter -= size
}

func (p *trimPass) readCandidates() []trimCandidate {
	file, err := os.Open(p.statePath(".candidates.jsonl"))
	if err != nil {
		return nil
	}
	defer file.Close()
	var candidates []trimCandidate
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		var candidate trimCandidate
		// A torn last line from a killed pass is skipped: that batch was
		// never checkpointed and is measured again.
		if json.Unmarshal(scanner.Bytes(), &candidate) == nil && candidate.Shard != "" && isEntryName(candidate.Name) {
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

func encodeCandidates(candidates []trimCandidate) []byte {
	var out []byte
	for _, candidate := range candidates {
		line, _ := json.Marshal(candidate)
		out = append(append(out, line...), '\n')
	}
	return out
}

// appendCandidates records a measured batch's candidates before its
// checkpoint is written.
func (p *trimPass) appendCandidates(candidates []trimCandidate) error {
	if len(candidates) == 0 {
		return nil
	}
	file, err := os.OpenFile(p.statePath(".candidates.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.Write(encodeCandidates(candidates)); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func (p *trimPass) writeCandidates(candidates []trimCandidate) error {
	return writeAtomic(p.statePath(".candidates.jsonl"), encodeCandidates(candidates))
}

func (p *trimPass) persist() error {
	data, err := json.MarshalIndent(p.report, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(p.statePath(".json"), append(data, '\n'))
}

// writeAtomic replaces a state file by rename, never by removal.
func writeAtomic(path string, data []byte) error {
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
