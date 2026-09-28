package main

// The disk object (design engine-owns-disk-lifetimes Part B, 3.8): what
// MetaSystem keeps on this computer's disk. `disk show` reads the last pass
// reports and changes nothing; `disk clean` runs the steward's pass now,
// and a person's flags remove what only a person may remove. `machine` stays
// the fleet object (list, start).

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// diskOwners are the seams disk show and disk clean call; zero values are
// production.
type diskOwners struct {
	pass func(ctx context.Context, top string, pass steward.DiskPass) (steward.DiskPassResult, error)
	home func() (string, error)
	now  func() time.Time
	// person proves the person at the enrolled terminal and returns the
	// name to record.
	person func(root string) (string, error)
	census func() *diskstore.UseCensus
	// proofs are the owner-kind proofs --release may use.
	proofs map[diskstore.OwnerKind]diskstore.OwnerProof
	// tempRoots are the temporary roots --strays may remove under.
	tempRoots func() []string
	// userCacheDir and stateDir are the cache trimmer's seams (Part A):
	// nil and "" are the user's cache dir and the state beside the machine
	// registry.
	userCacheDir func() (string, error)
	stateDir     string
}

func (o diskOwners) withDefaults() diskOwners {
	if o.pass == nil {
		o.pass = steward.SweepDiskStores
	}
	if o.home == nil {
		o.home = steward.HomeStateRoot
	}
	if o.now == nil {
		o.now = time.Now
	}
	if o.person == nil {
		o.person = func(root string) (string, error) {
			if _, err := humanauthority.Prove(root, int64(os.Getppid()), humanauthority.KernelReader{}, time.Now().UTC()); err != nil {
				return "", err
			}
			if enrollment, readErr := humanauthority.ReadEnrollment(root); readErr == nil && enrollment.Human != "" {
				return enrollment.Human, nil
			}
			return "the enrolled person", nil
		}
	}
	if o.census == nil {
		o.census = func() *diskstore.UseCensus {
			census := diskstore.TakeUseCensus(context.Background(), diskstore.KernelCensusReader(uint32(os.Getuid())))
			return &census
		}
	}
	if o.proofs == nil {
		o.proofs = map[diskstore.OwnerKind]diskstore.OwnerProof{}
	}
	if o.tempRoots == nil {
		o.tempRoots = func() []string {
			roots := []string{"/tmp"}
			if host, err := diskstore.HostTempRoot(); err == nil {
				roots = append(roots, host)
			}
			return roots
		}
	}
	return o
}

func diskIntentCommands() []intentCommand {
	return []intentCommand{
		{
			object: "disk", action: "show", audience: "both", summary: "what MetaSystem keeps on this computer's disk, changing nothing",
			usage: []string{"metasystem disk show"},
			details: []string{
				"Prints the last report of this checkout's pass and of the machine pass: free space per volume, each kind of store, what was released, what is kept or pending and the command that settles each, strays, and the evidence roots of this host.",
				"The steward writes both reports every cycle; metasystem disk clean writes them now. This command only reads.",
			},
			flags:    []intentFlag{},
			maxArgs:  0,
			examples: []string{"metasystem disk show", "metasystem disk show --json"},
			run:      runIntentDiskShow,
		},
		{
			object: "disk", action: "clean", audience: "both", summary: "reclaim disk space MetaSystem can prove is no longer needed, and trim its caches, now",
			usage: []string{
				"metasystem disk clean [--preview] [--floor GIB]",
				"metasystem disk clean --strays [--plan ID]",
				"metasystem disk clean --release ID",
				"metasystem disk clean --discard j2:ID --reason TEXT",
				"metasystem disk clean --go-cache",
			},
			details: []string{
				"Runs the steward's pass now for this checkout and this machine: a store goes only when its owner is proven ended and nothing still uses it; everything else is kept or pending with the command that settles it. Then it trims the machine's Go and staticcheck caches to their caps (disk.go-cache-cap-gib, disk.delegate-go-cache-cap-gib, disk.staticcheck-cache-cap-gib), least recently used first, never an entry used within disk.go-cache-keep-hours. A repeat with nothing to do is success.",
				"--go-cache runs only the cache trim.",
				"--preview changes nothing and writes one plan file whose id it prints. --strays, --release and --discard are a person's acts at the enrolled terminal: --strays removes the engine-named leftovers a preview listed, --release ID removes one registered store whose only obstacle is a use check the machine could not complete, --discard records that a chain's uncaptured work may be dropped.",
				"Nothing unregistered is ever removed by the pass itself; nothing a live process uses is removed by anyone.",
			},
			flags: []intentFlag{
				{name: "preview", usage: "plan only: change nothing and write the plan file"},
				{name: "floor", value: "GIB", advanced: true, usage: "run as if free space were below GIB gibibytes"},
				{name: "strays", usage: "a person's act: remove the strays a preview listed"},
				{name: "plan", value: "ID", usage: "the preview to act on (default: the newest)"},
				{name: "release", value: "ID", usage: "a person's act: release the registered store ID that only an incomplete use check keeps"},
				{name: "discard", value: "j2:ID", advanced: true, usage: "a person's act: record that chain ID's uncaptured work may be dropped"},
				{name: "reason", value: "TEXT", advanced: true, usage: "why the work may be dropped (with --discard)"},
				{name: "go-cache", usage: "only trim the Go and staticcheck caches to their caps"},
			},
			maxArgs:  0,
			examples: []string{"metasystem disk clean --preview", "metasystem disk clean", "metasystem disk clean --go-cache", "metasystem disk clean --strays --plan 01K2Z7Q3M8XW1V0P9D4J6S5R2T"},
			run:      runIntentDiskClean,
		},
	}
}

