package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

// Work references. Every stored record keeps its own identity; a reference
// is a qualified form over it, decoded only here, and owners always receive
// the raw id:
//
//	j1:ID     a launch record of the current user
//	j2:ID     a dispatch job of the selected repository
//	run:ID    a unit run of the current user
//	read:REF  a diagnostic read
//	wait:ID   a durable wait of the selected repository
//	proof:ID  a proof attempt of the selected repository
//
// A bare word is a goal name when the action takes goals and the ledger
// knows it; otherwise it is searched as an exact raw id in every store the
// action accepts. Exactly one match is used; several refuse and list each
// qualified reference, which is never ambiguous.
const (
	launchJobPrefix   = "j1:"
	dispatchJobPrefix = "j2:"
	unitRunPrefix     = "run:"
	readRefPrefix     = "read:"
	waitRefPrefix     = "wait:"
	proofRefPrefix    = "proof:"
)

// The reference kinds, in the order a bare id is searched.
const (
	refGoal  = "goal"
	refJ1    = "j1"
	refJ2    = "j2"
	refRun   = "run"
	refRead  = "read"
	refWait  = "wait"
	refProof = "proof"
)

var refKindNames = map[string]string{
	refGoal: "goal", refJ1: "launch", refJ2: "dispatch job", refRun: "unit run", refRead: "diagnostic read",
	refWait: "durable wait", refProof: "proof attempt",
}

var refSearchOrder = []string{refJ1, refJ2, refRun, refRead, refWait, refProof}

// workRef is one resolved reference.
type workRef struct {
	kind string
	id   string // the owner's raw id, or the goal name
	job  intentJob
}

func (ref workRef) qualified() string {
	if ref.kind == refGoal {
		return ref.id
	}
	return ref.kind + ":" + ref.id
}

// qualifiedJob is the public reference of a dispatch job id, which may
// already carry its prefix.
func qualifiedJob(id string) string {
	if strings.HasPrefix(id, dispatchJobPrefix) || strings.HasPrefix(id, launchJobPrefix) {
		return id
	}
	return dispatchJobPrefix + id
}

// splitReference decodes a qualified reference's kind and raw id; a bare
// word has no kind.
func splitReference(ref string) (string, string) {
	if kind, id, found := strings.Cut(ref, ":"); found {
		if _, known := refKindNames[kind]; known && kind != refGoal {
			return kind, id
		}
	}
	return "", ref
}

// acceptingActions names the public actions that take a reference kind.
func acceptingActions(kind string) []string {
	var names []string
	for _, command := range publicIntentCommands() {
		if slices.Contains(command.accepts, kind) {
			names = append(names, "metasystem "+command.name)
		}
	}
	return names
}

func (inv *intentInvocation) refusedKind(ref, kind string) *intentResult {
	accepting := acceptingActions(kind)
	result := &intentResult{Outcome: intentRefused, code: 2, Targets: []intentTarget{{Kind: kind, ID: ref}},
		Summary: fmt.Sprintf("%s does not take a %s reference (%s); nothing was done", inv.command.name, refKindNames[kind], ref)}
	if len(accepting) > 0 {
		result.Decision = "a " + refKindNames[kind] + " reference is taken by " + strings.Join(accepting, ", ")
	}
	return result
}

// resolveWorkRef resolves one reference against the kinds the action
// accepts, before any effect.
func (inv *intentInvocation) resolveWorkRef(ref string, accepts []string) (workRef, *intentResult) {
	if strings.TrimSpace(ref) == "" {
		return workRef{}, &intentResult{Outcome: intentRefused, code: 2, Summary: inv.command.name + " needs a goal or a work reference; nothing was done"}
	}
	if kind, id := splitReference(ref); kind != "" {
		if !slices.Contains(accepts, kind) {
			return workRef{}, inv.refusedKind(ref, kind)
		}
		if id == "" {
			return workRef{}, &intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("%s names no id; nothing was done", ref)}
		}
		switch kind {
		case refJ1, refJ2:
			found, problem := inv.findJobs(id, kind == refJ1, kind == refJ2)
			if problem != nil {
				return workRef{}, problem
			}
			if len(found) == 0 {
				return workRef{}, inv.noReference(ref, []string{kind})
			}
			return workRef{kind: kind, id: id, job: found[0]}, nil
		}
		return workRef{kind: kind, id: id}, nil
	}
	if slices.Contains(accepts, refGoal) && inv.knownGoal(ref) {
		return workRef{kind: refGoal, id: ref}, nil
	}
	var found []workRef
	for _, kind := range refSearchOrder {
		if !slices.Contains(accepts, kind) {
			continue
		}
		matches, problem := inv.searchStore(kind, ref)
		if problem != nil {
			return workRef{}, problem
		}
		found = append(found, matches...)
	}
	switch len(found) {
	case 0:
		if slices.Contains(accepts, refGoal) {
			// The goal owner reports an unknown goal in its own words.
			return workRef{kind: refGoal, id: ref}, nil
		}
		return workRef{}, inv.noReference(ref, accepts)
	case 1:
		return found[0], nil
	}
	lines, candidates, choices := []string{}, []string{}, []map[string]any{}
	for _, one := range found {
		argv := inv.publicArgv(append(inv.command.words(), one.qualified())...)
		purpose := refKindNames[one.kind]
		if one.kind == refJ1 || one.kind == refJ2 {
			purpose = jobPurpose(one.job)
		}
		lines = append(lines, fmt.Sprintf("  %s: %s", shellCommand(argv), purpose))
		candidates = append(candidates, one.qualified())
		choices = append(choices, map[string]any{"reference": one.qualified(), "purpose": purpose, "argv": argv})
	}
	return workRef{}, &intentResult{Outcome: intentRefused, code: 1, Targets: []intentTarget{{Kind: "reference", ID: ref}}, text: lines,
		Summary:  fmt.Sprintf("%s names %d records (%s); nothing was done", shellCommand([]string{ref}), len(found), strings.Join(candidates, ", ")),
		Decision: "name the one meant by its qualified reference: " + strings.Join(candidates, " or "),
		Data:     map[string]any{"candidates": candidates, "choices": choices}}
}

