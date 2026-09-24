package batch

import (
	"slices"
	"testing"

	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

type branchPolicyCalls struct {
	status, attestation, entries, message int
}

func strictBranchPolicyReaders(t *testing.T, request BranchReadRequest, status goalbranch.Status, unit, commit, fold string) (branchReaders, *branchPolicyCalls) {
	t.Helper()
	calls := &branchPolicyCalls{}
	readers := branchReaders{
		status: func(repo, endpointTip, tip, goalID string) (goalbranch.Status, error) {
			calls.status++
			if calls.status != 1 || repo != request.Repo || endpointTip != request.EndpointTip || tip != request.BranchTip || goalID != request.GoalID {
				t.Fatalf("unexpected status read: %q %q %q %q", repo, endpointTip, tip, goalID)
			}
			return status, nil
		},
		attestation: func(repo, snapshot, endpointTip, goalID, gotUnit, gotCommit string) (goalbranch.Attestation, error) {
			calls.attestation++
			if calls.attestation != 1 || repo != request.Repo || snapshot != request.BranchTip || endpointTip != request.EndpointTip || goalID != request.GoalID || gotUnit != unit || gotCommit != commit {
				t.Fatalf("unexpected attestation read: %q %q %q %q %q %q", repo, snapshot, endpointTip, goalID, gotUnit, gotCommit)
			}
			return goalbranch.Attestation{Source: goalbranch.AttestationSource{Kind: "critic-root"}}, nil
		},
		entries: func(repo, gotCommit string) ([]goalbranch.Entry, error) {
			calls.entries++
			if calls.entries != 1 || repo != request.Repo || gotCommit != fold {
				t.Fatalf("unexpected fold entries read: %q %q", repo, gotCommit)
			}
			return []goalbranch.Entry{{Path: "metasystem/records/reads/goal-policy/" + commit + ".json"}}, nil
		},
		message: func(repo, gotCommit string) ([]byte, error) {
			calls.message++
			if calls.message != 1 || repo != request.Repo || gotCommit != commit {
				t.Fatalf("unexpected message read: %q %q", repo, gotCommit)
			}
			return []byte("unit message\nCo-Authored-By: Policy Reader <reader@example.invalid>\n"), nil
		},
	}
	return readers, calls
}

func assertBranchPolicyCalls(t *testing.T, calls *branchPolicyCalls, want branchPolicyCalls) {
	t.Helper()
	if *calls != want {
		t.Fatalf("reader calls = %+v, want %+v", *calls, want)
	}
}

func TestBranchPolicyThroughSelectsOnlyReadCleanPrefix(t *testing.T) {
	t.Parallel()
	request := BranchReadRequest{Repo: t.TempDir(), EndpointTip: "endpoint-base", BranchTip: "branch-tip", GoalID: "goal-policy", Through: "U-prefix"}
	status := goalbranch.Status{
		Tip: request.BranchTip,
		Commits: []goalbranch.Commit{
			{ID: "U-prefix", Kind: goalbranch.Unit, Unit: "prefix", Units: []string{"prefix"}},
			{ID: "R-prefix", Kind: goalbranch.Read, Unit: "prefix"},
			{ID: "U-later", Kind: goalbranch.Unit, Unit: "later", Units: []string{"later"}},
			{ID: "R-later", Kind: goalbranch.Read, Unit: "later"},
			{ID: "Plan-later", Kind: goalbranch.Plan},
		},
		Units: []goalbranch.UnitStatus{
			{Unit: "prefix", Units: []string{"prefix"}, Commit: "U-prefix", Digest: "prefix-digest", ReadState: "read clean"},
			{Unit: "later", Units: []string{"later"}, Commit: "U-later", Digest: "later-digest", ReadState: "read clean"},
		},
		Prefix: 2,
	}
	readers, calls := strictBranchPolicyReaders(t, request, status, "prefix", "U-prefix", "R-prefix")
	member, err := readGoalBranchWithReaders(request, readers)
	if err != nil {
		t.Fatal(err)
	}
	assertBranchPolicyCalls(t, calls, branchPolicyCalls{1, 1, 1, 1})
	if member.GoalID != request.GoalID || member.Tip != request.BranchTip || member.Last || len(member.Builds) != 1 {
		t.Fatalf("through member = %+v", member)
	}
	build := member.Builds[0]
	if build.Commit != "U-prefix" || build.Digest != "prefix-digest" || !slices.Equal(build.Units, []string{"prefix"}) || len(build.Folds) != 1 || build.Folds[0].ID != "R-prefix" {
		t.Fatalf("through build = %+v", build)
	}
	if !slices.Equal(build.FoldPaths, []string{"metasystem/records/reads/goal-policy/U-prefix.json"}) || !slices.Equal(build.CoAuthors, []string{"Policy Reader <reader@example.invalid>"}) {
		t.Fatalf("fold paths and coauthors = %v, %v", build.FoldPaths, build.CoAuthors)
	}
	patches := map[string][]byte{
		"R-prefix": []byte("diff --git a/metasystem/records/reads/goal-policy/U-prefix.json b/metasystem/records/reads/goal-policy/U-prefix.json\nnew file mode 100644\n--- /dev/null\n+++ b/metasystem/records/reads/goal-policy/U-prefix.json\n@@ -0,0 +1 @@\n+read\n"),
		"U-prefix": []byte("diff --git a/metasystem/prefix/value.go b/metasystem/prefix/value.go\nnew file mode 100644\n--- /dev/null\n+++ b/metasystem/prefix/value.go\n@@ -0,0 +1 @@\n+package prefix\n"),
	}
	var requested []string
	patch, err := branchMemberPatchWithReader(request.Repo, member, func(repo, commit string) ([]byte, error) {
		if repo != request.Repo {
			t.Fatalf("unexpected patch repo %q", repo)
		}
		fact, ok := patches[commit]
		if !ok {
			t.Fatalf("unexpected patch commit %q", commit)
		}
		requested = append(requested, commit)
		return fact, nil
	})
	if err != nil || !slices.Equal(requested, []string{"R-prefix", "U-prefix"}) {
		t.Fatalf("patch order = %v, error = %v", requested, err)
	}
	paths := ChangedPaths(patch)
	if _, ok := paths["metasystem/prefix/value.go"]; !ok || len(paths) != 2 {
		t.Fatalf("prefix changed paths = %v", paths)
	}
	for _, path := range []string{"metasystem/later/value.go", "metasystem/plans/later.md"} {
		if _, ok := paths[path]; ok {
			t.Fatalf("through member includes later path %q: %v", path, paths)
		}
	}

	t.Run("through outside read-clean prefix", func(t *testing.T) {
		refused := request
		refused.Through = "U-later"
		status.Prefix = 1
		readers, calls := strictBranchPolicyReaders(t, refused, status, "prefix", "U-prefix", "R-prefix")
		_, err := readGoalBranchWithReaders(refused, readers)
		if err == nil || err.Error() != "BATCH_JOIN_UNREAD: through commit U-later is outside the read-clean prefix" {
			t.Fatalf("outside-prefix refusal = %v", err)
		}
		assertBranchPolicyCalls(t, calls, branchPolicyCalls{status: 1})
	})
}

func TestBranchPolicyLastUsesOnlyBranchCommits(t *testing.T) {
	t.Parallel()
	request := BranchReadRequest{Repo: t.TempDir(), EndpointTip: "endpoint-moved", BranchTip: "branch-tip", GoalID: "goal-policy", Last: true}
	status := goalbranch.Status{
		Tip: request.BranchTip,
		Commits: []goalbranch.Commit{
			{ID: "U-member", Kind: goalbranch.Unit, Unit: "member", Units: []string{"member"}},
			{ID: "R-member", Kind: goalbranch.Read, Unit: "member"},
		},
		Units:  []goalbranch.UnitStatus{{Unit: "member", Units: []string{"member"}, Commit: "U-member", Digest: "member-digest", ReadState: "read clean"}},
		Prefix: 1,
	}
	readers, calls := strictBranchPolicyReaders(t, request, status, "member", "U-member", "R-member")
	member, err := readGoalBranchWithReaders(request, readers)
	if err != nil {
		t.Fatal(err)
	}
	assertBranchPolicyCalls(t, calls, branchPolicyCalls{1, 1, 1, 1})
	if member.GoalID != request.GoalID || member.Tip != request.BranchTip || !member.Last || len(member.Builds) != 1 {
		t.Fatalf("last member = %+v", member)
	}
	build := member.Builds[0]
	if build.Commit != "U-member" || build.Digest != "member-digest" || !slices.Equal(build.Units, []string{"member"}) || len(build.Folds) != 1 || build.Folds[0].ID != "R-member" {
		t.Fatalf("last build = %+v", build)
	}
	patches := map[string][]byte{
		"R-member": []byte("diff --git a/metasystem/records/reads/goal-policy/U-member.json b/metasystem/records/reads/goal-policy/U-member.json\nnew file mode 100644\n--- /dev/null\n+++ b/metasystem/records/reads/goal-policy/U-member.json\n@@ -0,0 +1 @@\n+read\n"),
		"U-member": []byte("diff --git a/metasystem/member/value.go b/metasystem/member/value.go\nnew file mode 100644\n--- /dev/null\n+++ b/metasystem/member/value.go\n@@ -0,0 +1 @@\n+package member\n"),
	}
	var requested []string
	patch, err := branchMemberPatchWithReader(request.Repo, member, func(repo, commit string) ([]byte, error) {
		if repo != request.Repo {
			t.Fatalf("unexpected patch repo %q", repo)
		}
		fact, ok := patches[commit]
		if !ok {
			t.Fatalf("unexpected patch commit %q", commit)
		}
		requested = append(requested, commit)
		return fact, nil
	})
	if err != nil || !slices.Equal(requested, []string{"R-member", "U-member"}) {
		t.Fatalf("patch order = %v, error = %v", requested, err)
	}
	paths := ChangedPaths(patch)
	if _, ok := paths["metasystem/member/value.go"]; !ok || len(paths) != 2 {
		t.Fatalf("member changed paths = %v", paths)
	}
	for _, path := range []string{"metasystem/trunkgone/value.go", "metasystem/trunk-only.txt"} {
		if _, ok := paths[path]; ok {
			t.Fatalf("last member includes endpoint path %q: %v", path, paths)
		}
	}

	t.Run("last with unread branch tip", func(t *testing.T) {
		unread := status
		unread.Prefix = 0
		readers, calls := strictBranchPolicyReaders(t, request, unread, "member", "U-member", "R-member")
		_, err := readGoalBranchWithReaders(request, readers)
		if err == nil || err.Error() != "BATCH_JOIN_UNREAD: goal goal-policy is not read clean through its branch tip" {
			t.Fatalf("unread-tip refusal = %v", err)
		}
		assertBranchPolicyCalls(t, calls, branchPolicyCalls{status: 1})
	})
}
