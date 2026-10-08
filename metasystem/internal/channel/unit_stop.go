package channel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// UnitStopQuestion binds one executable remedy to one finding in one read.
type UnitStopQuestion struct {
	Loop           string   `json:"loop"`
	Subject        string   `json:"subject"`
	Attempt        int      `json:"attempt"`
	Finding        string   `json:"finding"`
	Review         string   `json:"review"`
	Needs          string   `json:"needs"`
	AcceptableActs []string `json:"acceptableActs"`
}

func (q UnitStopQuestion) sameKey(other UnitStopQuestion) bool {
	return q.Loop == other.Loop && q.Subject == other.Subject && q.Attempt == other.Attempt && q.Finding == other.Finding
}

// ActCommand returns the executable act a stop asks for.
func ActCommand(q Question) string {
	if q.UnitStop != nil {
		return q.UnitStop.Needs
	}
	return LaneStopCommand(q)
}

func withUnitStopLock(root string, fn func() error) error {
	if err := os.MkdirAll(channelRoot(root), 0o700); err != nil {
		return err
	}
	held, err := lock.File(filepath.Join(channelRoot(root), "unit-stops.lock"), 0o600, lock.Exclusive)
	if err != nil {
		return err
	}
	defer held.Release()
	return fn()
}

func askUnitStop(r AskRequest) (q Question, found bool, err error) {
	stop := r.UnitStop
	if r.Goal == "" || stop.Loop == "" || stop.Subject == "" || stop.Attempt < 1 || stop.Finding == "" || strings.TrimSpace(stop.Needs) == "" || len(stop.AcceptableActs) == 0 {
		return q, false, fmt.Errorf("a unit stop question needs its goal, loop, subject, attempt, finding and executable act")
	}
	r.Facts = append(slices.Clone(r.Facts), "Acceptable acts: "+strings.Join(stop.AcceptableActs, ", ")+". Only a successful act addressing this finding and subject, or this unit's closure, closes this question; a text answer does not.")
	err = withUnitStopLock(r.RepoRoot, func() error {
		var e error
		q, found, e = askOrFind(r)
		if e != nil {
			return e
		}
		return reconcileUnitStopQuestions(r.RepoRoot)
	})
	if err == nil {
		q, err = ReadQuestion(r.RepoRoot, q.ID)
	}
	return
}

// UnitStopAct is written only after its owning command succeeds. Findings
// identify exactly the work admitted or resolved; UnitClosed ends all asks
// for that subject without claiming that its read was clean.
type UnitStopAct struct {
	ID         string    `json:"id"`
	Goal       string    `json:"goal"`
	Loop       string    `json:"loop"`
	Subject    string    `json:"subject"`
	Attempt    int       `json:"attempt"`
	Findings   []string  `json:"findings"`
	Kind       string    `json:"kind"`
	Reason     string    `json:"reason"`
	At         time.Time `json:"at"`
	UnitClosed bool      `json:"unitClosed,omitempty"`
}

func RecordUnitStopAct(root string, act UnitStopAct) error {
	if act.Goal == "" || act.Subject == "" || act.Loop == "" || act.Kind == "" || act.At.IsZero() || (!act.UnitClosed && (act.Attempt < 1 || len(act.Findings) == 0)) {
		return fmt.Errorf("a successful unit act needs its goal, subject, loop, kind, time and addressed findings")
	}
	if act.ID == "" {
		var err error
		act.ID, err = goal.NewOperationULID()
		if err != nil {
			return err
		}
	}
	return withUnitStopLock(root, func() error {
		// The filename is a digest so command-owned operation ids cannot
		// select any path outside this store.
		path := filepath.Join(channelRoot(root), "unit-stop-acts", Digest(act.ID)+".json")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := writeJSON(path, act); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		return reconcileUnitStopQuestions(root)
	})
}

func ReconcileUnitStopQuestions(root string) error {
	return withUnitStopLock(root, func() error { return reconcileUnitStopQuestions(root) })
}

func reconcileUnitStopQuestions(root string) error {
	paths, err := filepath.Glob(filepath.Join(channelRoot(root), "unit-stop-acts", "*.json"))
	if err != nil {
		return err
	}
	questions, unreadable := WalkOpenQuestions(root)
	if len(unreadable) > 0 {
		return fmt.Errorf("unit question reconciliation cannot read %s", strings.Join(unreadable, "; "))
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var act UnitStopAct
		if err := json.Unmarshal(data, &act); err != nil {
			return err
		}
		for _, q := range questions {
			stop := q.UnitStop
			if stop == nil || q.Goal != act.Goal || stop.Loop != act.Loop || stop.Subject != act.Subject {
				continue
			}
			if act.UnitClosed && ((act.Attempt > 0 && stop.Attempt > act.Attempt) || (act.Attempt == 0 && q.OpenedAt.After(act.At))) {
				continue
			}
			if !act.UnitClosed && (stop.Attempt != act.Attempt || !slices.Contains(stop.AcceptableActs, act.Kind) || !slices.Contains(act.Findings, stop.Finding)) {
				continue
			}
			if err := Close(root, q.ID, act.Kind+": "+act.Reason, nil, DestinationConfig{}); err != nil {
				return err
			}
		}
	}
	return nil
}

// CloseGoalUnitStopQuestions records conclusion separately for every unit
// subject. Repeated cleanup reconciles those successful acts idempotently.
func CloseGoalUnitStopQuestions(root, goalID, reason string, at time.Time) error {
	questions, unreadable := WalkOpenQuestions(root)
	if len(unreadable) > 0 {
		return fmt.Errorf("goal cleanup cannot read %s", strings.Join(unreadable, "; "))
	}
	for _, q := range questions {
		if q.Goal != goalID || q.UnitStop == nil {
			continue
		}
		stop := q.UnitStop
		if err := RecordUnitStopAct(root, UnitStopAct{ID: "goal-done:" + goalID + ":" + stop.Subject + ":" + fmt.Sprint(stop.Attempt), Goal: goalID, Loop: stop.Loop, Subject: stop.Subject, Attempt: stop.Attempt, Kind: "goal-done", Reason: reason, At: at, UnitClosed: true}); err != nil {
			return err
		}
	}
	return nil
}
