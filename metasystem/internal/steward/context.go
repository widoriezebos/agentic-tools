package steward

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/output"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

const RoleContext HealthRole = "context-budget"

type ContextOptions struct {
	Runtime    string
	Session    string
	Transcript string
	Home       string // Explicit test/discovery input; empty derives it.
	Toplevel   string // Explicit test/discovery input; empty derives it.
}

type contextIdentity struct {
	runtime  string
	session  string
	process  identity.Ref
	explicit bool
}

type contextLease struct {
	HolderMainID string `json:"holderMainId"`
}

type contextAnnouncement struct {
	SessionID     string `json:"sessionId"`
	MainID        string `json:"mainId"`
	Runtime       string `json:"runtime"`
	PID           int64  `json:"pid"`
	PIDStartedAt  int64  `json:"pidStartedAt"`
	PIDStartTicks int64  `json:"pidStartTicks"`
	BootID        string `json:"bootId"`
}

var (
	makeContextDiagnosticRoot   = os.MkdirTemp
	removeContextDiagnosticRoot = os.RemoveAll
)

// ContextBudgetLine owns holder selection, one usage read, and the role
// verdict shared by health and the explicit status command. The thresholds,
// the holder's announcement, the call samples and the spills are all run
// state of the installation, so one installation root answers the line.
func ContextBudgetLine(installation stateroot.Installation, now time.Time, opts ContextOptions) (RoleVerdict, usage.Reading, error) {
	// A named root that is not there has no holder to report on (EM-07).
	if _, err := os.Stat(installation.Path()); errors.Is(err, fs.ErrNotExist) {
		err = fmt.Errorf("%s does not exist, so there is no installation to read; nothing was read", installation.Path())
		return roleUnknown(RoleContext, err.Error(), "", RemedyFact{Cause: CauseUnreadable}), usage.Reading{}, err
	}
	return contextBudgetLineWithProber(installation, now, opts, identity.KernelProber{})
}

func contextBudgetLineWithProber(installation stateroot.Installation, now time.Time, opts ContextOptions, prober identity.Prober) (RoleVerdict, usage.Reading, error) {
	installationRoot := installation.Path()
	diagnostic := opts.Transcript != ""
	budget, err := config.ContextBudget(installation)
	if err != nil {
		return roleUnknown(RoleContext, err.Error(), "", RemedyFact{Cause: CauseSettingsInvalid}), usage.Reading{}, err
	}
	holder, noHolderReason, unobservableReason, err := resolveContextIdentity(installationRoot, opts, prober)
	if err != nil {
		return labelContextDiagnostic(roleUnknown(RoleContext, err.Error(), "", RemedyFact{Cause: CauseUnreadable}), diagnostic), usage.Reading{}, err
	}
	if holder == nil {
		return labelContextDiagnostic(roleAlive(RoleContext, noHolderReason), diagnostic), usage.Reading{}, nil
	}

	declaration, registered := runtimes.Lookup(holder.runtime)
	if !registered {
		reason := fmt.Sprintf("unknown (runtime %s is not registered)", holder.runtime)
		if holder.explicit {
			err := fmt.Errorf("runtime %s is not registered", holder.runtime)
			return labelContextDiagnostic(roleUnknown(RoleContext, err.Error(), "", RemedyFact{Cause: CauseSettingsInvalid}), diagnostic), usage.Reading{}, err
		}
		return labelContextDiagnostic(roleAlive(RoleContext, reason), diagnostic), usage.Reading{}, nil
	}
	capability := usage.Capability(declaration.ContextSample)
	if unobservableReason != "" && capability == usage.PerCall {
		return labelContextDiagnostic(roleUnknown(RoleContext, unobservableReason, "", RemedyFact{Cause: CauseUnavailable}), diagnostic), usage.Reading{}, nil
	}
	readOpts := usage.ReadOptions{
		Capability:   capability,
		Transcript:   opts.Transcript,
		Home:         opts.Home,
		Toplevel:     opts.Toplevel,
		Installation: installationRoot,
		Now:          now,
		NonBlocking:  true,
	}
	if diagnostic {
		reading, readErr := readContextTranscriptOverride(holder.runtime, holder.session, readOpts)
		if readErr != nil {
			return labelContextDiagnostic(roleUnknown(RoleContext, readErr.Error(), "", RemedyFact{Cause: CauseUnreadable}), true), reading, readErr
		}
		return contextVerdict(reading, budget, installationRoot, true), reading, nil
	}
	if !holder.explicit && unobservableReason == "" {
		if err := usage.RegisterSessionNonBlocking(installationRoot, holder.runtime, holder.session, holder.process.Pid, holder.process.StartedAtSec); err != nil {
			return roleUnknown(RoleContext, err.Error(), "", RemedyFact{Cause: CauseUnreadable}), usage.Reading{}, err
		}
	}

	toplevel := opts.Toplevel
	if toplevel == "" && declaration.ContextSample == string(usage.PerCall) {
		toplevel, err = contextGitToplevel(installationRoot)
		if err != nil {
			return roleUnknown(RoleContext, err.Error(), "", RemedyFact{Cause: CauseUnreadable}), usage.Reading{}, err
		}
	}
	readOpts.Toplevel = toplevel
	reading, err := usage.LatestCall(installation, holder.runtime, holder.session, readOpts)
	if err != nil {
		return roleUnknown(RoleContext, err.Error(), "", RemedyFact{Cause: CauseUnreadable}), reading, err
	}

	verdict := contextVerdict(reading, budget, installationRoot, false)
	if spillPath, bytes, found := output.NewestSince(installationRoot, reading.PreviousReadAt); found {
		verdict.Reason += fmt.Sprintf("; newest spill: %s (%d bytes)", filepath.Base(spillPath), bytes)
	}
	return verdict, reading, nil
}

