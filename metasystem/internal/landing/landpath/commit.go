package landpath

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/shellquote"
)

// CommitRequest is one pass through the commit boundary: the landing's
// declarations, the commit message, and the stamping the caller asks for.
type CommitRequest struct {
	// Root is the metasystem installation root (the directory holding
	// metasystem.conf), inside a Git work tree.
	Root string
	// HeldEpoch is the lease epoch a caller that already holds the lease
	// passes ("human" or a positive claim epoch); empty acquires it here.
	HeldEpoch string
	Push      bool

	Chain, Attested, AttestedSnapshot, AttestedBase string
	DirectFix, RevertOf                             string
	// Goal is stamped as Goal-Item when GoalSet.
	Goal                   string
	GoalSet                bool
	RootJob, TestReceipt   string
	Recertification        string
	Carried, LedgerTip     string
	CarriedBy, CarriedPast string

	// MessageFile is the commit message file (git commit -F).
	MessageFile string
	// OwnerLineage is the seat's owner lineage (METASYSTEM_OWNER_LINEAGE at
	// the caller's boundary); an agent commit requires it.
	OwnerLineage string
	// LandedBy is stamped as Landed-By when set.
	LandedBy string
	// LaneJoin marks a commit whose change joins the landing lane, which
	// proves it before the push; the seat's proof scope follows the change.
	LaneJoin bool
	// AllowNewPlan acknowledges a new plan file for the pre-commit guard.
	AllowNewPlan bool
	// Env is added to the git commit's environment (the approver identity
	// a batch landing commits as).
	Env []string
	// Stop, when set, receives what a refused commit tells the person: the
	// reason and the one command (stop.go).
	Stop *Stop
}

var goalIdentifier = regexp.MustCompile(`^[a-z0-9-]+$`)

// differs is the test check's word that the files on disk are not the staged
// candidate.
var differs = regexp.MustCompile(`differs from relevant working-tree inputs|working-tree inputs differ`)

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (request CommitRequest) landingDeclared() bool {
	return request.Chain != "" || request.DirectFix != "" || request.RevertOf != "" || request.RootJob != "" ||
		request.TestReceipt != "" || request.Recertification != "" || request.Carried != "" ||
		request.Attested != "" || request.AttestedSnapshot != "" || request.AttestedBase != ""
}

// Commit runs the commit boundary and returns the exit status the former
// wrapper returned: 0 committed (and pushed, with Push); 1 refused or failed;
// 2 refused before any effect; 3 a carried landing's ask or an attested
// landing's refusal. A refusal's two lines (the reason and the one command)
// are written to stderr and recorded in request.Stop; git's own output and
// the refusal's background (codes, verdicts, paths) go to details.
func Commit(owners Owners, request CommitRequest, details, stderr io.Writer) int {
	stop := request.Stop
	if stop == nil {
		stop = &Stop{}
	}
	b := &boundary{owners: owners, request: request, details: details, stderr: stderr, stopped: stop}
	return b.run()
}

type boundary struct {
	owners  Owners
	request CommitRequest
	// details is what only --verbose shows; stderr carries a refusal's two
	// lines.
	details, stderr io.Writer
	stopped         *Stop

	agent            bool
	prefix, toplevel string
	provedTree       string
}

// stop refuses the commit: the reason and the one command on stderr, the
// background in details, and the stop recorded for the caller.
func (b *boundary) stop(code int, reason string, run []string, then string, details ...string) int {
	writeDetails(b.details, details...)
	b.stopped.said(reason, run, then)
	writeStop(b.stderr, Stop{Reason: reason, Run: run, Then: then})
	return code
}

// failed refuses the commit for an owner's error the person cannot act on
// by its words: reason in plain words, the error in details.
func (b *boundary) failed(code int, reason string, err error) int {
	cause := ""
	if err != nil {
		cause = err.Error()
	}
	then := ""
	if reason == notHolderReason {
		then = notHolderThen
	}
	if b.stopped.Reason == "" {
		b.stopped.cause = cause
	}
	return b.stop(code, reason, nil, then, cause)
}

func (b *boundary) git(dir string, args ...string) GitResult {
	return b.owners.Git(GitCall{Dir: dir, Args: args})
}

