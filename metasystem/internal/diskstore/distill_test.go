package diskstore

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const testMiB = 1 << 20

// distillBed is a bundle with the four members U5c names: a nested .git
// with a unique commit, a Mach-O executable, a file identical to one in a
// repository outside the bundle, and a 2 MiB log; plus an untouched small
// file.
type distillBed struct {
	root, bundle, repository string
	blobs                    BlobStore
	original                 map[string]string
}

func newDistillBed(t *testing.T) distillBed {
	t.Helper()
	root := realDir(t)
	bed := distillBed{root: root, bundle: filepath.Join(root, "checkout", "suite-failures", "20260926T101010Z-detached-section-proof-mtviifgb-af048a24c818711d-1"),
		repository: filepath.Join(root, "repository"), blobs: BlobStore{Dir: filepath.Join(root, "home", "metasystem-evidence", ".blobs")}}
	macho := append([]byte("\xcf\xfa\xed\xfe"), bytes.Repeat([]byte("engine"), testMiB/4)...)
	shared := []byte("the same text as the repository's README\n")
	files := map[string][]byte{
		"source-001-tmp/work/.git/HEAD":                         []byte("ref: refs/heads/main\n"),
		"source-001-tmp/work/.git/objects/ab/cdef0123456789":    []byte("a unique commit's loose object"),
		"source-001-tmp/work/.git/refs/heads/main":              []byte("abcdef0123456789\n"),
		"source-001-tmp/work/README":                            shared,
		"source-002-bin/metasystem":                             macho,
		"source-003-log/run.log":                                bytes.Repeat([]byte("a log line of a failing run\n"), 2*testMiB/28+1),
		"source-003-log/small.txt":                              []byte("small and untouched"),
		"source-003-log/nested/deeper/only-a-large-file.log":    bytes.Repeat([]byte("z"), testMiB+1),
		"source-001-tmp/work/.git/objects/pack/pack-empty.pack": {},
	}
	for rel, data := range files {
		writeBedFile(t, filepath.Join(bed.bundle, filepath.FromSlash(rel)), data)
	}
	writeBedFile(t, filepath.Join(bed.repository, "README"), shared)
	bed.original = treeFiles(t, bed.bundle)
	return bed
}

func writeBedFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// rules are the bed's distillation rules: the repository knows README.
func (bed distillBed) rules(stage string) DistillRules {
	return DistillRules{CompressAbove: testMiB, Blobs: bed.blobs, Referrer: "107e72c67539-" + filepath.Base(bed.bundle),
		Installation: filepath.Join(bed.root, "checkout"), Segment: "107e72c67539", Stage: stage,
		Known: func(_ context.Context, paths []string) (map[string]bool, error) {
			known := map[string]bool{}
			want, err := os.ReadFile(filepath.Join(bed.repository, "README"))
			if err != nil {
				return known, nil
			}
			for _, path := range paths {
				if data, err := os.ReadFile(path); err == nil && bytes.Equal(data, want) {
					known[path] = true
				}
			}
			return known, nil
		}}
}

