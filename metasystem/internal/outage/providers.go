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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

type Interval struct {
	RecoveryAfter  string `json:"recoveryAfter,omitempty"`
	ResetAt        string `json:"resetAt,omitempty"`
	Since, Until   string
	FirstSuccessAt string `json:"firstSuccessAt,omitempty"`
	Stale          bool   `json:"stale,omitempty"`
	AlertDelivered bool   `json:"alertDelivered,omitempty"`
}
type Condition struct {
	Mark      Mark
	ClearedAt string
	Intervals []Interval
}

// RecoveryDue never turns a reset, person clear or stale expiry into success.
func (i Interval) RecoveryDue() (time.Time, error) {
	delay, err := time.ParseDuration(i.RecoveryAfter)
	if err != nil || delay <= 0 {
		return time.Time{}, fmt.Errorf("provider episode %s lacks a readable recovery interval declaration", i.Since)
	}
	if i.FirstSuccessAt == "" {
		return time.Time{}, nil
	}
	success, err := time.Parse(time.RFC3339Nano, i.FirstSuccessAt)
	return success.Add(delay), err
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
	if err := s.expire(at); err != nil {
		return Mark{}, err
	}
	c := s.Current[key]
	last, _ := time.Parse(time.RFC3339Nano, c.Mark.LastAt)
	cleared, _ := time.Parse(time.RFC3339Nano, c.ClearedAt)
	if at.Before(last) || (class != "" && (!at.After(last) || !at.After(cleared))) || at.Before(cleared) {
		return c.Mark, s.write()
	}
	stamp := at.UTC().Format(time.RFC3339Nano)
	if class == "" {
		if c.Mark.ConsecutiveFailures > 0 {
			c.close(at, false)
		}
		for i := range c.Intervals {
			interval := &c.Intervals[i]
			until, err := time.Parse(time.RFC3339Nano, interval.Until)
			if err != nil {
				return Mark{}, fmt.Errorf("provider %s wait history is unreadable", key)
			}
			reset, _ := time.Parse(time.RFC3339Nano, interval.ResetAt)
			if interval.FirstSuccessAt == "" && !at.Before(until) && !at.Before(reset) {
				interval.FirstSuccessAt = stamp
			}
		}
		c.Mark, c.ClearedAt = Mark{}, stamp
	} else {
		if c.Mark.ConsecutiveFailures == 0 {
			conf := filepath.Join(owner.Install, "metasystem.conf")
			if _, err := os.Stat(conf); os.IsNotExist(err) {
				conf = ""
			}
			value, _, err := config.Get(config.GetParams{Key: "provider.recovery-alert-after", ConfPath: conf})
			c.Mark.RecoveryAfter = ""
			if delay, parseErr := time.ParseDuration(value); err == nil && parseErr == nil && delay > 0 {
				c.Mark.RecoveryAfter = value
			}
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
	return c.Mark, s.write()
}

func (s Providers) Standing(runtime string, now time.Time) (Mark, bool) {
	return s.Current[Provider(runtime)].Mark.standingAt(now)
}