func (b *boundary) run() int {
	request := b.request
	// Landing authority is fenced before lease re-entry so a declared brain
	// gets the one required refusal regardless of its lease posture.
	if request.landingDeclared() {
		detail, err := b.owners.BrainFence(request.Root, "land")
		if err != nil {
			return b.failed(1, "this checkout's role couldn't be read, so nothing was committed", err)
		}
		if detail != "" {
			return b.stop(2, brainReason, nil, brainThen, detail)
		}
	}
	if request.HeldEpoch != "" {
		return b.held(request.HeldEpoch)
	}
	epoch, err := b.owners.RequireHolder(request.Root, b.owners.CallerPID, nil)
	if err != nil {
		return b.failed(1, notHolderReason, err)
	}
	held := "human"
	if epoch != nil {
		held = strconv.FormatInt(*epoch, 10)
	}
	status := 0
	if heldErr := b.owners.WithHeld(request.Root, b.owners.CallerPID, epoch, func() error {
		status = b.held(held)
		return nil
	}); heldErr != nil {
		return b.failed(1, notHolderReason, heldErr)
	}
	return status
}

// held is the boundary under the lease: the former wrapper's __lease-held
// half.
func (b *boundary) held(epoch string) int {
	request := b.request
	if value, err := strconv.ParseInt(epoch, 10, 64); err == nil && value > 0 && epoch[0] != '0' {
		b.agent = true
		if _, err := b.owners.RequireHolder(request.Root, b.owners.CallerPID, &value); err != nil {
			return b.failed(1, notHolderReason, err)
		}
	} else {
		if epoch != "human" {
			return 2
		}
		if _, err := b.owners.RequireHolder(request.Root, b.owners.CallerPID, nil); err != nil {
			return b.failed(1, notHolderReason, err)
		}
	}
	if b.agent && request.OwnerLineage == "" {
		return b.stop(2, "this agent shell doesn't say which session it is, so nothing was committed", nil,
			"export METASYSTEM_OWNER_LINEAGE in the session's shell, then repeat this command",
			"the checkout is held under a claim, and an agent commit names its owner lineage")
	}
	if request.Carried != "" && (!request.GoalSet || request.LedgerTip == "" || request.CarriedBy == "" || request.CarriedPast == "") {
		return b.stop(2, "a carried commit needs its goal, ledger tip, person and past refusal; one is missing", nil, "",
			"--carried requires --goal, --ledger-tip, --carried-by, and --carried-past")
	}
	if request.GoalSet && (request.Goal == "" || len(request.Goal) > 100 || !goalIdentifier.MatchString(request.Goal)) {
		return b.stop(2, fmt.Sprintf("%q is not a goal id (lowercase words joined by dashes), so nothing was committed", request.Goal), nil,
			"name the goal by its id, as metasystem goal list shows it")
	}
	if status := b.scanMessage(); status != 0 {
		return status
	}
	started, err := b.owners.StartedAt(b.owners.Getpid())
	if err != nil {
		return b.failed(1, "this process couldn't be read, so nothing was committed; repeat the command", err)
	}
	nonce, err := b.owners.TokenNonce()
	if err != nil {
		return b.failed(1, "the commit couldn't mark itself as the landing's own, so nothing was committed", err)
	}
	token := TokenPath(request.Root)
	if err := b.owners.WriteToken(token, WrapperToken{WrapperPid: b.owners.Getpid(), WrapperPidStartedAt: started,
		Nonce: nonce, CreatedAt: b.owners.Now().UTC().Format("2006-01-02T15:04:05Z")}); err != nil {
		return b.failed(1, "the commit couldn't mark itself as the landing's own, so nothing was committed", err)
	}
	defer b.owners.RemoveFile(token)
	// A malformed session trailer has slipped through four times (claude.ac
	// for claude.ai): refuse it at the door. The message argument is
	// scanned, not the repository.
	if strings.Contains(request.MessageFile, "claude.ac/") {
		return b.stop(2, "the commit message's session link says claude.ac; the domain is claude.ai", nil,
			"correct it to claude.ai in the message, then repeat this command")
	}
	return b.proveAndCommit()
}

var (
	goalItemLine    = regexp.MustCompile(`(?im)^Goal-Item:`)
	carriedItemLine = regexp.MustCompile(`(?m)^(Carry|Carried-By|Carried-Tree|Carried-Past|Carried-Battery|Carried-Judge|Carried-Ledger):`)
	machineLine     = regexp.MustCompile(`(?im)^Machine:`)
)

