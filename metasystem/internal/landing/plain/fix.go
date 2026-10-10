package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

// Fix is one bounded repair of a batch, retained beside its brief.
type Fix struct {
	Attempt string   `json:"attempt"`
	Round   int      `json:"round"`
	Parent  string   `json:"parent"`
	Verdict string   `json:"verdict,omitempty"`
	Goal    string   `json:"goal"`
	Units   []string `json:"units"`
	Job     string   `json:"job"`
	Read    string   `json:"read"`
	Commit  string   `json:"commit"`
	State   string   `json:"state"`
}

func readFix(install string) (*Fix, error) {
	paths, err := filepath.Glob(filepath.Join(Dir(install), "fixes", "*.json"))
	if err != nil {
		return nil, err
	}
	var active *Fix
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var fix Fix
		if err := json.Unmarshal(data, &fix); err != nil {
			return nil, err
		}
		if fix.State != "running" && fix.State != "reviewing" && fix.State != "building" {
			continue
		}
		if fix.Goal == "" || len(fix.Units) == 0 || fix.Job == "" || fix.State != "building" && fix.Commit == "" {
			return nil, fmt.Errorf("incomplete lane fix in %s", path)
		}
		if active != nil {
			return nil, errors.New("more than one lane fix is running")
		}
		if fix.Attempt == "" {
			fix.Attempt = strings.TrimSuffix(filepath.Base(path), ".json")
		}
		active = &fix
	}
	return active, nil
}

func fixHeadline(fix *Fix) string {
	commit := fix.Commit
	if commit == "" {
		commit = fix.Parent
	}
	round := max(1, fix.Round)
	return fmt.Sprintf("Fixing %s of %s on %s (fix round %d)", strings.Join(fix.Units, ", "), fix.Goal, Short(commit), round)
}

// WriteFix retains a fix's lifecycle beside its attempt's brief.
func WriteFix(install string, fix Fix) error {
	data, err := json.Marshal(fix)
	if err != nil {
		return err
	}
	dir := filepath.Join(Dir(install), "fixes")
	_, err = atomicfile.WriteText(filepath.Join(dir, fix.Attempt+".json"), string(data)+"\n", dir)
	return err
}

func ActiveFix(install string) (*Fix, error) { return readFix(install) }

func closeFix(install, goal string) error {
	fix, err := readFix(install)
	if err != nil || fix == nil || goal != "" && fix.Goal != goal {
		return err
	}
	fix.State = "closed"
	return WriteFix(install, *fix)
}
