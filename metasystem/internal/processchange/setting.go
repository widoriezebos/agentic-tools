// Package processchange records process settings before admitting their effects.
package processchange

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/roots"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

type ProcessAct struct {
	SelectedArgv, FullArgv                                           []string
	BeforeDeclaration, AfterDeclaration                              *Declaration
	Unit, Operation, Predecessor                                     string
	BeforeArgv, AfterArgv, ApplicableArgv                            []string
	SettingsSHA256                                                   string
	Undo                                                             string
	Unset                                                            bool
	ID, Goal, Lineage, Actor, Checkout, Key, Layer, Class            string
	Proof, AppliedProof                                              humanauthority.Proof
	Before                                                           *string
	After, Rule, Digest, Citation, Measure, Reason, Question, Status string
	ProposedAt, AppliedAt                                            time.Time
}

type Setting struct {
	Root, Act, Conf, PersonName string
	ProcessAct
	Person bool
	Now    time.Time
	Remedy func(string) string
}

// ApplySetting reconciles target bytes under process and settings locks.
func ApplySetting(s Setting) (act ProcessAct, err error) {
	if err = os.MkdirAll(filepath.Join(s.Root, "process", "acts"), 0700); err != nil {
		return
	}
	held, err := lock.File(filepath.Join(s.Root, "process", "lock"), 0600, lock.TryExclusive)
	if err != nil {
		return
	}
	defer held.Release()
	err = config.WithPolicyLock(s.Checkout, func() error {
		current, present, err := config.ConfLookup(s.Conf+".local", s.Key)
		var before *string
		if present {
			before = &current
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		matches := func(value *string) bool {
			return (before == nil && value == nil) || (before != nil && value != nil && *before == *value)
		}
		act = s.ProcessAct
		if act.Undo != "" {
			if s.Act != "" || filepath.Base(act.Undo) != act.Undo || strings.ContainsAny(act.Undo, `/\\`) {
				return fmt.Errorf("undo needs one original process act id")
			}
			body, err := os.ReadFile(filepath.Join(s.Root, "process", "acts", act.Undo+".json"))
			var original ProcessAct
			if err != nil || json.Unmarshal(body, &original) != nil {
				return fmt.Errorf("original process act unavailable: %v", err)
			}
			if original.ID != act.Undo || (original.Status != "applied" && (original.Status != "superseded" || original.AppliedAt.IsZero())) || original.Checkout != act.Checkout || original.Key != act.Key || original.Layer != act.Layer || act.Unset != (original.Before == nil) || (!act.Unset && act.After != *original.Before) {
				return fmt.Errorf("undo does not restore the original setting layer")
			}
			act.Goal = original.Goal
		}
		if act.Undo == "" && s.Act == "" && before != nil && *before == act.After {
			act.Status = "unchanged"
			return nil
		}
		act.Before, act.ProposedAt, act.Status = before, s.Now.UTC(), "proposed"
		identity, _ := json.Marshal([]any{act.Checkout, act.Key, before, act.After, act.Goal, act.Lineage, act.Rule, act.Measure, act.Reason, act.Undo, act.Unset})
		act.ID = fmt.Sprintf("%x", sha256.Sum256(identity))
		if s.Act != "" {
			act.ID = s.Act
		} else if s.Person {
			act.ID, err = goal.NewOperationULID()
			if err != nil {
				return err
			}
		}
		if filepath.Base(act.ID) != act.ID || strings.ContainsAny(act.ID, `/\\`) {
			return fmt.Errorf("invalid process act id")
		}
		var path string
		for {
			path = filepath.Join(s.Root, "process", "acts", act.ID+".json")
			data, readErr := os.ReadFile(path)
			if os.IsNotExist(readErr) && s.Act == "" {
				break
			}
			if readErr != nil {
				return fmt.Errorf("process history unavailable: %w", readErr)
			}
			var retained ProcessAct
			if err := json.Unmarshal(data, &retained); err != nil {
				return fmt.Errorf("process history unavailable: %w", err)
			}
			if retained.ID != act.ID || retained.Checkout != act.Checkout || retained.Key != act.Key || retained.After != act.After || (s.Goal != "" && retained.Goal != s.Goal) {
				return fmt.Errorf("the process act names a different target or value")
			}
			if s.Act == "" && !s.Person && (retained.Status == "applied" || retained.Status == "superseded") && !matches(&retained.After) {
				identity, _ = json.Marshal([]any{json.RawMessage(identity), retained.ID})
				act.ID = fmt.Sprintf("%x", sha256.Sum256(identity))
				continue
			}
			act = retained
			break
		}
		if act.Status == "superseded" || (!matches(&act.After) && (act.Status == "applied" || !matches(act.Before))) {
			act.Status = "superseded"
			return save(s.Root, path, act)
		}
		if err := save(s.Root, path, act); err != nil {
			return err
		}
		if !s.Person {
			about := ""
			if act.Goal == "" {
				about = "machine"
			}
			delta, _ := json.Marshal(act.Before)
			q, _, err := channel.AskOrFind(channel.AskRequest{RepoRoot: s.Root, Goal: act.Goal, About: about, Kind: "other", Lineage: act.Lineage, ProcessAct: act.ID, Now: s.Now, Facts: []string{"Process act " + act.ID + ": " + act.Key + " from " + string(delta) + " to " + act.After, act.Citation, act.Reason + "; expected measure: " + act.Measure}, Wants: s.Remedy(act.ID)})
			act.Question = q.ID
			if err != nil {
				return err
			}
			return save(s.Root, path, act)
		}
		if (!act.Unset && !matches(&act.After)) || (act.Unset && before != nil) {
			if _, err := os.Stat(s.Conf + ".local"); os.IsNotExist(err) {
				if err := os.WriteFile(s.Conf+".local", nil, 0600); err != nil {
					return err
				}
			}
			if err := validate.SetConfKeys(s.Conf+".local", []validate.ConfSetting{{Key: act.Key, Value: act.After, Unset: act.Unset}}); err != nil {
				return err
			}
			if err := config.RecordPolicy(act.Checkout, s.Conf, act.Key, act.After, s.PersonName, s.Now); err != nil {
				return err
			}
		}
		act.Status, act.AppliedProof = "applied", s.ProcessAct.Proof
		if local, err := os.ReadFile(s.Conf + ".local"); err == nil {
			act.SettingsSHA256 = fmt.Sprintf("%x", sha256.Sum256(local))
		}
		if act.AppliedAt.IsZero() {
			act.AppliedAt = s.Now.UTC()
		}
		if err := save(s.Root, path, act); err != nil {
			return err
		}
		return resolveUndo(s.Root, roots.Installation(filepath.Dir(s.Conf)), act)
	})
	return
}

func save(root, path string, act ProcessAct) error {
	data, _ := json.MarshalIndent(act, "", "  ")
	if _, err := atomicfile.WriteFile(path, append(data, '\n'), 0600, ""); err != nil {
		return err
	}
	if act.Question == "" || (act.Status != "applied" && act.Status != "superseded") {
		return nil
	}
	q, err := channel.ReadQuestion(root, act.Question)
	if err != nil {
		return err
	}
	if q.ProcessAct != act.ID || q.Goal != act.Goal || q.Kind != "other" {
		return fmt.Errorf("the question belongs to a different process act")
	}
	_, err = channel.Withdraw(root, q.ID, "process act "+act.ID+" "+act.Status, nil, channel.DestinationConfig{})
	return err
}

// ReadActs reads a goal's process changes and names unreadable records.
func ReadActs(root, goal string) (acts []ProcessAct, unknown []string, err error) {
	entries, err := os.ReadDir(filepath.Join(root, "process", "acts"))
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, "process", "acts", entry.Name()))
		var act ProcessAct
		if err != nil || json.Unmarshal(body, &act) != nil {
			unknown = append(unknown, "process act "+entry.Name()+" unavailable")
		} else if goal == "" || act.Goal == goal {
			acts = append(acts, act)
		}
	}
	return acts, unknown, nil
}