// scanMessage refuses a message that types a trailer the boundary stamps.
func (b *boundary) scanMessage() int {
	path := b.request.MessageFile
	if path == "-" {
		return b.stop(2, "the commit message must be a file, not standard input, so it can be checked", nil,
			"write the message to a file and pass it with --message")
	}
	if !b.owners.FileReadable(path) {
		return b.stop(2, "the commit message file can't be read, so nothing was committed", nil,
			"check the file named by --message, then repeat this command", "message file: "+path)
	}
	data, err := b.owners.ReadFile(path)
	if err != nil {
		return b.stop(2, "the commit message file can't be read, so nothing was committed", nil,
			"check the file named by --message, then repeat this command", "message file: "+path, err.Error())
	}
	text := string(data)
	typed := "the commit message types a line the landing adds itself (%s:), so nothing was committed"
	remove := "delete that line from the message, then repeat this command"
	stamped := "Goal-Item, Machine and the Carr* trailers are stamped by the commit, never typed"
	switch {
	case carriedItemLine.MatchString(text):
		return b.stop(1, fmt.Sprintf(typed, carriedItemLine.FindStringSubmatch(text)[1]), nil, remove, stamped)
	case machineLine.MatchString(text):
		return b.stop(2, fmt.Sprintf(typed, "Machine"), nil, remove, stamped)
	case goalItemLine.MatchString(text):
		return b.stop(2, fmt.Sprintf(typed, "Goal-Item"), nil, remove, stamped)
	}
	return 0
}

// criticalSymlink names the proof inputs a symlink may not stand in for.
func criticalSymlink(relative string) bool {
	base := relative
	if slash := strings.LastIndexByte(relative, '/'); slash >= 0 {
		base = relative[slash+1:]
	}
	switch base {
	case "AGENTS.md", "wow.md", "go.mod", "go.sum", "go.work", "go.work.sum", "metasystem.conf", "docs", "cmd", "internal", "scripts":
		return true
	}
	if strings.HasSuffix(relative, ".go") || relative == "docs/project-rules.md" || strings.HasSuffix(relative, "/docs/project-rules.md") {
		return true
	}
	for _, directory := range []string{"docs", "cmd", "internal", "scripts"} {
		if strings.HasPrefix(relative, directory+"/") || strings.Contains(relative, "/"+directory+"/") {
			return true
		}
	}
	return false
}

func splitNUL(data []byte) []string {
	var out []string
	for _, part := range bytes.Split(data, []byte{0}) {
		if len(part) > 0 {
			out = append(out, string(part))
		}
	}
	return out
}

// unboundInputs enumerates working-tree bytes the commit would not record
// and returns those inside the LANDING projection.
func (b *boundary) unboundInputs() ([]string, error) {
	var paths []string
	for _, args := range [][]string{
		{"diff", "--no-renames", "--name-only", "-z", "--"},
		{"ls-files", "--others", "--exclude-standard", "--full-name", "-z"},
		{"ls-files", "--others", "-i", "--exclude-standard", "--full-name", "-z"},
	} {
		paths = append(paths, splitNUL(b.git(b.toplevel, args...).Stdout)...)
	}
	return b.owners.SelectLanding(paths, b.prefix)
}

// proofScope is the seat's share of the delivery proof. Without a lane it
// is the whole plan. A change joining the lane proves only its admission
// groups when a staged path is in the ENGINE or PAYLOAD projection, and
// nothing when none is: the lane proves the batch tip before any push.
func (b *boundary) proofScope() (ProofScope, error) {
	if !b.request.LaneJoin {
		return ProofFull, nil
	}
	if b.owners.SelectCode == nil {
		return ProofAdmission, nil
	}
	staged := b.git(b.toplevel, "diff", "--cached", "--no-renames", "--name-only", "-z", "--")
	if staged.Code != 0 {
		return "", fmt.Errorf("the staged files can't be listed: %s", strings.TrimSpace(string(staged.Stderr)))
	}
	code, err := b.owners.SelectCode(splitNUL(staged.Stdout), b.prefix)
	if err != nil {
		return "", err
	}
	if len(code) > 0 {
		return ProofAdmission, nil
	}
	return ProofNone, nil
}

// pathLines are paths as a refusal's details list them.
func pathLines(paths []string) []string {
	lines := make([]string, len(paths))
	for index, path := range paths {
		lines[index] = "  " + shellquote.Token(path)
	}
	return lines
}

// firstPath is the first of paths and how many more there are, for line 1.
func firstPath(paths []string) string {
	if len(paths) == 1 {
		return paths[0]
	}
	return fmt.Sprintf("%s and %d more", paths[0], len(paths)-1)
}

