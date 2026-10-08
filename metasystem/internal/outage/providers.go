package outage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

type Interval struct{ Since, Until string }
type Condition struct {
	Mark      Mark
	ClearedAt string
	Intervals []Interval
}
type Providers struct {
	Owner   lane.Record
	Current map[string]Condition
	home    string
}

func Provider(runtime string) string {
	runtime = strings.TrimSuffix(runtime, "-headless")
	if provider := map[string]string{"claude": "anthropic", "codex": "openai", "codex-exec": "openai", "devin-print": "devin"}[runtime]; provider != "" {
		return provider
	}
	return runtime
}
func ReadProviders(home string) (Providers, error) {
	if home == "" {
		var err error
		home, err = board.Home()
		if err != nil {
			return Providers{}, err
		}
	}
	owner, ok, err := lane.Read(home)
	if err == nil && !ok {
		err = fmt.Errorf("provider ownership is unknown; a person registers it with metasystem landing set PATH")
	}
	s := Providers{Owner: owner, Current: map[string]Condition{}, home: home}
	if err != nil {
		return s, fmt.Errorf("provider ownership is unknown; a person repairs it with metasystem landing set PATH: %w", err)
	}
	lock, err := acquireMarkLock(owner.Install)
	if err != nil {
		return s, err
	}
	defer lock.release()
	return readProviders(home, owner)
}

// readProviders reads or carries conditions while the installation lock is held.
func readProviders(home string, owner lane.Record) (Providers, error) {
	s := Providers{Owner: owner, Current: map[string]Condition{}, home: home}
	path := filepath.Join(owner.Install, "artifacts", "agents", fmt.Sprintf("providers-%d.json", owner.CustodyEpoch))
	readPath := path
	data, err := os.ReadFile(readPath)
	carry := os.IsNotExist(err)
	if carry {
		for epoch := owner.CustodyEpoch; epoch > 1 && os.IsNotExist(err); {
			epoch--
			readPath = filepath.Join(owner.Install, "artifacts", "agents", fmt.Sprintf("providers-%d.json", epoch))
			data, err = os.ReadFile(readPath)
		}
	}
	if err != nil && !os.IsNotExist(err) {
		return s, fmt.Errorf("provider state %s is unreadable: %w", readPath, err)
	}
	if err == nil {
		s.Owner, s.Current = lane.Record{}, nil
		if err := json.Unmarshal(data, &s); err != nil {
			return s, fmt.Errorf("provider state %s is unreadable: %w", readPath, err)
		}
		if carry && s.Owner.Install == owner.Install && s.Owner.CustodyEpoch < owner.CustodyEpoch {
			s.Owner = owner
		}
	}

	after, _, err := lane.Read(home)
	if err != nil {
		return s, fmt.Errorf("provider ownership is unknown; a person repairs it with metasystem landing set PATH: %w", err)
	}
	if err == nil && (owner != after || s.Owner != owner || s.Current == nil) {
		err = fmt.Errorf("provider state %s is unreadable or ownership changed; inspect that file and run metasystem machine list again", path)
	}
	if err == nil && carry && len(data) > 0 {
		data, err = json.Marshal(s)
		if err == nil {
			err = atomicfile.WriteVolatile(path, string(data)+"\n")
		}
	}
	return s, err
}

func Observe(home, runtime, model, class, detail, source string, at time.Time) (Mark, error) {
	if runtime == "" || model == "" || at.IsZero() {
		return Mark{}, fmt.Errorf("provider evidence needs runtime, model and observation time")
	}
	s, err := ReadProviders(home)
	if err != nil {
		return Mark{}, err
	}
	owner, home := s.Owner, s.home
	lock, err := acquireMarkLock(owner.Install)
	if err != nil {
		return Mark{}, err
	}
	defer lock.release()
	s, err = readProviders(home, owner)
	if err != nil {
		return Mark{}, err
	}
	if s.Owner != owner {
		return Mark{}, fmt.Errorf("provider ownership changed before the observation")
	}
	key := Provider(runtime)
	c := s.Current[key]
	last, _ := time.Parse(time.RFC3339Nano, c.Mark.LastAt)
	cleared, _ := time.Parse(time.RFC3339Nano, c.ClearedAt)
	if at.Before(last) || (class != "" && !at.After(last)) || !at.After(cleared) {
		return c.Mark, nil
	}
	stamp := at.UTC().Format(time.RFC3339Nano)
	if class == "" {
		if c.Mark.ConsecutiveFailures > 0 {
			spans, err := s.Waiting(runtime, time.Time{}, at)
			if err != nil {
				return Mark{}, err
			}
			end := stamp
			if len(spans) > 0 {
				end = spans[len(spans)-1].End.UTC().Format(time.RFC3339Nano)
			}
			c.Intervals = append(c.Intervals, Interval{c.Mark.Since, end})
		}
		c.Mark, c.ClearedAt = Mark{}, stamp
	} else {
		if c.Mark.ConsecutiveFailures == 0 {
			c.Mark.Since = stamp
		}
		c.Mark.ConsecutiveFailures++
		c.Mark.Provider, c.Mark.Runtime, c.Mark.Model = key, runtime, model
		c.Mark.LastAt, c.Mark.LastClass, c.Mark.LastDetail, c.Mark.Source = stamp, class, clip(detail), source
		c.Mark.ResetAt = ""
		if reset, ok := limitReset(class, detail, at); ok {
			c.Mark.ResetAt = reset.UTC().Format(time.RFC3339Nano)
		}
	}
	s.Current[key] = c
	data, err := json.Marshal(s)
	if err == nil {
		err = atomicfile.WriteVolatile(filepath.Join(owner.Install, "artifacts", "agents", fmt.Sprintf("providers-%d.json", owner.CustodyEpoch)), string(data)+"\n")
	}
	return c.Mark, err
}

func (s Providers) Standing(runtime string, now time.Time) (Mark, bool) {
	return s.Current[Provider(runtime)].Mark.standingAt(now)
}
