package branch

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	publicationEndpointRef = "refs/heads/main"
	publicationLandingRef  = "refs/heads/landing/goal-a"
	publicationFetchedRef  = "refs/metasystem/goals/landing-push/goal-a"
)

var (
	publicationBase      = strings.Repeat("1", 40)
	publicationCandidate = strings.Repeat("2", 40)
	publicationLanding   = strings.Repeat("3", 40)
	publicationIntruder  = strings.Repeat("4", 40)
)

type publicationCall struct {
	operation string
	args      []string
}

func publicationExpect(operation string, args ...string) publicationCall {
	return publicationCall{operation: operation, args: args}
}

type publicationFixture struct {
	t                *testing.T
	repo             string
	prepared         string
	remote           map[string]string
	fetched          map[string]string
	want             []publicationCall
	checks           int
	claimErr         error
	unknown          bool
	unknownPublished bool
}

func newPublicationFixture(t *testing.T) *publicationFixture {
	t.Helper()
	repo := t.TempDir()
	prepared := filepath.Join(t.TempDir(), "prepared")
	if err := os.Mkdir(prepared, 0o755); err != nil {
		t.Fatal(err)
	}
	trunk := fmt.Sprintf("endpoint=%s\ncandidate=%s\nlanding=%s\nbranch=landing/goal-a\n",
		publicationBase, publicationCandidate, publicationLanding)
	if err := os.WriteFile(filepath.Join(prepared, "trunk"), []byte(trunk), 0o644); err != nil {
		t.Fatal(err)
	}
	return &publicationFixture{t: t, repo: repo, prepared: prepared,
		remote:  map[string]string{publicationEndpointRef: publicationBase, publicationLandingRef: publicationLanding},
		fetched: map[string]string{}}
}

func (f *publicationFixture) expect(calls ...publicationCall) { f.want = append(f.want, calls...) }

func (f *publicationFixture) take(operation string, args ...string) {
	f.t.Helper()
	got := publicationExpect(operation, args...)
	if len(f.want) == 0 || !reflect.DeepEqual(got, f.want[0]) {
		f.t.Fatalf("publication call = %+v, next expected = %+v", got, f.want)
	}
	f.want = f.want[1:]
}

func (f *publicationFixture) done(endpoint, landing string, checks int) {
	f.t.Helper()
	if len(f.want) != 0 {
		f.t.Fatalf("unconsumed publication calls: %+v", f.want)
	}
	if f.remote[publicationEndpointRef] != endpoint || f.remote[publicationLandingRef] != landing || f.checks != checks || len(f.fetched) != 0 {
		f.t.Fatalf("endpoint=%q landing=%q claims=%d fetched=%v; want %q %q %d and no fetched ref",
			f.remote[publicationEndpointRef], f.remote[publicationLandingRef], f.checks, f.fetched, endpoint, landing, checks)
	}
}

func (f *publicationFixture) request() LandPushRequest {
	return LandPushRequest{Repo: f.repo, Remote: "origin", EndpointRef: publicationEndpointRef,
		GoalID: "goal-a", Prepared: f.prepared, CheckClaim: func() error {
			f.checks++
			f.take("claim", fmt.Sprint(f.checks))
			if f.checks == 2 {
				return f.claimErr
			}
			return nil
		}}
}

func (f *publicationFixture) repository() landPushRepository {
	return landPushRepository{
		RemoteTip: func(repo, remote, ref string) (string, bool, error) {
			f.take("remote-tip", repo, remote, ref)
			tip, ok := f.remote[ref]
			return tip, ok, nil
		},
		Fetch: func(repo, remote, ref, destination string) error {
			f.take("fetch", repo, remote, ref, destination)
			tip, ok := f.remote[ref]
			if !ok {
				f.t.Fatalf("fetch of absent remote ref %s", ref)
			}
			f.fetched[destination] = tip
			return nil
		},
		Ancestor: func(repo, older, newer string) error {
			f.take("ancestor", repo, older, newer)
			if older != publicationBase || newer != publicationLanding || f.fetched[publicationFetchedRef] != publicationLanding {
				f.t.Fatal("ancestry checked without the fetched prepared landing")
			}
			return nil
		},
		Clear: func(repo, ref string) error {
			f.take("clear", repo, ref)
			delete(f.fetched, ref)
			return nil
		},
		Publish: func(req LandPushRequest, prepared PreparedLanding) (CASOutcome, error) {
			f.take("atomic-publish", req.Repo, req.Remote, req.EndpointRef, req.GoalID, req.Prepared,
				prepared.Endpoint, prepared.Candidate, prepared.Landing, prepared.Branch,
				"--force-with-lease="+req.EndpointRef+":"+prepared.Endpoint,
				"--force-with-lease="+publicationLandingRef+":"+prepared.Landing,
				prepared.Landing+":"+req.EndpointRef, ":"+publicationLandingRef)
			if f.remote[publicationEndpointRef] != prepared.Endpoint || f.remote[publicationLandingRef] != prepared.Landing {
				return CASRefused, errors.New("atomic lease refused")
			}
			if f.unknown {
				if f.unknownPublished {
					f.remote[publicationEndpointRef] = prepared.Landing
					delete(f.remote, publicationLandingRef)
				}
				return CASUnknown, errors.New("connection closed after push")
			}
			f.remote[publicationEndpointRef] = prepared.Landing
			delete(f.remote, publicationLandingRef)
			return CASLanded, nil
		},
	}
}

