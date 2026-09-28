package diskstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"golang.org/x/sys/unix"
)

// Mode is what a pass may change (3.3, 3.8).
type Mode string

const (
	// ModeApply plans, then acts on every item whose proof holds, and writes
	// the report and cursors: the steward's pass and `disk clean`.
	ModeApply Mode = "apply"
	// ModeReport plans and writes the report, acting on nothing: the pass a
	// steward runs while a person holds the helm (the report belongs to the
	// whole machine).
	ModeReport Mode = "report"
	// ModePreview plans and writes exactly one thing, the plan file:
	// `disk clean --preview`.
	ModePreview Mode = "preview"
)

// VolumeFree is one filesystem's free space as the headroom seam reports it
// (the janitor's measurement in production).
type VolumeFree struct {
	Path       string
	FreeBytes  int64
	FloorBytes int64
}

// Item is one thing a class found: a store, a stray, a foreign entry.
type Item struct {
	Class    string  `json:"class"`
	Key      string  `json:"key"`
	Path     string  `json:"path"`
	Bytes    int64   `json:"bytes,omitempty"`
	Device   uint64  `json:"device,omitempty"`
	Inode    uint64  `json:"inode,omitempty"`
	Record   string  `json:"record,omitempty"`
	Verdict  Verdict `json:"verdict"`
	Stray    bool    `json:"stray,omitempty"`
	Foreign  bool    `json:"foreign,omitempty"`
	IdleSecs int64   `json:"idleSeconds,omitempty"`
}

// Class is one store class of a pass: its plan is an observation that
// creates nothing; its apply acts on one planned item and re-checks the
// item's proof first.
type Class interface {
	Name() string
	Plan(ctx context.Context, pass *Pass) ([]Item, error)
	Apply(ctx context.Context, pass *Pass, item Item) Verdict
}

// PassOptions configure one pass.
type PassOptions struct {
	// Kind is "checkout" or "machine"; Name the checkout path or "machine".
	Kind, Name string
	// Registry holds the pass's cursors; LockPath is its .sweep.flock.
	Registry Registry
	LockPath string
	// ReportPath is where the report goes; PlanDir where a preview's plan
	// goes (~/.metasystem/stores/plans).
	ReportPath, PlanDir string
	Mode                Mode
	// Now is the pass's instant; Clock reads the time left in the budget
	// (a test passes a fake). Entropy mints the plan id.
	Now     time.Time
	Clock   func() time.Time
	Entropy io.Reader
	Classes []Class
	// Volumes are the store roots whose free space the floor watches.
	Volumes    []string
	FloorBytes int64
	// FloorForced runs the pass as if every volume were below the floor
	// (`disk clean --floor`).
	FloorForced     bool
	FloorMinAge     time.Duration
	CensusMinBudget time.Duration
	CensusReader    *CensusReader
	Headroom        func(paths []string, floorBytes int64) ([]VolumeFree, error)
	// Trim is Part A's Go cache trimmer at half caps; nil until Part A's
	// trimmer lands, which the report says.
	Trim func(ctx context.Context) (string, error)
	// Consumers inventories the largest unregistered consumers at the floor
	// (the 2026-09-29 amendment).
	Consumers func(ctx context.Context, census *UseCensus) []Consumer
	// Notes are settings conflicts and other lines the caller resolved.
	Notes []string
	// HostUnknown are the "host settings unknown" lines: the host policy
	// could not be read and nothing that depends on it acts.
	HostUnknown []string
}

// Pass is the state classes share during one pass.
type Pass struct {
	Now   time.Time
	Mode  Mode
	Floor bool
	// FloorMinAge replaces an ageing threshold while the floor is active;
	// it is never zero and never lowers a keep window that R6 protects.
	FloorMinAge time.Duration
	options     PassOptions
	census      *UseCensus
	censusTried bool
}

// Remaining is the budget left, by the pass's clock; no deadline is
// unbounded.
func (p *Pass) Remaining(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok || p.options.Clock == nil {
		return time.Duration(1<<63 - 1)
	}
	return deadline.Sub(p.options.Clock())
}

// Census takes the use census once per pass. It is not taken without a
// reader, or when less than disk.census-min-budget-sec of the budget
// remains; the returned census then says why.
func (p *Pass) Census(ctx context.Context) *UseCensus {
	if p.censusTried {
		return p.census
	}
	p.censusTried = true
	switch {
	case p.options.CensusReader == nil:
		p.census = &UseCensus{NotTaken: "no use census reader on this platform"}
	case p.Remaining(ctx) < p.options.CensusMinBudget:
		p.census = &UseCensus{NotTaken: fmt.Sprintf("use census not taken: less than %s of the pass budget left", p.options.CensusMinBudget)}
	default:
		taken := TakeUseCensus(ctx, *p.options.CensusReader)
		p.census = &taken
	}
	return p.census
}

// CensusReader is the pass's reader, for a fresh read inside a critical
// section.
func (p *Pass) CensusReader() *CensusReader { return p.options.CensusReader }

