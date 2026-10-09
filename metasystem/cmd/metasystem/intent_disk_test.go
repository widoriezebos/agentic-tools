package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

var diskNow = time.Date(2026, 9, 28, 21, 0, 0, 0, time.UTC)

// diskProof releases a process-owned store whose owner reference is "dead".
type diskProof struct{}

func (diskProof) Kind() diskstore.OwnerKind { return diskstore.OwnerProcess }
func (diskProof) Observe(_ context.Context, record diskstore.Record) diskstore.Verdict {
	if record.Owner.Ref == "dead" {
		return diskstore.Verdict{Decision: diskstore.Release, Reason: "owner dead"}
	}
	return diskstore.Verdict{Decision: diskstore.Keep, Reason: "owner alive", Command: "metasystem session stop fixture"}
}
func (diskProof) Apply(ctx context.Context, critical *diskstore.Critical) error {
	return diskstore.RemoveStore(ctx, critical.Record())
}

type diskBed struct {
	t                     *testing.T
	root, inst, home, tmp string
	owners                intentOwners
	person                error
	census                *diskstore.UseCensus
}

func newDiskBed(t *testing.T) *diskBed {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	inst := filepath.Join(root, "metasystem")
	for _, dir := range []string{filepath.Join(root, ".git"), filepath.Join(root, "development"), filepath.Join(inst, "scripts", "agents")} {
		helmMust(t, os.MkdirAll(dir, 0o755))
	}
	helmMust(t, os.WriteFile(filepath.Join(root, "development", "metasystem-design.md"), []byte("x\n"), 0o644),
		os.WriteFile(filepath.Join(inst, "metasystem.conf"), []byte("metasystem.template=true\ndisk.floor-gib=1\n"), 0o644))
	bed := &diskBed{t: t, root: root, inst: inst, home: filepath.Join(root, "home", ".metasystem"), tmp: filepath.Join(root, "tmp")}
	helmMust(t, os.MkdirAll(bed.home, 0o700), os.MkdirAll(bed.tmp, 0o700))
	bed.census = &diskstore.UseCensus{Taken: true}
	bed.owners = intentOwners{resolver: stateroot.NewResolver(fakeTop(root), noExecutable), disk: diskOwners{
		pass: func(ctx context.Context, top string, pass steward.DiskPass) (steward.DiskPassResult, error) {
			pass.Home, pass.TempRoots, pass.Volumes, pass.Checkouts = bed.home, []string{bed.tmp}, []string{root}, []string{}
			pass.Proofs = map[diskstore.OwnerKind]diskstore.OwnerProof{diskstore.OwnerProcess: diskProof{}}
			pass.Clock = func() time.Time { return diskNow }
			// The bed starts no process: the pass's use census reads an
			// empty table, never the host's.
			pass.Processes = identity.ListedProcessTable{}
			pass.UserHome = filepath.Join(root, "user")
			pass.Facts = func(context.Context, string) (diskstore.CheckoutFacts, error) {
				return diskstore.CheckoutFacts{}, errors.New("the fixture runs no git")
			}
			return steward.SweepDiskStores(ctx, top, pass)
		},
		home:         func() (string, error) { return bed.home, nil },
		now:          func() time.Time { return diskNow },
		person:       func(string) (string, error) { return "Wido", bed.person },
		census:       func() *diskstore.UseCensus { return bed.census },
		proofs:       map[diskstore.OwnerKind]diskstore.OwnerProof{diskstore.OwnerProcess: diskProof{}},
		tempRoots:    func() []string { return []string{bed.tmp} },
		userCacheDir: func() (string, error) { return filepath.Join(root, "user-cache"), nil },
		stateDir:     filepath.Join(root, "cache-trim"),
	}}
	return bed
}

func (b *diskBed) run(args ...string) (int, string) {
	command, rest, ok := resolveIntentArgv(args)
	if !ok {
		b.t.Fatalf("no public command %q", args)
	}
	var out bytes.Buffer
	code := runIntentIn(command, rest, &out, &out, b.root, b.owners)
	return code, out.String()
}

