package landpath

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ledgerfence"
)

// refuseAgent writes an agent refusal for the deciding verdict: its cause,
// the staged paths, the repair and the lawful exits.
func (b *boundary) refuseAgent(decided decision) int {
	staged := splitNUL(b.git(b.request.Root, "diff", "--cached", "--name-only", "-z", "--").Stdout)
	repair := ""
	switch decided.code {
	case "evaluator-unavailable":
		fmt.Fprintf(b.stderr, "agent commit refused: the landing evaluator failed or returned an incomplete decision (%s)\n", decided.verdict)
		repair = "restore or rebuild the proof-built landing evaluator, then retry"
	case "path-unclassified":
		fmt.Fprintf(b.stderr, "agent commit refused: the landing contains an unclassified path (%s)\n", decided.verdict)
		if decided.refusal != "" {
			fmt.Fprintln(b.stderr, decided.refusal)
		}
		repair = "classify every named path in scripts/agents/path-classes.txt, then retry"
	case "ledger-path-not-goal-verb":
		fmt.Fprintf(b.stderr, "agent commit refused: ledger paths change only through goal verbs (%s)\n", decided.verdict)
		repair = "use the owning goal verb instead of the commit wrapper"
	case "runtime-path-refused":
		fmt.Fprintf(b.stderr, "agent commit refused: runtime paths cannot be landed (%s)\n", decided.verdict)
		repair = "remove runtime output from the staged tree"
	case "exact-revert-record-refused":
		fmt.Fprintf(b.stderr, "agent commit refused: exact revert cannot delete or truncate records (%s)\n", decided.verdict)
		repair = "restore the record and carry a forward record instead"
	case "goal-item-not-held":
		fmt.Fprintf(b.stderr, "agent commit refused: the Goal-Item is not held by this machine and lineage (%s)\n", decided.verdict)
		repair = "use a goal claimed by this machine and lineage"
	case "goal-revision-moved":
		fmt.Fprintf(b.stderr, "agent commit refused: the Goal-Item's claim revision moved since this chain was dispatched (%s)\n", decided.verdict)
		repair = "the work belongs to a claim that no longer exists; re-dispatch under the current claim, or abandon the work"
	case "goal-binding-missing":
		fmt.Fprintf(b.stderr, "agent commit refused: this landing names no goal and the ledger is not Goal-free (%s)\n", decided.verdict)
		repair = "name the held goal with --goal <id>; a goal-bound chain lands under the goal it was dispatched for"
	case "goal-binding-mismatch":
		fmt.Fprintf(b.stderr, "agent commit refused: the chain was dispatched under a different goal than --goal names (%s)\n", decided.verdict)
		repair = "land the chain under the goal it was dispatched for"
	case "record-not-owned":
		fmt.Fprintf(b.stderr, "agent commit refused: the staged record is not owned by this landing (%s)\n", decided.verdict)
		repair = "carry only new records or records owned by the held goal or actor"
	case "register-carriage-policy-unreadable", "direct-fix-policy-unreadable":
		fmt.Fprintf(b.stderr, "agent commit refused: the base path-class policy is unreadable (%s)\n", decided.verdict)
		repair = "repair the path-class manifest through a reviewed implementation chain"
	case "register-carriage-not-append-only":
		fmt.Fprintf(b.stderr, "agent commit refused: register carriage rewrote or deleted existing record bytes (%s)\n", decided.verdict)
		repair = "restore existing bytes and append complete lines only"
	default:
		fmt.Fprintf(b.stderr, "agent commit refused: landing verdict %s\n", decided.verdict)
	}
	fmt.Fprintln(b.stderr, "staged paths:")
	b.listPaths(staged)
	if repair != "" {
		fmt.Fprintln(b.stderr, repair)
	}
	fmt.Fprintln(b.stderr, "lawful classification exits: declare the reviewed implementation chain with --chain <root-job-id>, declare the attested branch commit with --attested <commit>, or fix the Change-Class classification and retry")
	if b.request.Attested != "" {
		return 3
	}
	return 1
}

// carriedTrailers are the carried facts the boundary stamps.
type carriedTrailers struct {
	workspace, battery, missing, failing, judge string
}

