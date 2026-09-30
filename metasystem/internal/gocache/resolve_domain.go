package gocache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ContextEnv carries the authenticated cache context (disk-lifetimes 3.1,
// rule 4): the domain and the two paths an engine resolved, and the exact
// identity of that engine, the issuer. A child that inherits it takes the
// paths from it only while the issuer is a live ancestor with that exact
// identity, so a replaced HOME (a test namespace, a bed) never computes a
// private cache, and a context copied into an unrelated process is refused.
const ContextEnv = "METASYSTEM_CACHE_CONTEXT"

// The delegate markers the adapters set in a delegate round's environment.
const (
	delegateStateRootEnv    = "METASYSTEM_HOOK_DELEGATE_STATE_ROOT"
	delegateInstallationEnv = "METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT"
	delegateJobEnv          = "METASYSTEM_HOOK_DELEGATE_JOB"
	proofControlRootEnv     = "METASYSTEM_PROOF_CONTROL_ROOT"
	proofAttemptEnv         = "METASYSTEM_PROOF_ATTEMPT"
)

// Context is the decoded cache context.
type Context struct {
	Domain           Domain `json:"domain"`
	GoCache          string `json:"gocache"`
	StaticcheckCache string `json:"staticcheck"`
	Issuer           string `json:"issuer"`
}

// EncodeContext renders a context as one line.
func EncodeContext(context Context) (string, error) {
	if err := context.valid(); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(context)
	return string(encoded), err
}

// DecodeContext parses one context line.
func DecodeContext(value string) (Context, error) {
	var context Context
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&context); err != nil || decoder.More() {
		return Context{}, errors.New("malformed")
	}
	if err := context.valid(); err != nil {
		return Context{}, err
	}
	return context, nil
}

func (c Context) valid() error {
	if _, err := ParseDomain(string(c.Domain)); err != nil {
		return errors.New("malformed: " + err.Error())
	}
	if !filepath.IsAbs(c.GoCache) || !filepath.IsAbs(c.StaticcheckCache) || c.Issuer == "" {
		return errors.New("malformed: it needs two absolute paths and an issuer")
	}
	return nil
}

// Evidence is what the domain is decided from; every seam is required. The
// production composition authenticates against records and the kernel's
// process ancestry (internal/cachedomain); tests answer from fields.
type Evidence struct {
	// UserCacheDir is the real user's cache dir, for the outermost process.
	UserCacheDir func() (string, error)
	// DelegateCustody reports whether this process descends from the exact
	// recorded CLI custody of the named job.
	DelegateCustody func(stateRoot, installationRoot, job string) (bool, error)
	// JobWorktree reports whether installationRoot is a job worktree the
	// dispatcher made, named by its job record.
	JobWorktree func(installationRoot string) (bool, error)
	// ProofRecord finds the scratch record of a proof attempt under a
	// control root: whether one exists, whether its launcher or a recorded
	// custodian is a live ancestor with that exact identity, and the cache
	// pair it recorded (empty for a run that recorded none).
	ProofRecord func(controlRoot, attempt string) (found, authenticated bool, paths Paths, err error)
	// LiveAncestor reports whether an encoded process reference is this
	// process or a live ancestor with that exact identity.
	LiveAncestor func(ref string) (bool, error)
	// Self is this process's encoded reference: the issuer of the context
	// its children get.
	Self func() (string, error)
	// EvidenceRoots lists the host's evidence roots: no cache may lie
	// under one (engine-owns-disk-lifetimes 3.12 placement rules).
	EvidenceRoots func() []string
}

// Resolution is the decided domain, its paths, the rule that decided it,
// and this process's reference as the issuer of its children's context.
type Resolution struct {
	Domain Domain
	Paths  Paths
	Rule   string
	issuer string
}

// Refusal is an input refusal: a marker, locator or context that does not
// authenticate. It never falls through with its paths.
type Refusal struct{ reason string }

func (r Refusal) Error() string {
	return "this process's Go cache was refused: " + r.reason + "\nrun: metasystem system check"
}

