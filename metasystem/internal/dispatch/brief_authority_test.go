package dispatch

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBriefAuthorityRefusesMissingBasePathThenAdmitsCommittedPath(t *testing.T) {
	repo := newBriefAuthorityRepo(t)
	brief := writeBriefAuthorityFile(t, repo.root, "brief.md", "Working Mode: implement\nAuthority: records/two-bars/missing.md\n")

	err := briefAuthorityError(brief, repo.root, repo.facts)
	var refusal *BriefAuthorityRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("missing record error = %v, want typed BriefAuthorityRefusal", err)
	}
	if !reflect.DeepEqual(refusal.MissingPaths, []string{"records/two-bars/missing.md"}) ||
		!strings.Contains(err.Error(), "records/two-bars/missing.md") {
		t.Fatalf("missing record refusal = %+v, %q", refusal, err)
	}

	path := filepath.Join(repo.root, "records", "two-bars", "missing.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("landed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := briefAuthorityError(brief, repo.root, repo.facts); err == nil {
		t.Fatal("an uncommitted working-tree file was accepted as delegate base content")
	}
	repo.facts.commitPaths("records/two-bars/missing.md")
	if err := briefAuthorityError(brief, repo.root, repo.facts); err != nil {
		t.Fatalf("committed authority path refused: %v", err)
	}
}

func TestBriefAuthoritySkipsTemplatesAndChecksArtifactsOnDisk(t *testing.T) {
	repo := newBriefAuthorityRepo(t)
	artifact := filepath.Join(repo.root, "artifacts", "agents", "jobs", "live.json")
	if err := os.MkdirAll(filepath.Dir(artifact), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifact, []byte("runtime state\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	brief := writeBriefAuthorityFile(t, repo.root, "brief.md", strings.Join([]string{
		"Working Mode: implement",
		"Skip records/**/*.md records/<name>.md records/${name}.md.",
		"Read `artifacts/agents/jobs/live.json` and `artifacts/agents/jobs/absent.json`.",
	}, "\n"))

	err := briefAuthorityError(brief, repo.root, repo.facts)
	var refusal *BriefAuthorityRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("missing artifact error = %v, want typed BriefAuthorityRefusal", err)
	}
	if !reflect.DeepEqual(refusal.MissingPaths, []string{"artifacts/agents/jobs/absent.json"}) {
		t.Fatalf("artifact and template extraction = %v", refusal.MissingPaths)
	}

	missing := filepath.Join(repo.root, "artifacts", "agents", "jobs", "absent.json")
	if err := os.WriteFile(missing, []byte("runtime state\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := briefAuthorityError(brief, repo.root, repo.facts); err != nil {
		t.Fatalf("on-disk artifact paths refused: %v", err)
	}
}

func TestBriefAuthorityAdmitsBriefWithNoCandidatePaths(t *testing.T) {
	repo := newBriefAuthorityRepo(t)
	brief := writeBriefAuthorityFile(t, repo.root, "brief.md", "Working Mode: implement\nExplain the change without citing repository inputs.\n")
	if err := briefAuthorityError(brief, repo.root, repo.facts); err != nil {
		t.Fatalf("zero-candidate brief refused: %v", err)
	}
}

func TestBriefAuthorityTreatsMetasystemDiffBoundaryExampleAsPrefixOnly(t *testing.T) {
	repo := newBriefAuthorityRepo(t)
	member := "metasystem/internal/not-an-authority.go"
	for _, tc := range []struct{ name, content string }{
		{"headerless", "Working Mode: implement\ndiffBoundary example: [\"" + member + "\"]\n"},
		{"bounded-member", boundedAuthority([]string{member}, "diffBoundary example: [\""+member+"\"]")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			brief := writeBriefAuthorityFile(t, repo.root, "brief.md", tc.content)
			if err := briefAuthorityError(brief, repo.root, repo.facts); err != nil {
				t.Fatalf("repository-relative diffBoundary example was treated as cited authority: %v", err)
			}
		})
	}
}

