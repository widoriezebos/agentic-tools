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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/evidence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
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
	// proofs replaces the owner-kind proofs --release uses (fixtures); nil
	// is the engine's own for the checkout (steward.DiskOwnerProofs).
	proofs map[diskstore.OwnerKind]diskstore.OwnerProof
	// leases is a person's settlement of the host's dirty admission leases
	// (--leases); nil is proofrun.SettleHostLeases.
	leases func(top string) ([]proofrun.HostLeaseReport, bool, error)
	// tempRoots are the temporary roots --strays may remove under.
	tempRoots func() []string
	// userCacheDir and stateDir are the cache trimmer's seams (Part A):
	// nil and "" are the user's cache dir and the state beside the machine
	// registry.
	userCacheDir func() (string, error)
	stateDir     string
	// trimPass replaces one trim pass (fixtures); nil is the steward's.
	trimPass steward.CacheTrimPass
	// git runs git for workspaces; nil is the real git.
	git diskstore.WorkspaceGit
	// discarder is the delegate proof a person's --discard releases
	// through (fixtures); nil is steward.DelegateProof.
	discarder func(top string, now time.Time) (diskstore.Discarder, error)
	// evidenceEnv builds a person's evidence verb's environment (fixtures);
	// nil is steward.EvidenceEnv.
	evidenceEnv func(ctx context.Context, top, by string) (evidence.Env, error)
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
		o.person = provenPerson(humanauthority.KernelReader{}, func() int64 { return int64(os.Getppid()) }, goalCommandNow)
	}
	if o.census == nil {
		o.census = func() *diskstore.UseCensus {
			home, _ := steward.HomeStateRoot()
			census := diskstore.TakeUseCensus(context.Background(), *steward.KernelCensusReader(home, steward.ArmedCheckouts()))
			return &census
		}
	}
	if o.leases == nil {
		o.leases = proofrun.SettleHostLeases
	}
	if o.git == nil {
		o.git = steward.ExecWorkspaceGit
	}
	if o.discarder == nil {
		o.discarder = func(top string, now time.Time) (diskstore.Discarder, error) { return steward.DelegateProof(top, now) }
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

// proofsFor is the owner-kind proofs a person's --release judges a store of
// the checkout top by: the fixtures' when set, else every proof the
// steward's checkout pass has (process scratch, goal and session
// worktrees, handed-out workspaces, delegate workspaces).
func (o diskOwners) proofsFor(top string) map[diskstore.OwnerKind]diskstore.OwnerProof {
	if o.proofs != nil {
		return o.proofs
	}
	return steward.DiskOwnerProofs(top, o.now().UTC())
}

func diskIntentCommands() []intentCommand {
	return []intentCommand{
		{
			object: "disk", action: "show", audience: "both", summary: "what MetaSystem keeps on this computer's disk, changing nothing",
			usage: []string{"metasystem disk show"},
			details: []string{
				"Prints the last report of this checkout's pass and of the machine pass: free space per volume, each kind of store, what was released, what is kept or pending and the command that settles each, strays, the machine caches as the last trim pass left them, and the evidence roots of this host. A finding repeated for many paths is one line with its count and three of them; --verbose prints every path on its own line.",
				"The steward writes both reports every cycle; metasystem disk clean writes them now. This command only reads.",
			},
			flags:    []intentFlag{intentVerboseFlag},
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
				"metasystem disk clean --leases",
				"metasystem disk clean --discard j2:ID --reason TEXT",
				"metasystem disk clean --go-cache",
			},
			details: []string{
				"Runs the steward's pass now for this checkout and this machine: a store goes only when its owner is proven ended and nothing still uses it; everything else is kept or pending with the command that settles it. Then it trims the machine's Go and staticcheck caches to their caps (disk.go-cache-cap-gib, disk.delegate-go-cache-cap-gib, disk.staticcheck-cache-cap-gib), least recently used first: everything unused for disk.go-cache-keep-hours goes before anything used within it, and a cache still over its cap loses the rest oldest first, never an entry used within disk.cache-min-keep-minutes. Run at a terminal it finishes the job: pass after pass of the steward's disk.cache-trim-budget-sec, telling each on stderr, until every cache is measured and trimmed or disk.cache-trim-person-budget-sec is spent. A repeat with nothing to do is success.",
				"It also forgets the host registry's registrations of checkouts whose directories no longer exist (one reaped record each, reason checkout-gone); a registration whose recorded process still runs is kept. The steward's own pass only counts them.",
				"--go-cache runs only the cache trim.",
				"--preview changes nothing and writes one plan file whose id it prints. --strays, --release and --discard are a person's acts at the enrolled terminal: --strays removes the engine-named leftovers a preview listed, --release ID removes one registered store whose only obstacle is a use check the machine could not complete, --discard drops a chain's uncommitted work and releases its workspace now, archiving every commit it holds; --leases settles the host's dirty proof-admission leases whose owners are proven ended, by the proof the admission path uses, and names what keeps each other one.",
				"Nothing unregistered is ever removed by the pass itself; nothing a live process uses is removed by anyone.",
				"Output is a short summary: what was removed and the space freed, what was kept grouped by reason with its count and the three largest, and the command that settles it. --verbose prints every item on its own line; --json carries every item either way.",
			},
			flags: []intentFlag{
				{name: "preview", usage: "plan only: change nothing and write the plan file"},
				{name: "floor", value: "GIB", advanced: true, usage: "run as if free space were below GIB gibibytes"},
				{name: "strays", usage: "a person's act: remove the strays a preview listed"},
				{name: "plan", value: "ID", usage: "the preview to act on (default: the newest)"},
				{name: "release", value: "ID", usage: "a person's act: release the registered store ID that only an incomplete use check keeps"},
				{name: "leases", usage: "a person's act: settle the host's dirty proof-admission leases whose owners are proven ended"},
				{name: "discard", value: "j2:ID", advanced: true, usage: "a person's act: drop chain ID's uncommitted work and release its workspace now"},
				{name: "reason", value: "TEXT", advanced: true, usage: "why the work may be dropped (with --discard)"},
				{name: "go-cache", usage: "only trim the Go and staticcheck caches to their caps"},
				intentVerboseFlag,
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
			lines = append(lines, diskReportLines(inv, report)...)
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

// diskEvidenceRootLines names this checkout's evidence root. Every root of
// the host, with its owner, is in the machine pass's report above (the
// evidence bound, 3.12).
func diskEvidenceRootLines(installation, home string) []string {
	settings, err := diskstore.LoadSettings(filepath.Join(installation, "metasystem.conf"), nil)
	if err != nil {
		return []string{"evidence root of this checkout: unknown, the settings cannot be read: " + err.Error() + "; run metasystem settings check"}
	}
	return []string{"evidence root of this checkout: " + settings.EvidenceRoot.Path + " (" + settings.EvidenceRoot.Origin + ")"}
}

func runIntentDiskClean(inv *intentInvocation) int {
	top, problem := inv.diskRoot()
	if problem != nil {
		return inv.render(*problem)
	}
	owners := inv.owners.disk.withDefaults()
	chosen := 0
	for _, name := range []string{"strays", "release", "leases", "discard", "go-cache", "preview"} {
		if inv.input.has(name) {
			chosen++
		}
	}
	if chosen > 1 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "disk clean does one thing at a time: choose one of --preview, --strays, --release ID, --leases, --discard j2:ID or --go-cache; nothing was done",
			next:    inv.publicArgv("disk", "clean", "--preview"), nextReason: "see what a pass would do first"})
	}
	switch {
	case inv.input.has("go-cache"):
		return runDiskGoCache(inv, owners)
	case inv.input.has("strays"):
		return runDiskStrays(inv, owners, top)
	case inv.input.has("release"):
		return runDiskRelease(inv, owners, top)
	case inv.input.has("leases"):
		return runDiskLeases(inv, owners, top)
	case inv.input.has("discard"):
		return runDiskDiscard(inv, owners, top)
	}
	if inv.input.has("plan") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "--plan ID names the preview --strays acts on; alone it does nothing: metasystem disk clean --strays --plan ID; nothing was done"})
	}
	pass := steward.DiskPass{Mode: diskstore.ModeApply, Now: owners.now().UTC(), Clock: owners.now, ForgetRemoved: true, Clones: true}
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
	lines := append(diskReportLines(inv, result.Checkout), diskReportLines(inv, result.Machine)...)
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
		summary += fmt.Sprintf("; forgot the stale registrations of %d removed %s", len(result.Forgotten), diskPlural(len(result.Forgotten), "checkout", "checkouts"))
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

