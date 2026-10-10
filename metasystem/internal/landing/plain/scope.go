package plain

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathclass"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

type scopeRecord struct {
	Scope        string           `json:"scope"`
	ScopeReason  string           `json:"scopeReason"`
	Base         string           `json:"base,omitempty"`
	BaseCommit   string           `json:"baseCommit,omitempty"`
	PlanHash     string           `json:"planHash,omitempty"`
	ChangedPaths []string         `json:"changedPaths"`
	Durations    map[string]int64 `json:"durations"`
	Packages     []PackageTiming  `json:"packages"`
	Groups       []scopeGroup     `json:"groups,omitempty"`
}

type scopeGroup struct {
	ID         string   `json:"id"`
	State      string   `json:"state"`
	Why        []string `json:"why,omitempty"`
	From       string   `json:"from"`
	DurationMS int64    `json:"durationMs"`
}

type scopeDecision struct {
	scopeRecord
	base     Result
	contract testpolicy.Contract
	affected testpolicy.AffectedResult
}

// decideScope keeps a batch's full green for at most an hour. Any unreadable
// selection fact widens the proof to full; no language rule lives here.
func decideScope(install, checkout string, running Running, seams ProveSeams) scopeDecision {
	d := scopeDecision{scopeRecord: scopeRecord{Scope: "full", ScopeReason: "no green proof of this batch yet"}}
	results, err := Results(install)
	if err != nil {
		return d
	}
	batch, err := seams.git(checkout, "rev-list", "--no-merges", running.Commit, "^origin/main")
	if err != nil || strings.TrimSpace(batch) == "" {
		return d
	}
	for i := len(results) - 1; i >= 0; i-- {
		candidate := results[i]
		if candidate.Result != Green || candidate.Scope == "" {
			continue
		}
		other, err := seams.git(checkout, "rev-list", "--no-merges", candidate.Commit, "^origin/main")
		if err == nil && sameBatch(batch, other) {
			d.base = candidate
			break
		}
	}
	recordsOnly := false
	if d.base.Scope == "" {
		changed, err := seams.git(checkout, "diff", "--name-only", "--no-renames", "origin/main", running.Tree)
		if err != nil || strings.TrimSpace(changed) == "" || !recordsOnlyPaths(install, checkout, strings.Split(strings.TrimSpace(changed), "\n")) {
			return d
		}
		recordsOnly = true
		d.ScopeReason = "records-only batch has no full green on main under an hour old"
		for i := len(results) - 1; i >= 0; i-- {
			candidate := results[i]
			if candidate.Result != Green || candidate.Scope != "full" {
				continue
			}
			at, err := time.Parse(time.RFC3339, candidate.At)
			if age := seams.now().Sub(at); err != nil || age < 0 || age >= time.Hour {
				continue
			}
			if _, err := seams.git(checkout, "merge-base", "--is-ancestor", candidate.Commit, "origin/main"); err == nil {
				d.base = candidate
				d.base.FullTree, d.base.FullAt = candidate.Tree, candidate.At
				break
			}
		}
		if d.base.Scope == "" {
			return d
		}
	}
	fullAt, err := time.Parse(time.RFC3339, d.base.FullAt)
	if err != nil {
		d.ScopeReason = "the batch's full proof time cannot be read"
		return d
	}
	if age := seams.now().Sub(fullAt); age > time.Hour {
		d.ScopeReason = fmt.Sprintf("the batch's full proof is %s old, more than one hour", age.Round(time.Second))
		return d
	}
	changed, err := seams.git(checkout, "diff", "--name-only", "--no-renames", d.base.Tree, running.Tree)
	if err != nil {
		d.ScopeReason = "the changed paths cannot be read: " + err.Error()
		return d
	}
	if strings.TrimSpace(changed) == "" {
		d.ScopeReason = "the tree comparison names no changed paths"
		return d
	}
	d.ChangedPaths = strings.Split(strings.TrimSpace(changed), "\n")
	executablePaths := d.ChangedPaths
	if d.base.FullTree != "" && d.base.FullTree != d.base.Tree {
		changed, err := seams.git(checkout, "diff", "--name-only", "--no-renames", d.base.FullTree, running.Tree)
		if err != nil {
			d.ScopeReason = "the paths changed since the full proof cannot be read: " + err.Error()
			return d
		}
		executablePaths = append(append([]string{}, executablePaths...), strings.Split(strings.TrimSpace(changed), "\n")...)
	}
	for _, path := range executablePaths {
		if path == "" {
			continue
		}
		if !recordsOnlyPaths(install, checkout, []string{path}) {
			d.ScopeReason = "an executable input changed: " + path
			return d
		}
	}
	relative, _, err := config.CommittedLookup(filepath.Join(install, "metasystem.conf"), "testing.contract")
	if err != nil || relative == "" {
		d.ScopeReason = "the testing contract declaration cannot be read"
		return d
	}
	prefix, err := filepath.Rel(checkout, install)
	if err != nil {
		d.ScopeReason = "the testing contract cannot be placed in the checkout"
		return d
	}
	path := filepath.ToSlash(filepath.Join(prefix, relative))
	data, err := seams.git(checkout, "show", running.Tree+":"+path)
	if err != nil {
		d.ScopeReason = "the testing contract cannot be read: " + err.Error()
		return d
	}
	d.contract, err = testpolicy.Decode([]byte(data))
	if err != nil {
		d.ScopeReason = "the testing contract is invalid: " + err.Error()
		return d
	}
	if slices.Contains(d.ChangedPaths, path) {
		d.ScopeReason = "the testing contract changed"
		return d
	}
	closure := seams.Closure
	if closure == nil {
		closure = func(root, base, tree string) (adapter.Closure, error) {
			language, err := adapter.Detect(root)
			if err != nil {
				return adapter.Closure{}, err
			}
			return language.Closure(root, base, tree)
		}
	}
	units, err := closure(checkout, d.base.Tree, running.Tree)
	if err != nil {
		d.ScopeReason = "the language closure cannot be read: " + err.Error()
		return d
	}
	if len(units.Units()) != 0 {
		d.ScopeReason = "the changed paths affect code units"
		return d
	}
	d.affected, err = testpolicy.Affected(d.contract, d.ChangedPaths)
	switch {
	case err != nil:
		d.ScopeReason = "the affected test groups cannot be selected: " + err.Error()
	case len(d.affected.Uncovered) != 0:
		d.ScopeReason = "changed paths have no declared test inputs: " + strings.Join(d.affected.Uncovered, ", ")
	case len(d.affected.TemplateCovered) != 0:
		d.ScopeReason = "changed paths reach a test group template: " + strings.Join(d.affected.TemplateCovered, ", ")
	case len(d.affected.Groups) == 0:
		d.ScopeReason = "no test groups cover the changed paths"
	default:
		d.Scope, d.Base = "scoped", d.base.Tree
		d.ScopeReason = "the batch's full proof is under an hour old; only groups whose inputs cover changed paths run"
		if recordsOnly {
			d.ScopeReason = "records-only batch; main's full green is under an hour old; only groups whose inputs cover changed paths run"
		}
	}
	return d
}