func (f *publicationFixture) begin() {
	f.expect(publicationExpect("claim", "1"),
		publicationExpect("remote-tip", f.repo, "origin", publicationEndpointRef),
		publicationExpect("remote-tip", f.repo, "origin", publicationLandingRef))
}

func (f *publicationFixture) afterRead() {
	f.expect(publicationExpect("fetch", f.repo, "origin", publicationLandingRef, publicationFetchedRef),
		publicationExpect("ancestor", f.repo, publicationBase, publicationLanding))
}

func (f *publicationFixture) push() {
	f.expect(publicationExpect("atomic-publish", f.repo, "origin", publicationEndpointRef, "goal-a", f.prepared,
		publicationBase, publicationCandidate, publicationLanding, "landing/goal-a",
		"--force-with-lease="+publicationEndpointRef+":"+publicationBase,
		"--force-with-lease="+publicationLandingRef+":"+publicationLanding,
		publicationLanding+":"+publicationEndpointRef, ":"+publicationLandingRef))
}

func (f *publicationFixture) clear() {
	f.expect(publicationExpect("clear", f.repo, publicationFetchedRef))
}

func requirePublicationCode(t *testing.T, err error, code string) {
	t.Helper()
	var refusal *OpError
	if !errors.As(err, &refusal) || refusal.Code != code {
		t.Fatalf("publication refusal=%v, want %s", err, code)
	}
}

func TestGoalLandingPublicationAndVerification(t *testing.T) {
	t.Parallel()
	t.Run("whole series", func(t *testing.T) {
		t.Parallel()
		f := newPublicationFixture(t)
		f.begin()
		f.afterRead()
		f.expect(publicationExpect("claim", "2"))
		f.push()
		f.clear()
		result, err := landPushWithRepository(f.request(), f.repository())
		if err != nil || result.Landing != publicationLanding {
			t.Fatalf("land push=%+v err=%v", result, err)
		}
		f.done(publicationLanding, "", 2)
		verifyPublicationSeries(t, f.repo)
	})

	for _, test := range []struct{ name, code, movedRef string }{
		{"endpoint moved", LandTrunkMovedCode, publicationEndpointRef},
		{"landing moved", LandBranchMovedCode, publicationLandingRef},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			f := newPublicationFixture(t)
			f.begin()
			f.afterRead()
			f.expect(publicationExpect("hook", test.movedRef), publicationExpect("claim", "2"))
			f.push()
			f.expect(publicationExpect("remote-tip", f.repo, "origin", publicationEndpointRef),
				publicationExpect("remote-tip", f.repo, "origin", publicationLandingRef))
			f.clear()
			req := f.request()
			req.Hooks.AfterRemoteRead = func() error {
				f.take("hook", test.movedRef)
				f.remote[test.movedRef] = publicationIntruder
				return nil
			}
			_, err := landPushWithRepository(req, f.repository())
			requirePublicationCode(t, err, test.code)
			if test.movedRef == publicationEndpointRef {
				f.done(publicationIntruder, publicationLanding, 2)
			} else {
				f.done(publicationBase, publicationIntruder, 2)
			}
		})
	}
}

func TestLandPushRerunAfterPublishedCrashReportsLanded(t *testing.T) {
	t.Parallel()
	f := newPublicationFixture(t)
	f.remote[publicationEndpointRef] = publicationLanding
	delete(f.remote, publicationLandingRef)
	f.begin()
	result, err := landPushWithRepository(f.request(), f.repository())
	want := PreparedLanding{Endpoint: publicationBase, Candidate: publicationCandidate,
		Landing: publicationLanding, Branch: "landing/goal-a"}
	if err != nil || result != want {
		t.Fatalf("published rerun = %+v, %v; want %+v", result, err, want)
	}
	f.done(publicationLanding, "", 1)
}

func TestLandPushRechecksClaimBeforePush(t *testing.T) {
	t.Parallel()
	f := newPublicationFixture(t)
	f.claimErr = errors.New("claim moved before push")
	f.begin()
	f.afterRead()
	f.expect(publicationExpect("claim", "2"))
	f.clear()
	_, err := landPushWithRepository(f.request(), f.repository())
	requirePublicationCode(t, err, NotHolderCode)
	f.done(publicationBase, publicationLanding, 2)
}

