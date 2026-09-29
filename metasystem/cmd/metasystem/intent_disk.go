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
	// trimPass replaces one trim pass (fixtures); nil is the steward's.
	trimPass steward.CacheTrimPass
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
			home, _ := steward.HomeStateRoot()
			census := diskstore.TakeUseCensus(context.Background(), *steward.KernelCensusReader(home, steward.ArmedCheckouts()))
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
				"Prints the last report of this checkout's pass and of the machine pass: free space per volume, each kind of store, what was released, what is kept or pending and the command that settles each, strays, the machine caches as the last trim pass left them, and the evidence roots of this host. A finding repeated for many paths is one line with its count and three of them.",
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
				"Runs the steward's pass now for this checkout and this machine: a store goes only when its owner is proven ended and nothing still uses it; everything else is kept or pending with the command that settles it. Then it trims the machine's Go and staticcheck caches to their caps (disk.go-cache-cap-gib, disk.delegate-go-cache-cap-gib, disk.staticcheck-cache-cap-gib), least recently used first, never an entry used within disk.go-cache-keep-hours. Run at a terminal it finishes the job: pass after pass of the steward's disk.cache-trim-budget-sec, telling each on stderr, until every cache is measured and trimmed or disk.cache-trim-person-budget-sec is spent. A repeat with nothing to do is success.",
				"It also forgets the host registry's registrations of checkouts whose directories no longer exist (one reaped record each, reason checkout-gone); a registration whose recorded process still runs is kept. The steward's own pass only counts them.",
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
	cacheLines, caches := diskCacheLines(owners)
	lines = append(lines, cacheLines...)
	if caches != nil {
		data["caches"] = caches
	}
	lines = append(lines, diskEvidenceRootLines(inv.layout.InstallationRoot, home)...)
	return inv.render(intentResult{Outcome: intentConfirmed, Summary: "what MetaSystem keeps on this computer's disk", text: lines, Data: data,
		Targets: []intentTarget{{Kind: "checkout", ID: top}}})
}

// diskCacheLines are the machine caches as the last trim pass left them,
// read from the trimmer's state and changing nothing.
func diskCacheLines(owners diskOwners) ([]string, []gocache.TrimReport) {
	stateDir := owners.stateDir
	if stateDir == "" {
		var err error
		if stateDir, err = steward.CacheTrimStateDir(); err != nil {
			return []string{"caches: the trimmer's state cannot be found: " + err.Error()}, nil
		}
	}
	reports, err := gocache.LastTrimReports(stateDir)
	switch {
	case err != nil:
		return []string{"caches: " + err.Error() + "; metasystem disk clean --go-cache writes fresh reports"}, nil
	case len(reports) == 0:
		return []string{"caches: no trim pass has run yet; metasystem disk clean --go-cache trims them now"}, nil
	}
	summary, _ := diskTrimSummary(reports)
	lines := []string{"caches, as the last trim pass left them: " + summary}
	for _, report := range reports {
		lines = append(lines, "  "+diskTrimLine(report))
	}
	return lines, reports
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
	pass := steward.DiskPass{Mode: diskstore.ModeApply, Now: owners.now().UTC(), Clock: owners.now, ForgetRemoved: true}
	if inv.input.switched("preview") {
		pass.Mode, pass.ForgetRemoved = diskstore.ModePreview, false
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
	summary := diskCleanStoresSummary(result, released)
	if len(result.Forgotten) > 0 {
		data["forgotten"] = result.Forgotten
		summary += fmt.Sprintf("; forgot %d stale %s of removed checkouts", len(result.Forgotten), diskPlural(len(result.Forgotten), "registration", "registrations"))
	}
	trimSummary, _ := diskTrimSummary(trimData)
	summary += "; " + trimSummary
	if released == 0 && trimmed == 0 && len(result.Forgotten) == 0 {
		return inv.render(intentResult{Outcome: intentUnchanged, Summary: summary, text: lines, Data: data})
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Summary: summary, text: lines, Data: data})
}

// diskCleanStoresSummary tells what the passes did with the stores, never
// more than they did: a pass whose settings are unknown acted on nothing,
// and a machine pass another steward is running is not this one's.
func diskCleanStoresSummary(result steward.DiskPassResult, released int) string {
	summary := fmt.Sprintf("released %d store(s)", released)
	if released == 0 {
		summary = "nothing to release; every store is kept, pending or in use"
	}
	var unknown []string
	if len(result.Checkout.HostUnknown) > 0 {
		unknown = append(unknown, "the checkout pass acted on nothing: this checkout's settings are unknown")
	}
	if len(result.Machine.HostUnknown) > 0 {
		unknown = append(unknown, "the machine pass acted on nothing: the host settings are unknown")
	}
	if len(unknown) > 0 {
		lead := fmt.Sprintf("released %d store(s); ", released)
		if released == 0 {
			lead = ""
		}
		return lead + strings.Join(unknown, "; ") + " (the lines below name why; metasystem settings check names the fix)"
	}
	if result.Machine.Running {
		summary += "; the machine pass is running in another steward now"
	}
	return summary
}

