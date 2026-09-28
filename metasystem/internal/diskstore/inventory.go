package diskstore

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// Measure is a stat walk: the allocated bytes (st_blocks x 512) of every
// entry under path, never following a symlink, bounded by the context. A
// walk the context cuts short reports complete=false and the bytes so far.
func Measure(ctx context.Context, path string) (bytes int64, newest time.Time, complete bool) {
	err := filepath.WalkDir(path, func(entry string, info fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return nil
		}
		stat, statErr := info.Info()
		if statErr != nil {
			return nil
		}
		if raw, ok := stat.Sys().(*syscall.Stat_t); ok {
			bytes += int64(raw.Blocks) * 512
		} else {
			bytes += stat.Size()
		}
		if stat.ModTime().After(newest) {
			newest = stat.ModTime()
		}
		return nil
	})
	return bytes, newest, err == nil
}

// Consumer is one large unregistered consumer on a watched volume, named at
// the floor with what reclaims it (the 2026-09-29 amendment). Machinery
// never deletes one.
type Consumer struct {
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Bytes    int64  `json:"bytes"`
	Measured bool   `json:"measured"`
	AgeSecs  int64  `json:"ageSeconds"`
	Use      string `json:"use"`
	Command  string `json:"command"`
}

// Line renders a consumer for a person.
func (c Consumer) Line() string {
	size := formatBytes(c.Bytes)
	if !c.Measured {
		size = "at least " + size + " (not fully measured within the pass budget)"
	}
	return fmt.Sprintf("%s (%s): %s, last written %s ago, %s; %s", c.Path, c.Kind, size,
		(time.Duration(c.AgeSecs) * time.Second).Round(time.Minute), c.Use, c.Command)
}

// ConsumerRoot is a place the floor looks: the root itself (Children
// false) or each entry directly under it.
type ConsumerRoot struct {
	Path     string
	Kind     string
	Children bool
	// Engine marks the engine's own namespace, whose unregistered entries a
	// person removes with `metasystem disk clean --strays`.
	Engine bool
}

// ConsumerLimit is how many consumers the floor names.
const ConsumerLimit = 20

// InventoryConsumers measures the roots' entries, skips every registered
// store path, and names the largest with a live or idle judgement from the
// use census when one was taken. It deletes nothing and creates nothing.
func InventoryConsumers(ctx context.Context, now time.Time, roots []ConsumerRoot, registered map[string]bool, census *UseCensus) []Consumer {
	var consumers []Consumer
	for _, root := range roots {
		paths := []string{root.Path}
		if root.Children {
			paths = nil
			entries, err := os.ReadDir(root.Path)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				paths = append(paths, filepath.Join(root.Path, entry.Name()))
			}
		}
		for _, path := range paths {
			if registered[path] {
				continue
			}
			if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSymlink != 0 {
				continue
			}
			bytes, newest, complete := Measure(ctx, path)
			consumer := Consumer{Path: path, Kind: root.Kind, Bytes: bytes, Measured: complete, Use: "use not judged (no complete use census)"}
			if !newest.IsZero() {
				consumer.AgeSecs = int64(now.Sub(newest) / time.Second)
			}
			if census != nil && census.Complete() {
				if holders := census.Holders(path); len(holders) != 0 {
					consumer.Use = fmt.Sprintf("in use by pid %d (%s)", holders[0].Pid, holders[0].Command)
				} else {
					consumer.Use = "no live process has it open"
				}
			}
			if root.Engine {
				consumer.Command = "metasystem disk clean --preview, then metasystem disk clean --strays"
			} else {
				consumer.Command = "a person removes it once nothing needs it: rm -rf -- " + shellQuote(path)
			}
			consumers = append(consumers, consumer)
		}
	}
	sort.SliceStable(consumers, func(i, j int) bool { return consumers[i].Bytes > consumers[j].Bytes })
	if len(consumers) > ConsumerLimit {
		consumers = consumers[:ConsumerLimit]
	}
	return consumers
}

func shellQuote(path string) string {
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

// strayPrefixes are the engine-prefixed TMPDIR names (3.10) that no store
// owns; bare tmp.* entries are counted as foreign.
var strayPrefixes = []string{"metasystem-", "goal-txn-", "steward-test-home-", "goal-branch-read-gate-", "mission-reap-patch.",
	"goal-land-prep-", "goal-read-", "goal-branch-tree-", "mission-"}

// notStrays are engine-prefixed names another owner reconciles: the
// process-scratch parent and testenv's registry homes and their sidecars.
var notStrays = []string{"metasystem-test-registry-", "metasystem-test-process-"}

// StrayIdle is how long an engine-prefixed entry must be idle before it is
// a stray a person may remove (3.8: "idle a day").
const StrayIdle = 24 * time.Hour

// TempStrays is the TMPDIR recognizer class (3.10): engine-prefixed entries
// no store owns are strays, reported with size and idle time; bare tmp.*
// entries are foreign, counted. It observes only; strays go only by a
// person's `disk clean --strays` from a preview.
type TempStrays struct {
	Roots []string
}

func (TempStrays) Name() string { return "tmpdir strays" }

func (t TempStrays) Plan(ctx context.Context, pass *Pass) ([]Item, error) {
	var items []Item
	seen := map[string]bool{}
	for _, root := range t.Roots {
		resolved, err := filepath.EvalSymlinks(root)
		if err != nil || seen[resolved] {
			continue
		}
		seen[resolved] = true
		entries, err := os.ReadDir(resolved)
		if err != nil {
			continue
		}
		foreign := 0
		for _, entry := range entries {
			name := entry.Name()
			path := filepath.Join(resolved, name)
			switch {
			case strings.HasPrefix(name, "tmp."):
				foreign++
				continue
			case name == "metasystem" || hasAnyPrefix(name, notStrays) || !hasAnyPrefix(name, strayPrefixes):
				continue
			}
			info, err := os.Lstat(path)
			if err != nil {
				continue
			}
			bytes, newest, complete := Measure(ctx, path)
			if newest.IsZero() {
				newest = info.ModTime()
			}
			idle := pass.Now.Sub(newest)
			reason := "engine-prefixed entry no store owns"
			if !complete {
				reason += " (size not fully measured within the pass budget)"
			}
			command := "metasystem disk clean --preview, then metasystem disk clean --strays"
			if idle < StrayIdle {
				reason += fmt.Sprintf(", written %s ago: not removable before it is idle a day", idle.Round(time.Minute))
				command = "metasystem disk show"
			}
			device, inode, _ := fileID(info)
			items = append(items, Item{Class: t.Name(), Key: path, Path: path, Bytes: bytes, Device: device, Inode: inode, Stray: true,
				IdleSecs: int64(idle / time.Second), Verdict: Verdict{Decision: Keep, Reason: reason, Command: command}})
		}
		if foreign > 0 {
			items = append(items, Item{Class: t.Name(), Key: filepath.Join(resolved, "tmp.*"), Path: filepath.Join(resolved, "tmp.*"), Foreign: true,
				Verdict: Verdict{Decision: Keep, Reason: fmt.Sprintf("%d bare tmp.* entries: not engine-named, never touched", foreign)}})
		}
	}
	return items, nil
}

func (TempStrays) Apply(context.Context, *Pass, Item) Verdict {
	return Verdict{Decision: Keep, Reason: "strays are removed only by a person", Command: "metasystem disk clean --strays"}
}

func hasAnyPrefix(name string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// errNotAStray is a strays-plan item that changed since the preview.
var errNotAStray = errors.New("changed since the preview")
