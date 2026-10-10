package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Fix is one bounded repair of a batch, retained beside its brief.
type Fix struct {
	Goal   string   `json:"goal"`
	Units  []string `json:"units"`
	Job    string   `json:"job"`
	Read   string   `json:"read"`
	Commit string   `json:"commit"`
	State  string   `json:"state"`
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
		if fix.State != "running" && fix.State != "reviewing" {
			continue
		}
		if fix.Goal == "" || len(fix.Units) == 0 || fix.Job == "" || fix.Commit == "" {
			return nil, fmt.Errorf("incomplete lane fix in %s", path)
		}
		if active != nil {
			return nil, errors.New("more than one lane fix is running")
		}
		active = &fix
	}
	return active, nil
}

func fixHeadline(fix *Fix) string {
	return fmt.Sprintf("Fixing %s of %s on %s (fix round 1)", strings.Join(fix.Units, ", "), fix.Goal, Short(fix.Commit))
}
