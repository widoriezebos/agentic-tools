package gittree

import (
	"runtime"
	"strings"
)

// FsyncComponents covers the loose object, pack and reference files Git publishes.
func FsyncComponents(value string) bool {
	var add, remove int
	for _, item := range strings.Split(value, ",") {
		word := strings.TrimLeft(item, " \t\r\n")
		component := strings.TrimPrefix(word, "-")
		bits := map[string]int{"all": 7, "committed": 3, "added": 3, "objects": 3, "loose-object": 1, "pack": 2, "reference": 4}[component]
		if word == component {
			add |= bits
		} else {
			remove |= bits
		}
	}
	return add&^remove == 7
}

// PrepareFsync sets the local minimum once. A preparation warning is advisory.
func PrepareFsync(root string, git func(string, ...string) (string, error)) (changed bool, warning string) {
	if runtime.GOOS != "linux" {
		return false, ""
	}
	value, _ := git(root, "config", "--local", "--no-includes", "--get", "core.fsync")
	if FsyncComponents(strings.TrimSuffix(value, "\n")) {
		return false, ""
	}
	if _, err := git(root, "config", "--local", "--replace-all", "core.fsync", "committed,reference"); err != nil {
		return false, "Could not set core.fsync in " + root
	}
	return true, ""
}
