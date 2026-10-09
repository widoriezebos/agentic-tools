package processchange

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
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

type Declaration = launch.UnitDeclaration
type DeclarationAdmission struct {
	Check
	Resume  bool
	Source  func() (candidate, main, inherited Declaration, err error)
	Recheck func() error
	Publish func(ProcessAct) error
	Impact  func() error
}
type declarationReference struct {
	Goal, Seed, Operation, Act string
	Snapshot                   Declaration
}

// AdmitDeclaration owns each goal's committed declaration history and approval.
func AdmitDeclaration(c DeclarationAdmission) (act ProcessAct, err error) {
	directory := filepath.Join(c.Root, "process")
	if err = os.MkdirAll(filepath.Join(directory, "acts"), 0700); err != nil {
		return
	}
	held, err := lock.File(filepath.Join(directory, "lock"), 0600, lock.Exclusive)
	if err != nil {
		return
	}
	defer held.Release()
	path := filepath.Join(directory, fmt.Sprintf("declaration-%x.json", sha256.Sum256([]byte(c.Goal))))
	write := func(target string, value any) error {
		data, _ := json.Marshal(value)
		_, err := atomicfile.WriteFile(target, append(data, '\n'), 0600, "")
		return err
	}
	body, readErr := os.ReadFile(path)
	var previous declarationReference
	if readErr != nil && !os.IsNotExist(readErr) || readErr == nil && (json.Unmarshal(body, &previous) != nil || previous.Goal != c.Goal || previous.Seed == "" || previous.Snapshot.Commit == "" || slices.Contains(previous.Snapshot.Values[:], "")) {
		return act, fmt.Errorf("the goal's declaration baseline is unavailable; a person can supply --reason TEXT --by NAME --check COMMAND")
	}
	act = c.ProcessAct
	if c.Resume {
		if previous.Operation != c.Operation || previous.Act == "" {
			return act, nil
		}
		actPath := filepath.Join(directory, "acts", previous.Act+".json")
		data, problem := os.ReadFile(actPath)
		if problem != nil || json.Unmarshal(data, &act) != nil || act.Goal != c.Goal || act.ID != previous.Act || act.Class != "declaration" || act.Status != "applied" {
			return act, fmt.Errorf("the admitted declaration act is unavailable")
		}
		return act, save(c.Root, actPath, act)
	}
	candidate, main, inherited, problem := c.Source()
	if problem != nil {
		return act, problem
	}
	if os.IsNotExist(readErr) {
		previous = declarationReference{Goal: c.Goal, Seed: main.Commit, Snapshot: main}
		if err = write(path, previous); err != nil {
			return
		}
	}
	act.BeforeDeclaration, act.AfterDeclaration = &previous.Snapshot, &candidate
	act.Predecessor, act.Class, act.Actor = previous.Act, "declaration", "unknown"
	changed := false
	for index, value := range candidate.Values {
		changed = changed || value != previous.Snapshot.Values[index] && value != inherited.Values[index]
	}
	defer func() {
		if err != nil || act.Status == "proposed" || act.Status == "superseded" {
			return
		}
		if err = c.Recheck(); err == nil {
			err = write(path, declarationReference{c.Goal, previous.Seed, c.Operation, act.ID, candidate})
		}
		if err == nil {
			err = c.Publish(act)
		}
		if err == nil && act.Status == "applied" {
			err = save(c.Root, filepath.Join(directory, "acts", act.ID+".json"), act)
		}
	}()
	if !changed && c.Act == "" {
		if candidate.Values != previous.Snapshot.Values {
			previous.Act = ""
		}
		act.ID, act.Status = previous.Act, "unchanged"
		return act, nil
	}
	identity, _ := json.Marshal([]any{c.Goal, c.Unit, c.Operation, act.Predecessor, act.BeforeDeclaration, act.AfterDeclaration})
	act.ID, act.Status, act.ProposedAt = fmt.Sprintf("%x", sha256.Sum256(identity)), "proposed", c.Now.UTC()
	if c.Act != "" {
		act.ID = c.Act
	}
	if filepath.Base(act.ID) != act.ID {
		return act, fmt.Errorf("invalid declaration act id")
	}
	actPath := filepath.Join(directory, "acts", act.ID+".json")
	retained, problem := os.ReadFile(actPath)
	if problem == nil {
		var proposal ProcessAct
		if json.Unmarshal(retained, &proposal) != nil || proposal.Class != "declaration" || proposal.ID != act.ID || proposal.Goal != c.Goal || proposal.Unit != c.Unit || proposal.Operation != c.Operation || proposal.AfterDeclaration == nil || !reflect.DeepEqual(proposal.AfterDeclaration, act.AfterDeclaration) {
			return act, fmt.Errorf("the act names a different declaration, tree or admission")
		}
		act = proposal
	} else if !os.IsNotExist(problem) || c.Act != "" {
		return act, fmt.Errorf("declaration act unavailable: %w", problem)
	}
	if previous.Operation != c.Operation && (act.BeforeDeclaration == nil || !reflect.DeepEqual(*act.BeforeDeclaration, previous.Snapshot) || act.Predecessor != previous.Act) {
		act.Status = "superseded"
		return act, save(c.Root, actPath, act)
	}
	if act.Status == "superseded" {
		return act, nil
	}
	if act.Status != "applied" {
		if err = save(c.Root, actPath, act); err != nil {
			return
		}
		if !c.Person {
			q, _, problem := channel.AskOrFind(channel.AskRequest{RepoRoot: c.Root, Goal: act.Goal, Kind: "other", Lineage: act.Lineage, ProcessAct: act.ID, Now: c.Now, Facts: []string{fmt.Sprintf("Committed checks from %q to %q", act.BeforeDeclaration.Values, act.AfterDeclaration.Values), act.Reason}, Wants: c.Remedy(act.ID)})
			if problem != nil {
				return act, problem
			}
			act.Question = q.ID
			return act, save(c.Root, actPath, act)
		}
		if err = c.Impact(); err != nil {
			return
		}
		act.AppliedProof = c.Proof
		act.Status, act.AppliedAt = "applied", c.Now.UTC()
		if err = write(actPath, act); err != nil {
			return
		}
	}
	return act, nil
}