func (c carriedTrailers) batteryTrailer() string {
	if c.battery == "red" {
		return "red missing=" + c.missing + " failing=" + c.failing
	}
	return c.battery
}

// carriedFacts binds a carried decision to the requested word and reads the
// carried workspace and battery through the deciding judge.
func (b *boundary) carriedFacts(decided decision, judge Judge, settledTree string) (carriedTrailers, int) {
	request := b.request
	facts := carriedTrailers{missing: "-", failing: "-"}
	if request.Carried == "" {
		return facts, 0
	}
	if decided.mode != "observe" || decided.code != "human-carried" ||
		!strings.Contains(decided.provenance, " opid="+request.Carried+" ") ||
		!strings.Contains(decided.provenance, " past="+request.CarriedPast+" ") ||
		!strings.Contains(decided.provenance, " ledger="+request.LedgerTip+" ") {
		return facts, b.refuse(3, "carried landing asks: the deciding observation does not bind the requested word, refusal, and ledger")
	}
	workspace, err := judge.Workspace(request.Root, settledTree)
	if err != nil {
		fmt.Fprintln(b.stderr, err)
		return facts, b.refuse(1, "agent commit refused: the carried workspace projection is unreadable")
	}
	facts.workspace = workspace
	encoded, status := judge.VerifyCarried(request.Root, settledTree, request.Goal)
	var result struct {
		Delivery struct {
			Sufficient    *bool     `json:"sufficient"`
			MissingGroups *[]string `json:"missingGroups"`
			FailingGroups *[]string `json:"failingGroups"`
		} `json:"delivery"`
	}
	if json.Unmarshal(encoded, &result) != nil || result.Delivery.Sufficient == nil {
		return facts, b.refuse(3, "test verify failed: no structured delivery result; no word carries an unverified battery; repair the testing tool or its evidence and rerun")
	}
	if *result.Delivery.Sufficient {
		if status != 0 {
			return facts, b.refuse(1, "agent commit refused: test verify returned success evidence with a failing process status")
		}
		facts.battery = "green"
		return facts, 0
	}
	facts.battery = "red"
	if result.Delivery.MissingGroups == nil {
		return facts, b.refuse(1, "agent commit refused: test verify returned an unreadable missing-groups list")
	}
	if result.Delivery.FailingGroups == nil {
		return facts, b.refuse(1, "agent commit refused: test verify returned an unreadable failing-groups list")
	}
	facts.missing, facts.failing = groupList(*result.Delivery.MissingGroups), groupList(*result.Delivery.FailingGroups)
	return facts, 0
}

func groupList(groups []string) string {
	if len(groups) == 0 {
		return "-"
	}
	return strings.Join(groups, ",")
}