// store registers and creates a marker store in the checkout registry.
func (b *diskBed) store(name, ref string) diskstore.Record {
	b.t.Helper()
	registry := diskstore.CheckoutRegistry(b.inst)
	path := filepath.Join(b.root, "stores-data", name)
	record, err := registry.Register(diskstore.Registration{Path: path, Class: "fixture-store", Owner: diskstore.Owner{Kind: diskstore.OwnerProcess, Ref: ref},
		Lifetime: diskstore.LifetimeOwner, CapKind: diskstore.CapTarget}, diskNow, rand.Reader)
	helmMust(b.t, err, os.MkdirAll(path, 0o700))
	helmMust(b.t, diskstore.WriteMarker(record), os.WriteFile(filepath.Join(path, "payload"), []byte("bytes"), 0o600))
	_, err = registry.Accept(record.ID)
	helmMust(b.t, err)
	return record
}

func (b *diskBed) stray(name string, age time.Duration) string {
	b.t.Helper()
	path := filepath.Join(b.tmp, name)
	helmMust(b.t, os.MkdirAll(path, 0o700), os.WriteFile(filepath.Join(path, "out"), []byte("bed"), 0o600))
	at := diskNow.Add(-age)
	helmMust(b.t, os.Chtimes(filepath.Join(path, "out"), at, at), os.Chtimes(path, at, at))
	return path
}

func diskSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	helmMust(t, filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := info.Mode().String()
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(data)
			value += " " + hex.EncodeToString(sum[:])
		}
		files[strings.TrimPrefix(path, root)] = value
		return nil
	}))
	return files
}

func diskSnapshotDiff(before, after map[string]string) []string {
	var changed []string
	for path, value := range after {
		if before[path] != value {
			changed = append(changed, path)
		}
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			changed = append(changed, "-"+path)
		}
	}
	sort.Strings(changed)
	return changed
}

// The router and help (U6a): disk has show and clean; machine keeps list,
// stop, start and provider clear and has no clean.
func TestDiskObjectRoutesAndHelps(t *testing.T) {
	t.Parallel()
	var actions []string
	for _, command := range objectActions("disk") {
		actions = append(actions, command.action)
	}
	if !slices.Equal(actions, []string{"show", "clean"}) {
		t.Fatalf("disk actions = %v", actions)
	}
	var machine []string
	for _, command := range objectActions("machine") {
		machine = append(machine, command.action)
	}
	if !slices.Equal(machine, []string{"list", "clear-provider", "stop", "revive", "start"}) {
		t.Fatalf("machine actions = %v", machine)
	}
	if _, _, ok := resolveIntentArgv([]string{"machine", "clean"}); ok {
		t.Fatal("machine clean is routed; the disk object owns clean")
	}
	for _, args := range [][]string{{"disk", "show", "--help"}, {"disk", "clean", "--help"}} {
		code, stdout, _ := routeWith(nil, args...)
		if code != 0 || !strings.Contains(stdout, "  usage   ") || !strings.Contains(stdout, "metasystem "+args[0]+" "+args[1]) {
			t.Errorf("%v help = %d %q", args, code, stdout)
		}
	}
}

// disk clean is idempotent: the first run releases what its proof allows,
// the second finds nothing and changes no store, record or marker (R9).
func TestDiskCleanTwiceSecondIsEmpty(t *testing.T) {
	t.Parallel()
	bed := newDiskBed(t)
	witnessDiskSweepRepeat(t, bed)
}

func witnessDiskSweepRepeat(t *testing.T, bed *diskBed) {
	t.Helper()
	dead := bed.store("dead", "dead")
	alive := bed.store("alive", "alive")
	code, out := bed.run("disk", "clean")
	if code != 0 || !strings.Contains(out, "released 1 store") {
		t.Fatalf("first clean = %d:\n%s", code, out)
	}
	if _, err := os.Stat(dead.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the dead owner's store survived: %v", err)
	}
	if !strings.Contains(flatPage(out), "kept: "+bed.shown(alive.Path)+": owner alive; run metasystem session stop fixture") {
		t.Fatalf("the kept store is not named with its command:\n%s", out)
	}
	stores := filepath.Join(bed.root, "stores-data")
	registry := diskstore.CheckoutRegistry(bed.inst).Dir
	before := [2]map[string]string{diskSnapshot(t, stores), diskSnapshot(t, registry)}
	code, out = bed.run("disk", "clean")
	if code != 0 || !strings.Contains(out, "nothing to release") {
		t.Fatalf("second clean = %d:\n%s", code, out)
	}
	for index, root := range []string{stores, registry} {
		if changed := diskSnapshotDiff(before[index], diskSnapshot(t, root)); len(changed) != 0 {
			t.Fatalf("the second clean changed %v", changed)
		}
	}
}

