package steward

// Engine pins as a class of the checkout pass (engine-owns-disk-lifetimes
// Part B, 3.5 "Engine pins", DL-10, DL3B-08). A pin is kept when it is the
// installed generation's, when a component record whose process is not
// proven dead names its generation, when a preparation lease holds it (a
// LOCK_SH an EnrolledBinary keeps from PrepareForExecution to Close), or
// when a live process runs it; everything else goes, one pin at a time,
// under a nonblocking .prepare.flock with the pin's own LOCK_EX held while
// it is unlinked. An incomplete use census, a held .prepare.flock or a
// record that cannot be read removes nothing that pass (fail-closed rule 1).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"golang.org/x/sys/unix"
)

// PinClass is one checkout's engine pins.
type PinClass struct {
	Top string
	// Installed reads the installed identity; nil is VerifyIdentity.
	Installed func() (InstallIdentity, error)
	// Prober judges a component record's process; nil is the kernel's.
	Prober identity.Prober
	// Grace is disk.pin-grace-hours: how long a pin stays after its
	// generation stopped being installed (Round D2 F-2).
	Grace time.Duration
}

func (PinClass) Name() string { return "engine pins" }

var pinName = regexp.MustCompile(`^generation-([0-9]+)-[0-9a-f]{64}$`)

func (c PinClass) directory() string {
	return filepath.Join(c.Top, "artifacts", "agents", "steward", "engine-pins")
}

// pinKeep is the judgement of every pin that holds whatever the census says:
// the installed generation's pin (by name: generation and digest) and the
// generations component records name.
type pinKeep struct {
	installed  string
	generation map[int]string
	// current is the installed generation and replacedAt when it was
	// minted: every older generation stopped being installed then.
	current    int
	replacedAt time.Time
}

// keeps reads what keeps pins; any read error holds the class.
func (c PinClass) keeps() (pinKeep, error) {
	keep := pinKeep{generation: map[int]string{}}
	installed := c.Installed
	if installed == nil {
		installed = func() (InstallIdentity, error) { return VerifyIdentity(RepoIdentityPath(c.Top), c.Top) }
	}
	identityRecord, err := installed()
	if err != nil {
		// Without the installed identity nothing says when a generation
		// stopped being installed (Round D2 F-2): the class holds.
		return keep, fmt.Errorf("the installed engine identity cannot be read: %w", err)
	}
	keep.installed = filepath.Base(EnrolledExecutionPath(c.Top, identityRecord))
	keep.current = identityRecord.Generation
	if keep.replacedAt, err = time.Parse(time.RFC3339, identityRecord.MintedAt); err != nil {
		return keep, fmt.Errorf("the install time %q of the installed engine cannot be read", identityRecord.MintedAt)
	}
	prober := c.Prober
	if prober == nil {
		prober = identity.KernelProber{}
	}
	components := filepath.Join(c.Top, "artifacts", "agents", "steward", "components")
	entries, err := os.ReadDir(components)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return keep, fmt.Errorf("the component records cannot be listed: %w", err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(components, entry.Name()))
		if err != nil {
			return keep, fmt.Errorf("component record %s cannot be read: %w", entry.Name(), err)
		}
		var record ComponentEvidence
		if err := json.Unmarshal(data, &record); err != nil {
			return keep, fmt.Errorf("component record %s is unreadable: %w", entry.Name(), err)
		}
		if record.Generation <= 0 {
			continue
		}
		// The recorded start is whole seconds in legacy records: it names a
		// generation to keep, never a pin to remove. Only a process proven
		// dead releases its generation.
		if record.Pid > 0 {
			if _, state, err := prober.Probe(record.Pid); err == nil && state == identity.Dead {
				continue
			}
		}
		keep.generation[record.Generation] = record.Component
	}
	return keep, nil
}

// probePin reports whether a preparation lease holds the pin, by a
// LOCK_EX|LOCK_NB probe that holds nothing afterwards.
func probePin(path string) (held bool, err error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return true, nil
		}
		return false, err
	}
	return false, unix.Flock(int(file.Fd()), unix.LOCK_UN)
}

// installedAgo is how long ago the installed generation was minted, in
// whole minutes; under a minute is said so rather than printed as zero.
func installedAgo(elapsed time.Duration) string {
	if elapsed < time.Minute {
		return "under a minute ago"
	}
	return elapsed.Round(time.Minute).String() + " ago"
}