// readContextTranscriptOverride keeps an operator-supplied transcript outside
// the evidence that health and reports consume: a scratch directory stands in
// for the installation for the length of one read.
func readContextTranscriptOverride(runtime, session string, opts usage.ReadOptions) (usage.Reading, error) {
	// In the process's registered scratch root (Part B U1b-2): a killed
	// read leaves a root the sweeper proves about, never a TMPDIR entry.
	scratch, err := diskstore.ProcessScratch()
	if err != nil {
		return usage.Reading{}, fmt.Errorf("cannot create diagnostic context store: %w", err)
	}
	root, err := makeContextDiagnosticRoot(scratch, "metasystem-context-diagnostic-")
	if err != nil {
		return usage.Reading{}, fmt.Errorf("cannot create diagnostic context store: %w", err)
	}
	reading, readErr := readContextScratch(root, runtime, session, opts)
	cleanupErr := removeContextDiagnosticRoot(root)
	if cleanupErr != nil {
		cleanupErr = fmt.Errorf("cannot remove diagnostic context store %s: %w", root, cleanupErr)
	}
	return reading, errors.Join(readErr, cleanupErr)
}

// readContextScratch admits the scratch directory, marked with an empty
// metasystem.conf, as the installation one read keeps its samples under.
func readContextScratch(root, runtime, session string, opts usage.ReadOptions) (usage.Reading, error) {
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o600); err != nil {
		return usage.Reading{}, fmt.Errorf("cannot create diagnostic context store: %w", err)
	}
	installation, err := stateroot.ParseInstallation(root)
	if err != nil {
		return usage.Reading{}, fmt.Errorf("cannot create diagnostic context store: %w", err)
	}
	return usage.LatestCall(installation, runtime, session, opts)
}

func checkContextBudget(installation stateroot.Installation, now time.Time, prober identity.Prober) RoleVerdict {
	verdict, _, _ := contextBudgetLineWithProber(installation, now, ContextOptions{}, prober)
	return verdict
}

// checkInstalledContextBudget is the health role over the root health names
// as the installation. The line reads that installation's configuration and
// run state, so a root that holds no metasystem.conf, such as a state root
// apart from its installation, is refused rather than read as one.
func checkInstalledContextBudget(metasystemRoot string, now time.Time, prober identity.Prober) RoleVerdict {
	installation, err := stateroot.ParseInstallation(metasystemRoot)
	if err != nil {
		return roleUnknown(RoleContext, err.Error(), "", RemedyFact{Cause: CauseSettingsInvalid})
	}
	return checkContextBudget(installation, now, prober)
}