// diskRoot selects the state root the steward sweeps for this checkout.
func (inv *intentInvocation) diskRoot() (string, *intentResult) {
	if problem := inv.selectLayoutRoot(); problem != nil {
		return "", problem
	}
	return inv.stateRoot, nil
}

func runIntentDiskShow(inv *intentInvocation) int {
	top, problem := inv.diskRoot()
	if problem != nil {
		return inv.render(*problem)
	}
	owners := inv.owners.disk.withDefaults()
	home, err := owners.home()
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "disk show: the home state root cannot be found: " + err.Error(),
			Decision: "set HOME to your home directory and run metasystem disk show again"})
	}
	var lines []string
	data := map[string]any{}
	for _, source := range []struct{ name, path string }{{"checkout", diskstore.CheckoutReportPath(top)}, {"machine", diskstore.MachineReportPath(home)}} {
		report, err := diskstore.ReadReport(source.path)
		switch {
		case errors.Is(err, os.ErrNotExist):
			lines = append(lines, fmt.Sprintf("%s: no pass has run yet; metasystem disk clean --preview shows what one would do", source.name))
		case err != nil:
			lines = append(lines, fmt.Sprintf("%s: %v; metasystem disk clean writes a fresh report", source.name, err))
		default:
			lines = append(lines, report.Lines()...)
			data[source.name] = report
		}
	}
	lines = append(lines, diskEvidenceRootLines(inv.layout.InstallationRoot, home)...)
	return inv.render(intentResult{Outcome: intentConfirmed, Summary: "what MetaSystem keeps on this computer's disk", text: lines, Data: data,
		Targets: []intentTarget{{Kind: "checkout", ID: top}}})
}

// diskEvidenceRootLines names this checkout's evidence root and the other
// directories under $HOME/metasystem-evidence. Retired and unclaimed roots
// are judged by the evidence bound (U5h); until it lands they are listed,
// never judged.
func diskEvidenceRootLines(installation, home string) []string {
	var lines []string
	own := ""
	if settings, err := diskstore.LoadSettings(filepath.Join(installation, "metasystem.conf"), nil); err == nil {
		own = settings.EvidenceRoot.Path
		lines = append(lines, "evidence root of this checkout: "+own+" ("+settings.EvidenceRoot.Origin+")")
	} else {
		lines = append(lines, "evidence root of this checkout: unknown, the settings cannot be read: "+err.Error()+"; run metasystem settings check")
	}
	parent := filepath.Join(filepath.Dir(home), "metasystem-evidence")
	entries, err := os.ReadDir(parent)
	if err != nil {
		return lines
	}
	var others []string
	for _, entry := range entries {
		path := filepath.Join(parent, entry.Name())
		if entry.IsDir() && entry.Name() != ".blobs" && path != own {
			others = append(others, path)
		}
	}
	sort.Strings(others)
	for _, path := range others {
		lines = append(lines, "evidence directory: "+path+" (not judged until the evidence bound lands)")
	}
	return lines
}

