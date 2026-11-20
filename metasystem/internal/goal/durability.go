package goal

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

func commonStore(path string) string {
	if path == "" {
		return ""
	}
	dir, err := gitIn(path, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return ""
	}
	dir, _ = filepath.EvalSymlinks(strings.TrimSuffix(dir, "\n"))
	return dir
}

// Git resolves a relative push destination from the checkout top, even when
// the ledger's working directory is a nested installation.
func checkoutTop(path string) string {
	if _, err := os.Lstat(filepath.Join(path, ".git")); err == nil || gittree.OutsideRepository(path) {
		return path
	}
	if top, err := gitIn(path, "rev-parse", "--show-toplevel"); err == nil {
		return strings.TrimSuffix(top, "\n")
	}
	return path
}

func ownedStores(home, root string) (map[string]string, error) {
	stores := map[string]string{}
	add := func(path string) {
		// Git otherwise searches upwards from a former checkout directory.
		if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
			if _, err := gitIn(path, "rev-parse", "--resolve-git-dir", "."); err != nil {
				return
			}
		}
		if common := commonStore(path); common != "" {
			stores[common] = path
		}
	}
	record, ok, _, laneErr := lane.ReadGuarded(home)
	sources := []string{root}
	if ok && laneErr == nil {
		add(record.Root)
		sources = append(sources, record.Root)
	}
	// Supervision locates the installations that keep launch records. A
	// supervised checkout alone conveys no ownership of its Git store.
	rows, err := registry.HostCheckouts(filepath.Join(home, "armed-checkouts.jsonl"))
	for _, row := range rows {
		sources = append(sources, row.Path)
	}
	for _, source := range sources {
		if !filepath.IsAbs(source) {
			continue
		}
		source = checkoutTop(source)
		dir := filepath.Join(source, "artifacts", "agents", "ui", "launches")
		entries, _ := os.ReadDir(dir)
		for _, entry := range entries {
			if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			tracked, err := gitIn(source, "ls-files", "--", path)
			if err != nil || tracked != "" {
				continue
			}
			var launch struct {
				Launch, Destination string
				Created             struct{ Destination bool }
			}
			data, e := os.ReadFile(path)
			if e == nil && json.Unmarshal(data, &launch) == nil && launch.Created.Destination && filepath.IsAbs(launch.Destination) && filepath.Base(path) == launch.Launch+".json" {
				add(launch.Destination)
			}
		}
	}
	return stores, errors.Join(err, laneErr)
}

// Resolve push URLs without displaying their credentials. An empty path denotes
// a receiver whose filesystem this engine cannot address.
func pushReceivers(root, remote string) []string {
	out, err := gitIn(root, "remote", "get-url", "--push", "--all", "--", remote)
	if err != nil {
		out, _ = gitIn(root, "-c", "remote.metasystem-probe.url="+remote, "remote", "get-url", "--push", "metasystem-probe")
	}
	var receivers []string
	for path := range strings.SplitSeq(strings.TrimSuffix(out, "\n"), "\n") {
		if strings.HasPrefix(path, "file://") {
			u, err := url.Parse(path)
			if err != nil || u.User != nil || (u.Host != "" && u.Host != "localhost") {
				path = ""
			} else {
				path = u.Path
			}
		} else if strings.Contains(path, ":") && !filepath.IsAbs(path) {
			path = ""
		}
		if path != "" && !filepath.IsAbs(path) {
			path = filepath.Join(checkoutTop(root), path)
		}
		receivers = append(receivers, path)
	}
	return receivers
}

func DurabilityReport(home, root string) ([]string, error) {
	stores, err := ownedStores(home, root)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, path := range stores {
		get := func(args ...string) string {
			out, _ := gitIn(path, append([]string{"config"}, args...)...)
			return strings.TrimSpace(out)
		}
		local := get("--local", "--no-includes", "--get", "core.fsync")
		method := get("--get", "core.fsyncMethod")
		if method == "" {
			method = "fsync"
			if runtime.GOOS == "darwin" {
				method = "writeout-only"
			}
		}
		verdict := "below minimum (Linux submits the directory entries, but file contents may be lost in a crash)"
		if local == "" {
			local = "missing"
		} else if gittree.FsyncComponents(local) && gittree.FsyncComponents(get("--get", "core.fsync")) && method == "fsync" {
			verdict = "meets minimum (Git syncs file contents and Linux submits the directory entries)"
		}
		lines = append(lines, fmt.Sprintf("%s: local core.fsync %q, effective core.fsyncMethod %q, %s", path, local, method, verdict))
	}
	for _, path := range append(slices.Collect(maps.Values(stores)), root) {
		e, err := ResolveEndpoint(path)
		if err != nil {
			return nil, err
		}
		receivers := []string{path}
		if !e.LocalMode() {
			receivers = pushReceivers(path, e.Remote)
		}
		for i, dest := range receivers {
			if _, held := stores[commonStore(dest)]; !held {
				lines = append(lines, fmt.Sprintf("%s: ledger receiver %d not controlled", path, i+1))
			}
		}
	}
	slices.Sort(lines)
	return slices.Compact(lines), nil
}
