package batch

import (
	"bytes"
	"strconv"
	"strings"
)

type patchChange map[string]bool // path -> deleted
type gateChange struct {
	Deleted, BaseAbsent bool
	baseKnown           bool
}
type gateChanges map[string]gateChange

func patchChangedPaths(patch []byte) patchChange {
	return patchGateChanges(patch).paths()
}

func (changes gateChanges) paths() patchChange {
	paths := patchChange{}
	for path, change := range changes {
		paths[path] = change.Deleted
	}
	return paths
}

func patchGateChanges(patch []byte) gateChanges {
	set := gateChanges{}
	old := ""
	header := false
	update := func(path string, deleted, baseAbsent, baseKnown bool) {
		path = strings.TrimPrefix(strings.TrimPrefix(path, "a/"), "b/")
		if path == "" || path == "/dev/null" {
			return
		}
		change, present := set[path]
		if !present || baseKnown && !change.baseKnown {
			change.BaseAbsent, change.baseKnown = baseAbsent, baseKnown
		}
		change.Deleted = deleted
		set[path] = change
	}
	for _, line := range bytes.Split(patch, []byte("\n")) {
		if bytes.HasPrefix(line, []byte("diff --git ")) {
			if _, changed, ok := strings.Cut(string(line), " b/"); ok {
				update(decodePatchPath(changed), false, false, false)
			}
			header, old = true, ""
		} else if header && bytes.HasPrefix(line, []byte("rename from ")) {
			update(decodePatchPath(string(line[len("rename from "):])), true, false, true)
		} else if header && bytes.HasPrefix(line, []byte("rename to ")) {
			update(decodePatchPath(string(line[len("rename to "):])), false, true, true)
		} else if header && bytes.HasPrefix(line, []byte("--- ")) {
			old = decodePatchPath(string(line[4:]))
		} else if header && bytes.HasPrefix(line, []byte("+++ ")) {
			changed := decodePatchPath(string(line[4:]))
			deleted := changed == "/dev/null"
			if deleted {
				changed = old
			}
			update(changed, deleted, old == "/dev/null", true)
			header = false
		}
	}
	return set
}

// ChangedPaths derives a unit's path manifest from its certified patch.
func ChangedPaths(patch []byte) map[string]bool { return patchChangedPaths(patch) }

func decodePatchPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, `"`) {
		if decoded, err := strconv.Unquote(raw); err == nil {
			return decoded
		}
	}
	return raw
}
