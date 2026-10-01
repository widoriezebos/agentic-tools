package batch

import (
	"path/filepath"
	"sort"
	"strings"
)

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