// judge is one pin's verdict against what keeps pins and the census.
func judgePin(path string, keep pinKeep, census *diskstore.UseCensus, now time.Time, grace time.Duration) diskstore.Verdict {
	match := pinName.FindStringSubmatch(filepath.Base(path))
	generation, _ := strconv.Atoi(match[1])
	switch {
	case filepath.Base(path) == keep.installed:
		return diskstore.Verdict{Decision: diskstore.Wait}
	case generation >= keep.current:
		return diskstore.Verdict{Decision: diskstore.Keep, Reason: fmt.Sprintf("generation %d is not older than the installed generation %d", generation, keep.current),
			Command: "metasystem disk show"}
	case now.Sub(keep.replacedAt) < grace:
		// An older engine's run may still exec this pin without holding it
		// (Round D2 F-2): it stays for the grace after the re-arm.
		// Only the installed generation's minting time is known; an older
		// generation's own replacement is not recorded, and a re-arm seconds
		// ago is never printed as zero (F4).
		return diskstore.Verdict{Decision: diskstore.Keep, Reason: fmt.Sprintf("generation %d was replaced (time unknown); it stays for disk.pin-grace-hours after generation %d was installed %s", generation, keep.current, installedAgo(now.Sub(keep.replacedAt))),
			Command: "metasystem disk clean, after the grace"}
	case keep.generation[generation] != "":
		return diskstore.Verdict{Decision: diskstore.Keep, Reason: fmt.Sprintf("component %s runs generation %d", keep.generation[generation], generation),
			Command: "metasystem system stop, or the component's next re-arm"}
	case census == nil || !census.Taken || !census.Complete():
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "use census not complete; no pin is removed this pass", Command: "metasystem disk show"}
	}
	if holders := census.Holders(path); len(holders) != 0 {
		return diskstore.Verdict{Decision: diskstore.Keep, Reason: fmt.Sprintf("pid %d (%s) runs this engine", holders[0].Pid, holders[0].Command),
			Command: "metasystem system stop"}
	}
	held, err := probePin(path)
	switch {
	case err != nil:
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "the pin cannot be probed: " + err.Error(), Command: "metasystem disk show"}
	case held:
		return diskstore.Verdict{Decision: diskstore.Keep, Reason: "a preparation lease holds it (an engine is being started from it)", Command: "metasystem disk clean, once that start has returned"}
	}
	return diskstore.Verdict{Decision: diskstore.Release, Reason: "no installation, component, process or preparation uses this generation"}
}

// prepareLockHeld probes .prepare.flock without creating it. Every
// preparation creates it beside its pins, so pins without it are not the
// layout this class knows: absent is an error that holds the class (Round
// D2 F-6).
func (c PinClass) prepareLockHeld() (bool, error) {
	file, err := os.Open(filepath.Join(c.directory(), ".prepare.flock"))
	if err != nil {
		return false, err
	}
	defer file.Close()
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return true, nil
		}
		return false, err
	}
	return false, unix.Flock(int(file.Fd()), unix.LOCK_UN)
}

func (c PinClass) Plan(ctx context.Context, pass *diskstore.Pass) ([]diskstore.Item, error) {
	entries, err := os.ReadDir(c.directory())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var pins []string
	for _, entry := range entries {
		if pinName.MatchString(entry.Name()) && entry.Type().IsRegular() {
			pins = append(pins, filepath.Join(c.directory(), entry.Name()))
		}
	}
	if len(pins) == 0 {
		return nil, nil
	}
	hold := func(reason, command string) ([]diskstore.Item, error) {
		return []diskstore.Item{{Class: c.Name(), Key: "~class", Path: c.directory(),
			Verdict: diskstore.Verdict{Decision: diskstore.Pending, Reason: reason + "; no pin is removed this pass", Command: command}}}, nil
	}
	if held, err := c.prepareLockHeld(); err != nil || held {
		if err != nil {
			return hold("the preparation lock cannot be probed: "+err.Error(), "metasystem disk show")
		}
		return hold("an engine is being prepared (.prepare.flock is held)", "metasystem disk clean")
	}
	keep, err := c.keeps()
	if err != nil {
		return hold(err.Error(), "metasystem system check")
	}
	census := pass.Census(ctx)
	var items []diskstore.Item
	for _, pin := range pins {
		item := diskstore.Item{Class: c.Name(), Key: filepath.Base(pin), Path: pin, Verdict: judgePin(pin, keep, census, pass.Now, c.Grace)}
		if item.Verdict.Decision == diskstore.Release {
			item.Bytes, _, _ = diskstore.Measure(ctx, pin)
		}
		items = append(items, item)
	}
	return items, nil
}

