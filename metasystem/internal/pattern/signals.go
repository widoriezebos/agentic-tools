package pattern

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// Signals are what the readers fill once per cycle, one reader per record
// kind. A reader that fails marks its kind unreadable.
type Signals struct {
	// Batches are the lane's batches. BatchesUnreadable says the store could
	// not be listed at all; a record that could not be read is one batch
	// marked Unreadable.
	Batches           []BatchSignal
	BatchesUnreadable bool
	// Trunk is origin's main as the private ref holds it.
	Trunk TrunkSignal
}

// Step is one typed batch history entry: when, which verb, and the states
// it moved between. Its free text is never read.
type Step struct {
	At       time.Time
	Verb     string
	From, To string
}

// BatchSignal is one lane batch as the patterns read it. The reader seam is
// deliberately this small: a lane kernel whose records replace the batch
// store fills the same fields.
type BatchSignal struct {
	ID string
	// Record names the record for evidence, relative to the lane checkout.
	Record    string
	State     string
	Since     time.Time
	Steps     []Step
	SeatRoots []string
	// Unreadable is a record that could not be read or decoded.
	Unreadable bool
	// Held is a person's hold: the lane paused, the landing seat or a member
	// seat at the helm, or the batch held for a red on main (the trunk-red
	// role owns that). HoldUnreadable is a hold that could not be read.
	Held, HoldUnreadable bool
	// Active is the counted active time (§1 Active time), filled by the pass.
	Active time.Duration
}

// finished is a batch no pattern follows any more.
func (b BatchSignal) finished() bool {
	return b.State == batch.StateLanded || b.State == batch.StateDissolved
}

// BatchReader reads the lane's batches from the lane checkout.
type BatchReader func(laneRoot string) ([]BatchSignal, error)

// ReadBatchStore is the production BatchReader: the retained typed batch
// records (batch.Store).
func ReadBatchStore(laneRoot string) ([]BatchSignal, error) {
	directory := filepath.Join(laneRoot, "artifacts", "agents", "landing-batches")
	paths, err := filepath.Glob(filepath.Join(directory, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	store := batch.NewStore(laneRoot, nil)
	signals := make([]BatchSignal, 0, len(paths))
	for _, path := range paths {
		id := strings.TrimSuffix(filepath.Base(path), ".json")
		if !batch.ValidID(id) {
			continue
		}
		signal := BatchSignal{ID: id, Record: filepath.ToSlash(filepath.Join("artifacts", "agents", "landing-batches", id+".json"))}
		record, err := store.Load(id)
		if err != nil {
			signal.Unreadable = true
			signals = append(signals, signal)
			continue
		}
		signal.State = record.State
		for _, entry := range record.History {
			at, err := time.Parse(time.RFC3339Nano, entry.At)
			if err != nil {
				signal.Unreadable = true
				break
			}
			signal.Steps = append(signal.Steps, Step{At: at.UTC(), Verb: entry.Verb, From: entry.From, To: entry.To})
		}
		if len(signal.Steps) > 0 {
			signal.Since = signal.Steps[0].At
		}
		seen := map[string]bool{}
		for _, unit := range record.Units {
			if unit.SeatRoot != "" && !seen[unit.SeatRoot] {
				seen[unit.SeatRoot] = true
				signal.SeatRoots = append(signal.SeatRoots, unit.SeatRoot)
			}
		}
		signals = append(signals, signal)
	}
	return signals, nil
}

// HelmReader reads whether a seat is at the helm; readable is false when the
// seat's helm cannot be read at all (no repository there).
type HelmReader func(seatRoot string) (active, readable bool)

// ReadHelm is the production HelmReader. A signature that cannot be decoded
// is a helm taken (helm.Active's rule). A seat whose checkout was removed,
// or that is no repository at all, can hold no helm: it reads not held, so
// a batch whose member seat is gone is still followed (a person at the helm
// always has a checkout to hold it in).
func ReadHelm(seatRoot string) (bool, bool) {
	state := helm.Active(seatRoot)
	if state.Diagnostic != "" {
		return false, true
	}
	return state.Active, true
}

// PauseReader reads the lane's pause: paused, or an error when the pause
// record is present and unreadable.
type PauseReader func(home string) (bool, error)

// ReadPause is the production PauseReader.
func ReadPause(home string) (bool, error) {
	_, paused, err := lane.PauseState(home)
	return paused, err
}

// markHolds fills each unfinished batch's hold from the lane pause, the
// landing seat's helm and its member seats' helms.
func markHolds(batches []BatchSignal, paused bool, pauseErr error, landingHelm, landingReadable bool, readHelm HelmReader) {
	for index := range batches {
		b := &batches[index]
		if b.Unreadable || b.finished() {
			continue
		}
		held := paused || landingHelm || b.State == batch.StateHeldTrunkRed
		unreadable := pauseErr != nil || !landingReadable
		for _, seat := range b.SeatRoots {
			active, readable := readHelm(seat)
			held = held || active
			unreadable = unreadable || !readable
		}
		b.Held, b.HoldUnreadable = held, unreadable
	}
}
