// Package cachedomain decides, for one engine process, which machine Go
// cache it compiles into (disk-lifetimes A8): gocache.ResolveDomain's rules
// answered from authenticated evidence, the job record's recorded custody
// and the kernel's process ancestry (internal/delegatecustody, the walk the
// Stop hook's delegate check performs), the dispatcher's job
// worktrees (git), the proof run's scratch record, and the issuer of an
// inherited cache context. Every engine site that starts a compile takes its
// child environment from Carry.
package cachedomain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegatecustody"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// Seams are the kernel, git and file reads behind the evidence; the zero
// value is production.
type Seams struct {
	// Git runs one read-only git query in dir.
	Git func(dir string, args ...string) (string, error)
	// Self is this process's pid.
	Self func() int64
	// Probe reads a process's exact identity; Parent its parent pid.
	Probe  func(pid int64) (identity.Exact, bool)
	Parent func(pid int64) (int64, bool)
	// DelegateCustody decides whether pid descends from the job's recorded
	// custody; nil reads the job record and walks the ancestry by exact
	// identity (internal/delegatecustody).
	DelegateCustody func(stateRoot, installationRoot, job string, pid int64) (bool, error)
	// UserCacheDir is os.UserCacheDir.
	UserCacheDir func() (string, error)
}

func (s Seams) withDefaults() Seams {
	if s.Git == nil {
		s.Git = func(dir string, args ...string) (string, error) {
			output, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
			return strings.TrimSpace(string(output)), err
		}
	}
	if s.Self == nil {
		s.Self = func() int64 { return int64(os.Getpid()) }
	}
	if s.Probe == nil {
		s.Probe = func(pid int64) (identity.Exact, bool) {
			exact, state, err := identity.KernelProber{}.Probe(pid)
			return exact, err == nil && state == identity.Alive
		}
	}
	if s.Parent == nil {
		s.Parent = identity.ParentPid
	}
	if s.DelegateCustody == nil {
		probe, parent := s.Probe, s.Parent
		s.DelegateCustody = func(stateRoot, _, job string, pid int64) (bool, error) {
			owned, err := delegatecustody.Read(stateRoot, job, nil)
			if err != nil {
				return false, err
			}
			_, _, _, found, err := delegatecustody.Walk(owned, pid, probe, parent)
			return found, err
		}
	}
	if s.UserCacheDir == nil {
		s.UserCacheDir = os.UserCacheDir
	}
	return s
}

// Evidence composes the production evidence over the seams.
func (s Seams) Evidence() gocache.Evidence {
	s = s.withDefaults()
	return gocache.Evidence{
		UserCacheDir: s.UserCacheDir,
		DelegateCustody: func(stateRoot, installationRoot, job string) (bool, error) {
			return s.DelegateCustody(stateRoot, installationRoot, job, s.Self())
		},
		JobWorktree:  s.jobWorktree,
		ProofRecord:  s.proofRecord,
		LiveAncestor: s.liveAncestor,
		Self: func() (string, error) {
			exact, alive := s.Probe(s.Self())
			if !alive {
				return "", errors.New("this process cannot be probed")
			}
			return identity.EncodeRef(exact.Ref())
		},
	}
}

// Resolve decides this process's domain for installationRoot (empty when
// the site knows none) from environment.
func Resolve(environment []string, installationRoot string) (gocache.Resolution, error) {
	return Seams{}.Resolve(environment, installationRoot)
}

// Resolve is the package Resolve over these seams.
func (s Seams) Resolve(environment []string, installationRoot string) (gocache.Resolution, error) {
	return gocache.ResolveDomain(environment, installationRoot, s.Evidence())
}

// Carry is environment with the resolved GOCACHE, STATICCHECK_CACHE and a
// fresh cache context set for a child that may compile. A refusal is
// returned: the child is not started with paths the evidence does not
// support.
func Carry(environment []string, installationRoot string) ([]string, error) {
	resolution, err := Resolve(environment, installationRoot)
	if err != nil {
		return nil, err
	}
	return resolution.Apply(environment), nil
}

