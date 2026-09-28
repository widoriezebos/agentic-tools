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
	// AllowNewPlan acknowledges a new plan file for the pre-commit guard.
	AllowNewPlan bool
	// Env is added to the git commit's environment (the approver identity
	// a batch landing commits as).
	Env []string
}

var goalIdentifier = regexp.MustCompile(`^[a-z0-9-]+$`)

func (request CommitRequest) landingDeclared() bool {
	return request.Chain != "" || request.DirectFix != "" || request.RevertOf != "" || request.RootJob != "" ||
		request.TestReceipt != "" || request.Recertification != "" || request.Carried != "" ||
		request.Attested != "" || request.AttestedSnapshot != "" || request.AttestedBase != ""
}

// Commit runs the commit boundary and returns the exit status the former
// wrapper returned: 0 committed (and pushed, with Push); 1 refused or failed;
// 2 refused before any effect; 3 a carried landing's ask or an attested
// landing's refusal. Every refusal is written to stderr.
func Commit(owners Owners, request CommitRequest, stdout, stderr io.Writer) int {
	b := &boundary{owners: owners, request: request, stdout: stdout, stderr: stderr}
	return b.run()
}

type boundary struct {
	owners         Owners
	request        CommitRequest
	stdout, stderr io.Writer

	agent            bool
	prefix, toplevel string
	provedTree       string
}