func recordsOnlyPaths(install, checkout string, paths []string) bool {
	classes, err := pathclass.Load()
	if err != nil {
		return false
	}
	prefix, err := filepath.Rel(checkout, install)
	if err != nil {
		return false
	}
	resolver := stateroot.NewResolver(func(string) (string, error) { return checkout, nil }, nil)
	for _, path := range paths {
		owner, mode, err := resolver.OwnerForInstallation(install, path)
		if err != nil {
			return false
		}
		class := classes.ResolveRepositoryPath(pathclass.Mode(mode), owner, filepath.ToSlash(prefix), path).Class
		if class != pathclass.Record && class != pathclass.Ledger {
			return false
		}
	}
	return len(paths) != 0
}

func sameBatch(a, b string) bool {
	left, right := strings.Fields(a), strings.Fields(b)
	slices.Sort(left)
	slices.Sort(right)
	return len(left) != 0 && slices.Equal(slices.Compact(left), slices.Compact(right))
}

func (d scopeDecision) groupIDs() []string {
	var ids []string
	if d.Scope == "scoped" {
		for _, group := range d.affected.Groups {
			ids = append(ids, group.Group.ID)
		}
	}
	return ids
}

func scopePath(install, attempt string) string {
	return filepath.Join(Dir(install), "proofs", attempt+".scope.json")
}

func (d scopeDecision) writeRecord(install string, result Result, observed *proofOutput) error {
	if d.Scope == "scoped" {
		var previous scopeRecord
		if data, err := os.ReadFile(scopePath(install, d.base.Attempt)); err == nil {
			_ = json.Unmarshal(data, &previous)
		}
		for _, group := range d.contract.Groups {
			if group.PackageSelection != "" {
				continue
			}
			entry := scopeGroup{ID: group.ID, State: "kept", From: d.base.Tree}
			for _, old := range previous.Groups {
				if old.ID == group.ID {
					entry.From, entry.DurationMS = old.From, old.DurationMS
					break
				}
			}
			if duration, ran := observed.durations[group.ID]; ran {
				entry.State, entry.From, entry.DurationMS = "ran", result.Tree, duration
				for _, affected := range d.affected.Groups {
					if affected.Group.ID == group.ID {
						entry.Why = affected.Paths
					}
				}
			}
			d.Groups = append(d.Groups, entry)
		}
	}
	d.Packages, d.Durations = append([]PackageTiming{}, observed.packages...), observed.durations
	data, err := json.MarshalIndent(d.scopeRecord, "", "  ")
	if err != nil {
		return err
	}
	path := scopePath(install, result.Attempt)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// proofOutput passes every byte through while reading complete protocol lines.
// Only the command's exit decides green; these lines describe what it ran.
type proofOutput struct {
	mu          sync.Mutex
	output      io.Writer
	pending     string
	environment string
	static      string
	envSeen     bool
	ran         []string
	durations   map[string]int64
	packages    []PackageTiming
	report      checkReport
}

// readLog observes only this command's bytes after the shell exits. The size
// bounds the read even when a background descendant keeps writing to the log.
// A nonseekable or unreadable log leaves the environment unknown.
func (p *proofOutput) readLog(output io.Writer, offset int64) {
	file, ok := output.(*os.File)
	if !ok || offset < 0 {
		return
	}
	log, err := os.Open(file.Name())
	if err != nil {
		return
	}
	defer log.Close()
	info, err := log.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return
	}
	if _, err := log.Seek(offset, io.SeekStart); err != nil {
		return
	}
	copied := &commandTail{output: io.Discard}
	if _, err := io.Copy(io.MultiWriter(p, copied), io.LimitReader(log, info.Size()-offset)); err != nil {
		p.environment, p.pending = "", ""
		p.ran, p.durations, p.packages = nil, nil, nil
		return
	}
	p.report = readReport(copied.tail)
}