func fileDigest(owners Owners, path string) (string, error) {
	data, err := owners.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func countLines(message, pattern string) int {
	return len(regexp.MustCompile(pattern).FindAllStringIndex(message, -1))
}

func countExact(message, line string) int {
	count := 0
	for _, candidate := range strings.Split(message, "\n") {
		if candidate == line {
			count++
		}
	}
	return count
}

var carriedKeys = []string{"Carry", "Carried-By", "Carried-Tree", "Carried-Past", "Carried-Battery", "Carried-Judge", "Carried-Ledger"}

// commitEnvironment is the git commit's environment: this process's, without
// an inherited new-plan acknowledgment, with the caller's additions.
func (b *boundary) commitEnvironment() []string {
	var env []string
	for _, entry := range b.owners.Environ() {
		if strings.HasPrefix(entry, "METASYSTEM_ALLOW_NEW_PLAN=") {
			continue
		}
		env = append(env, entry)
	}
	if b.request.AllowNewPlan {
		env = append(env, "METASYSTEM_ALLOW_NEW_PLAN=1")
	}
	return append(env, b.request.Env...)
}

// commit records the proved index with the stamped trailers, proves the
// recorded commit is exactly what was proved and stamped (rolling it back
// softly otherwise), pushes when asked, and weighs the landing.
func (b *boundary) commit(decided decision, carried carriedTrailers, actor string) int {
	request := b.request
	head := b.git(request.Root, "rev-parse", "--verify", "--quiet", "HEAD")
	provedHead := ""
	if head.Code == 0 {
		provedHead = strings.TrimSpace(string(head.Stdout))
	}
	trailers := []string{"Machine: " + actor, "Landing-Provenance: " + decided.provenance, "Landing-Provenance-Verdict: " + decided.verdict}
	if request.LandedBy != "" {
		trailers = append(trailers, "Landed-By: "+request.LandedBy)
	}
	if request.Goal != "" {
		trailers = append(trailers, "Goal-Item: "+request.Goal)
	}
	var carriedLines []string
	if request.Carried != "" {
		carriedLines = []string{"Carry: " + request.Carried, "Carried-By: " + request.CarriedBy,
			"Carried-Tree: workspace=" + carried.workspace + " project=" + b.provedTreeSettled(),
			"Carried-Past: " + request.CarriedPast, "Carried-Battery: " + carried.batteryTrailer(),
			"Carried-Judge: " + carried.judge, "Carried-Ledger: " + request.LedgerTip}
		trailers = append(trailers, carriedLines[1], carriedLines[0], carriedLines[2], carriedLines[3], carriedLines[4], carriedLines[5], carriedLines[6])
	}
	passes := strings.HasPrefix(decided.verdict, "pass ")
	if request.Goal != "" && passes && decided.goalRevision != "" {
		trailers = append(trailers, "Goal-Revision: "+decided.goalRevision)
	}
	args := []string{"commit"}
	for _, trailer := range trailers {
		args = append(args, "--trailer", trailer)
	}
	args = append(args, "-F", request.MessageFile)
	committed := b.owners.Git(GitCall{Dir: request.Root, Args: args, Env: b.commitEnvironment()})
	b.stdout.Write(committed.Stdout)
	b.stderr.Write(committed.Stderr)
	if committed.Code != 0 {
		// A composer enrolled before the engine guard runs the deleted
		// pre-commit-guard.sh and refuses every commit without naming a fix;
		// the refusal a person meets names it (rule H1).
		if strings.Contains(string(committed.Stderr), ledgerfence.RetiredComposerRefusal) {
			fmt.Fprintln(b.stderr, "commit refused: "+ledgerfence.RetiredComposerRemedy)
		}
		if committed.Code < 0 {
			return 1
		}
		return committed.Code
	}
	landedTree := strings.TrimSpace(string(b.git(request.Root, "rev-parse", "HEAD^{tree}").Stdout))
	message := string(b.git(request.Root, "log", "-1", "--format=%B").Stdout)
	message = strings.TrimSuffix(message, "\n")
	goalItems := countLines(message, `(?im)^Goal-Item:`)
	machines := countLines(message, `(?im)^Machine:`)
	exactMachines := countExact(message, "Machine: "+actor)
	revisions := countLines(message, `(?im)^Goal-Revision:`)
	exactGoalItems, exactRevisions := 0, 0
	if request.Goal != "" {
		exactGoalItems = countExact(message, "Goal-Item: "+request.Goal)
	}
	expectRevision := request.Goal != "" && passes
	if expectRevision && decided.goalRevision != "" {
		exactRevisions = countExact(message, "Goal-Revision: "+decided.goalRevision)
	}
	postTrailer, postCount := "", 0
	switch {
	case machines != 1 || exactMachines != 1:
		postTrailer, postCount = "Machine", machines
	case expectRevision && (revisions != 1 || exactRevisions != 1):
		postTrailer, postCount = "Goal-Revision", revisions
	case !expectRevision && revisions != 0:
		postTrailer, postCount = "Goal-Revision", revisions
	}
	carriedFailed, carriedDetail := false, ""
	if request.Carried != "" {
		for _, key := range carriedKeys {
			if count := countLines(message, `(?m)^`+regexp.QuoteMeta(key)+`:`); count != 1 {
				carriedFailed, carriedDetail = true, fmt.Sprintf("%s count=%d", key, count)
				break
			}
		}
		if !carriedFailed {
			for _, line := range carriedLines {
				if countExact(message, line) != 1 {
					carriedFailed, carriedDetail = true, "wrong carried trailer: "+line
					break
				}
			}
		}
	} else {
		for _, key := range carriedKeys {
			if countLines(message, `(?m)^`+regexp.QuoteMeta(key)+`:`) != 0 {
				carriedFailed, carriedDetail = true, key+" must be absent"
				break
			}
		}
	}
	goalFailed := request.Goal != "" && (goalItems != 1 || exactGoalItems != 1) || request.Goal == "" && goalItems != 0
	if landedTree != b.provedTree || postTrailer != "" || goalFailed || carriedFailed {
		if provedHead != "" {
			b.git(request.Root, "reset", "--soft", provedHead)
		} else {
			b.git(request.Root, "update-ref", "-d", "HEAD")
		}
		switch {
		case landedTree != b.provedTree:
			fmt.Fprintln(b.stderr, "agent commit refused: the commit recorded a tree the static re-proof never judged (content selection beyond the index); the commit was rolled back — stage the exact bytes and commit them plainly")
		case postTrailer != "":
			fmt.Fprintf(b.stderr, "agent commit refused: the final commit message did not contain exactly one byte-exact Goal-Item stamped by --goal; the commit was rolled back; expected exactly one %s trailer, found %d\n", postTrailer, postCount)
		case carriedFailed:
			fmt.Fprintf(b.stderr, "agent commit refused: the final commit message failed the carried-trailer postcondition (%s); the commit was rolled back\n", carriedDetail)
		default:
			fmt.Fprintln(b.stderr, "agent commit refused: the final commit message did not contain exactly one byte-exact Goal-Item stamped by --goal; the commit was rolled back")
		}
		return 1
	}
	if request.Push {
		if status := b.push(); status != 0 {
			return status
		}
	}
	b.weigh()
	return 0
}

// provedTreeSettled is the whole-project tree the carried trailer names.
func (b *boundary) provedTreeSettled() string { return b.provedTree }

// push lands the commit on origin, then on transport when one is declared.
func (b *boundary) push() int {
	root := b.request.Root
	symbolic := b.git(root, "symbolic-ref", "--short", "HEAD")
	if symbolic.Code != 0 {
		b.stderr.Write(symbolic.Stderr)
		return b.refuse(1, "landing push refused: HEAD is not on a branch")
	}
	branch := strings.TrimSpace(string(symbolic.Stdout))
	if fetched := b.git(root, "fetch", "--quiet", "origin", "+refs/heads/"+branch+":refs/remotes/origin/"+branch); fetched.Code != 0 {
		b.stderr.Write(fetched.Stderr)
		return b.refuse(1, "landing push refused: origin could not be fetched; the commit stands locally")
	}
	if status := b.owners.Held(root, "refs/remotes/origin/"+branch, "HEAD", "origin", "refs/heads/"+branch, b.stdout, b.stderr); status != 0 {
		return 1
	}
	pushed := b.git(root, "push", "origin", branch)
	b.stdout.Write(pushed.Stdout)
	b.stderr.Write(pushed.Stderr)
	if pushed.Code != 0 {
		return b.refuse(1, "landing push failed at origin; the commit stands locally — resolve and push both remotes")
	}
	remotes := b.git(root, "remote")
	for _, remote := range strings.Split(string(remotes.Stdout), "\n") {
		if remote != "transport" {
			continue
		}
		// Transport receives origin's ref, never the local branch.
		if b.owners.SyncTransport(root, branch, b.stdout, b.stderr) != 0 {
			return b.refuse(1, "landing push failed at transport with origin already pushed; resolve the transport remote, then mirror origin to it: git fetch origin %s && git push transport refs/remotes/origin/%s:refs/heads/%s", branch, branch, branch)
		}
	}
	return 0
}

// weigh folds the landing into the validation weight, last, after every
// remote accepted it. Weight bookkeeping never refuses a concluded landing.
func (b *boundary) weigh() {
	root := b.request.Root
	numstat := b.git(root, "show", "--no-renames", "--numstat", "-z", "--format=", "HEAD")
	short := strings.TrimSpace(string(b.git(root, "rev-parse", "--short", "HEAD").Stdout))
	if numstat.Code != 0 || b.owners.WeightAdd(root, short, b.prefix, b.request.Goal, numstat.Stdout, b.stdout, b.stderr) != 0 {
		fmt.Fprintln(b.stderr, "validation-weight bookkeeping skipped (non-fatal)")
	}
}
