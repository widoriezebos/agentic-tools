package batch

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type WaitClock struct {
	Now   func() time.Time
	After func(time.Duration) <-chan time.Time
}

func WaitComplete(state string) bool {
	return state == StateLanded || state == StateDissolved || state == StateHeldTrunkRed || state == StateHeldUnclassified
}

// Wait observes durable records until a terminal or held state. The caller's
// injected clock owns both progress and the hard bound.
func Wait(store Store, id string, bound time.Duration, clock WaitClock) (Record, error) {
	if bound <= 0 || clock.Now == nil || clock.After == nil {
		return Record{}, fmt.Errorf("BATCH_WAIT_BOUND: a positive bound and injected clock are required")
	}
	started := clock.Now()
	interval := bound / 10
	if interval <= 0 {
		interval = bound
	}
	for {
		record, err := store.Load(id)
		if err != nil {
			return Record{}, err
		}
		if WaitComplete(record.State) {
			return record, nil
		}
		if !clock.Now().Before(started.Add(bound)) {
			return record, fmt.Errorf("BATCH_WAIT_BOUND: batch %s remained %s", id, record.State)
		}
		<-clock.After(interval)
	}
}

// Records returns the durable batches in id order.
func (store Store) Records() ([]Record, error) {
	paths, err := filepath.Glob(filepath.Join(store.root, "artifacts", "agents", "landing-batches", "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	records := make([]Record, 0, len(paths))
	for _, path := range paths {
		id := strings.TrimSuffix(filepath.Base(path), ".json")
		record, err := store.Load(id)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func FindByGoal(store Store, goalID string) (Record, error) {
	records, err := store.Records()
	if err != nil {
		return Record{}, err
	}
	for _, record := range records {
		for _, unit := range record.Units {
			if unit.GoalID == goalID {
				return record, nil
			}
		}
	}
	return Record{}, fmt.Errorf("batch for goal %s is absent", goalID)
}