// disk show reads the last reports and changes nothing; a preview changes
// nothing but its plan, even as the first pass on a machine root with no
// lock files (DL2-13, DL3B-05).
func TestDiskShowAndPreviewChangeNothing(t *testing.T) {
	t.Parallel()
	bed := newDiskBed(t)
	dead := bed.store("dead", "dead")
	stray := bed.stray("metasystem-audit.old", 72*time.Hour)
	foreign := bed.stray("tmp.foreign", 72*time.Hour)
	helmMust(t, os.RemoveAll(filepath.Join(bed.home, "stores")))
	before := diskSnapshot(t, bed.root)
	code, out := bed.run("disk", "show")
	if code != 0 || !strings.Contains(out, "This checkout\n  no pass has run yet") || !strings.Contains(out, "→ metasystem disk clean --preview") {
		t.Fatalf("show before any pass = %d:\n%s", code, out)
	}
	if changed := diskSnapshotDiff(before, diskSnapshot(t, bed.root)); len(changed) != 0 {
		t.Fatalf("disk show changed %v", changed)
	}
	code, out = bed.run("disk", "clean", "--preview")
	if code != 0 || !strings.Contains(out, "Preview: nothing was changed · plan") || !strings.Contains(flatPage(out), "would release: "+bed.shown(dead.Path)) ||
		!strings.Contains(flatPage(out), "stray: "+bed.shown(stray)) {
		t.Fatalf("preview = %d:\n%s", code, out)
	}
	for _, path := range diskSnapshotDiff(before, diskSnapshot(t, bed.root)) {
		allowed := strings.HasPrefix(path, "/home/.metasystem/stores/plans") || path == "/home/.metasystem/stores" ||
			path == "/home/.metasystem/stores/.sweep.flock"
		if !allowed {
			t.Errorf("the preview changed %s", path)
		}
	}
	if strings.Contains(out, bed.shown(foreign)+",") || strings.Contains(out, "stray: "+bed.shown(foreign)) {
		t.Fatalf("a foreign entry was listed as a stray:\n%s", out)
	}
	if _, err := os.Stat(dead.Path); err != nil {
		t.Fatal("the preview released a store")
	}
	code, out = bed.run("disk", "clean")
	if code != 0 {
		t.Fatalf("clean = %d:\n%s", code, out)
	}
	code, out = bed.run("disk", "show")
	if code != 0 || !strings.Contains(flatPage(out), "released: "+bed.shown(dead.Path)) || !strings.Contains(out, "Evidence\n  root") {
		t.Fatalf("show after a pass = %d:\n%s", code, out)
	}
}

