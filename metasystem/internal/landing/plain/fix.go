package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// Fix is one bounded repair of a batch, retained beside its brief.
type Fix struct {
	Goal   string   `json:"goal"`
	Units  []string `json:"units"`
	Job    string   `json:"job"`
	Read   string   `json:"read"`
	Commit string   `json:"commit"`
	State  string   `json:"state"`

	Attempt   string                 `json:"attempt,omitempty"`
	Paths     []conflict.Path        `json:"paths,omitempty"`
	Tip       string                 `json:"tip,omitempty"`
	Reason    string                 `json:"reason,omitempty"`
	Brief     string                 `json:"brief,omitempty"`
	Generated []testpolicy.Generated `json:"generated,omitempty"`
}

func ReadFix(install string) (*Fix, error) { return readFix(install, false) }

func readFix(install string, includeClosed bool) (*Fix, error) {
	paths, err := filepath.Glob(filepath.Join(Dir(install), "fixes", "*.json"))
	if err != nil {
		return nil, err
	}
	var active, latest *Fix
	var newest time.Time
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var fix Fix
		if err := json.Unmarshal(data, &fix); err != nil {
			return nil, err
		}
		if includeClosed {
			info, err := os.Stat(path)
			if err != nil {
				return nil, err
			}
			if latest == nil || info.ModTime().After(newest) {
				latest, newest = &fix, info.ModTime()
			}
		}
		if fix.State != "running" && fix.State != "reviewing" && fix.State != "resolving" && fix.State != "resolved" {
			continue
		}
		if fix.Goal == "" || len(fix.Units) == 0 || fix.Commit == "" || (fix.State != "resolving" && fix.Job == "") || (fix.State == "resolving" && (fix.Tip == "" || len(fix.Paths) == 0)) {
			return nil, fmt.Errorf("incomplete lane fix in %s", path)
		}
		if active != nil {
			return nil, errors.New("more than one lane fix is running")
		}
		active = &fix
	}
	if active == nil && latest != nil && latest.State == "abandoned" {
		return latest, nil
	}
	return active, nil
}

func fixHeadline(fix *Fix) string {
	if fix.State == "resolved" {
		return fmt.Sprintf("Resolved merge of %s, read pending: %s", fix.Goal, fix.Reason)
	}
	if fix.State == "resolving" {
		var paths []string
		for _, path := range fix.Paths {
			paths = append(paths, path.Path)
		}
		return fmt.Sprintf("Resolving %d conflicts of %s (%s)", len(paths), fix.Goal, strings.Join(paths, ", "))
	}
	return fmt.Sprintf("Fixing %s of %s on %s (fix round 1)", strings.Join(fix.Units, ", "), fix.Goal, Short(fix.Commit))
}

func WriteFix(install string, fix *Fix) error {
	path := filepath.Join(Dir(install), "fixes", fix.Attempt+".json")
	data, err := json.Marshal(fix)
	if err != nil {
		return err
	}
	return atomicfile.WriteVolatileFile(path, data, 0o600)
}

// RefreshMerge closes a resolution whose pending merge no longer exists.
func RefreshMerge(install, checkout string, fix *Fix, seams ProveSeams) error {
	if fix == nil || fix.State != "resolving" {
		return nil
	}
	return withLock(install, func() error { return refreshMergeLocked(install, checkout, fix, seams) })
}

func refreshMergeLocked(install, checkout string, fix *Fix, seams ProveSeams) error {
	if fix == nil || fix.State != "resolving" {
		return nil
	}
	// Status can wait behind a commit; the current record owns the transition.
	data, err := os.ReadFile(filepath.Join(Dir(install), "fixes", fix.Attempt+".json"))
	if err != nil {
		return err
	}
	var current Fix
	if err := json.Unmarshal(data, &current); err != nil {
		return err
	}
	*fix = current
	if fix.State != "resolving" {
		return nil
	}
	tip, err := seams.git(checkout, "rev-parse", "--verify", "MERGE_HEAD^{commit}")
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 128 {
			return err
		}
		fix.Reason = "the pending merge is gone; its resolution was abandoned"
	} else if tip != fix.Tip {
		fix.Reason = "the pending merge changed; its previous resolution was abandoned"
	} else {
		return nil
	}
	fix.State = "abandoned"
	return WriteFix(install, fix)
}

// RecordedFixRed binds an ordinary repair to the failed proof's checkpoint.
func RecordedFixRed(install string, running Running) bool {
	mode := ProveSeams{Gate: running.Gate}
	results, err := readLines[Result](mode.resultsPath(install))
	if err != nil {
		return false
	}
	for _, result := range slices.Backward(results) {
		if result.Commit == running.Commit && result.Tree == running.Tree && result.Attempt == running.Attempt && result.BatchID == running.BatchID && slices.Equal(result.BatchMembers, running.BatchMembers) {
			return result.Result == Red
		}
	}
	return false
}

func proofFixReady(install string, seams ProveSeams) error {
	if seams.Trunk {
		return nil
	}
	fix, err := ReadFix(install)
	if err != nil {
		return err
	}
	if fix != nil && (fix.State == "resolving" || fix.State == "resolved" || fix.State == "reviewing") {
		message := fmt.Sprintf("finish the resolution and its read for %s before landing prove (repair is %s)", fix.Goal, fix.State)
		if fix.Reason != "" {
			message += ": " + fix.Reason
		}
		return errors.New(message)
	}
	return nil
}