func TestBriefAuthorityExemptsDeclaredOutputsUnlessTheyAreAlsoInputs(t *testing.T) {
	repo := newBriefAuthorityRepo(t)
	brief := writeBriefAuthorityFile(t, repo.root, "brief.md", strings.Join([]string{
		"Working Mode: implement",
		"# Workspace",
		"May-write: records/identity/epoch-drift-design.md",
		"May touch: records/identity/second-output.md",
		"Create `records/identity/third-output.md`.",
		"# Goal",
		"Produce the listed deliverables.",
	}, "\n"))
	if err := briefAuthorityError(brief, repo.root, repo.facts); err != nil {
		t.Fatalf("absent paths declared only as outputs were refused: %v", err)
	}

	brief = writeBriefAuthorityFile(t, repo.root, "brief.md", strings.Join([]string{
		"Working Mode: implement",
		"# Workspace",
		"May-write: records/identity/epoch-drift-design.md",
		"# Inputs",
		"Follow the authority in records/identity/epoch-drift-design.md.",
	}, "\n"))
	err := briefAuthorityError(brief, repo.root, repo.facts)
	var refusal *BriefAuthorityRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("path cited as both output and input error = %v, want typed BriefAuthorityRefusal", err)
	}
	if !reflect.DeepEqual(refusal.MissingPaths, []string{"records/identity/epoch-drift-design.md"}) {
		t.Fatalf("input citation did not win over output declaration: %v", refusal.MissingPaths)
	}
}

func TestBriefAuthorityBoundaryIsOutput(t *testing.T) {
	for _, tc := range []struct{ name, member string }{
		{"file", "docs/new.md"}, {"directory", "docs/new/"}, {"glob", "docs/*.md"}, {"special-name", "docs/a b.md"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireAuthorityInRepo(t, newBriefAuthorityRepo(t), boundedAuthority([]string{tc.member}), nil)
		})
	}
}

func TestBriefAuthorityIndentedBoundsExampleIsInert(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		missing    bool
	}{
		{"spaces", boundedAuthority([]string{}, ` Boundary: ["records/missing.md"]`), false},
		{"tabs", boundedAuthority([]string{}, "\tCeiling: records/missing.md"), false},
		{"headerless", "Working Mode: implement\n Boundary: records/missing.md", false},
		{"separate-input", boundedAuthority([]string{}, " Boundary: records/example.md", "Read records/missing.md"), true},
		{"quoted", boundedAuthority([]string{}, `> Boundary: ["records/missing.md"]`), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var want []string
			if tc.missing {
				want = []string{"records/missing.md"}
			}
			requireAuthorityInRepo(t, newBriefAuthorityRepo(t), tc.text, want)
		})
	}
}

func TestBriefAuthorityBoundedUsesTrunkTokenScanner(t *testing.T) {
	repo := newBriefAuthorityRepo(t)
	for _, tc := range []struct {
		name, line string
		want       []string
	}{
		{"backticked-path-line", "Read `docs/missing.md:426`.", []string{"docs/missing.md"}},
		{"command-with-arguments", "Run `scripts/agents/go-gate.sh --fast`.", []string{"scripts/agents/go-gate.sh"}},
		{"placeholder", "Read `records/<placeholder>.md`.", nil},
		{"glob", "Read `records/**/*.md`.", nil},
		{"quoted-sentence", `The note says "read docs/quoted-missing.md before continuing".`, []string{"docs/quoted-missing.md"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, boundary := range []struct {
				name    string
				members []string
			}{
				{"empty-boundary", []string{}},
				{"nonempty-boundary", []string{"docs/missing.md"}},
			} {
				t.Run(boundary.name, func(t *testing.T) {
					requireAuthorityInRepo(t, repo, boundedAuthority(boundary.members, tc.line), tc.want)
				})
			}
		})
	}
}