// --strays is a person's act from a preview: an agent is told who runs it;
// a person removes the idle stray, and a held one is declined naming the
// holder while the rest completes; a repeat writes nothing.
func TestDiskStraysArePersonsActFromAPreview(t *testing.T) {
	t.Parallel()
	bed := newDiskBed(t)
	idle := bed.stray("metasystem-audit.idle", 72*time.Hour)
	held := bed.stray("goal-txn-held", 72*time.Hour)
	if code, out := bed.run("disk", "clean", "--strays"); code != 2 || !strings.Contains(out, "there is no preview to act on") || !strings.Contains(out, "metasystem disk clean --preview") {
		t.Fatalf("strays before a preview = %d:\n%s", code, out)
	}
	if code, out := bed.run("disk", "clean", "--preview"); code != 0 {
		t.Fatalf("preview = %d:\n%s", code, out)
	}
	bed.person = errors.New("an agent runs this terminal")
	if code, out := bed.run("disk", "clean", "--strays"); code != 3 || !strings.Contains(out, "an agent runs this terminal, so nothing was done") || !strings.Contains(out, "→ metasystem disk clean --strays  at your enrolled terminal") {
		t.Fatalf("strays by an agent = %d:\n%s", code, out)
	}
	bed.person = nil
	bed.census = &diskstore.UseCensus{Taken: true, Processes: []diskstore.CensusProcess{{Pid: 4242, UID: 501, Command: "bash bed.sh", Cwd: held}}}
	code, out := bed.run("disk", "clean", "--strays")
	if flat := flatPage(out); code != 0 || !strings.Contains(flat, "removed: "+bed.shown(idle)) || !strings.Contains(flat, "kept: "+bed.shown(held)+": in use by pid 4242") ||
		!strings.Contains(flat, "run metasystem disk clean --strays once pid 4242 has ended") {
		t.Fatalf("strays = %d:\n%s", code, out)
	}
	if _, err := os.Stat(idle); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the idle stray survived")
	}
	if _, err := os.Stat(held); err != nil {
		t.Fatal("the held stray was removed")
	}
	before := diskSnapshot(t, bed.root)
	if code, out := bed.run("disk", "clean", "--strays"); code != 0 || !strings.Contains(flatPage(out), "already gone: "+bed.shown(idle)) {
		t.Fatalf("a repeat = %d:\n%s", code, out)
	}
	if changed := diskSnapshotDiff(before, diskSnapshot(t, bed.root)); len(changed) != 0 {
		t.Fatalf("a repeat changed %v", changed)
	}
}

// Many strays print one line per outcome and reason with the count, the
// total size and the three largest, never one line per item (Wido's run of
// 2026-09-29 printed 2,371) and the space freed; --verbose prints every
// item, and --json carries every item whatever the flag.
func TestDiskStraysGroupTheirOutcomes(t *testing.T) {
	t.Parallel()
	bed := newDiskBed(t)
	var idle, young []string
	for index := range 5 {
		idle = append(idle, bed.stray(fmt.Sprintf("metasystem-audit.idle%d", index), 72*time.Hour))
		young = append(young, bed.stray(fmt.Sprintf("metasystem-wait-candidate-%d", index), time.Duration(index+1)*time.Hour))
	}
	helmMust(t, os.WriteFile(filepath.Join(idle[3], "big"), bytes.Repeat([]byte("x"), 1<<20), 0o600))
	helmMust(t, os.Chtimes(filepath.Join(idle[3], "big"), diskNow.Add(-72*time.Hour), diskNow.Add(-72*time.Hour)))
	helmMust(t, os.Chtimes(idle[3], diskNow.Add(-72*time.Hour), diskNow.Add(-72*time.Hour)))
	if code, out := bed.run("disk", "clean", "--preview"); code != 0 || strings.Count(out, "metasystem-audit.idle") != 3 {
		t.Fatalf("preview = %d, want the idle strays grouped with three named:\n%s", code, out)
	}
	if code, out := bed.run("disk", "clean", "--preview", "--verbose"); code != 0 || strings.Count(out, "  stray: ") != 10 {
		t.Fatalf("preview --verbose = %d, want one line per stray:\n%s", code, out)
	}
	code, out := bed.run("disk", "clean", "--strays")
	flat := flatPage(out)
	if code != 0 || !strings.Contains(flat, "strays: 5 done, 5 kept") || !strings.Contains(flat, "MiB freed") || !strings.Contains(flat, "--verbose prints every item") {
		t.Fatalf("strays = %d:\n%s", code, out)
	}
	removed := regexp.MustCompile(`removed: 5 strays, [0-9.]+ [KM]iB \(largest: ` + regexp.QuoteMeta(bed.shown(idle[3])) + ` `).FindString(flat)
	kept := regexp.MustCompile(`kept: 5 strays, [0-9.]+ [KM]?i?B: written less than a day ago; a stray is removed once it has been idle a day; run metasystem disk clean --preview tomorrow, then --strays \(largest: `).FindString(flat)
	if removed == "" || kept == "" {
		t.Fatalf("the strays were not grouped with their count, size and largest:\n%s", out)
	}
	named := 0
	for _, path := range append(append([]string{}, idle...), young...) {
		named += strings.Count(flat, bed.shown(path)+" ")
	}
	if named != 6 {
		t.Fatalf("the grouped output named %d paths, want the three largest of each group:\n%s", named, out)
	}
	code, out = bed.run("disk", "clean", "--strays", "--verbose")
	if code != 0 || !strings.Contains(flatPage(out), "already gone: "+bed.shown(idle[0])) || !strings.Contains(flatPage(out), "kept "+bed.shown(young[4])+": written ") {
		t.Fatalf("strays --verbose = %d:\n%s", code, out)
	}
	for _, path := range append(append([]string{}, idle...), young...) {
		if !strings.Contains(out, bed.shown(path)) {
			t.Fatalf("--verbose did not name %s:\n%s", path, out)
		}
	}
	code, out = bed.run("disk", "clean", "--strays", "--json")
	if code != 0 || strings.Count(out, `"path"`) != 10 {
		t.Fatalf("strays --json does not carry every item = %d:\n%s", code, out)
	}
	if code, out := bed.run("disk", "clean"); code != 0 {
		t.Fatalf("clean = %d:\n%s", code, out)
	}
	if code, out := bed.run("disk", "show"); code != 0 || !strings.Contains(out, "  strays: 5 items") || strings.Contains(out, "  stray: ") {
		t.Fatalf("show = %d, want the five young strays on one line:\n%s", code, out)
	}
	if code, out := bed.run("disk", "show", "--verbose"); code != 0 || strings.Count(out, "  stray: ") != 5 {
		t.Fatalf("show --verbose = %d, want one line per stray:\n%s", code, out)
	}
}

