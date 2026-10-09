package outage

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

const ProbeInterval = 2 * time.Minute

func (m Mark) waitBound() (time.Time, error) {
	last, err := time.Parse(time.RFC3339Nano, m.LastAt)
	if err != nil {
		return time.Time{}, err
	}
	if m.ResetAt != "" {
		reset, err := time.Parse(time.RFC3339Nano, m.ResetAt)
		return reset.Add(ProbeInterval), err
	}
	return last.Add(Horizon), nil
}

func (c *Condition) close(at time.Time, stale bool) {
	stamp := at.UTC().Format(time.RFC3339Nano)
	c.Intervals = append(c.Intervals, Interval{Since: c.Mark.Since, Until: stamp, Stale: stale})
	c.Mark, c.ClearedAt = Mark{}, stamp
}

func (s Providers) write() error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteText(filepath.Join(s.Owner.Install, "artifacts", "agents", fmt.Sprintf("providers-%d.json", s.Owner.CustodyEpoch)), string(data)+"\n", s.Owner.Install)
	if err == nil && !durable {
		err = fmt.Errorf("provider state was published with durability unknown")
	}
	return err
}

func (s Providers) expire(at time.Time) error {
	for provider, c := range s.Current {
		if c.Mark.ConsecutiveFailures == 0 {
			continue
		}
		bound, err := c.Mark.waitBound()
		since, sinceErr := time.Parse(time.RFC3339Nano, c.Mark.Since)
		if err != nil || sinceErr != nil || bound.Before(since) {
			return fmt.Errorf("provider %s observation or reset is unreadable", provider)
		}
		if !at.Before(bound) {
			c.close(bound, true)
			s.Current[provider] = c
		}
	}
	return nil
}

// ExpireAndNotify runs under the steward's tick arbitration. Delivery releases
// the provider lock so a slow notifier cannot suppress fresh provider evidence.
func ExpireAndNotify(home, self string, at time.Time, notify func(string, Interval) error) error {
	s, err := ReadProviders(home)
	if !lane.OwnsLane(self, s.Owner) {
		return nil
	}
	if err != nil {
		return err
	}
	lock, err := acquireMarkLock(s.Owner.Install)
	if err != nil {
		return err
	}
	defer func() {
		if lock != nil {
			lock.release()
		}
	}()
	s, err = readProviders(s.home, s.Owner)
	if err != nil {
		return err
	}
	if err := s.expire(at); err != nil {
		return err
	}
	if err := s.write(); err != nil {
		return err
	}
	lock.release()
	lock = nil
	for provider, c := range s.Current {
		for i, interval := range c.Intervals {
			if !interval.Stale || interval.AlertDelivered {
				continue
			}
			if err := notify(provider, interval); err != nil {
				return err
			}
			lock, err = acquireMarkLock(s.Owner.Install)
			if err != nil {
				return err
			}
			current, err := readProviders(s.home, s.Owner)
			if err != nil {
				return err
			}
			c := current.Current[provider]
			if i >= len(c.Intervals) || c.Intervals[i].Since != interval.Since || c.Intervals[i].Until != interval.Until {
				return fmt.Errorf("provider %s wait history changed during notification", provider)
			}
			c.Intervals[i].AlertDelivered = true
			current.Current[provider] = c
			if err := current.write(); err != nil {
				return err
			}
			lock.release()
			lock = nil
		}
	}
	return nil
}

// Clear removes one advisory hold without inventing a provider answer.
// The public machine verb owns proof that this is a person's act.
func Clear(home, provider string, at time.Time) error {
	if provider == "" || at.IsZero() {
		return fmt.Errorf("provider clear needs a provider and a time")
	}
	s, err := ReadProviders(home)
	if err != nil {
		return err
	}
	lock, err := acquireMarkLock(s.Owner.Install)
	if err != nil {
		return err
	}
	defer lock.release()
	s, err = readProviders(s.home, s.Owner)
	if err != nil {
		return err
	}
	if err := s.expire(at); err != nil {
		return err
	}
	key := Provider(provider)
	c := s.Current[key]
	if c.Mark.ConsecutiveFailures == 0 {
		return s.write()
	}
	last, err := time.Parse(time.RFC3339Nano, c.Mark.LastAt)
	if err != nil || at.Before(last) {
		return fmt.Errorf("provider %s cannot be cleared before its observation", key)
	}
	if _, err := s.Waiting(key, time.Time{}, at); err != nil {
		return err
	}
	c.close(at, false)
	s.Current[key] = c
	return s.write()
}
