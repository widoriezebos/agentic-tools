package rootaudit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// BaselineFile lists, at the module root, every crossing still open and the
// work that must remove it.
const BaselineFile = "run-state-audit.json"

// Entry is one listed crossing: its key, how many times the scan may find
// it, and the owner, the work that must remove it.
type Entry struct {
	Owner    string `json:"owner"`
	Count    int    `json:"count"`
	File     string `json:"file"`
	Function string `json:"function"`
	Source   string `json:"source"`
	Sink     string `json:"sink"`
}

func (e Entry) Key() Key { return Key{e.File, e.Function, e.Source, e.Sink} }

type baseline struct {
	Sites []Entry `json:"sites"`
}

// ReadBaseline loads and validates the list.
func ReadBaseline(path string) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("run-state audit list unreadable: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var list baseline
	if err := decoder.Decode(&list); err != nil {
		return nil, fmt.Errorf("run-state audit list unparsable: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("run-state audit list unparsable: more than one JSON value")
	}
	seen := map[Key]bool{}
	for _, e := range list.Sites {
		if e.Owner == "" || e.Count < 1 || e.File == "" || e.Function == "" || e.Source == "" || e.Sink == "" {
			return nil, fmt.Errorf("run-state audit list entry is incomplete: %+v", e)
		}
		if seen[e.Key()] {
			return nil, fmt.Errorf("run-state audit list names %s twice", e.Key())
		}
		seen[e.Key()] = true
	}
	return list.Sites, nil
}

// Check compares a scan with the list and returns one refusal per key that
// differs. A key found more often than its entry allows, or found with no
// entry, is a new crossing. An entry that allows more than the scan finds is
// a fixed site still listed. The list only shrinks: nothing here rewrites it.
func Check(entries []Entry, sites []Site) []string {
	found := map[Key][]Site{}
	for _, s := range sites {
		found[s.Key()] = append(found[s.Key()], s)
	}
	listed := map[Key]Entry{}
	for _, e := range entries {
		listed[e.Key()] = e
	}
	keys := make([]Key, 0, len(found))
	for k := range found {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
	var refusals []string
	for _, k := range keys {
		at, e := found[k], listed[k]
		if len(at) <= e.Count {
			continue
		}
		owner := e.Owner
		if owner == "" {
			owner = "none"
		}
		lines := make([]string, len(at))
		for i, s := range at {
			lines[i] = fmt.Sprintf("%s:%d", s.File, s.Line)
		}
		refusals = append(refusals, fmt.Sprintf("run-state audit: new crossing %s (owner %s; found %d, listed %d; at %s; reaches %s): run state lives under the installation, so build this path from the installation",
			k, owner, len(at), e.Count, strings.Join(lines, ", "), at[0].Witness))
	}
	for _, e := range entries {
		n := len(found[e.Key()])
		if n >= e.Count {
			continue
		}
		action := "delete this entry from " + BaselineFile
		if n > 0 {
			action = fmt.Sprintf("lower this entry's count to %d in %s", n, BaselineFile)
		}
		refusals = append(refusals, fmt.Sprintf("run-state audit: fixed site still listed %s (owner %s; listed %d, found %d): %s", e.Key(), e.Owner, e.Count, n, action))
	}
	return refusals
}