func resolveContextIdentity(installationRoot string, opts ContextOptions, prober identity.Prober) (*contextIdentity, string, string, error) {
	if (opts.Runtime == "") != (opts.Session == "") {
		return nil, "", "", fmt.Errorf("runtime and session must be supplied together")
	}
	if opts.Runtime != "" {
		return &contextIdentity{runtime: opts.Runtime, session: opts.Session, explicit: true}, "", "", nil
	}

	directory := filepath.Join(installationRoot, "artifacts", "agents", "mains")
	leasePath := filepath.Join(directory, "worktree-lease.json")
	var lease contextLease
	if err := readContextJSON(leasePath, &lease); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Sprintf("unknown (no announced holder: checkout lease is absent at %s)", leasePath), "", nil
		}
		return nil, "", "", fmt.Errorf("cannot read context holder lease %s: %w", leasePath, err)
	}
	if lease.HolderMainID == "" {
		return nil, "unknown (no announced holder: checkout lease has no holderMainId)", "", nil
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Sprintf("unknown (no announced holder: no announcement matches %s)", lease.HolderMainID), "", nil
		}
		return nil, "", "", fmt.Errorf("cannot list context holder announcements %s: %w", directory, err)
	}
	var matchingMalformed error
	var matchingUnknown *contextIdentity
	var matchingDead *contextIdentity
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" || !census.IsAnnouncementFile(entry.Name()) {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		var announcement contextAnnouncement
		if err := readContextJSON(path, &announcement); err != nil {
			continue
		}
		mainID := announcement.MainID
		if mainID == "" {
			mainID = announcement.SessionID
		}
		if mainID != lease.HolderMainID {
			continue
		}
		if announcement.SessionID == "" {
			if matchingMalformed == nil {
				matchingMalformed = fmt.Errorf("context holder announcement %s is malformed: sessionId is empty", path)
			}
			continue
		}
		if announcement.Runtime == "" || announcement.PID < 1 || announcement.PIDStartedAt < 1 {
			if matchingMalformed == nil {
				matchingMalformed = fmt.Errorf("context holder announcement %s is malformed", path)
			}
			continue
		}
		process := identity.Ref{
			Pid: announcement.PID, StartedAtSec: announcement.PIDStartedAt,
			StartTicks: announcement.PIDStartTicks, BootID: announcement.BootID,
		}
		candidate := &contextIdentity{
			runtime: announcement.Runtime, session: announcement.SessionID,
			process: process,
		}
		switch identity.AliveRef(prober, process) {
		case identity.Alive:
			return candidate, "", "", nil
		case identity.Unknown:
			if matchingUnknown == nil {
				matchingUnknown = candidate
			}
		case identity.Dead:
			if matchingDead == nil {
				matchingDead = candidate
			}
		}
	}
	if matchingUnknown != nil {
		return matchingUnknown, "", fmt.Sprintf("announced holder %s has unknown liveness; no context is attributed to it", lease.HolderMainID), nil
	}
	if matchingDead != nil {
		return matchingDead, "", fmt.Sprintf("announced holder %s is dead; no context is attributed to it", lease.HolderMainID), nil
	}
	if matchingMalformed != nil {
		return nil, "", "", matchingMalformed
	}
	return nil, fmt.Sprintf("unknown (no announced holder: no announcement matches %s)", lease.HolderMainID), "", nil
}

func readContextJSON(path string, target any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return err
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return fmt.Errorf("not a JSON object")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return json.Unmarshal(raw, target)
}

func contextGitToplevel(installationRoot string) (string, error) {
	current, err := filepath.Abs(installationRoot)
	if err != nil {
		return "", fmt.Errorf("cannot resolve installation root %s: %w", installationRoot, err)
	}
	for {
		marker := filepath.Join(current, ".git")
		info, err := os.Stat(marker)
		if err == nil {
			if info.Mode().IsRegular() || info.IsDir() {
				return current, nil
			}
			return "", fmt.Errorf("cannot resolve git toplevel from %s: %s is not a file or directory", installationRoot, marker)
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("cannot inspect git toplevel marker %s: %w", marker, err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("cannot resolve git toplevel from %s: no .git file or directory found", installationRoot)
		}
		current = parent
	}
}

func contextVerdict(reading usage.Reading, budget config.Budget, installationRoot string, diagnostic bool) RoleVerdict {
	if reading.Latest == nil {
		if contextBenignUnknown(reading.Capability, reading.Reason) {
			return labelContextDiagnostic(roleAlive(RoleContext, reading.Reason), diagnostic)
		}
		reason := reading.Reason
		if reason == "" {
			reason = "unknown (per-call reading has no sample or diagnostic)"
		} else if !strings.HasPrefix(reason, "unknown (") {
			reason = "unknown (" + reason + ")"
		}
		return labelContextDiagnostic(roleUnknown(RoleContext, reason, "", RemedyFact{Cause: CauseUnreadable}), diagnostic)
	}

	tokens := reading.Latest.PromptTokens
	display := contextThousands(tokens)
	reason := fmt.Sprintf("%d thousand tokens this call, trigger %d, proof line %d, proof maximum %d, ceiling %d",
		display, budget.Trigger/1000, ProofP95Tokens/1000, ProofMaxTokens/1000, budget.Ceiling/1000)
	if tokens > budget.Trigger {
		if diagnostic {
			reason += "; over the trigger"
		} else {
			reason += "; over the trigger: run " + contextHandoffRemedy(installationRoot)
		}
	}
	return labelContextDiagnostic(roleAlive(RoleContext, reason), diagnostic)
}

func contextHandoffRemedy(installationRoot string) string {
	return "metasystem session handoff --root " + installationRoot + " --note <configured-note-path> --no-delegates"
}

func labelContextDiagnostic(verdict RoleVerdict, diagnostic bool) RoleVerdict {
	if diagnostic {
		verdict.Reason = "diagnostic transcript override; " + verdict.Reason
	}
	return verdict
}

func contextBenignUnknown(capability usage.Capability, reason string) bool {
	if capability == usage.PerInvocation || capability == usage.NoStream {
		return true
	}
	switch reason {
	case "unknown (no call recorded yet)",
		"unknown (only sidechain records so far)",
		"unknown (rollout carries no token_usage_record (codex CLI before 0.153))":
		return true
	default:
		return false
	}
}

func contextThousands(tokens int64) int64 {
	thousands := tokens / 1000
	if tokens%1000 >= 500 {
		thousands++
	}
	return thousands
}
