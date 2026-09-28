package steward

// The disk sweeper's place in the steward (design engine-owns-disk-lifetimes
// Part B, 3.3): after the tick has returned and released arbitration, each
// steward runs its checkout's pass, and whichever steward wins the machine
// flock runs the machine pass. The owners whose records need arbitration
// (context handoffs) take it alone and nonblocking, one nonce at a time; the
// usage store retires a bounded batch per maintenance-lock acquisition,
// outside arbitration. While a person holds the helm the passes report and
// act on nothing (the report belongs to the whole machine).

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/janitor"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

// RoleDisk is the disk-lifetime health role (3.3).
const RoleDisk HealthRole = "disk"

// HandoffClass is the context-handoff owner in a checkout pass: complete,
// unprotected handoffs older than disk.context-keep-days, each removed under
// its own nonblocking arbitration acquisition.
type HandoffClass struct {
	Root string
	Keep time.Duration
}

func (HandoffClass) Name() string { return "context handoffs" }

func (c HandoffClass) cutoff(now time.Time) time.Time { return now.Add(-c.Keep) }

func (c HandoffClass) Plan(_ context.Context, pass *diskstore.Pass) ([]diskstore.Item, error) {
	nonces, problems := InspectHandoffs(c.Root, c.cutoff(pass.Now), pass.Now)
	var items []diskstore.Item
	for _, nonce := range nonces {
		items = append(items, diskstore.Item{Class: c.Name(), Key: nonce, Path: HandoffDir(c.Root, nonce),
			Verdict: diskstore.Verdict{Decision: diskstore.Release, Reason: "complete, not live or consumed, older than disk.context-keep-days"}})
	}
	for index, problem := range problems {
		items = append(items, diskstore.Item{Class: c.Name(), Key: fmt.Sprintf("~problem-%03d", index),
			Verdict: diskstore.Verdict{Decision: diskstore.Keep, Reason: problem.Error(), Command: "metasystem system check"}})
	}
	return items, nil
}

func (c HandoffClass) Apply(ctx context.Context, pass *diskstore.Pass, item diskstore.Item) diskstore.Verdict {
	removed, err := PruneHandoffs(ctx, c.Root, []string{item.Key}, c.cutoff(pass.Now), pass.Now, TryAcquireArbitration)
	switch {
	case len(removed) == 1:
		return diskstore.Verdict{Decision: diskstore.Release, Reason: "context handoff removed"}
	case errors.Is(err, ErrArbitrationHeld):
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "steward arbitration is held; the next pass retries", Command: "metasystem disk clean"}
	case err != nil:
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: err.Error(), Command: "metasystem system check"}
	}
	return diskstore.Verdict{Decision: diskstore.Keep, Reason: "the handoff became live, consumed or newer since the plan", Command: "metasystem disk show"}
}

// UsageClass is the usage store's owner in a checkout pass: complete call
// pairs past the store's own retention window, retired at most Limit per
// maintenance-lock acquisition, outside arbitration.
type UsageClass struct {
	StateRoot string
	Limit     int
}

func (UsageClass) Name() string { return "usage call sessions" }

func usageCutoff(now time.Time) time.Time {
	return now.UTC().Add(-usage.CallRetentionWindow - time.Nanosecond)
}

func (c UsageClass) Plan(_ context.Context, pass *diskstore.Pass) ([]diskstore.Item, error) {
	inspection, err := usage.InspectCallSessions(c.StateRoot, usageCutoff(pass.Now), pass.Now)
	if err != nil {
		return nil, err
	}
	var items []diskstore.Item
	if inspection.MaintenanceHeld {
		return []diskstore.Item{{Class: c.Name(), Key: "maintenance", Verdict: diskstore.Verdict{Decision: diskstore.Pending,
			Reason: "the usage store's maintenance lock is held; the next pass retries", Command: "metasystem disk clean"}}}, nil
	}
	pending := len(inspection.Candidates) + len(inspection.Orphans) + len(inspection.Interrupted)
	for batch := 0; pending > 0; batch++ {
		items = append(items, diskstore.Item{Class: c.Name(), Key: fmt.Sprintf("batch-%04d", batch),
			Path:    filepath.Join(c.StateRoot, "artifacts", "agents", "context"),
			Verdict: diskstore.Verdict{Decision: diskstore.Release, Reason: fmt.Sprintf("up to %d call pair(s) past the retention window", c.Limit)}})
		pending -= c.Limit
	}
	return items, nil
}