// Apply removes one pin under a nonblocking .prepare.flock (no preparer can
// open or re-publish a pin meanwhile), judged afresh there, with the pin's
// own LOCK_EX held through the unlink.
func (c PinClass) Apply(ctx context.Context, pass *diskstore.Pass, item diskstore.Item) diskstore.Verdict {
	lock, err := os.OpenFile(filepath.Join(c.directory(), ".prepare.flock"), os.O_RDWR, 0)
	if err != nil {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "the preparation lock cannot be opened: " + err.Error(), Command: "metasystem disk show"}
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "an engine is being prepared (.prepare.flock is held)", Command: "metasystem disk clean"}
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	keep, err := c.keeps()
	if err != nil {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: err.Error(), Command: "metasystem system check"}
	}
	census := pass.Census(ctx)
	if census != nil && census.Taken && pass.CensusReader() != nil {
		if err := census.ReadNew(ctx, *pass.CensusReader()); err != nil {
			return diskstore.Verdict{Decision: diskstore.Pending, Reason: "processes started since the census could not be read: " + err.Error(), Command: "metasystem disk clean"}
		}
	}
	if verdict := judgePin(item.Path, keep, census, pass.Now, c.Grace); verdict.Decision != diskstore.Release {
		return verdict
	}
	pin, err := os.Open(item.Path)
	if err != nil {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "the pin cannot be opened: " + err.Error(), Command: "metasystem disk show"}
	}
	defer pin.Close()
	if err := unix.Flock(int(pin.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return diskstore.Verdict{Decision: diskstore.Keep, Reason: "a preparation lease took it since the plan", Command: "metasystem disk clean"}
	}
	defer unix.Flock(int(pin.Fd()), unix.LOCK_UN)
	opened, openErr := pin.Stat()
	current, statErr := os.Lstat(item.Path)
	if openErr != nil || statErr != nil || !os.SameFile(opened, current) {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "the pin was replaced since it was opened", Command: "metasystem disk clean"}
	}
	if err := os.Remove(item.Path); err != nil {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "the pin could not be removed: " + err.Error(), Command: "metasystem disk show"}
	}
	return diskstore.Verdict{Decision: diskstore.Release, Reason: "engine pin of an ended generation removed"}
}

// LegacyCandidateEngines reports every entry of the checkout's
// candidate-engines namespace outside v2 as a stray (3.5, 3.10, DL2-11): an
// older engine may hold its path without any lock a census can see, so no
// pass removes one; a person does, once no older engine runs.
type LegacyCandidateEngines struct {
	Top string
}

func (LegacyCandidateEngines) Name() string { return "legacy candidate engines" }

func (c LegacyCandidateEngines) Plan(ctx context.Context, pass *diskstore.Pass) ([]diskstore.Item, error) {
	cache := filepath.Join(c.Top, "artifacts", "agents", "candidate-engines")
	entries, err := os.ReadDir(cache)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var items []diskstore.Item
	for _, entry := range entries {
		if entry.Name() == "v2" {
			continue
		}
		path := filepath.Join(cache, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			continue
		}
		bytes, newest, _ := diskstore.Measure(ctx, path)
		if newest.IsZero() {
			newest = info.ModTime()
		}
		items = append(items, diskstore.Item{Class: c.Name(), Key: path, Path: path, Bytes: bytes, Stray: true,
			IdleSecs: int64(pass.Now.Sub(newest).Seconds()),
			Verdict: diskstore.Verdict{Decision: diskstore.Keep, Reason: "a legacy candidate engine no engine of this version produces; an older engine may still hold its path",
				Command: "a person removes it once no older engine runs: rm -rf -- '" + strings.ReplaceAll(path, "'", `'\''`) + "'"}})
	}
	return items, nil
}

func (LegacyCandidateEngines) Apply(context.Context, *diskstore.Pass, diskstore.Item) diskstore.Verdict {
	return diskstore.Verdict{Decision: diskstore.Keep, Reason: "a legacy candidate engine is removed only by a person", Command: "metasystem disk show"}
}