func (b *boundary) proveAndCommit() int {
	request := b.request
	prefix := b.git(request.Root, "rev-parse", "--show-prefix")
	top := b.git(request.Root, "rev-parse", "--show-toplevel")
	if prefix.Code != 0 || top.Code != 0 {
		return b.stop(1, "this isn't a Git checkout git can read, so nothing was committed", nil, "", string(prefix.Stderr), string(top.Stderr))
	}
	b.prefix = strings.TrimRight(string(prefix.Stdout), "\n")
	b.toplevel = strings.TrimRight(string(top.Stdout), "\n")
	proved := b.git(b.toplevel, "write-tree")
	if proved.Code != 0 {
		return b.stop(1, "the staged files still hold merge conflicts, so nothing was committed", []string{"git", "status"},
			"resolve and stage the conflicted files, then repeat this command", string(proved.Stderr))
	}
	b.provedTree = strings.TrimSpace(string(proved.Stdout))
	// Every installation carries a testing contract (C1): the commit
	// consumes the already-admitted shared result for this exact tree and
	// starts neither tests nor builds. A carried landing defers that
	// judgment into its observer so a readable red battery can be carried.
	if b.owners.ConfValue(request.Root, "testing.contract") == "" {
		return b.stop(1, "metasystem.conf names no test contract (testing.contract), so nothing can be landed", nil,
			"add testing.contract to the committed metasystem.conf", "there is no landing without a testing contract")
	}
	if request.Carried == "" {
		scope, err := b.proofScope()
		if err != nil {
			return b.failed(1, "the staged files can't be read, so nothing was committed", err)
		}
		var verified bytes.Buffer
		if status := b.owners.Verify(VerifyRequest{Root: request.Root, Tree: b.provedTree, Goal: request.Goal, Scope: scope}, &verified, &verified); status != 0 {
			details := append(strings.Split(strings.TrimRight(verified.String(), "\n"), "\n"), "tests required on this checkout: "+string(scope))
			if differs.MatchString(verified.String()) {
				return b.stop(1, "files on disk differ from the staged change, so its test results don't apply to it", []string{"git", "status", "--short"},
					"stage or stash the other changes, then repeat this command", details...)
			}
			testRun := []string{"metasystem", "test", "run"}
			if request.Goal != "" {
				testRun = append(testRun, "--goal", request.Goal)
			}
			switch scope {
			case ProofNone:
				return b.stop(1, "the staged change can't be checked against the files on disk, so nothing was committed", []string{"git", "status", "--short"},
					"stage or stash the other changes, then repeat this command", details...)
			case ProofAdmission:
				return b.stop(1, "the change's tests haven't passed on this checkout yet, so nothing was committed", testRun, repeat,
					append(details, "a code change joining the landing lane needs its admission tests passed here; the lane runs the rest")...)
			default:
				return b.stop(1, "the change's tests haven't passed on this checkout yet, so nothing was committed", testRun, repeat,
					append(details, "without a landing lane the whole test plan must pass on this checkout")...)
			}
		}
		writeDetails(b.details, verified.String())
	}
	unbound, err := b.unboundInputs()
	if err != nil {
		return b.failed(1, "the files on disk can't be compared with the staged change, so nothing was committed", err)
	}
	if len(unbound) > 0 {
		return b.stop(1, "files on disk differ from the staged change: "+firstPath(unbound), []string{"git", "status", "--short"},
			"stage, stash or remove them, then repeat this command",
			append([]string{"working-tree files the commit would not record, inside what the landing checks:"}, pathLines(unbound)...)...)
	}
	staged := b.git(b.toplevel, "ls-files", "-s", "-z")
	var gitlinks, symlinks []string
	for _, record := range splitNUL(staged.Stdout) {
		metadata, path, ok := strings.Cut(record, "\t")
		if !ok {
			continue
		}
		mode, _, _ := strings.Cut(metadata, " ")
		switch mode {
		case "160000":
			gitlinks = append(gitlinks, path)
		case "120000":
			if criticalSymlink(strings.TrimPrefix(path, b.prefix)) {
				symlinks = append(symlinks, path)
			}
		}
	}
	if selected, err := b.owners.SelectLanding(gitlinks, b.prefix); err != nil {
		return b.failed(1, "the landing rules can't be read, so nothing was committed", err)
	} else if len(selected) > 0 {
		return b.stop(1, "a staged folder is a nested Git checkout the commit can't record: "+firstPath(selected),
			append([]string{"git", "rm", "--cached"}, selected...), repeat,
			append([]string{"staged gitlinks inside what the landing checks:"}, pathLines(selected)...)...)
	}
	if selected, err := b.owners.SelectLanding(symlinks, b.prefix); err != nil {
		return b.failed(1, "the landing rules can't be read, so nothing was committed", err)
	} else if len(selected) > 0 {
		return b.stop(1, "a staged file the tests read is a symlink, which the landing refuses: "+firstPath(selected), nil,
			"replace it with the real file, then repeat this command",
			append([]string{"symlinked inputs the tests would follow outside the commit:"}, pathLines(selected)...)...)
	}
	// assume-unchanged and skip-worktree entries hide index/worktree
	// divergence from every diff the closure runs.
	var hidden []string
	for _, record := range splitNUL(b.git(b.toplevel, "ls-files", "-v", "-z").Stdout) {
		marker, path, ok := strings.Cut(record, " ")
		if ok && (marker == "S" || len(marker) == 1 && marker[0] >= 'a' && marker[0] <= 'z') {
			hidden = append(hidden, path)
		}
	}
	if selected, err := b.owners.SelectLanding(hidden, b.prefix); err != nil {
		return b.failed(1, "the landing rules can't be read, so nothing was committed", err)
	} else if len(selected) > 0 {
		return b.stop(1, "Git is told to ignore changes to a file the tests read: "+firstPath(selected),
			append([]string{"git", "update-index", "--no-assume-unchanged", "--no-skip-worktree", "--"}, selected...), repeat,
			append([]string{"assume-unchanged or skip-worktree entries hide these inputs:"}, pathLines(selected)...)...)
	}
	settled := b.git(b.toplevel, "write-tree")
	if settled.Code != 0 {
		return b.stop(1, "the staged files changed while the landing checked them, so nothing was committed", nil,
			"stage the change again, then repeat this command", string(settled.Stderr))
	}
	settledTree := strings.TrimSpace(string(settled.Stdout))
	settledUnbound, err := b.unboundInputs()
	if err != nil {
		return b.failed(1, "the files on disk can't be compared with the staged change, so nothing was committed", err)
	}
	if settledTree != b.provedTree || len(settledUnbound) > 0 {
		return b.stop(1, "the staged files changed while the landing checked them, so nothing was committed", nil,
			"stage the change again, then repeat this command")
	}
	return b.decideAndCommit(settledTree)
}

