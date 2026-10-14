package processchange

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processmeasure"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/roots"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/shellquote"
)

type DriftStop struct {
	ID, Goal, Episode, Question string
	Impact                      string
	Resolution, Undo            string
	Stop                        loopstop.Stop
}

type State struct {
	Acts     []ProcessAct
	Stops    []DriftStop
	Resolved []DriftStop
	Unknown  []string
}

// ReadState preserves unreadable history instead of interpreting it as no changes.
func ReadState(root, goal string) (state State, err error) {
	state.Acts, state.Unknown, err = ReadActs(root, goal)
	if err != nil {
		return
	}
	err = filepath.WalkDir(filepath.Join(root, "process", "episodes"), func(path string, entry fs.DirEntry, problem error) error {
		if os.IsNotExist(problem) && path == filepath.Join(root, "process", "episodes") {
			return nil
		}
		if problem != nil {
			return problem
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			return nil
		}
		body, problem := os.ReadFile(path)
		var stop DriftStop
		if problem != nil || json.Unmarshal(body, &stop) != nil || stop.ID == "" || stop.Goal == "" || stop.Episode == "" || stop.Stop.Subject == "" || stop.Stop.Decision == "" || stop.Stop.Loop != "process" {
			state.Unknown = append(state.Unknown, "process stop unavailable: "+path)
		} else if goal == "" || stop.Goal == goal {
			if stop.Stop.Decision == "stop" {
				state.Stops = append(state.Stops, stop)
			} else if stop.Resolution == "resolved by removal" {
				state.Resolved = append(state.Resolved, stop)
			}
		}
		return nil
	})
	return state, err
}

// Observe keeps one unresolved stop per goal and unit under the settings process
// lock in project state, and asks the person through the serving installation.
func Observe(root string, installation roots.Installation, goal, episode string, drift processmeasure.Drift) error {
	if len(drift.Stops) == 0 {
		return nil
	}
	held, err := processLock(root, lock.TryExclusive)
	if err != nil {
		return err
	}
	defer held.Release()
	state, err := ReadState(root, goal)
	if err != nil || len(state.Unknown) > 0 {
		return fmt.Errorf("process history unavailable: %v %v", err, state.Unknown)
	}
	var retained *DriftStop
	subject := strings.SplitN(drift.Stops[0].Subject, "/", 3)
	unitPrefix := strings.Join(subject[:2], "/") + "/"
	for _, stop := range state.Resolved {
		if strings.HasPrefix(stop.Stop.Subject, unitPrefix) && stop.Stop.Evidence == drift.Stops[0].Evidence {
			return nil
		}
	}
	for _, stop := range state.Stops {
		if strings.HasPrefix(stop.Stop.Subject, unitPrefix) {
			if stop.Question != "" {
				if stop.Stop.Evidence != drift.Stops[0].Evidence {
					stop.Stop.Evidence = drift.Stops[0].Evidence
					body, _ := json.MarshalIndent(stop, "", "  ")
					_, err := atomicfile.WriteFile(filepath.Join(root, "process", "episodes", stop.ID, "stops", stop.ID+".json"), append(body, '\n'), 0600, "")
					return err
				}
				return nil
			}
			retained = &stop
		}
	}
	id := fmt.Sprintf("%x", sha256.Sum256([]byte(goal+"/"+episode+"/"+subject[1]+"/"+drift.Stops[0].Evidence)))
	s := DriftStop{ID: id, Goal: goal, Episode: episode, Stop: drift.Stops[0]}
	if retained != nil {
		s = *retained
		id = s.ID
	}
	path := filepath.Join(root, "process", "episodes", id, "stops", id+".json")
	write := func() error {
		body, _ := json.MarshalIndent(s, "", "  ")
		_, err := atomicfile.WriteFile(path, append(body, '\n'), 0600, "")
		return err
	}
	if err := write(); err != nil {
		return err
	}
	driftTime, _ := time.Parse(time.RFC3339Nano, s.Stop.At)
	wants, impact := "A person must decide the next scoped process change; ordinary admitted work may continue.", "Consumed process-act attribution unavailable"
	if s.Stop.Cause != nil && s.Stop.Cause.Kind == "process-change" {
		for _, act := range state.Acts {
			if act.ID == s.Stop.Cause.Name {
				wants = settingReverse(act)
				before, _ := json.Marshal(act.Before)
				if act.Before == nil {
					before = []byte("inheritance (no local override)")
				}
				impact = fmt.Sprintf("Restore %s from %q to %s; removes this intervention hold, recorded cost stays above band", act.Key, act.After, before)
			}
		}
	}
	s.Impact = impact
	q, _, err := channel.AskOrFind(channel.AskRequest{RepoRoot: installation.Path(), Goal: goal, Kind: "other", Now: driftTime, Facts: []string{"Process drift " + id + ": " + s.Stop.Class, "Observed " + fmt.Sprint(s.Stop.Measure.Now) + "; allowance " + fmt.Sprint(s.Stop.Measure.Previous), impact}, Wants: wants})
	if err != nil {
		return err
	}
	s.Question, s.Stop.Handoff = q.ID, "ask "+q.ID
	if s.Stop.Cause != nil && s.Stop.Cause.Kind == "process-change" {
		s.Stop.Handoff = wants
	}
	return write()
}

