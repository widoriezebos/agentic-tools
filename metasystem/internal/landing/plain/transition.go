package plain

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
)

// transitionWrite runs inside the raw record's owning lock. Only fields changed
// by this write are applied; unrelated hand edits remain visible to the keeper.
func transitionWrite(install, kind, by string, now time.Time, to string, raw func() error, effects ...ProveSeams) error {
	s := firstEffects(effects)
	s.Now = func() time.Time { return now }
	if s.TransitionBy != "" {
		by = s.TransitionBy
	}
	home := s.BudgetHome
	if home == "" {
		home, _ = board.Home()
	}
	registration, present, registrationErr := lane.Read(home)
	registered := registrationErr == nil && present && filepath.Clean(registration.Install) == filepath.Clean(install)
	_, _, retirement := strings.Cut(kind, ":")
	if !registered && !retirement {
		return raw()
	}
	rawCalled := false
	err := state.WithWriter(Dir(install), func(w *state.Writer) error {
		var before Observation
		var observationErr error
		if registered {
			before, observationErr = captureObservation(home, &registration, s, nil)
		}
		rawCalled = true
		if err := raw(); err != nil {
			return err
		}
		note := func(err error) error {
			if err != nil {
				state.ReportFailure(Dir(install), now, by, err)
			}
			return nil
		}
		if observationErr != nil {
			return note(observationErr)
		}
		if registered && !before.Coherent {
			return note(w.NoteUnstable(now))
		}
		if registered && len(before.Status.Problems) > 0 {
			return note(w.NoteObservation(now, before.Status.Problems))
		}
		if !registered {
			_, err := w.Transition(now, kind, "present", to, by, kind+"\n"+s.TransitionEvidence, nil)
			return note(err)
		}
		old := ShadowOf(before.Status, before.Status.Raw)
		// A push moves the Git reference before its confirmation record is written.
		// Its transition owns main and the waiting hand-ins that reference landed.
		if kind == "main" && before.Captured.Shadow != nil {
			shadow := before.Captured.Shadow
			old.Main = shadow.Main
			for i, entry := range old.HandIns {
				if entry.Word != StateLanded {
					continue
				}
				for _, previous := range shadow.HandIns {
					if previous.Goal == entry.Goal && previous.SHA == entry.SHA && previous.Ordinal == entry.Ordinal && previous.Word == StateWaiting {
						old.HandIns[i].Fact = previous.Fact
						break
					}
				}
			}
		}
		after, err := captureObservation(home, &registration, s, nil)
		if err != nil {
			return note(err)
		}
		if !after.Coherent {
			return note(w.NoteUnstable(now))
		}
		if len(after.Status.Problems) > 0 {
			return note(w.NoteObservation(now, after.Status.Problems))
		}
		next := ShadowOf(after.Status, after.Status.Raw)
		diff := state.Diff(old, next)
		if len(diff) == 0 && !retirement {
			return nil
		}
		if kind == "batch.execution" && old.Batch.Execution.Word == next.Batch.Execution.Word && old.Batch.Admission.Word != next.Batch.Admission.Word {
			kind = "batch.admission"
		}
		from := "present"
		if !retirement {
			from, to = state.Word(old, kind), state.Word(next, kind)
		}
		_, err = w.Transition(now, kind, from, to, by, kind+"\n"+s.TransitionEvidence+strings.Join(diff, "\n"), func(record *state.Record) {
			a, b, target := reflect.ValueOf(old), reflect.ValueOf(next), reflect.ValueOf(record).Elem()
			for i := 0; i < a.NumField(); i++ {
				if !reflect.DeepEqual(a.Field(i).Interface(), b.Field(i).Interface()) {
					target.Field(i).Set(b.Field(i))
				}
			}
		})
		return note(err)
	})
	if !rawCalled {
		if rawErr := raw(); rawErr != nil {
			return rawErr
		}
		state.ReportFailure(Dir(install), now, by, err)
		return nil
	}
	return err
}

func transitionLine(path string, value any, raw func() error, effects ...ProveSeams) error {
	kind, by, stamp := "", "", ""
	switch v := value.(type) {
	case Line:
		kind, by, stamp = "hand_ins", "hand-in", v.At
		if v.Outcome == StateReturned {
			by = "return"
		}
	case Result:
		kind, by, stamp = "proof", "prove", v.At
		if v.Scope == "gate" {
			by = "gate"
		}
	case Pushed:
		kind, by, stamp = "main", "push", v.At
	case Stop:
		kind, by, stamp = "stop", "prove", v.At
		if v.Decision == "close" && len(strings.Fields(v.Handoff)) > 0 {
			by = strings.Fields(v.Handoff)[0]
		}
	default:
		return raw()
	}
	now, _ := time.Parse(time.RFC3339Nano, stamp)
	install := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(path))))
	return transitionWrite(install, kind, by, now, "", raw, effects...)
}

func removeRunning(install string, effects ...ProveSeams) error {
	s := firstEffects(effects)
	return transitionWrite(install, "proof", "prove", s.now(), "", func() error { return os.Remove(runningPath(install)) }, s)
}

func firstEffects(effects []ProveSeams) ProveSeams {
	if len(effects) > 0 {
		return effects[0]
	}
	return ProveSeams{}
}

func (s ProveSeams) transitionAs(by string) ProveSeams {
	if s.TransitionBy == "" {
		s.TransitionBy = by
	}
	return s
}