func TestLandPushUnknownOutcomeReconcilesRemoteState(t *testing.T) {
	t.Parallel()
	for _, published := range []bool{true, false} {
		published := published
		t.Run(fmt.Sprint(published), func(t *testing.T) {
			t.Parallel()
			f := newPublicationFixture(t)
			f.unknown, f.unknownPublished = true, published
			f.begin()
			f.afterRead()
			f.expect(publicationExpect("claim", "2"))
			f.push()
			f.expect(publicationExpect("remote-tip", f.repo, "origin", publicationEndpointRef),
				publicationExpect("remote-tip", f.repo, "origin", publicationLandingRef))
			f.clear()
			result, err := landPushWithRepository(f.request(), f.repository())
			if published {
				if err != nil || result.Landing != publicationLanding {
					t.Fatalf("published unknown = %+v, %v", result, err)
				}
				f.done(publicationLanding, "", 2)
			} else {
				requirePublicationCode(t, err, PushUnknownCode)
				f.done(publicationBase, publicationLanding, 2)
			}
		})
	}
}

type publicationGitCall struct {
	args []string
	out  []byte
}

func verifyPublicationSeries(t *testing.T, repo string) {
	t.Helper()
	commits := []string{strings.Repeat("a", 40), strings.Repeat("b", 40), publicationLanding}
	base := publicationBase
	parents := map[string]string{commits[0]: base, commits[1]: commits[0], commits[2]: commits[1]}
	diffs := map[string][]byte{}
	trailers := map[string][]byte{}
	want := []publicationGitCall{}
	add := func(out []byte, args ...string) { want = append(want, publicationGitCall{args, out}) }
	for i, commit := range commits {
		included := fmt.Sprintf(":100644 100644 %s %s M%cmetasystem/file%d.go%c",
			strings.Repeat("0", 40), strings.Repeat(string('5'+rune(i)), 40), 0, i+1, 0)
		excluded := fmt.Sprintf(":100644 100644 %s %s M%cmetasystem/memory/receipts.log%c",
			strings.Repeat("0", 40), strings.Repeat("9", 40), 0, 0)
		folded := fmt.Sprintf(":100644 100644 %s %s M%cmetasystem/folded.go%c",
			strings.Repeat("0", 40), strings.Repeat("8", 40), 0, 0)
		diffs[commit] = []byte(included + excluded + folded)
		sum := sha256.Sum256([]byte(included))
		trailers[commit] = []byte(fmt.Sprintf("Goal-Unit: goal-a/u%d\nGoal-Digest: %s\nGoal-Fold: metasystem/folded.go\n",
			i+1, hex.EncodeToString(sum[:])))
	}
	manifest := func(commit string) {
		add(trailers[commit], "show", "-s", "--format=%(trailers:only,unfold=true)", commit)
	}
	parent := func(commit string) { add([]byte(parents[commit]+"\n"), "rev-parse", commit+"^") }
	manifest(commits[2])
	for i := 2; i >= 0; i-- {
		manifest(commits[i])
		parent(commits[i])
	}
	manifest(base)
	for _, commit := range commits {
		manifest(commit)
		add([]byte(commit+" "+parents[commit]+"\n"), "rev-list", "--parents", "-n", "1", commit)
		add(diffs[commit], "diff-tree", "-r", "-z", "--no-renames", "--full-index", commit+"^", commit)
	}
	read := func(gotRepo string, args ...string) ([]byte, error) {
		if gotRepo != repo || len(want) == 0 || !reflect.DeepEqual(args, want[0].args) {
			t.Fatalf("verification read repo=%q args=%q; next=%+v", gotRepo, args, want)
		}
		out := want[0].out
		want = want[1:]
		return out, nil
	}
	verified, err := verifyLandedSeriesWithGit(repo, commits[2], read)
	if err != nil || len(verified) != 3 {
		t.Fatalf("verified=%+v err=%v", verified, err)
	}
	for i, item := range verified {
		if item.Commit != commits[i] || item.Goal != "goal-a" || item.Units != fmt.Sprintf("u%d", i+1) || item.Actual != item.Expected {
			t.Fatalf("verification order or digest mismatch at %d: %+v", i, item)
		}
	}
	if len(want) != 0 {
		t.Fatalf("unconsumed verification reads: %+v", want)
	}
	first := commits[0]
	mutated := append([]byte(nil), diffs[first]...)
	oldBlob := strings.Repeat("5", 40)
	newBlob := strings.Repeat("f", 40)
	mutated = []byte(strings.Replace(string(mutated), oldBlob, newBlob, 1))
	manifest(first)
	add([]byte(first+" "+parents[first]+"\n"), "rev-list", "--parents", "-n", "1", first)
	add(mutated, "diff-tree", "-r", "-z", "--no-renames", "--full-index", first+"^", first)
	_, err = verifyLandedWithGit(repo, first, read)
	requirePublicationCode(t, err, LandVerifyCode)
	if len(want) != 0 {
		t.Fatalf("unconsumed mutated verification reads: %+v", want)
	}
}