// treeFiles maps every regular file under root to its bytes.
func treeFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files[filepath.ToSlash(strings.TrimPrefix(path, root+"/"))] = string(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// restoreBundle rebuilds the bundle's original files from its members, its
// recipe and the blob store, as a person restoring evidence would.
func restoreBundle(t *testing.T, bundle string, blobs BlobStore) map[string]string {
	t.Helper()
	_, lines, _, err := ReadDistilled(bundle)
	if err != nil {
		t.Fatal(err)
	}
	restored := treeFiles(t, bundle)
	delete(restored, DistilledName)
	delete(restored, OwnerFileName)
	for _, line := range lines {
		switch line.Kind {
		case RecipeGzip:
			data := restored[line.Replacement]
			delete(restored, line.Replacement)
			reader, err := gzip.NewReader(strings.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			plain, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			restored[line.Path] = string(plain)
		case RecipeBlob:
			data, err := os.ReadFile(blobs.Path(line.SHA256))
			if err != nil {
				t.Fatalf("blob of %s: %v", line.Path, err)
			}
			restored[line.Path] = string(data)
		case RecipeGit:
			data := restored[line.Replacement]
			delete(restored, line.Replacement)
			reader, err := gzip.NewReader(strings.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			archive := tar.NewReader(reader)
			for {
				header, err := archive.Next()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				if header.Typeflag == tar.TypeReg {
					plain, err := io.ReadAll(archive)
					if err != nil {
						t.Fatal(err)
					}
					restored[line.Path+"/"+strings.TrimPrefix(header.Name, "./")] = string(plain)
				}
			}
		}
	}
	return restored
}

func sameFiles(t *testing.T, got, want map[string]string) {
	t.Helper()
	var problems []string
	for rel, data := range want {
		if got[rel] != data {
			problems = append(problems, fmt.Sprintf("%s differs (%d bytes, want %d)", rel, len(got[rel]), len(data)))
		}
	}
	for rel := range got {
		if _, ok := want[rel]; !ok {
			problems = append(problems, "unexpected "+rel)
		}
	}
	sort.Strings(problems)
	if len(problems) > 0 {
		t.Fatalf("restoration is not byte-for-byte:\n%s", strings.Join(problems, "\n"))
	}
}

func TestDistillReplacesWithDurableRecipesAndRestoresWithoutTheRepository(t *testing.T) {
	t.Parallel()
	bed := newDistillBed(t)
	result, err := Distill(context.Background(), bed.bundle, bed.rules("01STAGE0000000000000000001"), testNow)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, line := range result.Replaced {
		kinds[line.Path] = line.Kind
	}
	want := map[string]string{
		"source-001-tmp/work/.git":                           RecipeGit,
		"source-001-tmp/work/README":                         RecipeBlob,
		"source-002-bin/metasystem":                          RecipeBlob,
		"source-003-log/run.log":                             RecipeGzip,
		"source-003-log/nested/deeper/only-a-large-file.log": RecipeGzip,
	}
	for path, kind := range want {
		if kinds[path] != kind {
			t.Errorf("%s: recipe %q, want %q (all: %v)", path, kinds[path], kind, kinds)
		}
	}
	if len(kinds) != len(want) {
		t.Errorf("replaced %v, want exactly %v", kinds, want)
	}
	if _, err := os.Stat(filepath.Join(bed.bundle, "source-003-log", "small.txt")); err != nil {
		t.Fatalf("a small file must stay as it is: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bed.bundle, "source-001-tmp", "work", ".git")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the nested .git must be packed, not left: %v", err)
	}
	for _, line := range result.Replaced {
		if line.Kind != RecipeBlob {
			continue
		}
		refs, err := bed.blobs.Refs(line.SHA256)
		if err != nil || len(refs) != 1 || refs[0].Referrer != "107e72c67539-"+filepath.Base(bed.bundle) ||
			refs[0].Recipe != filepath.Join(bed.bundle, DistilledName) {
			t.Fatalf("blob %s of %s must carry one reference naming the bundle's recipe: %+v %v", line.SHA256, line.Path, refs, err)
		}
	}
	// The repository the duplicate matched goes away; the recipe does not
	// depend on it (DL2-03).
	if err := os.RemoveAll(bed.repository); err != nil {
		t.Fatal(err)
	}
	sameFiles(t, restoreBundle(t, bed.bundle, bed.blobs), bed.original)

	before := treeFiles(t, bed.root)
	again, err := Distill(context.Background(), bed.bundle, bed.rules("01STAGE0000000000000000002"), testNow)
	if err != nil || again.Changed() {
		t.Fatalf("a second run must be a no-op: %+v %v", again, err)
	}
	sameFiles(t, treeFiles(t, bed.root), before)
}

func TestDistillResumesFromEveryDestructiveBoundary(t *testing.T) {
	t.Parallel()
	for _, step := range []string{"staged", "referenced", "published", "renamed"} {
		for _, target := range []string{"source-001-tmp/work/.git", "source-001-tmp/work/README", "source-003-log/run.log"} {
			t.Run(step+"/"+target, func(t *testing.T) {
				t.Parallel()
				bed := newDistillBed(t)
				rules := bed.rules("01STAGE0000000000000000001")
				stopped := false
				rules.interrupt = func(at, path string) bool {
					if at == step && path == target && !stopped {
						stopped = true
						return true
					}
					return false
				}
				_, err := Distill(context.Background(), bed.bundle, rules, testNow)
				if step == "referenced" && !strings.HasSuffix(target, "README") {
					if err != nil {
						t.Fatalf("no reference step for %s: %v", target, err)
					}
				} else if !errors.Is(err, errInterrupted) {
					t.Fatalf("the interruption at %s did not stop the run: %v", step, err)
				}
				// Every original is still restorable at the crash point.
				resumed, err := Distill(context.Background(), bed.bundle, bed.rules("01STAGE0000000000000000002"), testNow)
				if err != nil {
					t.Fatalf("resume: %v", err)
				}
				if len(resumed.Kept) != 0 {
					t.Fatalf("resume kept %v", resumed.Kept)
				}
				sameFiles(t, restoreBundle(t, bed.bundle, bed.blobs), bed.original)
				filepath.WalkDir(bed.bundle, func(path string, entry fs.DirEntry, err error) error {
					if err == nil && IsPartial(entry.Name()) {
						t.Errorf("a stage is left in the bundle after the resume: %s", path)
					}
					return nil
				})
			})
		}
	}
}

func TestDistillRemovesOnlyItsListedStagesAndKeepsACollision(t *testing.T) {
	t.Parallel()
	bed := newDistillBed(t)
	// A stage the manifest lists (the distiller died after staging) is its
	// own and goes; a partial-named file nothing lists is captured content.
	rules := bed.rules("01STAGE0000000000000000000")
	rules.interrupt = func(step, path string) bool { return step == "staged" && path == "source-001-tmp/work/.git" }
	if _, err := Distill(context.Background(), bed.bundle, rules, testNow); !errors.Is(err, errInterrupted) {
		t.Fatal(err)
	}
	listed := filepath.Join(bed.bundle, "source-001-tmp", "work", ".git.tar.gz"+PartialSuffix+"01STAGE0000000000000000000")
	if _, err := os.Stat(listed); err != nil {
		t.Fatalf("setup: the listed stage exists: %v", err)
	}
	orphan := filepath.Join(bed.bundle, "source-003-log", "run.log.gz"+PartialSuffix+"01DEAD000000000000000000000")
	writeBedFile(t, orphan, []byte("a partial-named file nothing lists"))
	foreign := filepath.Join(bed.bundle, "source-003-log", "nested", "deeper", "only-a-large-file.log.gz")
	writeBedFile(t, foreign, []byte("a member the bundle arrived with"))
	result, err := Distill(context.Background(), bed.bundle, bed.rules("01STAGE0000000000000000001"), testNow)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(listed); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a listed stage of a transaction that never published is removed: %v", err)
	}
	if data, err := os.ReadFile(orphan); err != nil || string(data) != "a partial-named file nothing lists" {
		t.Fatalf("an unlisted partial-named file is evidence and stays: %q %v", data, err)
	}
	if len(result.Discarded) != 1 {
		t.Fatalf("discarded %v, want the one listed stage", result.Discarded)
	}
	large := filepath.Join(bed.bundle, "source-003-log", "nested", "deeper", "only-a-large-file.log")
	if _, err := os.Stat(large); err != nil {
		t.Fatalf("the file whose replacement name is taken must be kept: %v", err)
	}
	if data, _ := os.ReadFile(foreign); string(data) != "a member the bundle arrived with" {
		t.Fatalf("a foreign member is never overwritten, got %q", data)
	}
	_, lines, _, err := ReadDistilled(bed.bundle)
	if err != nil {
		t.Fatal(err)
	}
	collisions := 0
	for _, line := range lines {
		if line.Kind == RecipeCollision && line.Path == "source-003-log/nested/deeper/only-a-large-file.log" {
			collisions++
		}
	}
	if collisions != 1 {
		t.Fatalf("the collision must be recorded once in the manifest: %+v", lines)
	}
	if again, err := Distill(context.Background(), bed.bundle, bed.rules("01STAGE0000000000000000002"), testNow); err != nil || again.Changed() {
		t.Fatalf("the collision is recorded once; a repeat changes nothing: %+v %v", again, err)
	}
}

func TestTwoBundlesUnderTwoRootsShareOneBlobWithTwoReferences(t *testing.T) {
	t.Parallel()
	root := realDir(t)
	blobs := BlobStore{Dir: filepath.Join(root, "metasystem-evidence", ".blobs")}
	engine := append([]byte("\x7fELF"), bytes.Repeat([]byte("x"), testMiB)...)
	var digest string
	for index, checkout := range []string{"one", "two"} {
		bundle := filepath.Join(root, checkout, "suite-failures", "20260901T101010Z-watchdog-standalone-1")
		writeBedFile(t, filepath.Join(bundle, "bin", "engine"), engine)
		rules := DistillRules{CompressAbove: testMiB, Blobs: blobs, Referrer: checkout + "-" + filepath.Base(bundle), Stage: fmt.Sprintf("01STAGE%019d", index)}
		result, err := Distill(context.Background(), bundle, rules, testNow)
		if err != nil || len(result.Replaced) != 1 || result.Replaced[0].Kind != RecipeBlob {
			t.Fatalf("%s: %+v %v", checkout, result, err)
		}
		digest = result.Replaced[0].SHA256
	}
	refs, err := blobs.Refs(digest)
	if err != nil || len(refs) != 2 {
		t.Fatalf("one blob, two references: %+v %v", refs, err)
	}
	entries, _ := os.ReadDir(blobs.Dir)
	count := 0
	for _, entry := range entries {
		if validDigest(entry.Name()) {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("the shared engine is stored once, found %d blobs", count)
	}
}

func TestDistillWaitsWhileTheReferenceCheckHoldsTheStore(t *testing.T) {
	t.Parallel()
	bed := newDistillBed(t)
	release, err := bed.blobs.TryExclusive()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := Distill(context.Background(), bed.bundle, bed.rules("01STAGE0000000000000000001"), testNow); !errors.Is(err, ErrBlobStoreBusy) {
		t.Fatalf("a held exclusive .gc.flock makes the distiller pending, got %v", err)
	}
	sameFiles(t, treeFiles(t, bed.bundle), bed.original)
}

// moveBed is a distilled bundle and its segment directory.
func moveBed(t *testing.T) (distillBed, MoveRules) {
	t.Helper()
	bed := newDistillBed(t)
	if _, err := Distill(context.Background(), bed.bundle, bed.rules("01STAGE0000000000000000001"), testNow); err != nil {
		t.Fatal(err)
	}
	// The evidence root exists (a move never creates it).
	if err := os.MkdirAll(filepath.Join(bed.root, "evidence"), 0o755); err != nil {
		t.Fatal(err)
	}
	rules := MoveRules{SegmentDir: filepath.Join(bed.root, "evidence", "suite-failures", "107e72c67539"), Blobs: bed.blobs,
		Referrer: "107e72c67539-" + filepath.Base(bed.bundle), Segment: "107e72c67539", Stage: "01MOVE00000000000000000001"}
	return bed, rules
}

func TestMoveCopiesVerifiesAndCarriesTheReferences(t *testing.T) {
	t.Parallel()
	bed, rules := moveBed(t)
	distilled := treeFiles(t, bed.bundle)
	result, err := MoveBundle(context.Background(), bed.bundle, rules)
	if err != nil || !result.Moved {
		t.Fatalf("%+v %v", result, err)
	}
	if _, err := os.Stat(bed.bundle); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the source goes once its copy verified: %v", err)
	}
	sameFiles(t, treeFiles(t, result.Destination), distilled)
	sameFiles(t, restoreBundle(t, result.Destination, bed.blobs), bed.original)
	_, lines, _, _ := ReadDistilled(result.Destination)
	for _, line := range lines {
		if line.Kind != RecipeBlob {
			continue
		}
		refs, err := bed.blobs.Refs(line.SHA256)
		if err != nil || len(refs) != 1 || refs[0].Recipe != filepath.Join(result.Destination, DistilledName) {
			t.Fatalf("the reference must resolve to the moved recipe: %+v %v", refs, err)
		}
	}
	entries, _ := os.ReadDir(filepath.Dir(bed.bundle))
	if len(entries) != 0 {
		t.Fatalf("nothing of the source may remain: %v", entries)
	}
}

func TestMoveResumesAfterAnInterruptedCopyAndAfterTheRename(t *testing.T) {
	t.Parallel()
	for _, step := range []string{"copied", "renamed", "set-aside"} {
		t.Run(step, func(t *testing.T) {
			t.Parallel()
			bed, rules := moveBed(t)
			distilled := treeFiles(t, bed.bundle)
			stopping := rules
			stopping.interrupt = func(at string) bool { return at == step }
			if _, err := MoveBundle(context.Background(), bed.bundle, stopping); !errors.Is(err, errInterrupted) {
				t.Fatalf("not interrupted at %s: %v", step, err)
			}
			if step == "set-aside" {
				// The source was set aside; the checkout pass finishes it.
				entries, _ := os.ReadDir(filepath.Dir(bed.bundle))
				if len(entries) != 1 {
					t.Fatalf("one set-aside source, got %v", entries)
				}
				name, ok := MovedSource(entries[0].Name())
				if !ok || name != filepath.Base(bed.bundle) {
					t.Fatalf("the set-aside source must name its bundle: %s", entries[0].Name())
				}
				sameFiles(t, treeFiles(t, filepath.Join(rules.SegmentDir, name)), distilled)
				return
			}
			rules.Stage = "01MOVE00000000000000000002"
			result, err := MoveBundle(context.Background(), bed.bundle, rules)
			if err != nil {
				t.Fatal(err)
			}
			if step == "copied" && (len(result.Discarded) != 1 || !result.Moved) {
				t.Fatalf("the interrupted copy is discarded and redone: %+v", result)
			}
			if step == "renamed" && !result.Equal {
				t.Fatalf("an equal destination removes the source: %+v", result)
			}
			sameFiles(t, treeFiles(t, result.Destination), distilled)
			if _, err := os.Stat(bed.bundle); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("source left behind: %v", err)
			}
		})
	}
}

func TestMoveKeepsASourceWhoseDestinationDiffers(t *testing.T) {
	t.Parallel()
	bed, rules := moveBed(t)
	writeBedFile(t, filepath.Join(rules.SegmentDir, filepath.Base(bed.bundle), "other"), []byte("someone else's bundle"))
	result, err := MoveBundle(context.Background(), bed.bundle, rules)
	if err != nil || result.Kept == "" || result.Moved {
		t.Fatalf("a differing destination keeps the source: %+v %v", result, err)
	}
	if _, err := os.Stat(bed.bundle); err != nil {
		t.Fatalf("the source must stay: %v", err)
	}
}

func suiteBed(t *testing.T) (distillBed, SuiteFailures, *Pass) {
	t.Helper()
	bed := newDistillBed(t)
	class := SuiteFailures{Dir: filepath.Dir(bed.bundle), SegmentDir: filepath.Join(bed.root, "evidence", "suite-failures", "107e72c67539"),
		Installation: filepath.Join(bed.root, "checkout"), GitRoot: filepath.Join(bed.root, "checkout"), Blobs: bed.blobs,
		CompressAbove: testMiB, DistillAfter: 24 * time.Hour, MoveAfter: 7 * 24 * time.Hour, Target: 2 << 30, Entropy: rand.Reader,
		Facts: func(context.Context) (CheckoutFacts, error) {
			return CheckoutFacts{RootCommit: "e83c5163316f89bfbde7d9ab23ca2e25604af290", LedgerIdentity: "01J9LEDGER0000000000000000"}, nil
		},
		AttemptGoal: func(attempt string) (string, bool) {
			if attempt == "proof-mtviifgb-af048a24c818711d" {
				return "engine-owns-disk-lifetimes", true
			}
			return "", false
		}}
	old := testNow.Add(-48 * time.Hour)
	filepath.WalkDir(bed.bundle, func(path string, _ fs.DirEntry, err error) error {
		if err == nil {
			os.Chtimes(path, old, old)
		}
		return nil
	})
	return bed, class, &Pass{Now: testNow, Mode: ModeApply}
}

func TestSuiteFailuresDistilAtIdleThenMoveAtAgeWithTheLegacyOwner(t *testing.T) {
	t.Parallel()
	bed, class, pass := suiteBed(t)
	young := &Pass{Now: testNow.Add(-47 * time.Hour), Mode: ModeApply}
	items, err := class.Plan(context.Background(), young)
	if err != nil || len(items) != 1 || items[0].Verdict.Decision != Wait {
		t.Fatalf("a bundle idle less than a day waits: %+v %v", items, err)
	}
	items, err = class.Plan(context.Background(), pass)
	if err != nil || len(items) != 1 || items[0].Verdict.Decision != Release || !strings.HasPrefix(items[0].Key, actionDistil) {
		t.Fatalf("a bundle idle a day is due to distil: %+v %v", items, err)
	}
	if verdict := class.Apply(context.Background(), pass, items[0]); verdict.Decision != Release {
		t.Fatalf("distil: %+v", verdict)
	}
	owner, err := ReadBundleOwner(bed.bundle)
	if err != nil || owner.Attempt != "proof-mtviifgb-af048a24c818711d" || owner.Goal != "engine-owns-disk-lifetimes" ||
		owner.LedgerIdentity != "01J9LEDGER0000000000000000" || owner.RootCommit == "" {
		t.Fatalf("a legacy bundle whose attempt record exists gains its owner at distillation: %+v %v", owner, err)
	}
	items, _ = class.Plan(context.Background(), pass)
	if len(items) != 1 || items[0].Verdict.Decision != Wait {
		t.Fatalf("a distilled bundle younger than seven days by its creation stamp waits: %+v", items)
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Dir(class.SegmentDir)), 0o755); err != nil {
		t.Fatal(err)
	}
	later := &Pass{Now: time.Date(2026, 10, 3, 10, 10, 10, 0, time.UTC), Mode: ModeApply}
	items, _ = class.Plan(context.Background(), later)
	if len(items) != 1 || !strings.HasPrefix(items[0].Key, actionMove) || items[0].Verdict.Decision != Release {
		t.Fatalf("seven days after its creation stamp the bundle is due to move: %+v", items)
	}
	if verdict := class.Apply(context.Background(), later, items[0]); verdict.Decision != Release {
		t.Fatalf("move: %+v", verdict)
	}
	moved := filepath.Join(class.SegmentDir, filepath.Base(bed.bundle))
	if _, err := ReadBundleOwner(moved); err != nil {
		t.Fatalf("the move carries OWNER.json: %v", err)
	}
	sameFiles(t, restoreBundle(t, moved, bed.blobs), withOwner(bed.original, nil))
}

