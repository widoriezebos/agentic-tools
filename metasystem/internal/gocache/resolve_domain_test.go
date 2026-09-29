package gocache_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
)

// fakeEvidence answers every seam from fields; nothing reads git, records or
// the process table.
type fakeEvidence struct {
	delegate      bool
	delegateErr   error
	worktree      bool
	proofFound    bool
	proofAuth     bool
	proofPaths    gocache.Paths
	ancestors     map[string]bool
	calledProof   bool
	calledCustody bool
	roots         []string
}

func (f *fakeEvidence) evidence() gocache.Evidence {
	return gocache.Evidence{
		UserCacheDir: func() (string, error) { return "/machine/cache", nil },
		DelegateCustody: func(stateRoot, installationRoot, job string) (bool, error) {
			f.calledCustody = true
			return f.delegate, f.delegateErr
		},
		JobWorktree: func(installationRoot string) (bool, error) { return f.worktree, nil },
		ProofRecord: func(controlRoot, attempt string) (bool, bool, gocache.Paths, error) {
			f.calledProof = true
			return f.proofFound, f.proofAuth, f.proofPaths, nil
		},
		LiveAncestor:  func(ref string) (bool, error) { return f.ancestors[ref], nil },
		Self:          func() (string, error) { return "pid=7;micro=70", nil },
		EvidenceRoots: func() []string { return f.roots },
	}
}

var (
	enginePaths   = gocache.Paths{GoCache: "/machine/cache/go-build", StaticcheckCache: "/machine/cache/staticcheck"}
	delegatePaths = gocache.Paths{GoCache: "/machine/cache/metasystem-delegate-go-build", StaticcheckCache: "/machine/cache/metasystem-delegate-staticcheck"}
	markers       = []string{"METASYSTEM_HOOK_DELEGATE_STATE_ROOT=/state", "METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT=/install", "METASYSTEM_HOOK_DELEGATE_JOB=job-1"}
)

func contextFor(t *testing.T, domain gocache.Domain, paths gocache.Paths, issuer string) string {
	t.Helper()
	value, err := gocache.EncodeContext(gocache.Context{Domain: domain, GoCache: paths.GoCache, StaticcheckCache: paths.StaticcheckCache, Issuer: issuer})
	if err != nil {
		t.Fatal(err)
	}
	return gocache.ContextEnv + "=" + value
}

// The cache domain by the order of evidence (disk-lifetimes 3.1, A8).
func TestResolveDomainByTheOrderOfEvidence(t *testing.T) {
	t.Parallel()
	recorded := gocache.Paths{GoCache: "/run/recorded/go-build", StaticcheckCache: "/run/recorded/staticcheck"}
	for _, test := range []struct {
		name        string
		environment []string
		evidence    fakeEvidence
		want        gocache.Paths
		domain      gocache.Domain
		rule        string
		refused     string
	}{
		{name: "authenticated hook marker", environment: append([]string{"GOCACHE=/elsewhere"}, markers...),
			evidence: fakeEvidence{delegate: true}, want: delegatePaths, domain: gocache.DomainDelegate, rule: "delegate-custody"},
		{name: "marker whose ancestry does not authenticate", environment: markers,
			evidence: fakeEvidence{delegate: false}, refused: "METASYSTEM_HOOK_DELEGATE_JOB=job-1: the delegate context is not this process's"},
		{name: "incomplete marker", environment: markers[2:], refused: "incomplete delegate context"},
		{name: "stripped marker in a job worktree", environment: []string{"GOCACHE=/machine/cache/go-build"},
			evidence: fakeEvidence{worktree: true}, want: delegatePaths, domain: gocache.DomainDelegate, rule: "job-worktree"},
		{name: "stripped marker in a shared checkout", environment: []string{"GOCACHE=/machine/cache/metasystem-delegate-go-build"},
			want: enginePaths, domain: gocache.DomainEngine, rule: "outermost"},
		{name: "proof child bound to its record", environment: []string{"METASYSTEM_PROOF_CONTROL_ROOT=/control", "METASYSTEM_PROOF_ATTEMPT=proof-1", "GOCACHE=/forged"},
			evidence: fakeEvidence{proofFound: true, proofAuth: true, proofPaths: recorded}, want: recorded, domain: gocache.DomainEngine, rule: "proof-record"},
		{name: "forged locator naming a real record whose launcher is not an ancestor", environment: []string{"METASYSTEM_PROOF_CONTROL_ROOT=/control", "METASYSTEM_PROOF_ATTEMPT=proof-1"},
			evidence: fakeEvidence{proofFound: true, proofAuth: false}, refused: "METASYSTEM_PROOF_CONTROL_ROOT=/control: no scratch record of attempt proof-1 names a live ancestor of this process as its launcher"},
		{name: "locator naming no scratch record", environment: []string{"METASYSTEM_PROOF_CONTROL_ROOT=/control", "GOCACHE=/forged"},
			want: enginePaths, domain: gocache.DomainEngine, rule: "outermost"},
		{name: "authenticated context in a bed-shaped environment", environment: []string{"HOME=/bed/home", "GOCACHE=/forged", contextFor(t, gocache.DomainEngine, recorded, "pid=3;micro=30")},
			evidence: fakeEvidence{ancestors: map[string]bool{"pid=3;micro=30": true}}, want: recorded, domain: gocache.DomainEngine, rule: "context"},
		{name: "context carrying the delegate domain", environment: []string{contextFor(t, gocache.DomainDelegate, delegatePaths, "pid=3;micro=30")},
			evidence: fakeEvidence{ancestors: map[string]bool{"pid=3;micro=30": true}}, want: delegatePaths, domain: gocache.DomainDelegate, rule: "context"},
		{name: "context whose issuer is not an ancestor", environment: []string{contextFor(t, gocache.DomainEngine, recorded, "pid=4;micro=40")},
			refused: gocache.ContextEnv + ": issuer pid=4;micro=40 is not a live ancestor of this process"},
		{name: "malformed context", environment: []string{gocache.ContextEnv + "=domain=engine"}, refused: gocache.ContextEnv + ": malformed"},
		{name: "nothing at all", environment: []string{"GOCACHE=/seat/private", "STATICCHECK_CACHE=/seat/sc"},
			want: enginePaths, domain: gocache.DomainEngine, rule: "outermost"},
		{name: "relative inherited values", environment: []string{"GOCACHE=rel", "STATICCHECK_CACHE=./sc"},
			want: enginePaths, domain: gocache.DomainEngine, rule: "outermost"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			evidence := test.evidence
			got, err := gocache.ResolveDomain(test.environment, "/install", evidence.evidence())
			if test.refused != "" {
				if err == nil || !strings.Contains(err.Error(), test.refused) {
					t.Fatalf("got %+v, %v; want a refusal naming %q", got, err, test.refused)
				}
				var refusal gocache.Refusal
				if !errors.As(err, &refusal) {
					t.Fatalf("the refusal is not an input refusal: %T", err)
				}
				return
			}
			if err != nil || got.Paths != test.want || got.Domain != test.domain || got.Rule != test.rule {
				t.Fatalf("got %+v, %v; want %+v %s by %s", got, err, test.want, test.domain, test.rule)
			}
			// Every resolution issues a fresh context for the children,
			// with this process as issuer.
			decoded, err := gocache.DecodeContext(strings.TrimPrefix(got.ChildContext(), gocache.ContextEnv+"="))
			if err != nil || decoded.Issuer != "pid=7;micro=70" || decoded.GoCache != test.want.GoCache || decoded.Domain != test.domain {
				t.Fatalf("child context %q: %+v %v", got.ChildContext(), decoded, err)
			}
		})
	}
}