func TestBriefAuthorityBacktickBoundaryIdentity(t *testing.T) {
	member := "docs/a b.md"
	requireAuthorityInRepo(t, newBriefAuthorityRepo(t), boundedAuthority([]string{member}, "Read `"+member+"`."), []string{member})
}

func TestBriefAuthorityJSONStringBoundaryIdentity(t *testing.T) {
	member := "docs/a b.md"
	requireAuthorityInRepo(t, newBriefAuthorityRepo(t), boundedAuthority([]string{member}, "Read "+jsonString(member)+"."), []string{member})
}

func TestBriefAuthoritySpecialCharacterInput(t *testing.T) {
	for _, tc := range []struct{ name, member, citation string }{
		{"space", "docs/a b.md", jsonString("docs/a b.md")},
		{"comma", "docs/a,b.md", jsonString("docs/a,b.md")},
		{"quote", `docs/a"b.md`, jsonString(`docs/a"b.md`)},
		{"tab", "docs/a\tb.md", jsonString("docs/a\tb.md")},
		{"newline", "docs/a\nb.md", jsonString("docs/a\nb.md")},
		{"json-in-backticks", "docs/a b.md", "`" + jsonString("docs/a b.md") + "`"},
		{"literal-backticks", "docs/a b.md", "`docs/a b.md`"},
		{"consumed-span", "docs/a b.md", "`docs/a b.md`"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newBriefAuthorityRepo(t)
			line := "Read " + tc.citation + "."
			requireAuthorityInRepo(t, repo, boundedAuthority([]string{tc.member}, line), []string{tc.member})
			requireAuthorityInRepo(t, repo, boundedAuthority([]string{}, line), []string{"docs/a"})
			requireAuthorityInRepo(t, repo, boundedAuthority([]string{"docs/other path.md"}, line), []string{"docs/a"})
			commitBriefAuthorityPath(t, repo, tc.member)
			requireAuthorityInRepo(t, repo, boundedAuthority([]string{tc.member}, line), nil)
		})
	}
}

func TestBriefAuthorityNonConcreteBoundaryMembersUseTrunkScanner(t *testing.T) {
	repo := newBriefAuthorityRepo(t)
	commitBriefAuthorityPath(t, repo, "metasystem/internal/dispatch/brief.go")
	for _, tc := range []struct {
		name, member, line string
		want               []string
	}{
		{"directory", "metasystem/internal/u1biipkg/", "Put the package in `metasystem/internal/u1biipkg/`.", nil},
		{"glob-backtick", "metasystem/scripts/agents/*.sh", "Edit `metasystem/scripts/agents/*.sh` only.", nil},
		{"glob-json", "metasystem/scripts/agents/*.sh", `Edit "metasystem/scripts/agents/*.sh" only.`, nil},
		{"glob-absent-directory", "metasystem/internal/u1biipkg/*.go", "Add files matching `metasystem/internal/u1biipkg/*.go`.", nil},
		{"placeholder", "metasystem/records/misc/<goal>-code-read.md", "Write `metasystem/records/misc/<goal>-code-read.md`.", nil},
		{"path-line", "metasystem/internal/dispatch/brief.go:47-53", "See `metasystem/internal/dispatch/brief.go:47-53`.", nil},
		{"pattern-hides-missing", "metasystem/u1biix/* metasystem/internal/u1bii-missing.md", "Edit `metasystem/u1biix/* metasystem/internal/u1bii-missing.md`.", []string{"metasystem/internal/u1bii-missing.md"}},
		{"backslash", `metasystem/internal/a\b.txt`, "Read " + jsonString(`metasystem/internal/a\b.txt`) + ".", []string{"metasystem/internal/a"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireAuthorityInRepo(t, repo, boundedAuthority([]string{tc.member}, tc.line), tc.want)
		})
	}
}

func TestBriefAuthorityUnterminatedBacktickFallsBack(t *testing.T) {
	requireAuthorityInRepo(t, newBriefAuthorityRepo(t), boundedAuthority([]string{"docs/a b.md"}, "Read `docs/a b.md"), []string{"docs/a"})
}