// IssuerIsAncestor reports whether an encoded process reference is this
// process or a live ancestor with that exact identity: the check a cache
// context's issuer must pass.
func IssuerIsAncestor(encoded string) (bool, error) {
	return Seams{}.withDefaults().liveAncestor(encoded)
}

// SelfReference is this process's encoded exact identity, the issuer of the
// context its children inherit.
func SelfReference() (string, error) {
	return Seams{}.Evidence().Self()
}

// jobWorktree: the installation root lies in a linked worktree whose git
// dir is <common>/worktrees/<name>, and the main installation's job record
// <name>.json names that worktree as its workspace.
func (s Seams) jobWorktree(installationRoot string) (bool, error) {
	top, err := s.Git(installationRoot, "rev-parse", "--show-toplevel")
	if err != nil {
		return false, nil
	}
	gitDir, err := s.Git(installationRoot, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return false, nil
	}
	common, err := s.Git(installationRoot, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil || filepath.Dir(gitDir) != filepath.Join(common, "worktrees") {
		return false, nil
	}
	relative, err := filepath.Rel(top, installationRoot)
	if err != nil || strings.HasPrefix(relative, "..") {
		return false, nil
	}
	name := filepath.Base(gitDir)
	record := filepath.Join(filepath.Dir(common), relative, "artifacts", "agents", "jobs", name+".json")
	data, err := os.ReadFile(record)
	if err != nil {
		return false, nil
	}
	var job struct {
		JobID         string `json:"jobId"`
		WorkspaceRoot string `json:"workspaceRoot"`
	}
	if json.Unmarshal(data, &job) != nil || job.WorkspaceRoot == "" {
		return false, nil
	}
	return samePath(job.WorkspaceRoot, top), nil
}

func samePath(a, b string) bool {
	if resolved, err := filepath.EvalSymlinks(a); err == nil {
		a = resolved
	}
	if resolved, err := filepath.EvalSymlinks(b); err == nil {
		b = resolved
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// scratchRecord is the part of a proof run's scratch record the domain
// reads (internal/proofrun.ScratchRecord).
type scratchRecord struct {
	Launcher   string `json:"launcher"`
	Attempt    string `json:"attempt"`
	Custodians []struct {
		Ref string `json:"ref"`
	} `json:"custodians"`
	GoCache          string `json:"goCache"`
	StaticcheckCache string `json:"staticcheckCache"`
}

// proofRecord finds the scratch record of attempt under controlRoot's
// scratch store (artifacts/agents/proof-runs/scratch).
func (s Seams) proofRecord(controlRoot, attempt string) (bool, bool, gocache.Paths, error) {
	if attempt == "" {
		return false, false, gocache.Paths{}, nil
	}
	store := filepath.Join(controlRoot, "artifacts", "agents", "proof-runs", "scratch")
	paths, err := filepath.Glob(filepath.Join(store, "*.json"))
	if err != nil {
		return false, false, gocache.Paths{}, err
	}
	sort.Strings(paths)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var record scratchRecord
		if json.Unmarshal(data, &record) != nil || record.Attempt != attempt {
			continue
		}
		refs := []string{record.Launcher}
		for _, custodian := range record.Custodians {
			refs = append(refs, custodian.Ref)
		}
		for _, ref := range refs {
			if live, _ := s.liveAncestor(ref); live {
				return true, true, gocache.Paths{GoCache: record.GoCache, StaticcheckCache: record.StaticcheckCache}, nil
			}
		}
		return true, false, gocache.Paths{}, nil
	}
	return false, false, gocache.Paths{}, nil
}

// liveAncestor walks this process's ancestry, itself first, comparing each
// live process with the exact reference.
func (s Seams) liveAncestor(encoded string) (bool, error) {
	ref, err := identity.ParseRef(encoded)
	if err != nil {
		return false, fmt.Errorf("unreadable reference: %w", err)
	}
	seen := map[int64]bool{}
	for current := s.Self(); current > 0 && !seen[current]; {
		seen[current] = true
		if current == ref.Pid {
			exact, alive := s.Probe(current)
			return alive && identity.Compare(exact, ref).Matches, nil
		}
		parent, present := s.Parent(current)
		if !present || parent == current {
			break
		}
		current = parent
	}
	return false, nil
}