func (c UsageClass) Apply(ctx context.Context, pass *diskstore.Pass, _ diskstore.Item) diskstore.Verdict {
	removed, err := usage.RetireCallSessions(ctx, c.StateRoot, usageCutoff(pass.Now), pass.Now, c.Limit)
	var busy *usage.CallStoreBusyError
	switch {
	case errors.As(err, &busy):
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "the usage store's maintenance lock is held; the next pass retries", Command: "metasystem disk clean"}
	case err != nil:
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "usage retirement stopped: " + err.Error(), Command: "metasystem system check"}
	}
	return diskstore.Verdict{Decision: diskstore.Release, Reason: fmt.Sprintf("%d call pair(s) retired", removed)}
}

// DiskPass selects what one SweepDiskStores call does.
type DiskPass struct {
	Mode diskstore.Mode
	Now  time.Time
	// Clock reads the budget left; production passes time.Now.
	Clock func() time.Time
	// FloorGiB, when positive, runs the machine pass as if free space were
	// below that floor (`disk clean --floor`).
	FloorGiB int64
	// SkipMachine leaves the machine pass to whichever steward wins it.
	SkipMachine bool
	// TempRoots and Volumes replace the host's temporary roots and the
	// watched volumes, and Home the home state root (fixtures); empty is
	// production.
	TempRoots, Volumes []string
	Home               string
	// Proofs replaces the engine's owner-kind proofs, and Checkouts the
	// armed checkouts read from the host registry (fixtures).
	Proofs    map[diskstore.OwnerKind]diskstore.OwnerProof
	Checkouts []string
}

func (p DiskPass) proofs() map[diskstore.OwnerKind]diskstore.OwnerProof {
	if p.Proofs != nil {
		return p.Proofs
	}
	return diskOwnerProofs()
}

// DiskPassResult is both reports.
type DiskPassResult struct {
	Checkout diskstore.Report
	Machine  diskstore.Report
	Machined bool
}

// HomeStateRoot is ~/.metasystem, or the run-scoped home a fixture selects
// for the armed-checkout registry.
func HomeStateRoot() (string, error) {
	path, err := registry.DefaultPath()
	return filepath.Dir(path), err
}

// SweepDiskStores runs the checkout pass for top and then tries the machine
// pass. Every lock it takes is nonblocking; its budget is
// disk.sweep-budget-sec.
func SweepDiskStores(ctx context.Context, top string, pass DiskPass) (DiskPassResult, error) {
	var result DiskPassResult
	if pass.Clock == nil {
		return result, errors.New("a disk pass needs a clock for its budget")
	}
	home := pass.Home
	if home == "" {
		var err error
		if home, err = HomeStateRoot(); err != nil {
			return result, err
		}
	}
	settings, settingsErr := diskSettingsFor(top)
	checkoutOptions := diskstore.PassOptions{Kind: "checkout", Name: top, Registry: diskstore.CheckoutRegistry(top),
		LockPath: filepath.Join(diskstore.CheckoutRegistry(top).Dir, ".sweep.flock"), ReportPath: diskstore.CheckoutReportPath(top),
		PlanDir: filepath.Join(home, "stores", "plans"), Mode: pass.Mode, Now: pass.Now, Clock: pass.Clock, Entropy: rand.Reader}
	if settingsErr != nil {
		// Unknown settings: the checkout pass acts on nothing and says why.
		checkoutOptions.HostUnknown = []string{"checkout settings unknown: " + settingsErr.Error()}
		checkoutOptions.Mode = reportOnly(pass.Mode)
	} else {
		budget, cancel := context.WithTimeout(ctx, settings.Duration(config.DiskSweepBudgetKey))
		defer cancel()
		ctx = budget
		checkoutOptions.Classes = []diskstore.Class{
			diskstore.RegisteredStores{Registry: diskstore.CheckoutRegistry(top), Proofs: pass.proofs()},
			HandoffClass{Root: top, Keep: settings.Duration(config.DiskContextKeepKey)},
			UsageClass{StateRoot: top, Limit: settings.Count(config.DiskSweepItemsPerLockKey)},
		}
		checkoutOptions.CensusMinBudget = settings.Duration(config.DiskCensusMinBudgetKey)
		checkoutOptions.CensusReader = kernelCensus()
	}
	var err error
	result.Checkout, err = diskstore.RunPass(ctx, checkoutOptions)
	if err != nil || pass.SkipMachine {
		return result, err
	}
	result.Machine, err = machinePass(ctx, home, top, settings, settingsErr, pass)
	result.Machined = err == nil && !result.Machine.Running
	return result, err
}

