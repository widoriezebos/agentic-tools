package diskstore

// What the engine itself placed in a goal or session worktree (Round D3
// ruling F-1): the adapters' declared local configuration copied in at
// creation, and a directory the engine owns (a session's main
// announcements). Each file is recorded with its sha256 when the engine
// writes it; an owned directory is recorded with the names and the format
// of the entries the engine writes there. The content check then excludes
// exactly those paths while they are unchanged: a changed engine file, an
// unknown name or format in an owned directory, and anything else untracked
// or ignored still keep the worktree.
//
// The record lives beside the store record, <registry>/<id>.engine.json
// (an older engine's inventory skips the name), because an engine verb
// writes it while it holds the store's record lock shared, and the only
// reader, the content check, runs inside the critical section, which no
// entrant shares.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// EngineContentSchema is the schema of a store's engine-content record.
const EngineContentSchema = "metasystem.diskstore.engine-content/1"

// EngineFile is one file the engine placed, by its path relative to the
// worktree and its sha256 when placed.
type EngineFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// EngineDir is a directory the engine owns inside the worktree: its direct
// entries whose names match Pattern and whose content has Format are the
// engine's own writes.
type EngineDir struct {
	Path    string `json:"path"`
	Pattern string `json:"pattern"`
	// Format is the one format this engine knows: "json-object".
	Format string `json:"format"`
}

// EngineContent is what the engine placed in one store.
type EngineContent struct {
	Schema string       `json:"schema"`
	Files  []EngineFile `json:"files,omitempty"`
	Dirs   []EngineDir  `json:"dirs,omitempty"`
}

// FormatJSONObject is an owned directory whose entries are JSON objects.
const FormatJSONObject = "json-object"

// FormatEngineState is a worktree installation's own state root
// (<installation>/artifacts), which only the engine writes: every regular
// file anywhere beneath it is the engine's (Round D3 F-1 as built; see the
// build's return).
const FormatEngineState = "engine-state"

// EngineContentPath is where a store's engine-content record lives.
func (r Registry) EngineContentPath(id string) string {
	return filepath.Join(r.Dir, id+".engine.json")
}

// ReadEngineContent reads a store's engine-content record; an absent one is
// empty. An unreadable one, or one of another schema, is an error: the
// caller keeps the store (fail-closed rule 1).
func (r Registry) ReadEngineContent(id string) (EngineContent, error) {
	data, err := os.ReadFile(r.EngineContentPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return EngineContent{Schema: EngineContentSchema}, nil
	}
	if err != nil {
		return EngineContent{}, err
	}
	var content EngineContent
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&content); err != nil {
		return EngineContent{}, fmt.Errorf("engine-content record of %s is unreadable: %w", id, err)
	}
	if content.Schema != EngineContentSchema {
		return EngineContent{}, fmt.Errorf("engine-content record of %s has schema %q", id, content.Schema)
	}
	return content, nil
}