// A marker set is checked before the proof locators: an authenticated
// delegate never reads a scratch record.
func TestResolveDomainChecksDelegateCustodyFirst(t *testing.T) {
	t.Parallel()
	evidence := fakeEvidence{delegate: true, proofFound: true, proofAuth: true}
	got, err := gocache.ResolveDomain(append([]string{"METASYSTEM_PROOF_CONTROL_ROOT=/control"}, markers...), "/install", evidence.evidence())
	if err != nil || got.Domain != gocache.DomainDelegate || evidence.calledProof {
		t.Fatalf("got %+v %v, proof read %v", got, err, evidence.calledProof)
	}
}

// The child environment sets both paths and the context, replacing any
// inherited value.
func TestResolutionEnvironmentReplacesInheritedValues(t *testing.T) {
	t.Parallel()
	evidence := fakeEvidence{}
	got, err := gocache.ResolveDomain([]string{"GOCACHE=/old"}, "", evidence.evidence())
	if err != nil {
		t.Fatal(err)
	}
	environment := got.Apply([]string{"PATH=/bin", "GOCACHE=/old", "STATICCHECK_CACHE=/old-sc", gocache.ContextEnv + "=stale"})
	joined := strings.Join(environment, "\n")
	if strings.Contains(joined, "/old") || strings.Contains(joined, "=stale") || !strings.Contains(joined, "GOCACHE=/machine/cache/go-build") || !strings.Contains(joined, "PATH=/bin") {
		t.Fatalf("child environment = %v", environment)
	}
}

// U5i (engine-owns-disk-lifetimes Part B, 3.12 placement rules): a cache
// path under an evidence root is refused, naming the engine cache and the
// environment line that selects it; so is a GOMODCACHE the environment sets
// under one.
func TestResolveDomainRefusesACacheUnderAnEvidenceRoot(t *testing.T) {
	t.Parallel()
	under := gocache.Paths{GoCache: "/evidence/root/gocache-run", StaticcheckCache: "/evidence/root/staticcheck"}
	cases := []struct {
		name        string
		environment []string
		evidence    fakeEvidence
		want        string
	}{
		{name: "a proof record's cache", environment: []string{"METASYSTEM_PROOF_CONTROL_ROOT=/control", "METASYSTEM_PROOF_ATTEMPT=proof-1"},
			evidence: fakeEvidence{proofFound: true, proofAuth: true, proofPaths: under, roots: []string{"/evidence/root"}}, want: "GOCACHE=/evidence/root/gocache-run"},
		{name: "the machine cache itself", evidence: fakeEvidence{roots: []string{"/machine"}}, want: "GOCACHE=/machine/cache/go-build"},
		{name: "an inherited module cache", environment: []string{"GOMODCACHE=/evidence/root/mod"}, evidence: fakeEvidence{roots: []string{"/evidence/root/"}},
			want: "GOMODCACHE=/evidence/root/mod"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, err := gocache.ResolveDomain(c.environment, "", c.evidence.evidence())
			var refusal gocache.Refusal
			if !errors.As(err, &refusal) || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), "evidence root") ||
				!strings.Contains(err.Error(), enginePaths.GoCache) {
				t.Fatalf("refused naming %q, the evidence root and the engine cache: %v", c.want, err)
			}
		})
	}
	if _, err := gocache.ResolveDomain(nil, "", (&fakeEvidence{roots: []string{"/evidence/root"}}).evidence()); err != nil {
		t.Fatalf("a cache outside every root resolves: %v", err)
	}
}