func reportOnly(mode diskstore.Mode) diskstore.Mode {
	if mode == diskstore.ModeApply {
		return diskstore.ModeReport
	}
	return mode
}

func machinePass(ctx context.Context, home, top string, own diskstore.Settings, ownErr error, pass DiskPass) (diskstore.Report, error) {
	checkouts := pass.Checkouts
	if checkouts == nil {
		checkouts = armedCheckouts()
	}
	checkouts = append([]string(nil), checkouts...)
	if !containsPath(checkouts, top) {
		checkouts = append(checkouts, top)
	}
	var participants []diskstore.Participant
	var evidenceRoots, gitRoots []string
	for _, checkout := range checkouts {
		layout, layoutErr := stateroot.ResolveLayout(checkout)
		if layoutErr == nil && !containsPath(gitRoots, layout.GitRoot) {
			gitRoots = append(gitRoots, layout.GitRoot)
		}
		settings, err := diskSettingsFor(checkout)
		if checkout == top {
			settings, err = own, ownErr
		}
		participants = append(participants, diskstore.Participant{Checkout: checkout, Settings: settings, Err: err})
		if err == nil && !containsPath(evidenceRoots, settings.EvidenceRoot.Path) {
			evidenceRoots = append(evidenceRoots, settings.EvidenceRoot.Path)
		}
	}
	host := diskstore.ResolveHost(participants)
	options := diskstore.PassOptions{Kind: "machine", Name: "machine", Registry: diskstore.MachineRegistry(home),
		LockPath: filepath.Join(diskstore.MachineRegistry(home).Dir, ".sweep.flock"), ReportPath: diskstore.MachineReportPath(home),
		PlanDir: filepath.Join(home, "stores", "plans"), Mode: pass.Mode, Now: pass.Now, Clock: pass.Clock, Entropy: rand.Reader,
		Notes: host.Conflicts, HostUnknown: host.Unknown}
	if !host.Known() {
		options.Mode = reportOnly(pass.Mode)
		return diskstore.RunPass(ctx, options)
	}
	tempRoots := pass.TempRoots
	if tempRoots == nil {
		hostTemp, _ := diskstore.HostTempRoot()
		tempRoots = nonEmpty(hostTemp, "/tmp")
	}
	options.Classes = []diskstore.Class{
		diskstore.RegisteredStores{Registry: diskstore.MachineRegistry(home), Proofs: pass.proofs()},
		diskstore.TempStrays{Roots: tempRoots},
	}
	options.Volumes = pass.Volumes
	if options.Volumes == nil {
		options.Volumes = existing(append(append([]string{home, top}, tempRoots...), evidenceRoots...))
	}
	options.FloorBytes = host.Bytes(config.DiskFloorKey)
	if pass.FloorGiB > 0 {
		options.FloorBytes, options.FloorForced = pass.FloorGiB<<30, true
	}
	options.FloorMinAge = own.Duration(config.DiskFloorMinAgeKey)
	options.CensusMinBudget = host.Duration(config.DiskCensusMinBudgetKey)
	options.CensusReader = kernelCensus()
	options.Headroom = func(paths []string, floor int64) ([]diskstore.VolumeFree, error) {
		measured, err := janitor.Headroom(paths, floor)
		var volumes []diskstore.VolumeFree
		for _, volume := range measured {
			volumes = append(volumes, diskstore.VolumeFree{Path: volume.Path, FreeBytes: volume.FreeBytes, FloorBytes: volume.FloorBytes})
		}
		return volumes, err
	}
	options.Consumers = func(ctx context.Context, census *diskstore.UseCensus) []diskstore.Consumer {
		roots := consumerRoots(home, tempRoots, gitRoots, evidenceRoots)
		return diskstore.InventoryConsumers(ctx, pass.Now, roots, registeredPaths(home, checkouts, gitRoots), census)
	}
	return diskstore.RunPass(ctx, options)
}