// ResolveDomain decides this process's cache domain by the order of evidence
// (disk-lifetimes 3.1, A8), the first that holds:
//  1. the delegate markers name a job whose recorded CLI custody this
//     process descends from: delegate (markers that do not authenticate
//     are refused, never ignored);
//  2. the installation root is a dispatcher-made job worktree: delegate,
//     whatever the environment says;
//  3. a proof locator names a scratch record whose launcher or custodian is
//     a live ancestor: engine, with the paths the record carries (a record
//     that does not authenticate is refused; a locator naming no scratch
//     record is no evidence at all);
//  4. an authenticated cache context: its domain and paths (a foreign or
//     malformed context is refused);
//  5. otherwise the outermost process: engine, computed from the user cache
//     dir, every inherited GOCACHE and STATICCHECK_CACHE discarded.
//
// No rule reads GOCACHE or STATICCHECK_CACHE from the environment.
func ResolveDomain(environment []string, installationRoot string, evidence Evidence) (Resolution, error) {
	resolution, err := resolveDomain(environment, installationRoot, evidence)
	if err != nil {
		return resolution, err
	}
	if refusal := underEvidenceRoot(environment, resolution, evidence); refusal != nil {
		return Resolution{}, refusal
	}
	return resolution, nil
}

// underEvidenceRoot refuses a resolved cache, or a GOMODCACHE the
// environment sets, that lies under an evidence root (3.12 placement rules,
// R21): the refusal names the path, the root, the engine cache and the
// environment line that selects it.
func underEvidenceRoot(environment []string, resolution Resolution, evidence Evidence) error {
	roots := evidence.EvidenceRoots()
	if len(roots) == 0 {
		return nil
	}
	engine, err := DomainPathsUsing(DomainEngine, evidence.UserCacheDir)
	if err != nil {
		return nil
	}
	// Each path is named with what this resolver read it from; it never
	// reads GOCACHE or STATICCHECK_CACHE from the environment.
	source := "the user cache directory (os.UserCacheDir: XDG_CACHE_HOME, else HOME)"
	switch resolution.Rule {
	case "proof-record":
		source = "the scratch record of the proof run " + proofControlRootEnv + " names"
	case "context":
		source = "the inherited " + ContextEnv
	}
	type candidate struct{ name, path, source string }
	candidates := []candidate{{"the Go build cache", resolution.Paths.GoCache, source}, {"the staticcheck cache", resolution.Paths.StaticcheckCache, source}}
	if modules := lookup(environment, "GOMODCACHE"); modules != "" {
		candidates = append(candidates, candidate{"GOMODCACHE=" + modules, modules, "the environment's GOMODCACHE"})
	}
	for _, c := range candidates {
		for _, root := range roots {
			if c.path != "" && liesUnder(c.path, root) {
				return Refusal{fmt.Sprintf("%s %s, read from %s, lies under the evidence root %s, and a cache never lives under an evidence root; the engine cache is %s (staticcheck %s): change %s to a place outside every evidence root",
					c.name, c.path, c.source, root, engine.GoCache, engine.StaticcheckCache, c.source)}
			}
		}
	}
	return nil
}

// liesUnder reports path at or below root: by its cleaned spelling, or by
// file identity of root and a physical ancestor of path when both exist.
func liesUnder(path, root string) bool {
	cleanPath, cleanRoot := filepath.Clean(path), filepath.Clean(root)
	if cleanPath == cleanRoot || strings.HasPrefix(cleanPath, cleanRoot+string(filepath.Separator)) {
		return true
	}
	rootInfo, err := os.Stat(cleanRoot)
	if err != nil {
		return false
	}
	current := cleanPath
	if resolved, err := filepath.EvalSymlinks(current); err == nil {
		current = resolved
	}
	for {
		if info, err := os.Stat(current); err == nil && os.SameFile(info, rootInfo) {
			return true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false
		}
		current = parent
	}
}