func (b *boundary) refuse(code int, format string, args ...any) int {
	fmt.Fprintf(b.stderr, format+"\n", args...)
	return code
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
			return b.refuse(1, "land refused: brain fence failed")
		}
		if detail != "" {
			return b.refuse(2, "%s", detail)
		}
	}
	if request.HeldEpoch != "" {
		return b.held(request.HeldEpoch)
	}
	epoch, err := b.owners.RequireHolder(request.Root, b.owners.CallerPID, nil)
	if err != nil {
		fmt.Fprintln(b.stderr, err)
		return 1
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
		fmt.Fprintln(b.stderr, heldErr)
		return 1
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
			fmt.Fprintln(b.stderr, err)
			return 1
		}
	} else {
		if epoch != "human" {
			return 2
		}
		if _, err := b.owners.RequireHolder(request.Root, b.owners.CallerPID, nil); err != nil {
			fmt.Fprintln(b.stderr, err)
			return 1
		}
	}
	if b.agent && request.OwnerLineage == "" {
		return b.refuse(2, "agent commit refused: the lease holder has a claim epoch but no owner lineage; export METASYSTEM_OWNER_LINEAGE in the seat's shell")
	}
	if request.Carried != "" && (!request.GoalSet || request.LedgerTip == "" || request.CarriedBy == "" || request.CarriedPast == "") {
		return b.refuse(2, "commit refused: --carried requires --goal, --ledger-tip, --carried-by, and --carried-past")
	}
	if request.GoalSet && (request.Goal == "" || len(request.Goal) > 100 || !goalIdentifier.MatchString(request.Goal)) {
		return b.refuse(2, "commit refused: --goal must be a lowercase kebab identifier of at most 100 characters")
	}
	if status := b.scanMessage(); status != 0 {
		return status
	}
	started, err := b.owners.StartedAt(b.owners.Getpid())
	if err != nil {
		return b.refuse(1, "agent commit wrapper refused: wrapper process start time is unreadable")
	}
	nonce, err := b.owners.TokenNonce()
	if err != nil {
		fmt.Fprintln(b.stderr, err)
		return 1
	}
	token := TokenPath(request.Root)
	if err := b.owners.WriteToken(token, WrapperToken{WrapperPid: b.owners.Getpid(), WrapperPidStartedAt: started,
		Nonce: nonce, CreatedAt: b.owners.Now().UTC().Format("2006-01-02T15:04:05Z")}); err != nil {
		fmt.Fprintln(b.stderr, err)
		return 1
	}
	defer b.owners.RemoveFile(token)
	// A malformed session trailer has slipped through four times (claude.ac
	// for claude.ai): refuse it at the door. The message argument is
	// scanned, not the repository.
	if strings.Contains(request.MessageFile, "claude.ac/") {
		return b.refuse(2, "commit refused: the session trailer says claude.ac — the domain is claude.ai")
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
		return b.refuse(2, "commit refused: -F - is an unscannable commit message source")
	}
	if !b.owners.FileReadable(path) {
		return b.refuse(2, "commit refused: commit message file is not readable: %s", path)
	}
	data, err := b.owners.ReadFile(path)
	if err != nil {
		return b.refuse(2, "commit refused: commit message file is not readable: %s", path)
	}
	text := string(data)
	switch {
	case carriedItemLine.MatchString(text):
		return b.refuse(1, "commit refused: Goal-Item and carried trailers are stamped by the wrapper, never typed")
	case machineLine.MatchString(text):
		return b.refuse(2, "commit refused: Machine is stamped by the wrapper, never typed")
	case goalItemLine.MatchString(text):
		return b.refuse(2, "commit refused: Goal-Item and carried trailers are stamped by the wrapper, never typed")
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

func (b *boundary) listPaths(paths []string) {
	for _, path := range paths {
		fmt.Fprintf(b.stderr, "  %s\n", shellquote.Token(path))
	}
}

func (b *boundary) proveAndCommit() int {
	request := b.request
	prefix := b.git(request.Root, "rev-parse", "--show-prefix")
	top := b.git(request.Root, "rev-parse", "--show-toplevel")
	if prefix.Code != 0 || top.Code != 0 {
		b.stderr.Write(prefix.Stderr)
		b.stderr.Write(top.Stderr)
		return 1
	}
	b.prefix = strings.TrimRight(string(prefix.Stdout), "\n")
	b.toplevel = strings.TrimRight(string(top.Stdout), "\n")
	proved := b.git(b.toplevel, "write-tree")
	if proved.Code != 0 {
		b.stderr.Write(proved.Stderr)
		return b.refuse(1, "agent commit refused: the index cannot be proved as a tree (unmerged entries?)")
	}
	b.provedTree = strings.TrimSpace(string(proved.Stdout))
	// Every installation carries a testing contract (C1): the commit
	// consumes the already-admitted shared result for this exact tree and
	// starts neither tests nor builds. A carried landing defers that
	// judgment into its observer so a readable red battery can be carried.
	if b.owners.ConfValue(request.Root, "testing.contract") == "" {
		return b.refuse(1, "agent commit refused: testing.contract is required in committed metasystem.conf; there is no contract-off landing")
	}
	if request.Carried == "" {
		if status := b.owners.Verify(VerifyRequest{Root: request.Root, Tree: b.provedTree, Goal: request.Goal}, b.stderr, b.stderr); status != 0 {
			return b.refuse(1, "agent commit refused: required shared testing proof is missing or insufficient")
		}
	}
	unbound, err := b.unboundInputs()
	if err != nil {
		fmt.Fprintln(b.stderr, err)
		return 1
	}
	if len(unbound) > 0 {
		fmt.Fprintln(b.stderr, "agent commit refused: the LANDING comparison found projected working-tree bytes that are not what the commit would record at its index endpoint:")
		b.listPaths(unbound)
		return b.refuse(1, "stage, stash, or remove them so the proof binds the bytes the commit records")
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
		fmt.Fprintln(b.stderr, err)
		return 1
	} else if len(selected) > 0 {
		fmt.Fprintln(b.stderr, "agent commit refused: a staged gitlink inside the proof scope carries a nested checkout the committed tree does not record:")
		b.listPaths(selected)
		return 1
	}
	if selected, err := b.owners.SelectLanding(symlinks, b.prefix); err != nil {
		fmt.Fprintln(b.stderr, err)
		return 1
	} else if len(selected) > 0 {
		fmt.Fprintln(b.stderr, "agent commit refused: a critical proof input is a symlink; the proofs would follow bytes the committed tree does not record:")
		b.listPaths(selected)
		return 1
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
		fmt.Fprintln(b.stderr, err)
		return 1
	} else if len(selected) > 0 {
		fmt.Fprintln(b.stderr, "agent commit refused: assume-unchanged or skip-worktree entries hide proof inputs from the divergence closure:")
		b.listPaths(selected)
		return 1
	}
	settled := b.git(b.toplevel, "write-tree")
	if settled.Code != 0 {
		b.stderr.Write(settled.Stderr)
		return b.refuse(1, "agent commit refused: the index cannot be re-proved as a tree")
	}
	settledTree := strings.TrimSpace(string(settled.Stdout))
	settledUnbound, err := b.unboundInputs()
	if err != nil {
		fmt.Fprintln(b.stderr, err)
		return 1
	}
	if settledTree != b.provedTree || len(settledUnbound) > 0 {
		return b.refuse(1, "agent commit refused: the index or a gate input moved while the proof ran; re-stage and retry")
	}
	return b.decideAndCommit(settledTree)
}

// decision is the deciding observation as the boundary reads it.
type decision struct {
	provenance, verdict, code, mode, refusal string
	refusesAgent                             bool
	goalRevision                             string
}

func decisionOf(observed observationView) (decision, bool) {
	refuses := observed.RefusesAgent
	if observed.Provenance == "" || observed.VerdictTrailer == "" || observed.Code == "" ||
		observed.Mode != "observe" && observed.Mode != "refuse" {
		return decision{}, false
	}
	out := decision{provenance: observed.Provenance, verdict: observed.VerdictTrailer, code: observed.Code,
		mode: observed.Mode, refusal: observed.Refusal, refusesAgent: refuses}
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
		return b.refuse(2, "commit refused: no machine nickname is enrolled and hostnames are never published — run  git config metasystem.goal.machine <nickname>  once on this machine")
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
				fmt.Fprintln(b.stderr, err)
				return 1
			}
			digest, err := fileDigest(b.owners, executable)
			if err != nil {
				fmt.Fprintln(b.stderr, err)
				return 1
			}
			judgeTrailer = "live sha256=" + digest
		} else {
			liveFailure := observed.Code
			if liveFailure == "" {
				liveFailure = "exit=" + strconv.Itoa(status)
			}
			base, cleanup, err := b.owners.BuildBaseJudge(b.toplevel, b.prefix, b.stderr)
			if err != nil {
				return b.refuse(3, "no live or base judge decided; the base judge build failed; rebuild and arm an engine at a good commit with steward arm")
			}
			defer cleanup()
			judgeTree := b.git(request.Root, "rev-parse", "HEAD^{tree}")
			if judgeTree.Code != 0 {
				b.stderr.Write(judgeTree.Stderr)
				return judgeTree.Code
			}
			baseRequest := observe
			baseRequest.Judge, baseRequest.LiveFailure = "base", liveFailure
			observed, status := base.Observe(baseRequest)
			read, ok := decisionOf(view(observed))
			if status != 0 || !ok {
				return b.refuse(3, "no live or base judge decided; base judge exit=%d; rebuild and arm an engine at a good commit with steward arm", status)
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
		if decided.refusal != "" {
			return b.refuse(3, "%s: %s", decided.code, decided.refusal)
		}
		return b.refuse(3, "carried landing asks: %s", decided.verdict)
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
		GoalRevision: observed.GoalRevision}
}