func (p *proofOutput) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	n, err := p.output.Write(data)
	p.pending += string(data[:n])
	for {
		line, rest, found := strings.Cut(p.pending, "\n")
		if !found {
			break
		}
		p.line(strings.TrimSuffix(line, "\r"))
		p.pending = rest
	}
	return n, err
}

func (p *proofOutput) finish() {
	p.line(strings.TrimSuffix(p.pending, "\r"))
	p.pending = ""
}

func (p *proofOutput) line(line string) {
	if environment, ok := strings.CutPrefix(line, "landing environment "); ok && !p.envSeen {
		p.environment, p.envSeen = environment, true
	}
	fields := strings.Fields(line)
	if len(fields) == 6 && fields[0] == "landing" && fields[1] == "package" {
		shard, se := strconv.Atoi(fields[3])
		ms, me := strconv.ParseInt(fields[5], 10, 64)
		if se == nil && me == nil && shard >= 0 && ms >= 0 {
			timing := PackageTiming{fields[2], shard, fields[4], ms}
			// The reporter prints a shard again when its final verdict differs
			// from the live one; the last line is the shard's record.
			for i, seen := range p.packages {
				if seen.Unit == timing.Unit && seen.Shard == timing.Shard {
					p.packages[i] = timing
					return
				}
			}
			p.packages = append(p.packages, timing)
		}
		return
	}
	if len(fields) != 5 || fields[0] != "landing" || fields[1] != "group" {
		return
	}
	duration, err := strconv.ParseInt(fields[4], 10, 64)
	if err != nil || duration < 0 {
		return
	}
	if p.durations == nil {
		p.durations = map[string]int64{}
	}
	if fields[2] == "fast-static-build" {
		p.static = fields[3]
	}
	p.ran = append(p.ran, fields[2])
	p.durations[fields[2]] = duration
}

// fullCurrent retains the legacy ledger shortcut; records with scope carry
// the original full proof's clock through every inherited or scoped result.
func (r Result) fullCurrent(now time.Time) bool {
	if r.Scope == "" {
		return true
	}
	fullAt, err := time.Parse(time.RFC3339, r.FullAt)
	return err == nil && now.Sub(fullAt) <= time.Hour
}

func (r Result) reusableGreen(now time.Time) bool {
	if r.Result != Green {
		return false
	}
	if r.Scope == "scoped" || strings.HasPrefix(r.Reason, "inherits green from tree ") {
		return r.fullCurrent(now)
	}
	return true
}

func (d scopeDecision) describe(result Result, observed *proofOutput) Result {
	result.Scope, result.ScopeReason, result.Base = d.Scope, d.ScopeReason, d.Base
	result.BaseCommit, result.PlanHash = d.BaseCommit, d.PlanHash
	result.Ran, result.Environment = observed.ran, observed.environment
	if d.Scope == "scoped" {
		result.FullTree, result.FullAt = d.base.FullTree, d.base.FullAt
	} else if result.Result == Green && d.Scope != "gate" && d.Scope != "impact" {
		result.FullTree, result.FullAt = result.Tree, result.At
	} else {
		result.FullTree, result.FullAt = "", ""
	}
	return result
}

func inheritScope(install string, result, from Result) (Result, error) {
	result.Scope, result.ScopeReason = from.Scope, result.Reason
	result.Base, result.FullTree, result.FullAt = from.Tree, from.FullTree, from.FullAt
	result.Environment = from.Environment
	if from.Scope != "" {
		result.Scope = "scoped"
	}
	decision := scopeDecision{scopeRecord: scopeRecord{Scope: result.Scope, ScopeReason: result.ScopeReason, Base: result.Base}}
	if data, err := os.ReadFile(scopePath(install, from.Attempt)); err == nil {
		var prior scopeRecord
		if json.Unmarshal(data, &prior) == nil {
			for _, group := range prior.Groups {
				group.State, group.Why = "kept", nil
				decision.Groups = append(decision.Groups, group)
			}
		}
	}
	return result, decision.writeRecord(install, result, &proofOutput{})
}
