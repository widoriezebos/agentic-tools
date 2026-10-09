package processchange

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

type Check struct {
	Root, Act string
	ProcessAct
	Person, Observation bool
	Now                 time.Time
	Remedy              func(string) string
	Applicable          func() []string
}

type checkSelection struct {
	Goal, Operation, Act string
	Argv                 []string
}

// AdmitCheck retains the last admitted selection, never a pending proposal.
func AdmitCheck(c Check) (act ProcessAct, err error) {
	if err = os.MkdirAll(filepath.Join(c.Root, "process", "acts"), 0700); err != nil {
		return
	}
	mode := lock.TryExclusive
	if c.Observation {
		mode = lock.Exclusive
	}
	held, err := lock.File(filepath.Join(c.Root, "process", "lock"), 0600, mode)
	if err != nil {
		return
	}
	defer held.Release()
	path := filepath.Join(c.Root, "process", fmt.Sprintf("check-%x.json", sha256.Sum256([]byte(c.Goal))))
	uncertain := filepath.Join(c.Root, "process", "acts", filepath.Base(path)+".unknown")
	_, markerErr := os.Stat(uncertain)
	unknown := !os.IsNotExist(markerErr)
	if c.Observation {
		// Failed publication leaves the comparison unknown.
		if _, err = atomicfile.WriteFile(uncertain, []byte("unknown\n"), 0600, ""); err != nil {
			return
		}
		defer func() {
			if err != nil {
				act.Status = "observation-unknown"
			}
		}()
	}
	acts, historyUnknown, historyErr := ReadActs(c.Root, c.Goal)
	if !c.Person && !c.Observation && (historyErr != nil || len(historyUnknown) > 0) {
		return act, fmt.Errorf("check history unavailable: %v %v", historyErr, historyUnknown)
	}
	body, readErr := os.ReadFile(path)
	if readErr != nil && !os.IsNotExist(readErr) {
		return act, fmt.Errorf("the check selection cannot be preserved: %w", readErr)
	}
	var previous checkSelection
	damaged := readErr == nil && (json.Unmarshal(body, &previous) != nil || previous.Goal != c.Goal || previous.Operation == "" || len(previous.Argv) == 0)
	unknown = unknown || damaged
	if unknown {
		previous = checkSelection{}
	}
	if c.Person && c.Act == "" && !unknown {
		for _, pending := range acts {
			if pending.Status == "proposed" && pending.Operation == c.Operation && pending.Checkout == c.Checkout && slices.Equal(pending.AfterArgv, c.AfterArgv) && (len(previous.Argv) == 0 || slices.Equal(pending.BeforeArgv, previous.Argv)) && pending.Predecessor == previous.Act {
				c.Act = pending.ID
			}
		}
	}
	if len(previous.Argv) == 0 && !c.Observation && !c.Person {
		c.ApplicableArgv = c.Applicable()
	}
	if !c.Person && !c.Observation && (unknown || (len(previous.Argv) == 0 && len(c.ApplicableArgv) == 0)) {
		return act, fmt.Errorf("the previous check or its declaration is unavailable; a person can supply an explicit check")
	}
	act = c.ProcessAct
	act.BeforeArgv, act.Predecessor, act.Status = previous.Argv, previous.Act, "unchanged"
	if len(act.BeforeArgv) == 0 && !unknown {
		act.BeforeArgv = c.ApplicableArgv
	}
	changed := !c.Observation && (unknown || !slices.Equal(act.BeforeArgv, c.AfterArgv))
	if c.Act != "" || changed || (previous.Operation == c.Operation && previous.Act != "") {
		act.Status, act.ProposedAt, act.Class = "proposed", c.Now.UTC(), "added-check"
		if len(previous.Argv) > 0 {
			act.Class = "widened-check"
		}
		identity, _ := json.Marshal([]any{c.Checkout, c.Goal, c.Unit, c.Operation, act.BeforeArgv, c.AfterArgv, act.Predecessor})
		act.ID = fmt.Sprintf("%x", sha256.Sum256(identity))
		if c.Act != "" {
			act.ID = c.Act
		} else if previous.Operation == c.Operation && previous.Act != "" && !changed {
			act.ID = previous.Act
		}
		if filepath.Base(act.ID) != act.ID || strings.ContainsAny(act.ID, `/\`) {
			return act, fmt.Errorf("invalid process act id")
		}
		actPath := filepath.Join(c.Root, "process", "acts", act.ID+".json")
		retained, problem := os.ReadFile(actPath)
		if problem == nil {
			var retainedAct ProcessAct
			if json.Unmarshal(retained, &retainedAct) != nil || retainedAct.ID != act.ID || retainedAct.Checkout != c.Checkout || retainedAct.Goal != c.Goal || retainedAct.Unit != c.Unit || retainedAct.Operation != c.Operation || !slices.Equal(retainedAct.AfterArgv, c.AfterArgv) {
				return act, fmt.Errorf("the process act names a different check or admission")
			}
			act = retainedAct
		} else if !os.IsNotExist(problem) || c.Act != "" {
			return act, fmt.Errorf("check act unavailable: %w", problem)
		}
		if act.Status == "superseded" || (previous.Act != act.ID && (!slices.Equal(previous.Argv, act.BeforeArgv) && len(previous.Argv) > 0 || previous.Act != act.Predecessor)) {
			act.Status = "superseded"
			return act, save(c.Root, actPath, act)
		}
		if act.Status != "applied" {
			if err := save(c.Root, actPath, act); err != nil {
				return act, err
			}
			if !c.Person {
				q, _, problem := channel.AskOrFind(channel.AskRequest{RepoRoot: c.Root, Goal: act.Goal, Kind: "other", Lineage: act.Lineage, ProcessAct: act.ID, Now: c.Now, Facts: []string{fmt.Sprintf("Check from %q to %q", act.BeforeArgv, act.AfterArgv), act.Reason}, Wants: c.Remedy(act.ID)})
				if problem != nil {
					return act, problem
				}
				act.Question = q.ID
				return act, save(c.Root, actPath, act)
			}
			act.Status, act.AppliedAt, act.AppliedProof = "applied", c.Now.UTC(), c.Proof
		}
		previous.Act = act.ID
	} else if !slices.Equal(previous.Argv, c.AfterArgv) {
		previous.Act = ""
	}
	if damaged {
		if _, err := atomicfile.WriteFile(path+fmt.Sprintf(".damaged-%x", sha256.Sum256(body)), body, 0600, ""); err != nil {
			return act, err
		}
	}
	selection, _ := json.Marshal(checkSelection{c.Goal, c.Operation, previous.Act, c.AfterArgv})
	if _, err = atomicfile.WriteFile(path, append(selection, '\n'), 0600, ""); err != nil {
		return act, err
	}
	if problem := os.Remove(uncertain); problem != nil && !os.IsNotExist(problem) {
		return act, problem
	}
	if act.ID == "" {
		act.ID = previous.Act
		return
	}
	return act, save(c.Root, filepath.Join(c.Root, "process", "acts", act.ID+".json"), act)
}