func settingReverse(act ProcessAct) string {
	if act.AfterDeclaration != nil {
		return "metasystem work revert " + shellquote.Word(act.Goal) + " --act " + act.ID + " --reason TEXT --repo " + shellquote.Word(act.Checkout)
	}
	command := fmt.Sprintf("metasystem settings unset %s", act.Key)
	if act.Before != nil {
		command = fmt.Sprintf("metasystem settings set %s %s", act.Key, shellquote.Word(*act.Before))
	}
	return command + " --repo " + shellquote.Word(act.Checkout) + " --undo " + act.ID
}

// resolveUndo removes only the hold attributed to the successfully reversed act.
func resolveUndo(root string, installation roots.Installation, act ProcessAct) error {
	if act.Undo == "" {
		return nil
	}
	state, err := ReadState(root, act.Goal)
	if err != nil {
		return err
	}
	for _, stop := range append(state.Stops, state.Resolved...) {
		if stop.Stop.Cause == nil || stop.Stop.Cause.Kind != "process-change" || stop.Stop.Cause.Name != act.Undo {
			continue
		}
		if stop.Stop.Decision == "stop" {
			stop.Resolution, stop.Undo, stop.Stop.Decision = "resolved by removal", act.ID, "close"
			body, _ := json.MarshalIndent(stop, "", "  ")
			path := filepath.Join(root, "process", "episodes", stop.ID, "stops", stop.ID+".json")
			if _, err := atomicfile.WriteFile(path, append(body, '\n'), 0600, ""); err != nil {
				return err
			}
		}
		if stop.Question != "" {
			if _, err := channel.Withdraw(installation.Path(), stop.Question, stop.Resolution, nil, channel.DestinationConfig{}); err != nil {
				return err
			}
		}
	}
	return nil
}