func (inv *intentInvocation) noReference(ref string, kinds []string) *intentResult {
	var searched []string
	for _, kind := range kinds {
		if kind != refGoal {
			searched = append(searched, refKindNames[kind])
		}
	}
	result := &intentResult{Outcome: intentRefused, code: 1, Targets: []intentTarget{{Kind: "reference", ID: ref}},
		Summary: fmt.Sprintf("no %s names %s; nothing was done", strings.Join(searched, " or "), shellCommand([]string{ref}))}
	if slices.Equal(searched, []string{refKindNames[refProof]}) {
		// work status lists launches and jobs, never proof attempts (EM-18).
		result.Decision = "a proof attempt's id is the proof:ID in the output of metasystem test run"
		return result
	}
	result.next, result.nextReason = inv.publicArgv("work", "status", "--all"), "lists this user's launches and this repository's dispatch jobs with their references"
	return result
}

// knownGoal reports whether the ledger of the selected repository knows the
// goal name, live or archived.
func (inv *intentInvocation) knownGoal(name string) bool {
	if inv.stateRoot == "" {
		if problem := inv.selectRoot(); problem != nil {
			return false
		}
	}
	projection, _, problem := inv.projection()
	if problem != nil {
		return false
	}
	file, _ := goalRecord(projection, name)
	return file != nil
}

// searchStore finds an exact raw id in one store.
func (inv *intentInvocation) searchStore(kind, id string) ([]workRef, *intentResult) {
	switch kind {
	case refJ1, refJ2:
		jobs, problem := inv.findJobs(id, kind == refJ1, kind == refJ2)
		var found []workRef
		for _, job := range jobs {
			found = append(found, workRef{kind: kind, id: job.id, job: job})
		}
		return found, problem
	case refRun:
		if inv.owners.processes.launches == nil {
			return nil, nil
		}
		runner := &launch.UnitRunner{Manager: inv.owners.processes.launches()}
		if _, err := runner.Status(id); err == nil {
			return []workRef{{kind: kind, id: id}}, nil
		}
	case refRead:
		if strings.HasPrefix(id, "read-") && inv.selectLayoutRoot() == nil {
			if _, err := inv.unitRunner().InspectRead(id); err == nil {
				return []workRef{{kind: kind, id: id}}, nil
			}
		}
	case refWait:
		if metarun.ValidWaitID(id) && inv.selectLayoutRoot() == nil {
			if _, _, err := metarun.FindWaiterByID(inv.stateRoot, id); err == nil {
				return []workRef{{kind: kind, id: id}}, nil
			}
		}
	case refProof:
		if inv.selectLayoutRoot() == nil {
			for _, root := range []string{inv.stateRoot, inv.layout.InstallationRoot} {
				if _, err := proofrun.ReadAttempt(root, id); err == nil {
					return []workRef{{kind: kind, id: id}}, nil
				}
			}
		}
	}
	return nil, nil
}

// findJobs reads the exact launch record and dispatch job an id names.
func (inv *intentInvocation) findJobs(id string, searchLaunch, searchDispatch bool) ([]intentJob, *intentResult) {
	targets := []intentTarget{{Kind: "job", ID: id}}
	var found []intentJob
	if searchLaunch && inv.owners.processes.launches != nil {
		manager := inv.owners.processes.launches()
		record, err := manager.Store.Read(id)
		switch {
		case err == nil:
			found = append(found, intentJob{id: id, kind: "launch", launch: record})
		case errors.Is(err, fs.ErrNotExist), strings.Contains(err.Error(), "invalid launch id"):
		default:
			return nil, &intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: fmt.Sprintf("the launch record %s is unreadable: %v", id, err)}
		}
	}
	if searchDispatch && dispatchJobIDPattern.MatchString(id) {
		if problem := inv.selectLayoutRoot(); problem == nil {
			path := filepath.Join(inv.stateRoot, "artifacts", "agents", "jobs", id+".json")
			_, statErr := os.Stat(path)
			object, readErr := dispatchcore.ReadRecordObject(path)
			switch {
			case errors.Is(statErr, fs.ErrNotExist):
			case readErr == nil:
				found = append(found, intentJob{id: id, kind: "dispatch", dispatch: object, recordPath: path})
			default:
				return nil, &intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: fmt.Sprintf("the dispatch job record %s is unreadable: %v", path, readErr)}
			}
		}
	}
	return found, nil
}

// resolveJob finds the exact launch or dispatch job a reference names: j1:
// and j2: select one store, a bare id is searched in both. Two matches, or
// none, refuse; an ambiguity lists both references.
func (inv *intentInvocation) resolveJob(ref, _ string) (intentJob, *intentResult) {
	resolved, problem := inv.resolveWorkRef(ref, []string{refJ1, refJ2})
	if problem != nil {
		return intentJob{}, problem
	}
	return resolved.job, nil
}
