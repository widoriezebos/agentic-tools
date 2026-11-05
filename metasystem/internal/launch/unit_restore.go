package launch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

type excludedFile struct {
	Path   string `json:"path"`
	New    bool   `json:"new"`
	SHA256 string `json:"sha256"`
}

func (runner *UnitRunner) restoreExcluded(record *UnitRunRecord, round *UnitRound, worktree string) (string, error) {
	archive := filepath.Join(round.Directory, "excluded")
	manifestPath := filepath.Join(archive, "excluded.json")
	var manifest []excludedFile
	data, err := os.ReadFile(manifestPath)
	fresh := os.IsNotExist(err)
	if !fresh {
		if err != nil {
			return "", err
		}
		if err := json.Unmarshal(data, &manifest); err != nil {
			return "", err
		}
	}
	status, err := runner.git().Run(worktree, []string{"GIT_OPTIONAL_LOCKS=0"}, "status", "--porcelain=v2", "-z", "--untracked-files=all", "--no-renames")
	if err != nil {
		return "", err
	}
	current := map[string]excludedFile{}
	for _, raw := range strings.Split(string(status), "\x00") {
		if raw == "" {
			continue
		}
		fields := strings.SplitN(raw, " ", 9)
		if raw[0] == '?' {
			fields = strings.SplitN(raw, " ", 2)
		}
		if raw[0] == 'u' {
			fields = strings.SplitN(raw, " ", 11)
		}
		path := fields[len(fields)-1]
		if !runner.Excluded(path) {
			continue
		}
		entry := excludedFile{Path: path, New: raw[0] == '?'}
		if entry.New || raw[0] == '1' && (fields[1] == ".M" || fields[1] == "M.") {
			entry.SHA256 = excludedPlainDigest(filepath.Join(worktree, path))
		}
		current[path] = entry
	}
	paths := make([]string, 0, len(current))
	for path := range current {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	if fresh {
		for _, path := range paths {
			entry := current[path]
			if entry.SHA256 == "" {
				return "the build left " + path + ", an engine-written path, in a shape other than a changed or new plain file; the engine restores plain files only; restore it to the branch's version or move it out of the worktree, then run work review with --reason and --by to retry", nil
			}
			manifest = append(manifest, entry)
		}
		if len(manifest) == 0 {
			return "", nil
		}
		if err := os.MkdirAll(filepath.Join(archive, "files"), 0700); err != nil {
			return "", err
		}
		for n, entry := range manifest {
			if entry.New {
				continue
			}
			kept := filepath.Join(archive, "files", fmt.Sprint(n))
			if _, err := atomicfile.CopyFile(filepath.Join(worktree, entry.Path), kept, archive); err != nil {
				return "", err
			}
			if excludedPlainDigest(kept) != entry.SHA256 {
				return "", fmt.Errorf("kept copy of %s differs from the build's bytes", entry.Path)
			}
		}
		if err := writeUnitJSON(manifestPath, manifest, archive); err != nil {
			return "", err
		}
	}
	round.ExcludedDropped = nil
	for _, entry := range manifest {
		round.ExcludedDropped = append(round.ExcludedDropped, entry.Path)
	}
	if err := runner.save(*record); err != nil {
		return "", err
	}
	repeatStop := func(path string) (string, error) {
		return "the worktree's " + path + " changed after the engine kept the build's engine-written files in " + archive + "/; restore it to the branch's version, then run work review with --reason and --by to retry", nil
	}
	for _, path := range paths {
		n := slices.IndexFunc(manifest, func(entry excludedFile) bool { return entry.Path == path })
		if n < 0 || current[path].SHA256 == "" || current[path] != manifest[n] {
			return repeatStop(path)
		}
	}
	for n, entry := range manifest {
		kept := filepath.Join(archive, "files", fmt.Sprint(n))
		_, differing := current[entry.Path]
		if entry.New && differing {
			if _, err := os.Lstat(kept); !os.IsNotExist(err) {
				return repeatStop(entry.Path)
			}
		} else {
			if excludedPlainDigest(kept) != entry.SHA256 {
				return repeatStop(entry.Path)
			}
		}
		if entry.New && !differing {
			if _, err := os.Lstat(filepath.Join(worktree, entry.Path)); !os.IsNotExist(err) {
				return repeatStop(entry.Path)
			}
		}
	}
	for n, entry := range manifest {
		if entry.New {
			if _, differing := current[entry.Path]; differing {
				if err := os.Rename(filepath.Join(worktree, entry.Path), filepath.Join(archive, "files", fmt.Sprint(n))); err != nil {
					return "", err
				}
			}
		} else if _, differing := current[entry.Path]; differing {
			if _, err := runner.git().Run(worktree, nil, "--literal-pathspecs", "checkout", "HEAD", "--", entry.Path); err != nil {
				return "", err
			}
		}
	}
	round.ExcludedRestored = true
	return "", nil
}

func excludedPlainDigest(path string) string {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return digestHex(data)
}