// decision is the deciding observation as the boundary reads it.
type decision struct {
	provenance, verdict, code, mode, refusal string
	refusesAgent                             bool
	goalRevision                             string
	carried                                  *landing.CarriedBinding
}

func decisionOf(observed observationView) (decision, bool) {
	refuses := observed.RefusesAgent
	if observed.Provenance == "" || observed.VerdictTrailer == "" || observed.Code == "" ||
		observed.Mode != "observe" && observed.Mode != "refuse" {
		return decision{}, false
	}
	out := decision{provenance: observed.Provenance, verdict: observed.VerdictTrailer, code: observed.Code,
		mode: observed.Mode, refusal: observed.Refusal, refusesAgent: refuses, carried: observed.Carried}
	if observed.GoalRevision > 0 {
		out.goalRevision = strconv.FormatUint(observed.GoalRevision, 10)
	}
	return out, true
}

// observationView is the part of an observation the boundary reads.
type observationView struct {
	Mode           string
	RefusesAgent   bool
	Code           string
	Provenance     string
	VerdictTrailer string
	Refusal        string
	GoalRevision   uint64
	Carried        *landing.CarriedBinding
}

func (b *boundary) observeRequest(tree, settledTree, actor string) ObserveRequest {
	request := b.request
	observe := ObserveRequest{Root: request.Root, Tree: tree, Chain: request.Chain, Attested: request.Attested,
		AttestedSnapshot: request.AttestedSnapshot, AttestedBase: request.AttestedBase, DirectFix: request.DirectFix,
		RevertOf: request.RevertOf, Goal: request.Goal, RootJob: request.RootJob, TestReceipt: request.TestReceipt,
		Recertification: request.Recertification, Actor: actor}
	if request.Carried != "" {
		observe.Carried, observe.ProjectTree, observe.LedgerTip, observe.CarriedBy = request.Carried, settledTree, request.LedgerTip, request.CarriedBy
	}
	return observe
}