func runIntentDiskClean(inv *intentInvocation) int {
	top, problem := inv.diskRoot()
	if problem != nil {
		return inv.render(*problem)
	}
	owners := inv.owners.disk.withDefaults()
	chosen := 0
	for _, name := range []string{"strays", "release", "discard", "go-cache", "preview"} {
		if inv.input.has(name) {
			chosen++
		}
	}
	if chosen > 1 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "disk clean does one thing at a time: choose one of --preview, --strays, --release ID, --discard j2:ID or --go-cache; nothing was done",
			next:    inv.publicArgv("disk", "clean", "--preview"), nextReason: "see what a pass would do first"})
	}
	switch {
	case inv.input.has("go-cache"):
		return runDiskGoCache(inv, owners)
	case inv.input.has("strays"):
		return runDiskStrays(inv, owners, top)
	case inv.input.has("release"):
		return runDiskRelease(inv, owners, top)
	case inv.input.has("discard"):
		return runDiskDiscard(inv, owners, top)
	}
	if inv.input.has("plan") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "--plan ID names the preview --strays acts on; alone it does nothing: metasystem disk clean --strays --plan ID; nothing was done"})
	}
	pass := steward.DiskPass{Mode: diskstore.ModeApply, Now: owners.now().UTC(), Clock: owners.now}
	if inv.input.switched("preview") {
		pass.Mode = diskstore.ModePreview
	}
	if inv.input.has("floor") {
		floor, err := strconv.ParseInt(inv.input.text("floor"), 10, 64)
		if err != nil || floor < 1 {
			return inv.render(intentResult{Outcome: intentRefused, code: 2,
				Summary: fmt.Sprintf("--floor takes a whole number of gibibytes, and %q is not one; nothing was done", inv.input.text("floor")),
				next:    inv.publicArgv("disk", "clean", "--preview", "--floor", "100"), nextReason: "preview a pass below a 100 GiB floor"})
		}
		pass.FloorGiB = floor
	}
	result, err := owners.pass(context.Background(), top, pass)
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "disk clean stopped: " + err.Error(),
			Decision: "metasystem disk show prints the last complete report; metasystem system check names what else is wrong"})
	}
	lines := append(result.Checkout.Lines(), result.Machine.Lines()...)
	data := map[string]any{"checkout": result.Checkout, "machine": result.Machine}
	if pass.Mode == diskstore.ModePreview {
		plans := []string{}
		for _, id := range []string{result.Checkout.Plan, result.Machine.Plan} {
			if id != "" {
				plans = append(plans, id)
			}
		}
		data["plans"] = plans
		summary := "preview: nothing was changed"
		if len(plans) > 0 {
			summary += "; plan " + strings.Join(plans, ", ")
		}
		return inv.render(intentResult{Outcome: intentConfirmed, Summary: summary, text: lines, Data: data})
	}
	released := len(result.Checkout.Actions) + len(result.Machine.Actions)
	trimmed, trimLines, trimData, trimProblem := diskTrim(inv, owners)
	lines = append(lines, trimLines...)
	data["caches"] = trimData
	if trimProblem != nil {
		trimProblem.text = append(lines, trimProblem.text...)
		trimProblem.Data = data
		return inv.render(*trimProblem)
	}
	summary := fmt.Sprintf("released %d store(s)", released)
	if released == 0 {
		summary = "nothing to release; every store is kept, pending or in use"
	}
	if trimmed > 0 {
		summary += fmt.Sprintf("; trimmed %d cache entries", trimmed)
	} else {
		summary += "; the caches are within their caps"
	}
	if released == 0 && trimmed == 0 {
		return inv.render(intentResult{Outcome: intentUnchanged, Summary: summary, text: lines, Data: data})
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Summary: summary, text: lines, Data: data})
}

// diskPerson proves the person at the enrolled terminal for the acts only a
// person makes; an agent is told who runs it and how.
func diskPerson(inv *intentInvocation, owners diskOwners, top, act string) (string, *intentResult) {
	by, err := owners.person(top)
	if err != nil {
		return "", &intentResult{Outcome: intentRefused, code: 3,
			Summary:  "disk clean " + act + " is a person's act at the enrolled terminal, and this terminal could not be proven as one: " + err.Error() + "; nothing was done",
			Decision: "a person runs metasystem disk clean " + act + " at their enrolled terminal; metasystem disk clean --preview shows what it would act on"}
	}
	return by, nil
}

func runDiskStrays(inv *intentInvocation, owners diskOwners, top string) int {
	by, problem := diskPerson(inv, owners, top, "--strays")
	if problem != nil {
		return inv.render(*problem)
	}
	home, err := owners.home()
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "disk clean --strays: the home state root cannot be found: " + err.Error(),
			Decision: "set HOME to your home directory and run metasystem disk clean --strays again"})
	}
	planDir := filepath.Join(home, "stores", "plans")
	id := inv.input.text("plan")
	if id == "" {
		id = newestPlan(planDir)
		if id == "" {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "there is no preview to act on; --strays removes only what a preview listed; nothing was done",
				next: inv.publicArgv("disk", "clean", "--preview"), nextReason: "list the strays first"})
		}
	}
	var outcomes []diskstore.PersonOutcome
	for _, planID := range strings.Split(id, ",") {
		plan, err := diskstore.ReadPlan(planDir, strings.TrimSpace(planID))
		if err != nil {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "plan " + planID + " cannot be read: " + err.Error() + "; nothing was done",
				next: inv.publicArgv("disk", "clean", "--preview"), nextReason: "write a fresh plan"})
		}
		outcomes = append(outcomes, diskstore.ExecuteStrays(context.Background(), plan, owners.now().UTC(), owners.census(), owners.tempRoots())...)
	}
	return renderPersonOutcomes(inv, "strays", by, outcomes)
}