func diskPlural(count int, one, many string) string {
	if count == 1 {
		return one
	}
	return many
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
	_, lines, reports, problem := diskTrim(inv, owners)
	if problem != nil {
		return inv.render(*problem)
	}
	summary, _ := diskTrimSummary(reports)
	return inv.render(intentResult{Outcome: intentConfirmed, Summary: summary, text: lines, Data: map[string]any{"caches": reports}})
}

// diskCacheNames are the machine caches as a person names them.
var diskCacheNames = map[string]string{
	"engine-go-build": "the engine Go cache", "engine-staticcheck": "the engine staticcheck cache",
	"delegate-go-build": "the delegate Go cache", "delegate-staticcheck": "the delegate staticcheck cache",
}

func diskCacheName(cache string) string {
	if name, ok := diskCacheNames[cache]; ok {
		return name
	}
	return cache
}

// diskTrimSummary is the headline of the cache reports, and whether every
// cache was measured whole and is within its cap. It never says more than
// the lines under it: a cache cut short says it is still measuring or
// trimming and how it resumes, a refused or held cache says so, and a cache
// over its cap inside the keep window says what stays.
func diskTrimSummary(reports []gocache.TrimReport) (string, bool) {
	removed, freed := 0, int64(0)
	var resuming, problems, over []string
	for _, report := range reports {
		removed += report.EntriesRemoved
		freed += report.BytesRemoved
		name := diskCacheName(report.Cache)
		switch report.EndedBy {
		case "budget", "cancelled":
			if report.Phase == "measure" {
				resuming = append(resuming, fmt.Sprintf("still measuring %s (%s counted so far)", name, diskBytes(report.Checkpoint.BytesSoFar)))
			} else {
				resuming = append(resuming, fmt.Sprintf("still trimming %s to its %s cap", name, diskBytes(report.CapBytes)))
			}
		case "lock-held":
			problems = append(problems, "another steward is trimming "+name+" now")
		case "refused":
			problems = append(problems, name+" was not trimmed: "+report.Reason)
		case "complete":
			if report.BytesAfter > report.CapBytes {
				over = append(over, fmt.Sprintf("%s stays over its %s cap: %s used within the keep window is never trimmed", name, diskBytes(report.CapBytes), diskBytes(report.KeepWindowBytes)))
			}
		}
	}
	var parts []string
	if removed > 0 {
		parts = append(parts, fmt.Sprintf("trimmed the machine caches: %d entries, %s freed", removed, diskBytes(freed)))
	}
	if len(resuming) > 0 {
		parts = append(parts, strings.Join(resuming, "; ")+"; "+diskPlural(len(resuming), "it resumes", "they resume")+
			" on the next pass: run metasystem disk clean --go-cache again or let the steward continue")
	}
	parts = append(parts, problems...)
	parts = append(parts, over...)
	done := len(resuming) == 0 && len(problems) == 0 && len(over) == 0
	switch {
	case done && removed > 0:
		parts = append(parts, "every cache is within its cap")
	case done:
		parts = append(parts, "the machine caches are within their caps: nothing removed")
	}
	return strings.Join(parts, "; "), done
}

// diskTrim runs the cache trim now (disk-lifetimes Part A, A12) with the
// settings of this checkout's installation. A person at a terminal expects
// it to finish the job: it continues pass after pass until every cache is
// done or disk.cache-trim-person-budget-sec is spent, telling each pass
// that left a cache unfinished on stderr.
func diskTrim(inv *intentInvocation, owners diskOwners) (int, []string, []gocache.TrimReport, *intentResult) {
	run := steward.CacheTrimRun{Top: inv.layout.InstallationRoot, UserCacheDir: owners.userCacheDir, StateDir: owners.stateDir, Clock: owners.now, Pass: owners.trimPass}
	reports, err := steward.TrimMachineCachesForPerson(context.Background(), run, func(pass int, reports []gocache.TrimReport) {
		for _, report := range reports {
			if report.EndedBy == "budget" {
				fmt.Fprintf(inv.stderr, "disk clean: pass %d: %s\n", pass, diskTrimLine(report))
			}
		}
	})
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