// consumerRoots are the places the floor names unregistered consumers in:
// every evidence root, the host temporary roots (the engine's namespace),
// each armed checkout's siblings (never an armed checkout itself), the Go
// and staticcheck caches, and ~/.metasystem.
func consumerRoots(home string, tempRoots, gitRoots, evidenceRoots []string) []diskstore.ConsumerRoot {
	var roots []diskstore.ConsumerRoot
	for _, root := range evidenceRoots {
		roots = append(roots, diskstore.ConsumerRoot{Path: root, Kind: "evidence root", Children: true})
	}
	for _, root := range tempRoots {
		roots = append(roots, diskstore.ConsumerRoot{Path: root, Kind: "tmpdir", Children: true, Engine: true})
	}
	parents := map[string]bool{}
	for _, gitRoot := range gitRoots {
		parents[filepath.Dir(gitRoot)] = true
	}
	for parent := range parents {
		roots = append(roots, diskstore.ConsumerRoot{Path: parent, Kind: "checkout sibling", Children: true})
	}
	for _, domain := range []gocache.Domain{gocache.DomainEngine, gocache.DomainDelegate} {
		if paths, err := gocache.DomainPaths(domain); err == nil {
			for _, cache := range []string{paths.GoCache, paths.StaticcheckCache} {
				if cache != "" {
					roots = append(roots, diskstore.ConsumerRoot{Path: cache, Kind: "Go cache"})
				}
			}
		}
	}
	roots = append(roots, diskstore.ConsumerRoot{Path: home, Kind: "home state", Children: true})
	sort.SliceStable(roots, func(i, j int) bool { return roots[i].Path < roots[j].Path })
	return roots
}

// registeredPaths are the paths the floor never names as unregistered: the
// registered stores, the armed checkouts and their git roots.
func registeredPaths(home string, checkouts, gitRoots []string) map[string]bool {
	paths := map[string]bool{}
	for _, gitRoot := range gitRoots {
		paths[gitRoot] = true
	}
	for _, checkout := range checkouts {
		paths[checkout] = true
		records, _ := diskstore.CheckoutRegistry(checkout).Inventory()
		for _, record := range records {
			paths[record.Path] = true
		}
	}
	records, _ := diskstore.MachineRegistry(home).Inventory()
	for _, record := range records {
		paths[record.Path] = true
	}
	return paths
}

// diskOwnerProofs are the owner-kind proofs this engine has. Slice 1 ships
// the engine and the interface; each owner kind's proof lands with its unit
// (process scratch U1a, attempts U5b, launches, units and leases U5d, goal
// and session worktrees U5e, delegate workspaces U5f, workspaces U6b), and a
// record whose kind has no proof yet is kept and reported pending.
func diskOwnerProofs() map[diskstore.OwnerKind]diskstore.OwnerProof {
	return map[diskstore.OwnerKind]diskstore.OwnerProof{}
}