// newestPlan is the newest plan file (plan ids sort by time) that lists a
// stray, so --strays after a preview acts on that preview.
func newestPlan(planDir string) string {
	entries, err := os.ReadDir(planDir)
	if err != nil {
		return ""
	}
	var ids []string
	for _, entry := range entries {
		if id, ok := strings.CutSuffix(entry.Name(), ".json"); ok {
			ids = append(ids, id)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	for _, id := range ids {
		if plan, err := diskstore.ReadPlan(planDir, id); err == nil && plan.Kind == "machine" {
			return id
		}
	}
	return ""
}

func renderPersonOutcomes(inv *intentInvocation, act, by string, outcomes []diskstore.PersonOutcome) int {
	var lines []string
	done, declined := 0, 0
	for _, outcome := range outcomes {
		if outcome.Done {
			done++
			lines = append(lines, fmt.Sprintf("%s: %s", outcome.Reason, outcome.Path))
			continue
		}
		declined++
		line := fmt.Sprintf("kept %s: %s", outcome.Path, outcome.Reason)
		if outcome.Command != "" {
			line += "; run " + outcome.Command
		}
		lines = append(lines, line)
	}
	data := map[string]any{"by": by, "outcomes": outcomes}
	switch {
	case len(outcomes) == 0:
		return inv.render(intentResult{Outcome: intentUnchanged, Summary: "the plan lists no " + act + "; nothing to do", Data: data})
	case declined == 0:
		return inv.render(intentResult{Outcome: intentConfirmed, Summary: fmt.Sprintf("%s: %d done", act, done), text: lines, Data: data})
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Summary: fmt.Sprintf("%s: %d done, %d kept with the reason and the command that settles each", act, done, declined),
		text: lines, Data: data})
}

func runDiskRelease(inv *intentInvocation, owners diskOwners, top string) int {
	id := inv.input.text("release")
	by, problem := diskPerson(inv, owners, top, "--release "+id)
	if problem != nil {
		return inv.render(*problem)
	}
	home, err := owners.home()
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "disk clean --release: the home state root cannot be found: " + err.Error(),
			Decision: "set HOME to your home directory and run metasystem disk clean --release " + id + " again"})
	}
	registry, record, err := diskstore.FindRecord([]diskstore.Registry{diskstore.CheckoutRegistry(top), diskstore.MachineRegistry(home)}, id)
	if errors.Is(err, diskstore.ErrNotFound) {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "no registered store has the id " + id + "; nothing was done",
			next: inv.publicArgv("disk", "show"), nextReason: "the reports name every registered store with its id"})
	}
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "store " + id + " cannot be read: " + err.Error(),
			Decision: "metasystem disk show names what the last pass could read"})
	}
	verdict, err := diskstore.ReleaseByPerson(context.Background(), registry, id, owners.proofs[record.Owner.Kind], owners.census(), by)
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "releasing " + id + " stopped: " + err.Error(), Decision: "metasystem disk show"})
	}
	outcome := diskstore.PersonOutcome{Path: record.Path, Done: verdict.Decision == diskstore.Release, Reason: verdict.Reason, Command: verdict.Command}
	if verdict.Reason == "already released" {
		return inv.render(intentResult{Outcome: intentUnchanged, Summary: "store " + id + " is already released; nothing to do", Data: map[string]any{"by": by, "outcomes": []diskstore.PersonOutcome{outcome}}})
	}
	return renderPersonOutcomes(inv, "release", by, []diskstore.PersonOutcome{outcome})
}

