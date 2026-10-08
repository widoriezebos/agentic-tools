package processchange

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processmeasure"
)

type DriftStop struct {
	ID, Goal, Episode, Question string
	Stop                        loopstop.Stop
}

type State struct {
	Acts    []ProcessAct
	Stops   []DriftStop
	Unknown []string
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
		} else if (goal == "" || stop.Goal == goal) && stop.Stop.Decision == "stop" {
			state.Stops = append(state.Stops, stop)
		}
		return nil
	})
	return state, err
}

// Observe keeps one unresolved stop per goal and unit under the settings process lock.
func Observe(root, goal, episode string, drift processmeasure.Drift) error {
	if len(drift.Stops) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Join(root, "process"), 0700); err != nil {
		return err
	}
	held, err := lock.File(filepath.Join(root, "process", "lock"), 0600, lock.TryExclusive)
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
	for _, stop := range state.Stops {
		if strings.HasPrefix(stop.Stop.Subject, unitPrefix) {
			if stop.Question != "" {
				return nil
			}
			retained = &stop
		}
	}
	id := fmt.Sprintf("%x", sha256.Sum256([]byte(goal+"/"+episode+"/"+subject[1])))
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
	q, _, err := channel.AskOrFind(channel.AskRequest{RepoRoot: root, Goal: goal, Kind: "other", Now: driftTime, Facts: []string{"Process drift " + id + ": " + s.Stop.Class, "Observed " + fmt.Sprint(s.Stop.Measure.Now) + "; allowance " + fmt.Sprint(s.Stop.Measure.Previous), "Consumed process-act attribution unavailable"}, Wants: "A person must decide the next scoped process change; ordinary admitted work may continue."})
	if err != nil {
		return err
	}
	s.Question, s.Stop.Handoff = q.ID, "ask "+q.ID
	return write()
}