func (b *boundary) decideAndCommit(settledTree string) int {
	request := b.request
	machine := strings.TrimSpace(string(b.git(request.Root, "config", "--get", "metasystem.goal.machine").Stdout))
	if machine == "" {
		return b.stop(2, "this machine has no name yet, so nothing was committed", []string{"git", "config", "metasystem.goal.machine", "NAME"}, repeat)
	}
	lineage := request.OwnerLineage
	if lineage == "" {
		lineage = "human"
	}
	actor := machine + "+" + lineage
	landingTree := settledTree
	if b.prefix != "" {
		resolved := b.git(request.Root, "rev-parse", settledTree+":"+strings.TrimSuffix(b.prefix, "/"))
		if resolved.Code == 0 && len(bytes.TrimSpace(resolved.Stdout)) > 0 {
			landingTree = strings.TrimSpace(string(resolved.Stdout))
		}
	}
	observe := b.observeRequest(landingTree, settledTree, actor)
	decided := decision{provenance: "none change=unknown", verdict: "would-refuse code=evaluator-unavailable",
		code: "evaluator-unavailable", mode: "refuse", refusesAgent: true}
	judge := b.owners.Live()
	judgeTrailer := ""
	if request.Carried != "" {
		live := observe
		live.Judge = "live"
		observed, status := judge.Observe(live)
		read, ok := decisionOf(view(observed))
		if status == 0 && ok {
			decided = read
			executable, err := b.owners.LiveEngine()
			if err != nil {
				return b.failed(1, "the running metasystem program can't be read, so nothing was committed", err)
			}
			digest, err := fileDigest(b.owners, executable)
			if err != nil {
				return b.failed(1, "the running metasystem program can't be read, so nothing was committed", err)
			}
			judgeTrailer = "live sha256=" + digest
		} else {
			liveFailure := observed.Code
			if liveFailure == "" {
				liveFailure = "exit=" + strconv.Itoa(status)
			}
			base, cleanup, err := b.owners.BuildBaseJudge(b.toplevel, b.prefix, b.details)
			if err != nil {
				return b.stop(3, "neither this metasystem nor one built from HEAD could judge the landing, so nothing was committed",
					[]string{"go", "run", "./cmd/devgate", "build"}, repeat, "the base judge build failed: "+err.Error(), "live judge: "+liveFailure)
			}
			defer cleanup()
			judgeTree := b.git(request.Root, "rev-parse", "HEAD^{tree}")
			if judgeTree.Code != 0 {
				b.stop(judgeTree.Code, "HEAD can't be read, so nothing was committed", nil, "", string(judgeTree.Stderr))
				return judgeTree.Code
			}
			baseRequest := observe
			baseRequest.Judge, baseRequest.LiveFailure = "base", liveFailure
			observed, status := base.Observe(baseRequest)
			read, ok := decisionOf(view(observed))
			if status != 0 || !ok {
				return b.stop(3, "neither this metasystem nor one built from HEAD could judge the landing, so nothing was committed",
					[]string{"go", "run", "./cmd/devgate", "build"}, repeat, fmt.Sprintf("the base judge exited %d", status), "live judge: "+liveFailure)
			}
			decided, judge = read, base
			judgeTrailer = "base tree=" + strings.TrimSpace(string(judgeTree.Stdout)) + " sha256=" + base.Digest + " live-failure=" + liveFailure
		}
	} else {
		observed, status := judge.Observe(observe)
		if read, ok := decisionOf(view(observed)); status == 0 && ok {
			decided = read
		}
	}
	if request.Carried != "" && decided.mode == "refuse" {
		details := []string{"verdict: " + decided.verdict}
		if decided.refusal != "" {
			details = append(details, decided.code+": "+decided.refusal)
		}
		if b.stopped.Reason == "" {
			b.stopped.cause = strings.TrimSuffix(decided.code+": "+decided.refusal, ": ")
		}
		return b.stop(3, "the landing check refused this exception landing too: "+oneLine(firstNonEmpty(decided.refusal, decided.code)), nil,
			"record a new exception for what the check refused (--verbose shows its answer)", details...)
	}
	if b.agent && decided.refusesAgent {
		return b.refuseAgent(decided)
	}
	carried, status := b.carriedFacts(decided, judge, settledTree)
	if status != 0 {
		return status
	}
	carried.judge = judgeTrailer
	return b.commit(decided, carried, actor)
}

func view(observed landing.Observation) observationView {
	return observationView{Mode: observed.Mode, RefusesAgent: observed.RefusesAgent, Code: observed.Code,
		Provenance: observed.Provenance, VerdictTrailer: observed.VerdictTrailer, Refusal: observed.Refusal,
		GoalRevision: observed.GoalRevision, Carried: observed.Carried}
}