type cursors struct {
	Next    string            `json:"next,omitempty"`
	Classes map[string]string `json:"classes,omitempty"`
}

func (o PassOptions) cursorPath() string {
	return filepath.Join(o.Registry.Dir, ".cursors-"+o.Kind+".json")
}

func readCursors(path string) cursors {
	data, err := os.ReadFile(path)
	var c cursors
	if err == nil {
		_ = json.Unmarshal(data, &c)
	}
	if c.Classes == nil {
		c.Classes = map[string]string{}
	}
	return c
}

// RunPass runs one sweep pass (3.3): it takes the pass's flock without
// waiting (a held flock is "a pass is running", success), reads headroom,
// enters floor mode below the floor, visits the classes round-robin from
// their cursors, acts in ModeApply only, and writes the report (or, in
// ModePreview, the plan and nothing else).
func RunPass(ctx context.Context, options PassOptions) (Report, error) {
	report := Report{Schema: ReportSchema, Kind: options.Kind, Name: options.Name, At: options.Now.UTC(), Mode: options.Mode,
		Notes: append([]string(nil), options.Notes...), HostUnknown: append([]string(nil), options.HostUnknown...)}
	release, running, err := takeSweepLock(options)
	if err != nil {
		return report, err
	}
	if running {
		report.Running = true
		report.Health = Health{Status: HealthOK, Reason: "a pass is running; it writes its own report"}
		return report, nil
	}
	defer release()
	pass := &Pass{Now: options.Now, Mode: options.Mode, FloorMinAge: options.FloorMinAge, options: options}
	pass.observeVolumes(ctx, &report)
	if pass.Floor {
		pass.floor(ctx, &report)
	}

	state := readCursors(options.cursorPath())
	order := rotate(options.Classes, state.Next)
	next := ""
	for index, class := range order {
		name := class.Name()
		if ctx.Err() != nil {
			report.Backlog = append(report.Backlog, name+": not visited, the pass budget ran out")
			if next == "" {
				next = name
			}
			continue
		}
		finished := pass.visit(ctx, class, &report, state.Classes)
		if !finished && next == "" {
			next = order[(index+1)%len(order)].Name()
		}
	}
	state.Next = next
	report.Cursors = state.Classes
	if census := pass.census; census != nil {
		report.Census = census
	}
	report.Health = healthOf(report)
	switch options.Mode {
	case ModePreview:
		id, err := writePlan(options, report)
		report.Plan = id
		return report, err
	default:
		if err := writeJSON(options.cursorPath(), state); err != nil {
			return report, err
		}
		return report, writeJSON(options.ReportPath, report)
	}
}

// visit plans one class and, in ModeApply, acts on its releasable items,
// resuming at the class's cursor. It reports whether the class finished.
func (p *Pass) visit(ctx context.Context, class Class, report *Report, positions map[string]string) bool {
	name := class.Name()
	items, err := class.Plan(ctx, p)
	summary := ClassReport{Name: name}
	if err != nil {
		report.Pending = append(report.Pending, Line{Class: name, Reason: "the class could not be observed: " + err.Error(), Command: "metasystem disk show"})
		report.Classes = append(report.Classes, summary)
		return true
	}
	sort.SliceStable(items, func(i, j int) bool {
		releaseI, releaseJ := items[i].Verdict.Decision == Release, items[j].Verdict.Decision == Release
		if releaseI != releaseJ {
			return releaseI
		}
		if releaseI && items[i].Bytes != items[j].Bytes {
			return items[i].Bytes > items[j].Bytes
		}
		return items[i].Key < items[j].Key
	})
	// A class cut short last pass resumes at the item it stopped on; the
	// items before it come after, so every item gets its turn.
	var releasable []Item
	for _, item := range items {
		summary.Items++
		summary.Bytes += item.Bytes
		switch {
		case item.Stray:
			report.Strays = append(report.Strays, item)
		case item.Foreign:
			report.Foreign = append(report.Foreign, item)
		case item.Verdict.Decision != Release:
			report.add(item, item.Verdict)
		case p.Mode != ModeApply:
			report.Planned = append(report.Planned, item)
		default:
			releasable = append(releasable, item)
		}
	}
	if resume := positions[name]; resume != "" {
		for index, item := range releasable {
			if item.Key == resume {
				releasable = append(releasable[index:], releasable[:index]...)
				break
			}
		}
	}
	finished := true
	for index, item := range releasable {
		if ctx.Err() != nil {
			positions[name] = item.Key
			summary.Backlog = len(releasable) - index
			finished = false
			break
		}
		outcome := class.Apply(ctx, p, item)
		if outcome.Decision == Release {
			report.Actions = append(report.Actions, Line{Class: name, Path: item.Path, Reason: outcome.Reason})
			summary.Released++
			continue
		}
		report.add(item, outcome)
	}
	if finished {
		delete(positions, name)
	} else {
		report.Backlog = append(report.Backlog, fmt.Sprintf("%s: %d item(s) left for the next pass", name, summary.Backlog))
	}
	report.Classes = append(report.Classes, summary)
	return finished
}

