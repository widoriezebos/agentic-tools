package lane

// The steward's lane-only detector (lane runtime design r10 §2, K3 row and
// K10): while a lane is registered, the lane checkout's own steward reads
// origin's main each cycle. A commit that carries the lane's trailers and
// that no lane publication put there, or a main that was rewound, pauses
// the lane and opens an alert. A plain push under another name is the
// accepted undetected residual risk (D6, D8; goal main-push-watcher).
//
// No lane, no detector: with no lane registered nothing is read or
// written, and a seat's own pushes are never judged. A lane registered
// again (a new custody epoch) starts from the main it finds.

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

// laneTrailers are the trailers only the lane writes on what it lands.
var laneTrailers = []string{LandingProofTrailer, batch.LaneResolvedTrailer, batch.LaneIntegrationTrailer}

// watchRef is where the watch keeps origin's main in the lane checkout,
// apart from every ref the agent works with.
const watchRef = "refs/metasystem/lanewatch/main"

// LaneWatch is the lane-only detector, one Step per steward cycle.
type LaneWatch struct {
	Home string
	Now  func() time.Time
	// Self is the checkout this steward keeps; only the lane checkout's own
	// steward watches.
	Self string
	// Alert opens a hit's alert (the steward's OpenAlert at the lane
	// installation); nil leaves it to the landing agent's keeper.
	Alert func(root string, hit Hit) error
}

// watchState is the main the watch last judged, in the lane registration
// it judged it for.
type watchState struct {
	Epoch uint64 `json:"epoch"`
	Root  string `json:"root"`
	Main  string `json:"main"`
	At    string `json:"at"`
}

func watchPath(home string) string { return filepath.Join(HostDir(home), "landing-lane-watch.json") }

// Step judges what reached origin's main since the last cycle. It returns
// the line a steward prints; empty when nothing is registered, this
// steward does not keep the lane, or nothing changed.
func (w LaneWatch) Step() string {
	record, ok, err := Read(w.Home)
	if err != nil || !ok || gone(record.Root) || !ownsLane(w.Self, record) {
		return ""
	}
	root := record.Root
	now := w.Now()
	current, err := RemoteMain(root, "origin")
	if err != nil {
		return "the landing lane's watch can't read origin's main: " + oneLineText(err.Error())
	}
	var state watchState
	if _, err := readJSON(watchPath(w.Home), &state); err != nil {
		state = watchState{}
	}
	if current == "" || state.Main == current && state.Epoch == record.CustodyEpoch && state.Root == root {
		return w.deliver(root, now)
	}
	if _, err := laneGit(root, nil, "fetch", "--quiet", "--no-tags", "origin", "+"+MainRef+":"+watchRef); err != nil {
		return "the landing lane's watch can't fetch origin's main: " + oneLineText(err.Error())
	}
	fetched, err := laneGit(root, nil, "rev-parse", "--verify", "--quiet", watchRef)
	if err != nil {
		return "the landing lane's watch can't read the main it fetched: " + oneLineText(err.Error())
	}
	next := watchState{Epoch: record.CustodyEpoch, Root: root, Main: fetched, At: stamp(now)}
	var finding string
	if state.Main != "" && state.Epoch == record.CustodyEpoch && state.Root == root {
		finding, err = w.judge(root, state.Main, fetched)
		if err != nil {
			return "the landing lane's watch can't judge main: " + oneLineText(err.Error())
		}
	}
	if err := withLock(w.Home, func() error {
		if finding != "" {
			if err := updateStopLossHeld(w.Home, now, func(store *StopLoss) error {
				return store.hitHeld(w.Home, HitWatch, "watch:"+fetched, finding, WatchBy, now)
			}); err != nil {
				return err
			}
		}
		return writeJSON(w.Home, watchPath(w.Home), next)
	}); err != nil {
		return "the landing lane's watch can't record what it read: " + err.Error()
	}
	line := w.deliver(root, now)
	if finding != "" && line == "" {
		line = "the landing lane stopped itself: " + finding
	}
	return line
}

func (w LaneWatch) deliver(root string, now time.Time) string {
	if w.Alert == nil {
		return ""
	}
	return DeliverHits(w.Home, now, func(hit Hit) error { return w.Alert(root, hit) })
}

// judge says what is wrong with main moving from old to current: a rewind,
// or a lane-trailed commit no lane publication put there. Empty when
// nothing is.
func (w LaneWatch) judge(root, old, current string) (string, error) {
	ancestor, err := isAncestor(root, old, current)
	if err != nil {
		return "", err
	}
	if !ancestor {
		return fmt.Sprintf("origin's main was rewound from %s to %s, so the landing lane is stopped", short(old), short(current)), nil
	}
	listing, err := laneGit(root, nil, "log", "--format=%H%x1f"+trailerFormat()+"%x1e", old+".."+current)
	if err != nil {
		return "", err
	}
	var trailed []string
	for _, entry := range strings.Split(listing, "\x1e") {
		commit, trailers, found := strings.Cut(strings.TrimSpace(entry), "\x1f")
		if found && strings.TrimSpace(trailers) != "" {
			trailed = append(trailed, commit)
		}
	}
	if len(trailed) == 0 {
		return "", nil
	}
	published, err := ReadPublications(w.Home)
	if err != nil {
		return "", err
	}
	covered := map[string]bool{}
	for _, publication := range published {
		commits, err := laneGit(root, nil, "rev-list", publication.Old+".."+publication.New)
		if err != nil {
			// A publication whose commits this checkout no longer has
			// covers nothing.
			continue
		}
		for _, commit := range strings.Fields(commits) {
			covered[commit] = true
		}
	}
	var stray []string
	for _, commit := range trailed {
		if !covered[commit] {
			stray = append(stray, short(commit))
		}
	}
	if len(stray) == 0 {
		return "", nil
	}
	slices.Reverse(stray)
	return fmt.Sprintf("origin's main took %d commit(s) carrying the landing lane's trailers that no lane publication pushed (%s), so the landing lane is stopped",
		len(stray), strings.Join(stray, ", ")), nil
}

func trailerFormat() string {
	keys := make([]string, 0, len(laneTrailers))
	for _, key := range laneTrailers {
		keys = append(keys, "key="+key)
	}
	return "%(trailers:" + strings.Join(keys, ",") + ",valueonly,separator=%x2C)"
}

// isAncestor says whether old is an ancestor of (or is) current.
func isAncestor(root, old, current string) (bool, error) {
	_, err := laneGit(root, nil, "merge-base", "--is-ancestor", old, current)
	var exit *exec.ExitError
	switch {
	case err == nil:
		return true, nil
	case errors.As(err, &exit) && exit.ExitCode() == 1:
		return false, nil
	}
	return false, err
}

// ownsLane says whether the steward of self keeps the registered lane: its
// checkout is the lane's checkout or its installation, as landing set
// recorded them (K-a), never guessed.
func ownsLane(self string, record Record) bool {
	layout, err := record.Layout()
	if self == "" || err != nil {
		return false
	}
	here := resolved(self)
	return here == resolved(string(layout.Checkout)) || here == resolved(string(layout.Install))
}

func oneLineText(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	return line
}