// --release (DL3B-12): an incomplete use check alone keeps a store; the
// person releases it; a readable holder declines it; a repeat succeeds.
func TestDiskReleaseByAPerson(t *testing.T) {
	t.Parallel()
	bed := newDiskBed(t)
	record := bed.store("plain", "dead")
	registry := diskstore.CheckoutRegistry(bed.inst)
	loaded, err := registry.Load(record.ID)
	helmMust(t, err)
	loaded.Layout = "plain"
	critical, err := registry.TryCritical(record.ID)
	helmMust(t, err, critical.Write(loaded), critical.Release())
	bed.census = &diskstore.UseCensus{Taken: true, Processes: []diskstore.CensusProcess{{Pid: 5151, UID: 501, Command: "vim notes", Cwd: record.Path}}}
	code, out := bed.run("disk", "clean", "--release", record.ID)
	if flat := flatPage(out); code != 0 || !strings.Contains(flat, "kept: "+bed.shown(record.Path)+": in use by pid 5151") || !strings.Contains(flat, "--release "+record.ID+" once pid 5151 has ended") {
		t.Fatalf("a held release = %d:\n%s", code, out)
	}
	bed.census = &diskstore.UseCensus{Taken: true, Unreadable: []diskstore.CensusGap{{Pid: 7, Reason: "unreadable"}}}
	code, out = bed.run("disk", "clean", "--release", record.ID)
	if code != 0 || !strings.Contains(out, "release: 1 done") {
		t.Fatalf("release = %d:\n%s", code, out)
	}
	if code, out := bed.run("disk", "clean", "--release", record.ID); code != 0 || !strings.Contains(out, "already released") {
		t.Fatalf("a repeat = %d:\n%s", code, out)
	}
	if code, out := bed.run("disk", "clean", "--release", "01ARZ3NDEKTSV4RRFFQ69G5FAV"); code != 2 || !strings.Contains(out, "no registered store has the id") {
		t.Fatalf("an unknown id = %d:\n%s", code, out)
	}
	if code, out := bed.run("disk", "clean", "--preview", "--strays"); code != 2 || !strings.Contains(out, "one thing at a time") {
		t.Fatalf("two acts at once = %d:\n%s", code, out)
	}
}

