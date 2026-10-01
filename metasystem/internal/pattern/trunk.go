package pattern

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// TrunkRef is the private ref the lane steward fetches origin's main into
// each cycle (D5). The fetch touches no working tree, no branch, no
// remote-tracking ref (an empty refmap) and no FETCH_HEAD.
const TrunkRef = "refs/metasystem/patterns/trunk"

// LaneLineage is the landing lane's stable identity (lane design r10, K7):
// commits whose Goal-Transaction carries its lineage hash are the lane's.
const LaneLineage = "landing-lane"

// GitRunner runs one git command in a directory.
type GitRunner func(dir string, args ...string) ([]byte, error)

// gitTimeout bounds one git command of the pass; the fetch is the slow one.
// It stays well inside the steward's tick patience (120 s), so a stalled
// fetch is a stale trunk (Unknown), never a runner that reads as stuck.
const gitTimeout = 30 * time.Second

// RunGit is the production GitRunner.
func RunGit(dir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	out, err := command.Output()
	if err != nil {
		return out, fmt.Errorf("git %s: %w", args[0], err)
	}
	return out, nil
}

// fetchTrunk fetches origin's main into the private ref.
func fetchTrunk(git GitRunner, laneRoot string) error {
	_, err := git(laneRoot, "fetch", "--quiet", "--no-tags", "--no-write-fetch-head", "--refmap=", "origin", "+refs/heads/main:"+TrunkRef)
	return err
}

// TrunkCommit is one attributed commit on main: its committer time, the
// execution identity its Goal-Transaction names and the paths it changed.
type TrunkCommit struct {
	SHA     string    `json:"sha"`
	At      time.Time `json:"at"`
	Machine string    `json:"machine"`
	Lineage string    `json:"lineage"`
	Paths   []string  `json:"paths"`
	// New is a commit this cycle observed for the first time.
	New bool `json:"-"`
}

// TrunkSignal is main as the churn reader saw it this cycle.
type TrunkSignal struct {
	// Read says the trunk reader ran; a pass with churn off leaves it false.
	Read bool
	// Unreadable is a ref that could not be read, or a fetch older than
	// max-gap (stale).
	Unreadable bool
	// Fresh is a fetch that succeeded this cycle: only a fresh fetch observes
	// a quiet window.
	Fresh     bool
	FetchedAt time.Time
	SeenAt    time.Time
	// Commits are the retained and the new attributed commits.
	Commits []TrunkCommit
	// Watched are the works a churn finding named that have not yet read
	// clear for clear-ticks cycles.
	Watched []string
	// LaneHashes are the lineage hashes of the lane's identities.
	LaneHashes map[string]bool
}

// trunkState is the churn reader's memory between cycles.
type trunkState struct {
	LastObserved string        `json:"lastObserved,omitempty"`
	FetchedAt    time.Time     `json:"fetchedAt,omitempty"`
	Recent       []TrunkCommit `json:"recent,omitempty"`
	// Watched counts, per work, the clear cycles since its last finding.
	Watched map[string]int `json:"watched,omitempty"`
}

// hash8 is the lineage hash an opid carries (goal.Opid).
func hash8(lineage string) string {
	sum := sha256.Sum256([]byte(lineage))
	return hex.EncodeToString(sum[:4])
}

// parseOpid splits a Goal-Transaction value, <ulid>-<machine>-<hash8>.
func parseOpid(value string) (machine, lineage string, ok bool) {
	value = strings.TrimSpace(value)
	first, last := strings.Index(value, "-"), strings.LastIndex(value, "-")
	if first != 26 || last <= first+1 || len(value)-last-1 != 8 {
		return "", "", false
	}
	lineage = value[last+1:]
	if _, err := hex.DecodeString(lineage); err != nil || strings.ToLower(lineage) != lineage {
		return "", "", false
	}
	return value[first+1 : last], lineage, true
}

// readTrunk reads the commits the private ref gained since the last
// observed one (the first cycle reads one window back) and folds them into
// the state. fresh says this cycle's fetch succeeded.
func (s *trunkState) read(git GitRunner, laneRoot string, fresh bool, now time.Time, maxGap, window time.Duration) TrunkSignal {
	signal := TrunkSignal{Read: true, Fresh: fresh, SeenAt: now.UTC()}
	if fresh {
		s.FetchedAt = now.UTC()
	}
	signal.FetchedAt = s.FetchedAt
	out, err := git(laneRoot, "rev-parse", "--verify", "-q", TrunkRef+"^{commit}")
	tip := strings.TrimSpace(string(out))
	if err != nil || tip == "" || s.FetchedAt.IsZero() || now.Sub(s.FetchedAt) > maxGap {
		signal.Unreadable = true
		signal.Fresh = false
		signal.Commits = s.Recent
		return signal
	}
	format := "--format=%x1e%H%x1f%ct%x1f%(trailers:key=Goal-Transaction,valueonly,separator=%x1d)"
	since := "--since=@" + strconv.FormatInt(now.Add(-window).Unix(), 10)
	var log []byte
	if s.LastObserved != "" && s.LastObserved != tip {
		log, err = git(laneRoot, "log", "--name-only", format, s.LastObserved+".."+tip)
	}
	if s.LastObserved == "" || err != nil {
		log, err = git(laneRoot, "log", "--name-only", format, since, tip)
	}
	if err != nil {
		signal.Unreadable, signal.Fresh = true, false
		signal.Commits = s.Recent
		return signal
	}
	if s.LastObserved == tip {
		log = nil
	}
	seen := map[string]bool{}
	for _, commit := range s.Recent {
		seen[commit.SHA] = true
	}
	combined := append([]TrunkCommit(nil), s.Recent...)
	for _, record := range strings.Split(string(log), "\x1e")[1:] {
		commit, ok := parseLogRecord(record)
		if !ok || seen[commit.SHA] {
			continue
		}
		seen[commit.SHA] = true
		commit.New = true
		combined = append(combined, commit)
	}
	sort.SliceStable(combined, func(i, j int) bool { return combined[i].At.Before(combined[j].At) })
	s.LastObserved = tip
	signal.Commits = combined
	// Only commits that can still share a window with the newest are kept.
	var newest time.Time
	for _, commit := range combined {
		if commit.At.After(newest) {
			newest = commit.At
		}
	}
	s.Recent = nil
	for _, commit := range combined {
		if newest.Sub(commit.At) < window {
			commit.New = false
			s.Recent = append(s.Recent, commit)
		}
	}
	return signal
}

// parseLogRecord reads one commit of the log: its SHA, committer time and
// Goal-Transaction, then the paths it changed. A commit without a valid
// trailer is hand work and is left out.
func parseLogRecord(record string) (TrunkCommit, bool) {
	header, paths, _ := strings.Cut(record, "\n")
	fields := strings.Split(header, "\x1f")
	if len(fields) != 3 {
		return TrunkCommit{}, false
	}
	seconds, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return TrunkCommit{}, false
	}
	value, _, _ := strings.Cut(fields[2], "\x1d")
	machine, lineage, ok := parseOpid(value)
	if !ok {
		return TrunkCommit{}, false
	}
	commit := TrunkCommit{SHA: fields[0], At: time.Unix(seconds, 0).UTC(), Machine: machine, Lineage: lineage}
	for _, path := range strings.Split(paths, "\n") {
		if path = strings.TrimSpace(path); path != "" {
			commit.Paths = append(commit.Paths, path)
		}
	}
	return commit, len(commit.Paths) > 0
}