func runDiskDiscard(inv *intentInvocation, owners diskOwners, top string) int {
	chain, found := strings.CutPrefix(inv.input.text("discard"), "j2:")
	if !found || chain == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--discard names a dispatch chain as j2:ID; nothing was done",
			next: inv.publicArgv("disk", "show"), nextReason: "the reports name each kept workspace with its chain"})
	}
	reason := strings.TrimSpace(inv.input.text("reason"))
	if reason == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--discard records why the work may be dropped, and no --reason was given; nothing was done",
			next: inv.publicArgv("disk", "clean", "--discard", "j2:"+chain, "--reason", "TEXT"), nextReason: "say why the work may be dropped"})
	}
	by, problem := diskPerson(inv, owners, top, "--discard j2:"+chain)
	if problem != nil {
		return inv.render(*problem)
	}
	record, changed, err := diskstore.RecordDiscard(diskstore.CheckoutRegistry(top), diskstore.Owner{Kind: diskstore.OwnerDelegate, Ref: chain}, by, reason, owners.now())
	switch {
	case errors.Is(err, diskstore.ErrNotFound):
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "chain j2:" + chain + " has no registered workspace to discard; nothing was done",
			next: inv.publicArgv("disk", "show"), nextReason: "the reports name every kept workspace and its chain"})
	case err != nil:
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the discard could not be recorded: " + err.Error(), Decision: "metasystem disk show"})
	case !changed:
		return inv.render(intentResult{Outcome: intentUnchanged, Summary: "the discard of j2:" + chain + " by " + by + " is already recorded"})
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Summary: "recorded: " + by + " allows j2:" + chain + "'s uncaptured work to be dropped; the next pass releases " + record.Path + " once its owner has ended",
		next: inv.publicArgv("disk", "clean"), nextReason: "release it now"})
}

func runDiskGoCache(inv *intentInvocation, owners diskOwners) int {
	removed, lines, reports, problem := diskTrim(inv, owners)
	if problem != nil {
		return inv.render(*problem)
	}
	var freed int64
	for _, report := range reports {
		freed += report.BytesRemoved
	}
	summary := fmt.Sprintf("trimmed the machine caches: %d entries, %s freed", removed, diskBytes(freed))
	if removed == 0 {
		summary = "the machine caches are within their caps: nothing removed"
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Summary: summary, text: lines, Data: map[string]any{"caches": reports}})
}

// diskTrim runs the steward's cache trim now (disk-lifetimes Part A, A12)
// with the settings of this checkout's installation.
func diskTrim(inv *intentInvocation, owners diskOwners) (int, []string, []gocache.TrimReport, *intentResult) {
	started := owners.now()
	reports, err := steward.TrimMachineCaches(context.Background(), inv.layout.InstallationRoot, owners.userCacheDir, owners.stateDir, started, owners.now, nil)
	if err != nil {
		summary := "disk clean: " + err.Error()
		if strings.Contains(err.Error(), "disk.") {
			return 0, nil, nil, &intentResult{Outcome: intentRefused, code: 2, Summary: summary + "; nothing was trimmed",
				Decision: "fix the disk setting with metasystem settings set, then run metasystem disk clean again"}
		}
		return 0, nil, nil, &intentResult{Outcome: intentFailed, code: 1, Summary: summary, Decision: "metasystem disk show prints the last reports"}
	}
	var lines []string
	removed := 0
	for _, report := range reports {
		removed += report.EntriesRemoved
		lines = append(lines, diskTrimLine(report))
	}
	return removed, lines, reports, nil
}

// diskTrimLine is one cache's line: its size against its cap, what went,
// and how the pass ended.
func diskTrimLine(report gocache.TrimReport) string {
	line := fmt.Sprintf("%s: %s", report.Cache, report.EndedBy)
	switch report.EndedBy {
	case "absent":
		return line + " (" + report.Root + " does not exist)"
	case "lock-held", "refused":
		return line + ": " + report.Reason
	}
	if report.Phase == "measure" {
		return line + fmt.Sprintf(", measuring (%s counted so far; the next pass resumes at shard %s)", diskBytes(report.Checkpoint.BytesSoFar), report.Checkpoint.Shard)
	}
	line += fmt.Sprintf(", %s of %s cap, %d removed (%s)", diskBytes(report.BytesAfter), diskBytes(report.CapBytes), report.EntriesRemoved, diskBytes(report.BytesRemoved))
	if report.BytesAfter > report.CapBytes && report.Phase == "idle" {
		line += fmt.Sprintf("; %s used within the keep window stays", diskBytes(report.KeepWindowBytes))
	}
	if report.UnknownCount > 0 {
		line += fmt.Sprintf("; %d entries not Go's layout were left untouched", report.UnknownCount)
	}
	return line
}

func diskBytes(size int64) string {
	const gib, mib = int64(1) << 30, int64(1) << 20
	switch {
	case size >= gib:
		return fmt.Sprintf("%.1f GiB", float64(size)/float64(gib))
	case size >= mib:
		return fmt.Sprintf("%.1f MiB", float64(size)/float64(mib))
	}
	return fmt.Sprintf("%d B", size)
}