func kernelCensus() *diskstore.CensusReader {
	reader := diskstore.KernelCensusReader(uint32(os.Getuid()))
	return &reader
}

// diskSettingsFor reads the settings of the checkout whose state root (or
// repository) is checkout: its metasystem.conf beside it in the template
// layout, else the installation its layout names.
func diskSettingsFor(checkout string) (diskstore.Settings, error) {
	if conf := filepath.Join(checkout, "metasystem.conf"); regularFileExists(conf) {
		return diskstore.LoadSettings(conf, nil)
	}
	layout, err := stateroot.ResolveLayout(checkout)
	if err != nil {
		return diskstore.Settings{}, err
	}
	return diskstore.LoadSettings(filepath.Join(layout.InstallationRoot, "metasystem.conf"), nil)
}

// armedCheckouts reads the host registry's open claims and owners.
func armedCheckouts() []string {
	path, err := registry.DefaultPath()
	if err != nil {
		return nil
	}
	frames, err := registry.ReadFrames(path)
	if err != nil {
		return nil
	}
	reduction, err := registry.Reduce(frames)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var checkouts []string
	add := func(path string) {
		if path != "" && !seen[path] {
			seen[path] = true
			checkouts = append(checkouts, path)
		}
	}
	for _, tag := range reduction.SortedTags() {
		if claim := reduction.Claims[tag]; claim != nil && claim.Open() {
			add(claim.CheckoutPath)
		}
	}
	for _, owner := range reduction.PublishedOwners {
		if owner.Open() {
			add(owner.CheckoutPath)
		}
	}
	sort.Strings(checkouts)
	return checkouts
}

func containsPath(paths []string, want string) bool {
	for _, path := range paths {
		if path == want {
			return true
		}
	}
	return false
}

func nonEmpty(paths ...string) []string {
	var kept []string
	for _, path := range paths {
		if path != "" {
			kept = append(kept, path)
		}
	}
	return kept
}

func existing(paths []string) []string {
	var kept []string
	seen := map[string]bool{}
	for _, path := range paths {
		if path == "" || seen[path] {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			seen[path] = true
			kept = append(kept, path)
		}
	}
	return kept
}

// checkDisk folds the last checkout and machine reports into the disk role:
// free space below the floor or an unknown host policy raises it with the
// report's remedy; an unreadable report is unknown; no report yet is alive.
func checkDisk(repoRoot string) RoleVerdict {
	home, _ := HomeStateRoot()
	return checkDiskAt(repoRoot, home)
}

func checkDiskAt(repoRoot, home string) RoleVerdict {
	paths := []string{diskstore.CheckoutReportPath(repoRoot)}
	if home != "" {
		paths = append(paths, diskstore.MachineReportPath(home))
	}
	read := 0
	for _, path := range paths {
		report, err := diskstore.ReadReport(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return roleUnknown(RoleDisk, err.Error(), "metasystem disk show")
		}
		read++
		if report.Health.Status == diskstore.HealthAttention {
			return roleDead(RoleDisk, report.Health.Reason, report.Health.Remedy)
		}
	}
	if read == 0 {
		return roleAlive(RoleDisk, "no disk pass has run yet")
	}
	return roleAlive(RoleDisk, "free space is above the floor and every disk setting is readable")
}

// diskFreeBytes is the least free space the last disk reports saw, for the
// machine's presence; nil when no pass has measured a volume.
func diskFreeBytes(repoRoot string) *int64 {
	paths := []string{diskstore.CheckoutReportPath(repoRoot)}
	if home, err := HomeStateRoot(); err == nil {
		paths = append(paths, diskstore.MachineReportPath(home))
	}
	var least *int64
	for _, path := range paths {
		report, err := diskstore.ReadReport(path)
		if err != nil {
			continue
		}
		for _, volume := range report.Volumes {
			if least == nil || volume.FreeBytes < *least {
				free := volume.FreeBytes
				least = &free
			}
		}
	}
	return least
}

func regularFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