// provenPerson proves the person at the enrolled terminal from the shell pid
// names and returns the enrolled name to record.
func provenPerson(reader humanauthority.Reader, pid func() int64, now func(root string) (time.Time, error)) func(root string) (string, error) {
	return func(root string) (string, error) {
		at, err := now(root)
		if err != nil {
			return "", err
		}
		if _, err := humanauthority.Prove(root, pid(), reader, at); err != nil {
			return "", err
		}
		if enrollment, readErr := humanauthority.ReadEnrollment(root); readErr == nil && enrollment.Human != "" {
			return enrollment.Human, nil
		}
		return "the enrolled person", nil
	}
}

// diskPerson proves the person at the enrolled terminal for the acts only a
// person makes; an agent is told who runs it and how.
func diskPerson(inv *intentInvocation, owners diskOwners, top, act string) (string, *intentResult) {
	by, err := owners.person(top)
	if err != nil {
		return "", &intentResult{Outcome: intentRefused, code: 3,
			Summary:  "disk clean " + act + " is a person's act, and this shell was not proven to be one: " + humanauthority.PlainReason(err) + "; nothing was done",
			Decision: humanauthority.PersonActRemedy("metasystem disk clean "+act) + "; metasystem disk clean --preview shows what it would act on"}
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
	return renderPersonOutcomes(inv, "strays", "strays", by, outcomes)
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

// renderPersonOutcomes tells a person's act in a handful of lines: what was
// done and the space it freed, what was kept grouped by finding with its
// count, size and largest three, and the command that settles each; every
// item with --verbose, and always in --json.
func renderPersonOutcomes(inv *intentInvocation, act, many, by string, outcomes []diskstore.PersonOutcome) int {
	verbose := inv.input.switched("verbose")
	lines := diskstore.OutcomeLines(many, outcomes, verbose)
	done, declined := 0, 0
	for _, outcome := range outcomes {
		if outcome.Done {
			done++
		} else {
			declined++
		}
	}
	data := map[string]any{"by": by, "outcomes": outcomes}
	summary := fmt.Sprintf("%s: %d done", act, done)
	if freed := diskstore.Freed(outcomes); freed > 0 {
		summary += fmt.Sprintf(", %s freed", textui.BytesMiB(freed))
	}
	if declined > 0 {
		summary = fmt.Sprintf("%s: %d done, %d kept with the reason and the command that settles each", act, done, declined)
		if freed := diskstore.Freed(outcomes); freed > 0 {
			summary += fmt.Sprintf("; %s freed", textui.BytesMiB(freed))
		}
	}
	if !verbose && len(lines) < len(outcomes) {
		summary += "; --verbose prints every item"
	}
	if len(outcomes) == 0 {
		return inv.render(intentResult{Outcome: intentUnchanged, Summary: "the plan lists no " + act + "; nothing to do", Data: data})
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Summary: summary, text: lines, Data: data})
}

// diskReportLines is a pass report for a person: grouped, or every item
// with --verbose.
func diskReportLines(inv *intentInvocation, report diskstore.Report) []string {
	if inv.input.switched("verbose") {
		return report.VerboseLines()
	}
	return report.Lines()
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
	verdict, err := diskstore.ReleaseByPerson(context.Background(), registry, id, owners.proofsFor(top)[record.Owner.Kind], owners.census(), by)
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "releasing " + id + " stopped: " + err.Error(), Decision: "metasystem disk show"})
	}
	outcome := diskstore.PersonOutcome{Path: record.Path, Done: verdict.Decision == diskstore.Release, Reason: verdict.Reason, Command: verdict.Command,
		Finding: verdict.Reason, FindingCommand: verdict.Command, Bytes: record.Bytes}
	if verdict.Reason == "already released" {
		return inv.render(intentResult{Outcome: intentUnchanged, Summary: "store " + id + " is already released; nothing to do", Data: map[string]any{"by": by, "outcomes": []diskstore.PersonOutcome{outcome}}})
	}
	return renderPersonOutcomes(inv, "release", "stores", by, []diskstore.PersonOutcome{outcome})
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
	registry := diskstore.CheckoutRegistry(top)
	records, unreadable := registry.Inventory()
	if len(unreadable) > 0 {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the store registry cannot be read (" + unreadable[0].Reason + "); nothing was done",
			Decision: "metasystem disk show names what the last pass could read"})
	}
	var record *diskstore.Record
	for index := range records {
		if records[index].Owner == (diskstore.Owner{Kind: diskstore.OwnerDelegate, Ref: chain}) && records[index].State != diskstore.StateReleased {
			record = &records[index]
		}
	}
	if record == nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "chain j2:" + chain + " has no registered workspace to discard; nothing was done",
			next: inv.publicArgv("disk", "show"), nextReason: "the reports name every kept workspace and its chain"})
	}
	discarder, err := owners.discarder(top, owners.now())
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the chain's workspace cannot be judged: " + err.Error(), Decision: "metasystem disk show"})
	}
	// The discard is this invocation's alone (Round B3-3 rule 2): it
	// releases now, or keeps the workspace with the reason; it leaves no
	// authority for a later pass.
	verdict, err := discarder.ReleaseDiscarded(context.Background(), registry, record.ID, owners.census(),
		diskstore.Discard{By: by, At: owners.now().UTC(), Reason: reason})
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the discard stopped: " + err.Error(), Decision: "metasystem disk show"})
	}
	outcome := diskstore.PersonOutcome{Path: record.Path, Done: verdict.Decision == diskstore.Release, Reason: verdict.Reason, Command: verdict.Command,
		Finding: verdict.Reason, FindingCommand: verdict.Command}
	if verdict.Reason == "already released" {
		return inv.render(intentResult{Outcome: intentUnchanged, Summary: "chain j2:" + chain + "'s workspace is already released; nothing to do"})
	}
	return renderPersonOutcomes(inv, "discard", "workspaces", by, []diskstore.PersonOutcome{outcome})
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
// still over its cap after the keep window yielded says what stays.
func diskTrimSummary(reports []gocache.TrimReport) (string, bool) {
	removed, freed := 0, int64(0)
	var resuming, problems, over []string
	for _, report := range reports {
		removed += report.EntriesRemoved
		freed += report.BytesRemoved
		name := diskCacheName(report.Cache)
		switch report.EndedBy {
		case "budget", "cancelled":
			switch {
			case report.Phase == "measure" && diskTrimNotStarted(report):
				resuming = append(resuming, name+" is not measured yet")
			case report.Phase == "measure":
				resuming = append(resuming, fmt.Sprintf("still measuring %s (%s counted so far)", name, textui.BytesMiB(report.Checkpoint.BytesSoFar)))
			default:
				resuming = append(resuming, fmt.Sprintf("still trimming %s to its %s cap", name, textui.BytesMiB(report.CapBytes)))
			}
		case "lock-held":
			problems = append(problems, "another steward is trimming "+name+" now")
		case "refused":
			problems = append(problems, name+" was not trimmed: "+report.Reason)
		case "complete":
			if report.BytesAfter > report.CapBytes {
				over = append(over, fmt.Sprintf("%s stays over its %s cap: %s used within the last %s is never trimmed", name, textui.BytesMiB(report.CapBytes), textui.BytesMiB(report.MinKeepBytes), diskMinutes(report.MinKeepMinutes)))
			}
		}
	}
	var parts []string
	if removed > 0 {
		parts = append(parts, fmt.Sprintf("trimmed the machine caches: %d entries, %s freed", removed, textui.BytesMiB(freed)))
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
	if report.Phase == "measure" && diskTrimNotStarted(report) {
		return line + ", not measured yet: the pass ended before it reached this cache; the next pass starts it"
	}
	if report.Phase == "measure" {
		line += fmt.Sprintf(", measuring (%s counted so far; the next pass resumes at shard %s)", textui.BytesMiB(report.Checkpoint.BytesSoFar), report.Checkpoint.Shard)
		if report.OverCap {
			line += fmt.Sprintf(", %d removed (%s)%s", report.EntriesRemoved, textui.BytesMiB(report.BytesRemoved), diskOverCap(report))
		}
		return line
	}
	line += fmt.Sprintf(", %s of %s cap, %d removed (%s)", textui.BytesMiB(report.BytesAfter), textui.BytesMiB(report.CapBytes), report.EntriesRemoved, textui.BytesMiB(report.BytesRemoved))
	if report.OverCap {
		line += diskOverCap(report)
	}
	if report.BytesAfter > report.CapBytes && report.Phase == "idle" {
		line += fmt.Sprintf("; %s used within the last %s stays", textui.BytesMiB(report.MinKeepBytes), diskMinutes(report.MinKeepMinutes))
	}
	if report.UnknownCount > 0 {
		line += fmt.Sprintf("; %d entries not Go's layout were left untouched", report.UnknownCount)
	}
	return line
}

// diskOverCap says plainly what a pass that found its cache over the cap
// does: the keep window yields, oldest first, down to the floor.
func diskOverCap(report gocache.TrimReport) string {
	return "; over the cap: removing the oldest entries, keeping the last " + diskMinutes(report.MinKeepMinutes)
}

// diskMinutes is the floor in whole minutes.
func diskMinutes(minutes float64) string {
	return fmt.Sprintf("%d min", int64(minutes))
}

// diskTrimNotStarted: a measurement that has not stat'ed its first entry.
func diskTrimNotStarted(report gocache.TrimReport) bool {
	return report.Checkpoint.BytesSoFar == 0 && report.Checkpoint.LastName == "" && (report.Checkpoint.Shard == "" || report.Checkpoint.Shard == "00")
}

// runDiskLeases is a person's settlement of the host's dirty admission
// leases (3.8 --leases, DL2-19, DL3B-02): each is judged by the proof the
// admission path uses under admission.lock and the lease's own flock; one
// proven settled is reclaimed, every other is kept with its holder or its
// unknown and the command that settles it. A person's word overrides no
// live holder and no unknown. A repeat with nothing dirty succeeds and
// writes nothing.
func runDiskLeases(inv *intentInvocation, owners diskOwners, top string) int {
	by, problem := diskPerson(inv, owners, top, "--leases")
	if problem != nil {
		return inv.render(*problem)
	}
	reports, busy, err := owners.leases(top)
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the proof-admission leases cannot be read: " + err.Error() + "; nothing was reclaimed",
			Decision: "metasystem system check names what else is wrong"})
	}
	var outcomes []diskstore.PersonOutcome
	for _, report := range reports {
		outcome := diskstore.PersonOutcome{Path: report.Lease}
		switch {
		case report.State == proofrun.HostLeaseReclaimed:
			outcome.Done, outcome.Reason = true, "reclaimed: "+report.Reason
		case report.State == proofrun.HostLeaseLive:
			outcome.Reason = fmt.Sprintf("kept: %s (owner pid %d)", report.Reason, report.Owner.Pid)
			outcome.Command = fmt.Sprintf("end owner pid %d and what it started, then metasystem disk clean --leases", report.Owner.Pid)
		case busy:
			outcome.Reason = "kept: a proof holds admission.lock, so nothing is reclaimed now (" + report.Reason + ")"
			outcome.Command = "metasystem disk clean --leases once that proof has ended"
		default:
			outcome.Reason, outcome.Command = "kept: "+report.Reason, report.Remedy
			if outcome.Command == "" {
				outcome.Command = "metasystem disk clean --leases"
			}
		}
		outcome.Finding, outcome.FindingCommand = outcome.Reason, outcome.Command
		outcomes = append(outcomes, outcome)
	}
	if len(outcomes) == 0 {
		return inv.render(intentResult{Outcome: intentUnchanged, Summary: "no dirty proof-admission lease on this host; nothing to do", Data: map[string]any{"by": by}})
	}
	return renderPersonOutcomes(inv, "leases", "leases", by, outcomes)
}
