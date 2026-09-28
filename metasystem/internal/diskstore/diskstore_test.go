package diskstore

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)

func testRegistry(t *testing.T) Registry {
	t.Helper()
	return Registry{Dir: filepath.Join(t.TempDir(), "stores")}
}

func realDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func plainRegistration(path string) Registration {
	return Registration{Path: path, Class: "process-scratch", Owner: Owner{Kind: OwnerProcess, Ref: "pid:1"},
		Lifetime: LifetimeOwner, CapKind: CapTarget, CapBytes: 4 << 30}
}

// snapshotTree records every path, mode and content digest under root.
func snapshotTree(t *testing.T, root string) string {
	t.Helper()
	var lines []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		line := fmt.Sprintf("%s %s", strings.TrimPrefix(path, root), info.Mode())
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(data)
			line += " " + hex.EncodeToString(sum[:])
		}
		lines = append(lines, line)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func TestRegisterIsIdempotentAndRefusesAnotherOwner(t *testing.T) {
	registry := testRegistry(t)
	store := filepath.Join(realDir(t), "scratch")
	first, err := registry.Register(plainRegistration(store), testNow, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if first.State != StateReserved || !first.Identity.Marker || first.Schema != Schema {
		t.Fatalf("first registration = %+v", first)
	}
	again, err := registry.Register(plainRegistration(store), testNow.Add(time.Hour), rand.Reader)
	if err != nil || again.ID != first.ID {
		t.Fatalf("repeat registration = %+v, %v; want record %s", again, err, first.ID)
	}
	records, unreadable := registry.Inventory()
	if len(records) != 1 || len(unreadable) != 0 {
		t.Fatalf("inventory after a repeat = %d records, %v", len(records), unreadable)
	}
	other := plainRegistration(store)
	other.Owner = Owner{Kind: OwnerGoal, Ref: "g1"}
	if _, err := registry.Register(other, testNow, rand.Reader); err == nil || !strings.Contains(err.Error(), first.ID) {
		t.Fatalf("another owner's registration = %v; want a refusal naming %s", err, first.ID)
	}
	if _, err := os.Stat(registry.LockPath(first.ID)); err != nil {
		t.Fatalf("the record lock is created with the record: %v", err)
	}
}

func TestRegisterRefusesAnIncompleteRegistration(t *testing.T) {
	registry := testRegistry(t)
	for name, mutate := range map[string]func(*Registration){
		"relative path":         func(r *Registration) { r.Path = "scratch" },
		"unknown owner":         func(r *Registration) { r.Owner.Kind = "nobody" },
		"no class":              func(r *Registration) { r.Class = "" },
		"rebuildable no recipe": func(r *Registration) { r.Lifetime = LifetimeRebuildable },
		"unknown cap kind":      func(r *Registration) { r.CapKind = "soft" },
	} {
		registration := plainRegistration("/nonexistent/store")
		mutate(&registration)
		if _, err := registry.Register(registration, testNow, rand.Reader); err == nil {
			t.Errorf("%s: registration accepted", name)
		}
	}
}

// A git worktree store carries no file inside the tree (DL2-10): registering
// one leaves its tree byte-identical, which is what keeps git status empty.
func TestGitWorktreeRegistrationWritesNothingInTheTree(t *testing.T) {
	registry := testRegistry(t)
	worktree := filepath.Join(realDir(t), "checkout-g1")
	gitdir := filepath.Join(realDir(t), "common", ".git", "worktrees", "checkout-g1")
	for _, dir := range []string{worktree, gitdir, filepath.Join(worktree, "src")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: "+gitdir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "src", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, worktree)
	registration := Registration{Path: worktree, Git: true, Class: "goal-worktree", Owner: Owner{Kind: OwnerGoal, Ref: "g1"},
		Lifetime: LifetimeOwner, CapKind: CapNone}
	record, err := registry.Register(registration, testNow, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if record.Identity.Marker || record.Identity.Gitdir != gitdir || record.Identity.GitFileInode == 0 {
		t.Fatalf("git identity = %+v", record.Identity)
	}
	if after := snapshotTree(t, worktree); after != before {
		t.Fatalf("registration changed the worktree:\nbefore %s\nafter  %s", before, after)
	}
	if err := Revalidate(record); err != nil {
		t.Fatalf("an unchanged worktree fails revalidation: %v", err)
	}
	// Replacing the .git file (a new worktree at a reused path) is caught.
	if err := os.Remove(filepath.Join(worktree, ".git")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: "+gitdir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Revalidate(record); err == nil {
		t.Fatal("a replaced .git file passed revalidation")
	}
}

func TestMarkerNamesItsRecordAndIsNeverReplaced(t *testing.T) {
	registry := testRegistry(t)
	store := filepath.Join(realDir(t), "plain")
	record, err := registry.Register(plainRegistration(store), testNow, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(store, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := WriteMarker(record); err != nil {
		t.Fatal(err)
	}
	if err := WriteMarker(record); err != nil {
		t.Fatalf("a repeated marker write is success: %v", err)
	}
	if err := Revalidate(record); err != nil {
		t.Fatal(err)
	}
	foreign := record
	foreign.ID = "01ZZZZZZZZZZZZZZZZZZZZZZZZ"
	if err := WriteMarker(foreign); err == nil {
		t.Fatal("a marker of another record was replaced")
	}
	if err := Revalidate(foreign); err == nil {
		t.Fatal("a store whose marker names another record passed revalidation")
	}
}

func TestTransitionsAreOwnedAndRepeatAsSuccess(t *testing.T) {
	registry := testRegistry(t)
	record, err := registry.Register(plainRegistration(filepath.Join(realDir(t), "s")), testNow, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := registry.Accept(record.ID)
	if err != nil || accepted.State != StateAccepted {
		t.Fatalf("accept = %+v, %v", accepted, err)
	}
	if _, err := registry.Accept(record.ID); err != nil {
		t.Fatalf("a repeated accept is success: %v", err)
	}
	released, err := registry.Transition(record.ID, []State{StateAccepted, StateReleasing}, StateReleased,
		func(r *Record) { r.ReleasedBy = "owner" })
	if err != nil || released.State != StateReleased || released.ReleasedBy != "owner" {
		t.Fatalf("release = %+v, %v", released, err)
	}
	if _, err := registry.Transition(record.ID, []State{StateReleased}, StateAccepted, nil); err == nil {
		t.Fatal("a released record became accepted")
	}
}

// The critical-section helper refuses a held lock at once and reloads the
// record from disk after acquiring, so a change made between a caller's
// lookup and its acquisition is what it sees (DL3B-01).
func TestCriticalSectionRefusesAHeldLockAndReloads(t *testing.T) {
	registry := testRegistry(t)
	record, err := registry.Register(plainRegistration(filepath.Join(realDir(t), "s")), testNow, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Accept(record.ID); err != nil {
		t.Fatal(err)
	}
	lookedUp, err := registry.Load(record.ID)
	if err != nil || lookedUp.State != StateAccepted {
		t.Fatal(lookedUp, err)
	}
	// Another holder changes the record between the lookup and acquisition.
	if _, err := registry.Transition(record.ID, []State{StateAccepted}, StateReleasing, nil); err != nil {
		t.Fatal(err)
	}
	critical, err := registry.TryCritical(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if critical.Record().State != StateReleasing {
		t.Fatalf("critical section saw %s, not the reloaded releasing", critical.Record().State)
	}
	started := time.Now()
	_, heldErr := registry.TryCritical(record.ID)
	var held *HeldError
	if !errors.As(heldErr, &held) {
		t.Fatalf("a second exclusive attempt = %v, want HeldError", heldErr)
	}
	if waited := time.Since(started); waited > 5*time.Second {
		t.Fatalf("the held lock was waited on for %s", waited)
	}
	if free, err := registry.ProbeRecordLock(record.ID); err != nil || free {
		t.Fatalf("probe of a held lock = %v, %v", free, err)
	}
	if err := critical.Release(); err != nil {
		t.Fatal(err)
	}
	if free, err := registry.ProbeRecordLock(record.ID); err != nil || !free {
		t.Fatalf("probe of a free lock = %v, %v", free, err)
	}
}

// An entrant holds the lock shared, so the sweeper's nonblocking exclusive
// attempt fails and the store is pending; an entrant arriving after the
// sweeper wrote releasing reads it and refuses. Never both proceed.
func TestEntrantAndSweeperNeverBothProceed(t *testing.T) {
	registry := testRegistry(t)
	record, err := registry.Register(plainRegistration(filepath.Join(realDir(t), "s")), testNow, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Accept(record.ID); err != nil {
		t.Fatal(err)
	}
	entrant, err := registry.Enter(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	var held *HeldError
	if _, err := registry.TryCritical(record.ID); !errors.As(err, &held) {
		t.Fatalf("sweeper during an entrant = %v, want HeldError", err)
	}
	if err := entrant.Leave(); err != nil {
		t.Fatal(err)
	}
	critical, err := registry.TryCritical(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	next := critical.Record()
	next.State = StateReleasing
	if err := critical.Write(next); err != nil {
		t.Fatal(err)
	}
	if err := critical.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Enter(record.ID); !errors.Is(err, ErrStoreGone) {
		t.Fatalf("entrant after releasing = %v, want ErrStoreGone", err)
	}
}

func TestProbeRecordLockCreatesNothing(t *testing.T) {
	registry := testRegistry(t)
	if err := os.MkdirAll(registry.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, registry.Dir)
	free, err := registry.ProbeRecordLock("01ARZ3NDEKTSV4RRFFQ69G5FAV")
	if err != nil || !free {
		t.Fatalf("an absent lock reads %v, %v; want free", free, err)
	}
	if after := snapshotTree(t, registry.Dir); after != before {
		t.Fatal("the probe created a file")
	}
	if records, unreadable := (Registry{Dir: filepath.Join(registry.Dir, "absent")}).Inventory(); records != nil || unreadable != nil {
		t.Fatal("an absent registry is not empty")
	}
	if _, err := os.Stat(filepath.Join(registry.Dir, "absent")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the inventory created the registry")
	}
}

func TestReservationKeySeparatesOwnersAndNames(t *testing.T) {
	keys := map[string]string{}
	for label, input := range map[string]struct {
		owner Owner
		name  string
	}{
		"goal default":   {Owner{Kind: OwnerGoal, Ref: "g1"}, ""},
		"session a":      {Owner{Kind: OwnerSession, Ref: "a"}, "bed"},
		"session b":      {Owner{Kind: OwnerSession, Ref: "b"}, "bed"},
		"goal bed":       {Owner{Kind: OwnerGoal, Ref: "g1"}, "bed"},
		"session g1 bed": {Owner{Kind: OwnerSession, Ref: "g1"}, "bed"},
	} {
		key, err := ReservationKey(input.owner, input.name)
		if err != nil {
			t.Fatal(err)
		}
		if other, dup := keys[key]; dup {
			t.Fatalf("%s and %s share a reservation key", label, other)
		}
		keys[key] = label
	}
	named, _ := ReservationKey(Owner{Kind: OwnerGoal, Ref: "g1"}, DefaultWorkspaceName)
	defaulted, _ := ReservationKey(Owner{Kind: OwnerGoal, Ref: "g1"}, "")
	if named != defaulted {
		t.Fatal("an unnamed request is not the default name")
	}
	if _, err := ReservationKey(Owner{Kind: OwnerGoal, Ref: "g1"}, "a/b"); err == nil {
		t.Fatal("a name with a slash was accepted")
	}
}

func buildTree(t *testing.T, root string, files int) {
	t.Helper()
	for index := range files {
		dir := filepath.Join(root, fmt.Sprintf("d%d", index%3), "nested")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d", index)), bytes.Repeat([]byte{byte(index)}, 100), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("/", filepath.Join(root, "link-to-root")); err != nil {
		t.Fatal(err)
	}
}

// RemoveTree under a cancelled context stops, leaves a partly removed tree,
// and the next call finishes it; it never follows a symlink.
func TestRemoveTreeStopsAtTheContextAndResumes(t *testing.T) {
	root := filepath.Join(realDir(t), "store")
	buildTree(t, root, 12)
	ctx, cancel := context.WithCancel(context.Background())
	removed := 0
	previous := afterTreeEntryRemoved
	afterTreeEntryRemoved = func(string) {
		removed++
		if removed == 5 {
			cancel()
		}
	}
	t.Cleanup(func() { afterTreeEntryRemoved = previous })
	err := RemoveTree(ctx, root)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("remove under a cancelled context = %v", err)
	}
	if removed != 5 {
		t.Fatalf("removed %d entries after the cancel, want exactly 5", removed)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("the cut-short tree is gone: %v", err)
	}
	afterTreeEntryRemoved = previous
	if err := RemoveTree(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the second call did not finish: %v", err)
	}
	if err := RemoveTree(context.Background(), root); err != nil {
		t.Fatalf("removing an absent tree is success: %v", err)
	}
	if _, err := os.Stat("/"); err != nil {
		t.Fatal("the symlink was followed")
	}
}

func TestRemoveTreeRefusesARelativeOrSymlinkedRoot(t *testing.T) {
	dir := realDir(t)
	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "keep"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"relative", link, dir + "/../" + filepath.Base(dir), "/"} {
		if err := RemoveTree(context.Background(), path); err == nil {
			t.Errorf("RemoveTree(%q) was accepted", path)
		}
	}
	if _, err := os.Stat(filepath.Join(target, "keep")); err != nil {
		t.Fatal("a refused removal removed the target")
	}
}

func TestCopyTreeCopiesAndStopsAtTheContext(t *testing.T) {
	dir := realDir(t)
	source := filepath.Join(dir, "source")
	buildTree(t, source, 6)
	destination := filepath.Join(dir, "copy")
	if err := CopyTree(context.Background(), source, destination); err != nil {
		t.Fatal(err)
	}
	if snapshotTree(t, source) != snapshotTree(t, destination) {
		t.Fatal("the copy differs from its source")
	}
	if err := CopyTree(context.Background(), source, destination); err == nil {
		t.Fatal("a copy overwrote an existing destination")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := CopyTree(ctx, source, filepath.Join(dir, "cancelled")); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled copy = %v", err)
	}
}

// Two processes with different TMPDIRs resolve the same shared host path
// (DL2-15): the path follows the host's temporary root, never TMPDIR.
func TestHostSharedIgnoresTheProcessTMPDIR(t *testing.T) {
	if os.Getenv("DISKSTORE_HOST_SHARED_HELPER") == "1" {
		shared, err := HostShared("metasystem-seat-launch.lock")
		if err != nil {
			fmt.Println("error:", err)
			os.Exit(3)
		}
		fmt.Printf("shared=%s\ntemp=%s\n", shared, ProcessTempRoot())
		return
	}
	mine, err := HostShared("metasystem-seat-launch.lock")
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestHostSharedIgnoresTheProcessTMPDIR$", "-test.count=1")
	nested := filepath.Join(realDir(t), "nested-tmp")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	child.Env = append(os.Environ(), "DISKSTORE_HOST_SHARED_HELPER=1", "TMPDIR="+nested)
	output, err := child.Output()
	if err != nil {
		t.Fatalf("helper: %v\n%s", err, output)
	}
	var theirs, theirTemp string
	for _, line := range strings.Split(string(output), "\n") {
		if value, ok := strings.CutPrefix(line, "shared="); ok {
			theirs = value
		}
		if value, ok := strings.CutPrefix(line, "temp="); ok {
			theirTemp = value
		}
	}
	if theirTemp == ProcessTempRoot() {
		t.Fatalf("the helper ran with this process's TMPDIR %s; the witness proves nothing", theirTemp)
	}
	if theirs != mine {
		t.Fatalf("shared host path differs: this process %s, a process with TMPDIR %s: %s", mine, theirTemp, theirs)
	}
	if _, err := HostShared("anything-else"); err == nil {
		t.Fatal("an unlisted shared host name was accepted")
	}
}

func TestNewIDSortsByTime(t *testing.T) {
	first, err := NewID(testNow, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewID(testNow.Add(time.Millisecond), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if !validID(first) || !validID(second) || first >= second {
		t.Fatalf("ids %s then %s", first, second)
	}
}