// RecordEngineContent records files the engine has just placed in the
// worktree at root (each a file or a directory tree, relative to root) and
// directories it owns. A path already recorded keeps its first digest: a
// later change is someone else's, and the check keeps it. Symbolic links and
// anything but regular files are never recorded.
func (r Registry) RecordEngineContent(id, root string, placed []string, dirs []EngineDir) error {
	held, err := lock.File(filepath.Join(r.Dir, id+".engine.lock"), 0o600, lock.Exclusive)
	if err != nil {
		return err
	}
	defer held.Release()
	content, err := r.ReadEngineContent(id)
	if err != nil {
		return err
	}
	known := map[string]bool{}
	for _, file := range content.Files {
		known[file.Path] = true
	}
	changed := false
	for _, relative := range placed {
		relative = filepath.Clean(relative)
		if filepath.IsAbs(relative) || relative == "." || strings.HasPrefix(relative, "..") {
			return fmt.Errorf("engine content %q is not a path inside the worktree", relative)
		}
		err := filepath.WalkDir(filepath.Join(root, relative), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.Type().IsRegular() {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil || known[rel] {
				return err
			}
			sum, err := fileSHA256(path)
			if err != nil {
				return err
			}
			content.Files, known[rel], changed = append(content.Files, EngineFile{Path: rel, SHA256: sum}), true, true
			return nil
		})
		if err != nil {
			return err
		}
	}
	for _, dir := range dirs {
		dir.Path = filepath.Clean(dir.Path)
		present := false
		for _, existing := range content.Dirs {
			present = present || existing == dir
		}
		if !present {
			content.Dirs, changed = append(content.Dirs, dir), true
		}
	}
	if !changed {
		return nil
	}
	sort.Slice(content.Files, func(i, j int) bool { return content.Files[i].Path < content.Files[j].Path })
	content.Schema = EngineContentSchema
	data, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteFile(r.EngineContentPath(id), append(data, '\n'), 0o600, filepath.Dir(r.Dir))
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("engine-content record of %s is published but its durability is unconfirmed", id)
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// engineAccepts reports whether a file at rel (relative to the worktree at
// root) is the engine's own and unchanged; why names what keeps it.
func (c EngineContent) engineAccepts(root, rel string) (bool, string) {
	path := filepath.Join(root, rel)
	info, err := os.Lstat(path)
	if err != nil {
		return false, rel + " cannot be read (" + err.Error() + ")"
	}
	if !info.Mode().IsRegular() {
		return false, rel
	}
	for _, file := range c.Files {
		if file.Path != rel {
			continue
		}
		sum, err := fileSHA256(path)
		if err != nil {
			return false, rel + " cannot be read (" + err.Error() + ")"
		}
		if sum != file.SHA256 {
			return false, rel + " (the engine placed it and it has changed since)"
		}
		return true, ""
	}
	for _, dir := range c.Dirs {
		if dir.Format == FormatEngineState {
			if inside, err := filepath.Rel(dir.Path, rel); err == nil && inside != "." && !strings.HasPrefix(inside, "..") {
				return true, ""
			}
			continue
		}
		if filepath.Dir(rel) != dir.Path {
			continue
		}
		if matched, err := filepath.Match(dir.Pattern, filepath.Base(rel)); err != nil || !matched {
			return false, rel + " (not a name the engine writes in " + dir.Path + ")"
		}
		if dir.Format != FormatJSONObject {
			return false, rel + " (an owned directory of an unknown format)"
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return false, rel + " cannot be read (" + err.Error() + ")"
		}
		var object map[string]json.RawMessage
		if json.Unmarshal(data, &object) != nil || object == nil {
			return false, rel + " (not the engine's format in " + dir.Path + ")"
		}
		return true, ""
	}
	return false, rel
}

// foreignEntries filters git status entries (porcelain v1, -z) of the
// worktree at root down to those that keep it: every entry but an
// untracked or ignored path whose every file is the engine's own and
// unchanged. A directory entry is walked; an unreadable one keeps.
func (c EngineContent) foreignEntries(root string, entries []string) []string {
	var foreign []string
	for _, entry := range entries {
		if len(entry) < 4 {
			foreign = append(foreign, entry)
			continue
		}
		status, rel := entry[:3], entry[3:]
		if status != "?? " && status != "!! " {
			foreign = append(foreign, entry)
			continue
		}
		if !strings.HasSuffix(rel, "/") {
			if ok, why := c.engineAccepts(root, filepath.FromSlash(rel)); !ok {
				foreign = append(foreign, status+why)
			}
			continue
		}
		dir := filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(rel, "/")))
		err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			fileRel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			if ok, why := c.engineAccepts(root, fileRel); !ok {
				foreign = append(foreign, status+why)
				return filepath.SkipAll
			}
			return nil
		})
		if err != nil {
			foreign = append(foreign, status+rel+" cannot be read ("+err.Error()+")")
		}
	}
	return foreign
}

// Placed lists which of relatives were absent under root before a copy
// (absent) and exist after it: what that copy placed. before is
// PresentPaths taken before the copy.
func Placed(root string, relatives []string, before map[string]bool) []string {
	var placed []string
	for relative, present := range PresentPaths(root, relatives) {
		if present && !before[relative] {
			placed = append(placed, relative)
		}
	}
	sort.Strings(placed)
	return placed
}

// PresentPaths reports, for each relative path, whether anything exists at
// it under root.
func PresentPaths(root string, relatives []string) map[string]bool {
	present := map[string]bool{}
	for _, relative := range relatives {
		_, err := os.Lstat(filepath.Join(root, relative))
		present[relative] = err == nil
	}
	return present
}

// EngineStateDir is a worktree installation's state root, which only the
// engine writes.
func EngineStateDir(installation string) EngineDir {
	return EngineDir{Path: filepath.Join(installation, "artifacts"), Pattern: "*", Format: FormatEngineState}
}

// MainAnnouncementsDir is the engine-owned directory of main announcements
// inside an installation: its *.json entries are JSON objects the engine
// writes (the session announcements, the worktree lease and commit token).
func MainAnnouncementsDir(installation string) EngineDir {
	return EngineDir{Path: filepath.Join(installation, "artifacts", "agents", "mains"), Pattern: "*.json", Format: FormatJSONObject}
}