// withOwner is the original files; the owner file is not part of the
// restoration comparison.
func withOwner(files map[string]string, _ []string) map[string]string { return files }

func TestSuiteFailuresLegacyOwnerIsUnknownWithoutItsAttemptRecord(t *testing.T) {
	t.Parallel()
	bed, class, pass := suiteBed(t)
	class.AttemptGoal = func(string) (string, bool) { return "", false }
	items, _ := class.Plan(context.Background(), pass)
	class.Apply(context.Background(), pass, items[0])
	owner, err := ReadBundleOwner(bed.bundle)
	if err != nil || owner.Goal != GoalUnknown {
		t.Fatalf("a legacy bundle whose record is gone is goal unknown: %+v %v", owner, err)
	}
}

func TestSuiteFailuresFloorLowersTheAgesButNeverToZero(t *testing.T) {
	t.Parallel()
	_, class, _ := suiteBed(t)
	floor := &Pass{Now: testNow.Add(-46 * time.Hour), Mode: ModeApply, Floor: true, FloorMinAge: time.Hour}
	items, _ := class.Plan(context.Background(), floor)
	if len(items) != 1 || items[0].Verdict.Decision != Release {
		t.Fatalf("below the floor a bundle idle an hour is distilled: %+v", items)
	}
	justNow := &Pass{Now: testNow.Add(-48*time.Hour + 30*time.Minute), Mode: ModeApply, Floor: true, FloorMinAge: time.Hour}
	items, _ = class.Plan(context.Background(), justNow)
	if len(items) != 1 || items[0].Verdict.Decision != Wait {
		t.Fatalf("the floor's minimum age still holds: %+v", items)
	}
}

func TestSuiteFailuresWithoutAnEvidenceRootMoveNothing(t *testing.T) {
	t.Parallel()
	bed, class, _ := suiteBed(t)
	class.SegmentDir = ""
	pass := &Pass{Now: testNow, Mode: ModeApply}
	items, _ := class.Plan(context.Background(), pass)
	class.Apply(context.Background(), pass, items[0])
	later := &Pass{Now: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), Mode: ModeApply}
	items, _ = class.Plan(context.Background(), later)
	if len(items) != 1 || items[0].Verdict.Decision != Pending {
		t.Fatalf("an unknown evidence root leaves the bundle pending: %+v", items)
	}
	if _, err := os.Stat(bed.bundle); err != nil {
		t.Fatal(err)
	}
}