func rotate(classes []Class, start string) []Class {
	for index, class := range classes {
		if class.Name() == start {
			return append(append([]Class(nil), classes[index:]...), classes[:index]...)
		}
	}
	return classes
}

func (p *Pass) observeVolumes(ctx context.Context, report *Report) {
	if p.options.Headroom == nil || len(p.options.Volumes) == 0 {
		if p.options.FloorForced {
			p.Floor = true
		}
		return
	}
	volumes, err := p.options.Headroom(p.options.Volumes, p.options.FloorBytes)
	if err != nil {
		report.Pending = append(report.Pending, Line{Class: "volumes", Reason: "free space could not be measured: " + err.Error(), Command: "metasystem disk show"})
	}
	for _, volume := range volumes {
		below := volume.FreeBytes < volume.FloorBytes
		report.Volumes = append(report.Volumes, Volume{Path: volume.Path, FreeBytes: volume.FreeBytes, FloorBytes: volume.FloorBytes, BelowFloor: below})
		if below {
			p.Floor = true
		}
	}
	if p.options.FloorForced {
		p.Floor = true
	}
}

// floor is floor mode (3.3 trigger 4): ageing thresholds lowered to
// disk.floor-min-age-hours (never zero), Part A's trimmer at half caps with
// its keep window unchanged, and the largest unregistered consumers named
// with the command that reclaims each. Nothing here deletes.
func (p *Pass) floor(ctx context.Context, report *Report) {
	floor := &FloorReport{Active: true, MinAge: p.FloorMinAge.String(), Remedy: "metasystem disk clean --preview"}
	if p.FloorMinAge <= 0 {
		p.FloorMinAge = time.Hour
		floor.MinAge = p.FloorMinAge.String()
	}
	switch {
	case p.options.Trim == nil:
		floor.Trim = "the caches are trimmed to their caps by the steward's cache step (Part A) each cycle; the floor's half-cap trim is not built yet"
	case p.Mode != ModeApply:
		floor.Trim = "the Go cache trimmer would run at half caps"
	default:
		line, err := p.options.Trim(ctx)
		if err != nil {
			line = "the Go cache trimmer failed: " + err.Error()
		}
		floor.Trim = line
	}
	if p.options.Consumers != nil {
		floor.Consumers = p.options.Consumers(ctx, p.Census(ctx))
	}
	report.Floor = floor
}

// takeSweepLock takes the pass's flock LOCK_EX|LOCK_NB. A preview never
// creates a checkout's lock file: it probes an existing one and holds
// nothing.
func takeSweepLock(options PassOptions) (release func(), running bool, err error) {
	if options.LockPath == "" {
		return func() {}, false, nil
	}
	flags := os.O_RDWR | os.O_CREATE
	if options.Mode == ModePreview && options.Kind != "machine" {
		flags = os.O_RDONLY
	} else if err := os.MkdirAll(filepath.Dir(options.LockPath), 0o700); err != nil {
		return nil, false, err
	}
	file, err := os.OpenFile(options.LockPath, flags, 0o600)
	if errors.Is(err, os.ErrNotExist) {
		return func() {}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, true, nil
		}
		return nil, false, err
	}
	return func() { _ = file.Close() }, false, nil
}

func writeJSON(path string, value any) error {
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	_, err = atomicfile.WriteFile(path, append(data, '\n'), 0o600, "")
	return err
}

// Plan is a preview's one write: every item with path, device and inode,
// reason and action (3.8), which `--strays`, `--release` and `--leases`
// execute after revalidating each item.
type Plan struct {
	Schema string    `json:"schema"`
	ID     string    `json:"id"`
	At     time.Time `json:"at"`
	Kind   string    `json:"kind"`
	Name   string    `json:"name"`
	Items  []Item    `json:"items"`
}

func writePlan(options PassOptions, report Report) (string, error) {
	if options.PlanDir == "" {
		return "", nil
	}
	id, err := NewID(options.Now, options.Entropy)
	if err != nil {
		return "", err
	}
	plan := Plan{Schema: Schema, ID: id, At: options.Now.UTC(), Kind: options.Kind, Name: options.Name}
	plan.Items = append(plan.Items, report.Planned...)
	plan.Items = append(plan.Items, report.Strays...)
	for _, line := range append(append([]Line(nil), report.Kept...), report.Pending...) {
		if line.Item != nil {
			plan.Items = append(plan.Items, *line.Item)
		}
	}
	return id, writeJSON(filepath.Join(options.PlanDir, id+".json"), plan)
}

// ReadPlan reads a preview's plan by id.
func ReadPlan(planDir, id string) (Plan, error) {
	if !validID(id) {
		return Plan{}, fmt.Errorf("%q is not a plan id; metasystem disk clean --preview prints one", id)
	}
	data, err := os.ReadFile(filepath.Join(planDir, id+".json"))
	if err != nil {
		return Plan{}, err
	}
	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return Plan{}, fmt.Errorf("plan %s is unreadable: %w", id, err)
	}
	return plan, nil
}