// PrepareInverse freezes the person's delta before releasing process ownership.
func PrepareInverse(root string, installation roots.Installation, original, branch, path, current string, explicit []byte, act ProcessAct, now time.Time) (ProcessAct, error) {
	if original == "" || filepath.Base(original) != original || strings.ContainsAny(original, `/\`) {
		return act, fmt.Errorf("invalid original act id")
	}
	held, err := processLock(root, lock.Exclusive)
	if err != nil {
		return act, err
	}
	defer held.Release()
	act.ID = fmt.Sprintf("%x", sha256.Sum256([]byte(act.Checkout+"/"+act.Goal+"/"+branch+"/inverse/"+original)))
	target := filepath.Join(root, "process", "acts", act.ID+".json")
	if body, err := os.ReadFile(target); err == nil {
		var retained ProcessAct
		if json.Unmarshal(body, &retained) != nil || retained.ID != act.ID || retained.Goal != act.Goal || retained.Checkout != act.Checkout || retained.Undo != original {
			return act, fmt.Errorf("inverse history unavailable")
		}
		if explicit != nil && string(explicit) != string(retained.Patch) {
			return act, fmt.Errorf("the retained inverse has a different patch")
		}
		if explicit != nil {
			retained.AfterDeclaration, retained.Citation = nil, "declaration reference unavailable; explicit person repair"
		}
		retained.Proof = act.Proof
		return retained, nil
	} else if !os.IsNotExist(err) {
		return act, err
	}
	var source ProcessAct
	body, readErr := os.ReadFile(filepath.Join(root, "process", "acts", original+".json"))
	available := readErr == nil && json.Unmarshal(body, &source) == nil && source.ID == original
	if available && (source.Goal != act.Goal || source.Checkout != act.Checkout || source.AfterDeclaration == nil || source.AfterDeclaration.Branch != branch) {
		return act, fmt.Errorf("the original act belongs to a different goal, branch or target")
	}
	if explicit == nil {
		if !available || source.BeforeDeclaration == nil || source.InheritedDeclaration == nil || source.Status != "applied" {
			return act, fmt.Errorf("inverse evidence unavailable; supply an explicit --patch FILE")
		}
		next := current
		for i, key := range []string{"proof.cheap", "proof.audits", "proof.deadline", "proof.full"} {
			before, after := source.BeforeDeclaration.Values[i], source.AfterDeclaration.Values[i]
			if before == after || source.InheritedDeclaration != nil && after == source.InheritedDeclaration.Values[i] {
				continue
			}
			value, _, err := config.CommittedContentLookup(current, key)
			if err != nil || value != after {
				return act, fmt.Errorf("%s changed again; supply an explicit --patch FILE", key)
			}
			line := regexp.MustCompile(`(?m)^[ \t]*` + regexp.QuoteMeta(key) + `[ \t]*=[^\n]*(?:\n|$)`)
			replacement := line.FindString(source.BeforeDeclaration.Content)
			if line.MatchString(next) {
				next = line.ReplaceAllStringFunc(next, func(string) string { return replacement })
			} else {
				next += replacement
			}
		}
		if next == current {
			return act, fmt.Errorf("this act has no current goal-owned declaration delta")
		}
		patchSide := func(prefix, content string) string {
			value := strings.TrimSuffix(strings.TrimPrefix(strings.ReplaceAll("\n"+content, "\n", "\n"+prefix), "\n"), prefix)
			if value != "" && !strings.HasSuffix(content, "\n") {
				value += "\n\\ No newline at end of file\n"
			}
			return value
		}
		explicit = []byte(fmt.Sprintf("--- a/%s\n+++ b/%s\n@@ -1,%d +1,%d @@\n%s%s", path, path, strings.Count("\n"+patchSide("-", current), "\n-"), strings.Count("\n"+patchSide("+", next), "\n+"), patchSide("-", current), patchSide("+", next)))
		restoredDeclaration := *source.AfterDeclaration
		restoredDeclaration.Content, restoredDeclaration.SourceSHA256 = next, fmt.Sprintf("%x", sha256.Sum256([]byte(next)))
		for i, key := range []string{"proof.cheap", "proof.audits", "proof.deadline", "proof.full"} {
			restoredDeclaration.Values[i], _, _ = config.CommittedContentLookup(next, key)
		}
		source.BeforeDeclaration = &restoredDeclaration
	} else {
		source.BeforeDeclaration = nil
	}
	act.Undo, act.Operation, act.Class, act.Status, act.Actor = original, act.ID, "declaration-inverse", "pending", "direct-person"
	act.Patch, act.ProposedAt, act.BeforeDeclaration, act.AfterDeclaration = explicit, now.UTC(), source.AfterDeclaration, source.BeforeDeclaration
	if !available || source.BeforeDeclaration == nil {
		act.Citation = "inverse history unavailable; explicit person patch"
	}
	return act, save(installation, target, act)
}

// CompleteInverse closes only the original intervention after confirmed publication.
func CompleteInverse(root string, installation roots.Installation, act ProcessAct, commit string, now time.Time) (ProcessAct, error) {
	if commit == "" {
		return act, fmt.Errorf("inverse publication is unconfirmed")
	}
	held, err := processLock(root, lock.Exclusive)
	if err != nil {
		return act, err
	}
	defer held.Release()
	if act.Status == "applied" {
		return act, resolveUndo(root, installation, act)
	}
	act.Status, act.PublishedCommit, act.AppliedAt, act.AppliedProof = "applied", commit, now.UTC(), act.Proof
	if act.AfterDeclaration != nil {
		path := filepath.Join(root, "process", fmt.Sprintf("declaration-%x.json", sha256.Sum256([]byte(act.Goal))))
		body, err := os.ReadFile(path)
		var reference declarationReference
		if err != nil || json.Unmarshal(body, &reference) != nil || reference.Goal != act.Goal {
			return act, fmt.Errorf("restored declaration reference unavailable")
		}
		act.AfterDeclaration.Commit = commit
		reference.Act, reference.Snapshot = "", *act.AfterDeclaration
		data, _ := json.Marshal(reference)
		if _, err := atomicfile.WriteFile(path, data, 0600, ""); err != nil {
			return act, err
		}
	}
	if err := save(installation, filepath.Join(root, "process", "acts", act.ID+".json"), act); err != nil {
		return act, err
	}
	return act, resolveUndo(root, installation, act)
}