// The H1 audit of the new texts (U6a, R9): every kept, pending or declined
// verdict of the disk code, and every refusal of the disk verbs, names a
// public command; a bare pid is never the remedy.
func TestAuditDiskKeptAndDeclinedTextsNameACommand(t *testing.T) {
	t.Parallel()
	_, module := verbRatchetRoots(t)
	var files []string
	matches, err := filepath.Glob(filepath.Join(module, "internal", "diskstore", "*.go"))
	helmMust(t, err)
	files = append(files, matches...)
	files = append(files, filepath.Join(module, "internal", "steward", "disk_role.go"), filepath.Join(module, "cmd", "metasystem", "intent_disk.go"))
	checked := 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, path, nil, 0)
		helmMust(t, err)
		ast.Inspect(parsed, func(node ast.Node) bool {
			literal, ok := node.(*ast.CompositeLit)
			if !ok {
				return true
			}
			fields := map[string]ast.Expr{}
			for _, element := range literal.Elts {
				if pair, ok := element.(*ast.KeyValueExpr); ok {
					if key, ok := pair.Key.(*ast.Ident); ok {
						fields[key.Name] = pair.Value
					}
				}
			}
			where := fmt.Sprintf("%s:%d", filepath.Base(path), fileSet.Position(literal.Pos()).Line)
			switch diskLiteralType(literal.Type) {
			case "Verdict":
				decision := diskExprText(fields["Decision"])
				if !strings.HasSuffix(decision, "Keep") && !strings.HasSuffix(decision, "Pending") {
					return true
				}
				checked++
				command, ok := fields["Command"]
				if _, literal := command.(*ast.BasicLit); !ok || literal && !diskNamesCommand(command) {
					t.Errorf("%s: a kept or pending verdict without a public command", where)
				}
			case "intentResult":
				outcome := diskExprText(fields["Outcome"])
				if outcome != "intentRefused" && outcome != "intentFailed" {
					return true
				}
				checked++
				_, next := fields["next"]
				decision, hasDecision := fields["Decision"]
				summary := fields["Summary"]
				if !next && !(hasDecision && diskNamesCommand(decision)) && !diskNamesCommand(summary) {
					t.Errorf("%s: a refusal that names no working command", where)
				}
			}
			return true
		})
	}
	if checked < 20 {
		t.Fatalf("checked only %d texts; the audit no longer reaches the disk code", checked)
	}
}

func diskLiteralType(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		return typed.Sel.Name
	}
	return ""
}

func diskExprText(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		return diskExprText(typed.X) + "." + typed.Sel.Name
	}
	return ""
}

// diskNamesCommand reports an expression whose text starts with or holds a
// "metasystem " command literal.
func diskNamesCommand(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		if literal, ok := node.(*ast.BasicLit); ok && literal.Kind == token.STRING {
			if value, err := strconv.Unquote(literal.Value); err == nil && strings.Contains(value, "metasystem ") {
				found = true
			}
		}
		return true
	})
	return found
}

// disk clean forgets the registrations of removed checkouts and says so; a
// preview never forgets; a machine pass that acted on nothing because the
// host settings are unknown is not told as "every store is kept".
func TestDiskCleanSummaryTellsForgottenAndUnknown(t *testing.T) {
	t.Parallel()
	bed := newDiskBed(t)
	var forget []bool
	var result steward.DiskPassResult
	bed.owners.disk.pass = func(_ context.Context, _ string, pass steward.DiskPass) (steward.DiskPassResult, error) {
		forget = append(forget, pass.ForgetRemoved)
		return result, nil
	}
	result.Forgotten = []string{"/private/var/folders/T/tmp.a/repo", "/private/var/folders/T/tmp.b/repo"}
	code, out := bed.run("disk", "clean")
	if code != 0 || !strings.Contains(flatPage(out), "forgot the stale registrations of 2 removed checkouts") {
		t.Fatalf("disk clean = %d:\n%s", code, out)
	}
	if code, out := bed.run("disk", "clean", "--preview"); code != 0 || strings.Contains(out, "forgot") {
		t.Fatalf("preview = %d:\n%s", code, out)
	}
	if !slices.Equal(forget, []bool{true, false}) {
		t.Fatalf("ForgetRemoved per pass = %v; want disk clean true, preview false", forget)
	}
	result = steward.DiskPassResult{Machine: diskstore.Report{HostUnknown: []string{"host settings unknown: /m1b unreadable: denied; run metasystem settings check there"}}}
	code, out = bed.run("disk", "clean")
	if code != 0 || strings.Contains(out, "every store is kept") || !strings.Contains(flatPage(out), "the machine pass acted on nothing: the host settings are unknown") {
		t.Fatalf("an unknown host = %d:\n%s", code, out)
	}
}