func TestBriefAuthorityUnterminatedJSONStringFallsBack(t *testing.T) {
	requireAuthorityInRepo(t, newBriefAuthorityRepo(t), boundedAuthority([]string{"docs/a b.md"}, `Read "docs/a b.md`), []string{"docs/a"})
}

func TestBriefAuthorityInvalidJSONStringFallsBack(t *testing.T) {
	requireAuthorityInRepo(t, newBriefAuthorityRepo(t), boundedAuthority([]string{"docs/a\tb.md"}, "Read \"docs/a\tb.md\""), []string{"docs/a"})
}

func TestBriefAuthorityBoundaryInputStillRequired(t *testing.T) {
	for _, tc := range []struct {
		name, member string
		lines        []string
		want         []string
	}{
		{"ordinary", "", []string{"Read records/missing.md."}, []string{"records/missing.md"}},
		{"root-file", "NOTICE.md", []string{"Read `NOTICE.md`."}, []string{"NOTICE.md"}},
		{"new-directory", "new/file.md", []string{"Read `new/file.md`."}, []string{"new/file.md"}},
		{"pattern-no-equality", "new/*.md", []string{"Read `new/*.md`."}, nil},
		{"backslash-no-equality", `new/a\b.md`, []string{"Read " + jsonString(`new/a\b.md`) + "."}, nil},
		{"directory-no-equality", "new/file.md/", []string{"Read `new/file.md/`."}, nil},
		{"output-then-input", "new/file.md", []string{"# Workspace", "May-write: `new/file.md`.", "# Inputs", "Read `new/file.md`."}, []string{"new/file.md"}},
		{"input-then-output", "new/file.md", []string{"Read `new/file.md`.", "# Workspace", "May-write: `new/file.md`."}, []string{"new/file.md"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			boundary := []string{}
			if tc.member != "" {
				boundary = []string{tc.member}
			}
			requireAuthorityInRepo(t, newBriefAuthorityRepo(t), boundedAuthority(boundary, tc.lines...), tc.want)
		})
	}
}

func TestBriefAuthorityBoundedCitationCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name, content, artifact string
		want                    []string
	}{
		{"unquoted", boundedAuthority([]string{}, "Read docs/missing.md."), "", []string{"docs/missing.md"}},
		{"workspace", boundedAuthority([]string{"records/a b.md"}, "# Workspace", `May-write: "records/a b.md".`), "", nil},
		{"create", boundedAuthority([]string{"records/a b.md"}, `Create "records/a b.md".`), "", nil},
		{"artifact", boundedAuthority([]string{"artifacts/a b.json"}, `Read "artifacts/a b.json".`), "artifacts/a b.json", nil},
		{"headerless", "Working Mode: implement\nRead \"docs/a b.md\".", "", []string{"docs/a"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newBriefAuthorityRepo(t)
			if tc.artifact != "" {
				path := filepath.Join(repo.root, filepath.FromSlash(tc.artifact))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("runtime state\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			requireAuthorityInRepo(t, repo, tc.content, tc.want)
		})
	}
}

func TestBriefAuthorityRealBriefRegression(t *testing.T) {
	repo := newBriefAuthorityRepo(t)
	// These inputs are cited by the example brief; the snapshot is independent
	// of the authority scanner's output.
	repo.facts.commitPaths(
		"metasystem/plans/landing-receipt-survives-records-drift-design.md",
		"metasystem/records/misc/landing-receipt-survives-records-drift-critique-r1.md",
		"metasystem/records/misc/landing-receipt-survives-records-drift-critique-r2.md",
		"metasystem/internal/gittree/detached_test.go",
		"metasystem/internal/gittree/detached.go",
		"metasystem/internal/gittree/snapshotscope.go",
		"metasystem/internal/landing/receipt.go",
		"metasystem/internal/landing/testing.go",
		"metasystem/internal/landing/receipt_test.go",
		"metasystem/internal/landing/observe.go",
		"metasystem/internal/landing/registers.go",
		"metasystem/internal/landing/registers_test.go",
		"metasystem/internal/landing/drift.go",
		"metasystem/internal/landing/drift_test.go",
		"metasystem/internal/landing/advance.go",
		"metasystem/internal/landing/advance_test.go",
		"metasystem/internal/lease/lease.go",
		"metasystem/internal/behaviorsurface/policy.v2.json",
		"metasystem/internal/behaviorsurface/policy_test.go",
		"metasystem/internal/behaviorsurface/consumer_wiring_test.go",
		"metasystem/internal/refusal/register.go",
		"metasystem/cmd/metasystem/landing_verbs.go",
		"metasystem/cmd/metasystem/main.go",
		"metasystem/scripts/agents/static-reproof-fixtures.sh",
		"metasystem/scripts/agents/land.sh",
		"metasystem/scripts/agents/land-fixtures.sh",
		"metasystem/scripts/agents/go-gate.sh",
		"scripts/agents/dispatch-fixtures.sh",
		"scripts/agents/go-gate.sh",
		"scripts/agents/goal-cli-fixtures.sh",
		"scripts/agents/land-fixtures.sh",
		"records/narrator-digest.log",
	)
	for _, tc := range []struct {
		name     string
		boundary []string
	}{
		{"empty-boundary", []string{}},
		{"nonempty-boundary", []string{"metasystem/internal/landing/receipt.go", "metasystem/internal/landing/testing.go"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, _ := json.Marshal(tc.boundary)
			brief := writeBriefAuthorityFile(t, t.TempDir(), "brief.md", realBriefFixture+"\nBoundary: "+string(encoded)+"\nCeiling: 1\n")
			if err := briefAuthorityError(brief, repo.root, repo.facts); err != nil {
				t.Fatalf("real implementer brief changed authority result: %v", err)
			}
		})
	}
}

const realBriefFixture = "Working Mode: implement\nOrchestrator Identity: m1e (lineage main-1789030447-51011-5722fc, coordinator under goal landing-receipt-survives-records-drift)\nDate: 2026-09-10\n\n# Build brief: carry the certified receipt-drift change forward onto current main\n\n## What this round is\n\nGoal landing-receipt-survives-records-drift was built and certified on another\nseat as chain lrsrd-build1: three rounds, round 2 read clean by an independent\ncritic (zero material findings), round 3 changed two lines of the land-fixtures\nbed, the five Go packages and the land-fixtures bed green on that tree. Its base\nwas commit 4b0b777a. It never landed, because an unrelated bed was red on main.\nSince then main landed 1b12f534 (\"Make testing risk-selected and retain proof for\nlanding\"), which rewrote receipt code the chain also changes.\n\nYour round produces, on current main, the behaviour the certified diff produced\non its base, reconciled with what 1b12f534 landed. The certified diff is the\nfile `/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/36c93128-ec16-4146-8def-3f706ba4ef10/scratchpad/lrsrd/round3.diff.patch`\n(sha256 begins f420254bbe098c76; 86918 bytes; 23 files). A second copy is\n`/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/metasystem/artifacts/agents/lrsrd-build1/rounds/3/diff.patch`.\nRead the whole diff before touching anything. If neither path is readable from\nyour sandbox, stop and report exactly that; do not rebuild the change from the\npage alone.\n\n## The specification, and how to read it\n\n`metasystem/plans/landing-receipt-survives-records-drift-design.md` revision 3\nis the contract; its critique ladder closed after two reads\n(`metasystem/records/misc/landing-receipt-survives-records-drift-critique-r1.md`,\n`metasystem/records/misc/landing-receipt-survives-records-drift-critique-r2.md`).\nRead \"Implementation map\" first, then Decisions 2, 3 and 4 and \"Fixtures\".\nWhere the certified diff and the page disagree, the page wins. Where the page\nand 1b12f534's landed code disagree, the reconciliation rule below wins, and\nyour return says where you applied it.\n\n## What a three-way apply shows (evidence level: ran, by the coordinator)\n\n`git apply --3way` of the certified diff onto current main: 21 files apply\ncleanly, two conflict.\n\n- `metasystem/internal/gittree/detached_test.go`: both sides added it. Main's\n  version (from 1b12f534) tests `NewDetachedWorktree`; the chain's version\n  tests `NewDetachedCommitWorktree`, `Rebase` and `ResetKeep`. Keep both test\n  sets whole: a union, no test dropped or renamed. `detached.go` itself applied\n  cleanly: the chain's functions add to main's, nothing overlaps.\n- `metasystem/internal/landing/receipt.go`: three hunks. The import block; the\n  `TestReceipt` struct, where main's schema 2 carries `Proof`, `Coverage`,\n  `ProvedTree`, `AttemptIDs` and `Testing` while the chain's schema 2 carries\n  `WorktreeProjection`; and the version rules inside `readTestReceipt`.\n\nWhat the coordinator read in main's current code (evidence level: read it):\n\n- `receiptPosture` at `metasystem/internal/landing/receipt.go:426` is the\n  legacy (command) receipt's posture read and is raw: index from the real\n  index, working tree from `Snapshot(\"HEAD\")`. This is the read Decision 2\n  filters.\n- `testingReceiptPosture` at `metasystem/internal/landing/testing.go:160` is\n  the testing-contract receipt's posture read: the index is raw, the working\n  tree is `SnapshotRelevant(candidate, paths)` over the contract's input\n  manifests, so an append to a register that no group lists as input already\n  leaves that read unchanged.\n- Both readers use `DisallowUnknownFields` (`receipt.go:210`, `:459`), so a\n  receipt field this engine adds is refused by the engine that does not know\n  it. The page's crossover procedure (Decision 4, last paragraphs) already\n  accounts for that direction.\n\n## The reconciliation rule (the coordinator's decision; the closing read judges it)\n\n1. 1b12f534's receipt schema 2, the testing-contract receipt with `Proof`,\n   `Coverage`, `ProvedTree`, `AttemptIDs` and `Testing`, is landed and stays\n   authoritative: it keeps its number, its fields, its reader rules and every\n   test 1b12f534 landed for it. The page's projection object cannot also be\n   \"schema 2\".\n2. The page's version rule is kept with its numbers shifted, not its meaning:\n   a receipt is read by the rule its writer required. Version 1 (legacy\n   command receipt) and version 2 (testing-contract receipt) keep exactly the\n   comparisons main performs today. The filtered projection of Decision 4\n   (`worktreeProjection` with `excludes` and `tree`) travels on a new version\n   number, 3, minted by this engine for the legacy command path, and the\n   reader applies Decision 4's rules 2 and 3 to it (excludes must equal\n   `appendOnlyRegisters`; worktree bindings must equal\n   `receiptIdentity(candidate)`; index bindings exact; a version-3 receipt\n   without the projection and a version-1 or version-2 receipt with it are\n   the mixed forms and refuse with the page's message). If you find that the\n   testing-contract path also needs the projection to satisfy the goal's DONE\n   line, add it there under the same version discipline and say so; do not\n   change what any receipt already on disk means.\n3. Posture reads. The legacy read `receiptPosture` gets the `FilterTree`\n   exclusion over `appendOnlyRegisters` exactly as the certified diff wrote\n   it. The testing read `testingReceiptPosture` keeps its index read exact\n   and its relevance-narrowed working-tree read; the page's Decision 2 says\n   the index is exact in every version, and Decision 3's `advance` verb and\n   land.sh edits are what keep a register append out of the index. Do not\n   filter the index anywhere.\n4. Nothing 1b12f534 landed gets weaker: no test deleted, no proof rule\n   relaxed, `DisallowUnknownFields` stays on both readers.\n\n## What you build, by the page's sections\n\nApply the certified diff to your worktree (three-way), resolve the two\nconflicts under the rule above, then make the tree what the page describes,\nin the map's order, each step leaving the fast gate green:\n\n1. `metasystem/internal/gittree/snapshotscope.go`: `Workspace.Status()` and\n   `StatusEntry`, `IsAncestor`, `ResetKeep`, `ResetKeepResult`;\n   `metasystem/internal/gittree/detached.go`: `NewDetachedCommitWorktree`,\n   `Rebase`, `RebaseResult`; their tests, unioned with main's as said above.\n2. `metasystem/internal/lease/lease.go`: `LockPath`, one exported line.\n3. Decision 1's register set in a new file `internal/landing/registers.go`\n   with `internal/landing/registers_test.go`, and the switch in\n   `metasystem/internal/landing/observe.go` (today near line 793) reading it.\n4. `metasystem/internal/landing/receipt.go`: `receiptPosture` filtered,\n   `receiptIdentity`, the version-3 form and Decision 4's reader rules under\n   the reconciliation; `metasystem/internal/landing/receipt_test.go` with the\n   page's receipt fixtures, retargeted to version 3 where the page says 2.\n5. The drift verb in a new file `internal/landing/drift.go` with\n   `internal/landing/drift_test.go`; the `landing drift` verb in\n   `metasystem/cmd/metasystem/landing_verbs.go` and its registry line in\n   `metasystem/cmd/metasystem/main.go`.\n6. The advance verb in a new file `internal/landing/advance.go` with\n   `internal/landing/advance_test.go`; the verb and its registry line; the\n   eight hand-written rows in `metasystem/internal/refusal/register.go`.\n7. `metasystem/internal/behaviorsurface/policy.v2.json`, the row in\n   `metasystem/internal/behaviorsurface/policy_test.go`,\n   `metasystem/internal/behaviorsurface/consumer_wiring_test.go`, the three\n   case lists in `metasystem/scripts/agents/static-reproof-fixtures.sh`.\n8. `metasystem/scripts/agents/land.sh`: the three edits of Decision 3b,\n   re-fitted to the land.sh that 1b12f534 left (it now consumes\n   `metasystem test verify` and `--test-receipt` on tier-1 too); keep every\n   message land.sh prints today.\n9. `metasystem/scripts/agents/land-fixtures.sh`: the seed, the second\n   candidate and its receipt, the three shell canaries of \"Fixtures\", as the\n   certified round 3 wrote them. You cannot run this bed in your sandbox;\n   write it and stop.\n\n## Canaries first, and red on the untouched tree\n\nThe Go passing canary `TestReadTestReceiptSurvivesRegisterAppendAfterReceipt`\nis written first and run on the UNTOUCHED tree; it must fail with the text\nmain prints today for a moved working tree. Record the observed text at\nevidence level `ran`. A canary that passes before the repair is a finding,\nnot a shortcut: stop and report it, because it would mean 1b12f534 already\ncovers that receipt form and the round's scope changes. The refusal canary\n`TestReadTestReceiptRefusesNonRegisterDrift` must pass before and after.\n\nAdd one canary the page could not know: a testing-contract (version-2)\nreceipt followed by an append to `records/narrator-digest.log`, then read; it\nmust be accepted unchanged, proving rule 1 of the reconciliation holds.\n\nThen the proving runs:\n\n```sh\ncd metasystem && go test ./internal/landing/ -run 'TestCreateTestReceipt|TestReadTestReceipt|TestWorktreeDrift|TestAdvance|TestAppendOnlyRegisters' -count=1 -timeout=5m\ngo test ./internal/landing/ ./internal/gittree/ ./internal/lease/ ./internal/behaviorsurface/ ./internal/refusal/ -count=1 -timeout=10m\nscripts/agents/go-gate.sh --fast\n```\n\nYour sandbox has no network egress and cannot write under the home directory.\nThe previous job on this goal made the fast gate pass by pointing Go at the\nlocal module cache (`GOPROXY=file://` at the module cache) and giving\nstaticcheck a writable cache directory under the scratch root; do the same\nrather than reporting the gate as unrunnable. The shell beds\n(`land-fixtures.sh`, `dispatch-fixtures.sh`, `goal-cli-fixtures.sh`) create\nrepositories the sandbox forbids; the orchestrator runs them on your tree.\nDo not weaken anything to make them runnable, and do not treat their absence\nas evidence.\n\n## Boundary: the certified diff's files are the wall\n\nYour `diffBoundary` is exactly the 23 files the certified diff touches, plus\n`metasystem/internal/landing/testing.go` only if the reconciliation forces a\nchange there, named in the return with the reason. No goal record, no plans\npage, no `AGENTS.md`, no other script. The receipt's index tree and candidate\ntree stay exact; only the working-tree projection is filtered. The\nappend-only rule in `observe.go` keeps its behaviour. If a step cannot be done\nwithout crossing this wall, stop and say which.\n\n## Standing rules\n\n- No round, slice, chain or finding references in source comments (no\n  `lrsrd`, no `LRD-`, no `revision 3`, no `1b12f534`). Comments describe the\n  application: what a version means, what a register is, why the index is\n  exact. The comment text the page prints for helpers is fine as printed.\n- Keep the page's exact strings: refusal codes, the drift verb's output\n  grammar, land.sh's existing messages, the receipt's field names.\n- The gap rule: at a specification gap, stop adding, keep what is built and\n  green, and report the gap with the resolution you propose. Never delete\n  passing work to return a clean tree. A numeric or naming contradiction the\n  page's own text settles is resolved and reported, not a stop.\n\n## Return\n\nPer the implementer schema. Evidence rows at level `ran` for: the passing\ncanary's observed fail-before text; the new version-2 canary; the proving run\ngreen; the package runs; the fast gate. Name every reconciliation decision\nyou took under the rule above and where in the code it lives, so the closing\nread can find each one. `riskiestPart` names what you are least sure of.\nReturn no later than 100 minutes after start with what is built and green,\nnamed as partial if unfinished; the round's wall-clock cap is 120 minutes.\n\n# Required full gate\n\nscripts/agents/go-gate.sh --fast && scripts/agents/land-fixtures.sh && scripts/agents/dispatch-fixtures.sh && scripts/agents/goal-cli-fixtures.sh\n\n# Return path form\n\nEvery path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.\n\n# Required testing contract\n\nUse the committed shared testing contract for goal landing-receipt-survives-records-drift (recorded gate width full). Compute the actual whole-project candidate, run the risk-selected public `metasystem test` plan through its admitted proof owner, collect every independent result in each entered stage, and require `test verify` to report sufficient delivery evidence before returning. A failed canary blocks every later stage; do not substitute a fixed full battery or erase the recorded risk.\n"

func boundedAuthority(boundary []string, lines ...string) string {
	encoded, _ := json.Marshal(boundary)
	return strings.Join(append([]string{"Working Mode: implement", "Boundary: " + string(encoded), "Ceiling: 1"}, lines...), "\n")
}

func jsonString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func commitBriefAuthorityPath(t *testing.T, repo briefAuthorityFixture, name string) {
	t.Helper()
	path := filepath.Join(repo.root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("landed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.facts.commitPaths(name)
}

func briefAuthorityError(brief, root string, facts briefTreeFacts) error {
	_, err := readBriefAdmissionAtRootWithFacts(brief, root, root, root, false, facts)
	return err
}

func requireAuthorityInRepo(t *testing.T, repo briefAuthorityFixture, content string, want []string) {
	t.Helper()
	err := briefAuthorityError(writeBriefAuthorityFile(t, repo.root, "brief.md", content), repo.root, repo.facts)
	var refusal *BriefAuthorityRefusal
	if want == nil && err != nil || want != nil && (!errors.As(err, &refusal) || !reflect.DeepEqual(refusal.MissingPaths, want)) {
		t.Fatalf("authority error = %#v, want missing paths %q", err, want)
	}
}

func writeBriefAuthorityFile(t *testing.T, directory, name, content string) string {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