func resolveDomain(environment []string, installationRoot string, evidence Evidence) (Resolution, error) {
	if evidence.UserCacheDir == nil || evidence.DelegateCustody == nil || evidence.JobWorktree == nil ||
		evidence.ProofRecord == nil || evidence.LiveAncestor == nil || evidence.Self == nil || evidence.EvidenceRoots == nil {
		return Resolution{}, errors.New("cache domain: incomplete evidence")
	}
	self, err := evidence.Self()
	if err != nil {
		return Resolution{}, fmt.Errorf("cache domain: this process's identity: %w", err)
	}
	decided := func(domain Domain, rule string) (Resolution, error) {
		paths, err := DomainPathsUsing(domain, evidence.UserCacheDir)
		return Resolution{Domain: domain, Paths: paths, Rule: rule, issuer: self}, err
	}
	stateRoot, installation, job := lookup(environment, delegateStateRootEnv), lookup(environment, delegateInstallationEnv), lookup(environment, delegateJobEnv)
	if stateRoot+installation+job != "" {
		if stateRoot == "" || installation == "" || job == "" {
			return Resolution{}, Refusal{"incomplete delegate context: " + delegateStateRootEnv + ", " + delegateInstallationEnv + " and " + delegateJobEnv + " come together"}
		}
		owned, err := evidence.DelegateCustody(stateRoot, installation, job)
		if err != nil {
			return Resolution{}, Refusal{fmt.Sprintf("%s=%s: %v", delegateJobEnv, job, err)}
		}
		if !owned {
			return Resolution{}, Refusal{fmt.Sprintf("%s=%s: the delegate context is not this process's (no recorded custody of that job is an ancestor)", delegateJobEnv, job)}
		}
		return decided(DomainDelegate, "delegate-custody")
	}
	if installationRoot != "" {
		worktree, err := evidence.JobWorktree(installationRoot)
		if err != nil {
			return Resolution{}, fmt.Errorf("cache domain: job worktree check of %s: %w", installationRoot, err)
		}
		if worktree {
			return decided(DomainDelegate, "job-worktree")
		}
	}
	if controlRoot := lookup(environment, proofControlRootEnv); controlRoot != "" {
		attempt := lookup(environment, proofAttemptEnv)
		found, authenticated, paths, err := evidence.ProofRecord(controlRoot, attempt)
		if err != nil {
			return Resolution{}, fmt.Errorf("cache domain: scratch record under %s: %w", controlRoot, err)
		}
		if found && !authenticated {
			return Resolution{}, Refusal{fmt.Sprintf("%s=%s: no scratch record of attempt %s names a live ancestor of this process as its launcher or custodian", proofControlRootEnv, controlRoot, nonEmpty(attempt, "(unnamed)"))}
		}
		if found && filepath.IsAbs(paths.GoCache) && filepath.IsAbs(paths.StaticcheckCache) {
			return Resolution{Domain: DomainEngine, Paths: paths, Rule: "proof-record", issuer: self}, nil
		}
	}
	if raw, present := lookupSet(environment, ContextEnv); present {
		context, err := DecodeContext(raw)
		if err != nil {
			return Resolution{}, Refusal{ContextEnv + ": " + err.Error()}
		}
		ancestor, err := evidence.LiveAncestor(context.Issuer)
		if err != nil {
			return Resolution{}, Refusal{fmt.Sprintf("%s: issuer %s cannot be checked: %v", ContextEnv, context.Issuer, err)}
		}
		if !ancestor {
			return Resolution{}, Refusal{fmt.Sprintf("%s: issuer %s is not a live ancestor of this process; unset it or run from the engine that issued it", ContextEnv, context.Issuer)}
		}
		return Resolution{Domain: context.Domain, Paths: Paths{GoCache: context.GoCache, StaticcheckCache: context.StaticcheckCache}, Rule: "context", issuer: self}, nil
	}
	return decided(DomainEngine, "outermost")
}

// ChildContext is the ContextEnv assignment this process gives its children.
func (r Resolution) ChildContext() string {
	value, err := EncodeContext(Context{Domain: r.Domain, GoCache: r.Paths.GoCache, StaticcheckCache: r.Paths.StaticcheckCache, Issuer: r.issuer})
	if err != nil {
		return ""
	}
	return ContextEnv + "=" + value
}

// Apply returns environment with GOCACHE, STATICCHECK_CACHE and the child
// context replaced by this resolution's.
func (r Resolution) Apply(environment []string) []string {
	var result []string
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if name != "GOCACHE" && name != "STATICCHECK_CACHE" && name != ContextEnv {
			result = append(result, entry)
		}
	}
	result = append(result, Environment(r.Paths)...)
	if context := r.ChildContext(); context != "" {
		result = append(result, context)
	}
	return result
}

func lookup(environment []string, name string) string {
	value, _ := lookupSet(environment, name)
	return value
}

func lookupSet(environment []string, name string) (string, bool) {
	value, present := "", false
	for _, entry := range environment {
		if key, found, ok := strings.Cut(entry, "="); ok && key == name {
			value, present = found, true
		}
	}
	return value, present
}

func nonEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
