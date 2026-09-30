package landpath

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ledgerfence"
)

// refuseAgent refuses an agent's commit for the deciding verdict: what is
// wrong with this change and the one thing to do, in plain words; the
// verdict, its code, the staged paths and the declarations that would make
// the change lawful are details.
func (b *boundary) refuseAgent(decided decision) int {
	staged := splitNUL(b.git(b.request.Root, "diff", "--cached", "--name-only", "-z", "--").Stdout)
	goal := b.request.Goal
	if goal == "" {
		goal = "G"
	}
	reason, then := "the landing check refused this change, so nothing was committed", "fix what the check refused (--verbose shows its answer), then repeat this command"
	var run []string
	switch decided.code {
	case "evaluator-unavailable":
		reason, run, then = "the landing check crashed or gave no answer, so nothing was committed", []string{"go", "run", "./cmd/devgate", "build"}, "rebuilds it; "+repeat
	case "path-unclassified":
		reason, then = "the change has files no landing rule covers yet, so nothing was committed",
			"add them to internal/pathclass/path-classes.txt, rebuild with go run ./cmd/devgate build, then repeat this command"
	case "ledger-path-not-goal-verb":
		reason, then = "the change edits goal files, which change only through goal commands, so nothing was committed",
			"unstage the goal files and make that change through the goal commands (goal edit, say)"
	case "runtime-path-refused":
		reason, then = "the change includes files the running system writes, which are never landed", "unstage them, then repeat this command"
	case "exact-revert-record-refused":
		reason, then = "a revert may not delete or shorten records, so nothing was committed", "put the record back and add a new record instead"
	case "goal-item-not-held":
		reason, run, then = "goal "+goal+" is not claimed by this session, so nothing was committed",
			[]string{"metasystem", "goal", "claim", goal, "--take-over", "--reason", "TEXT"}, "a person takes it over; then repeat this command"
	case "goal-revision-moved":
		reason, then = "goal "+goal+" was claimed again after this work started, so the work is no longer its own",
			"start the work again under the current claim, or drop it"
	case "goal-binding-missing":
		reason, then = "this change names no goal, and landings here need one", "name the goal: metasystem work land G --message FILE ..."
	case "goal-binding-mismatch":
		reason, then = "this work was started for another goal than "+goal+", so nothing was committed", "land it under the goal it was started for"
	case "record-not-owned":
		reason, then = "the change edits a record another goal owns, so nothing was committed", "unstage that record, or land it from the goal that owns it"
	case "register-carriage-policy-unreadable", "direct-fix-policy-unreadable":
		reason, then = "the landing rules on this branch can't be read, so nothing was committed",
			"repair internal/pathclass/path-classes.txt through a reviewed change"
	case "register-carriage-not-append-only":
		reason, then = "the change rewrites or deletes lines of an append-only record; only new lines may be added",
			"put the existing lines back, keep only the appended ones, then repeat this command"
	}
	details := []string{"verdict: " + decided.verdict}
	if decided.refusal != "" {
		details = append(details, decided.refusal)
	}
	details = append(details, "staged paths:")
	details = append(details, pathLines(staged)...)
	details = append(details, "an agent's change also lands when it declares its reviewed chain (--chain J) or attested commit (--attested C), or when its Change-Class is corrected")
	status := 1
	if b.request.Attested != "" {
		status = 3
	}
	return b.stop(status, reason, run, then, details...)
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
		return facts, b.stop(3, "the landing check's answer doesn't match this exception, so nothing was committed", nil,
			"record the exception again, then repeat this command", "the deciding observation does not bind the requested word, refusal, and ledger")
	}
	workspace, err := judge.Workspace(request.Root, settledTree)
	if err != nil {
		return facts, b.failed(1, "the files this exception covers can't be read, so nothing was committed", err)
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
	unreadable := "the test results for this change can't be read, so nothing was committed"
	testRun := []string{"metasystem", "test", "run", "--goal", request.Goal}
	if json.Unmarshal(encoded, &result) != nil || result.Delivery.Sufficient == nil {
		return facts, b.stop(3, unreadable, testRun, repeat,
			"test verify gave no structured delivery result; an exception never carries unread test results")
	}
	if *result.Delivery.Sufficient {
		if status != 0 {
			return facts, b.stop(1, unreadable, testRun, repeat, "test verify returned success evidence with a failing process status")
		}
		facts.battery = "green"
		return facts, 0
	}
	facts.battery = "red"
	if result.Delivery.MissingGroups == nil {
		return facts, b.stop(1, unreadable, testRun, repeat, "test verify returned an unreadable missing-groups list")
	}
	if result.Delivery.FailingGroups == nil {
		return facts, b.stop(1, unreadable, testRun, repeat, "test verify returned an unreadable failing-groups list")
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
	b.details.Write(committed.Stdout)
	if committed.Code != 0 {
		code := committed.Code
		if code < 0 {
			code = 1
		}
		// A composer enrolled before the engine guard runs the deleted
		// pre-commit-guard.sh and refuses every commit without naming a fix;
		// the refusal a person meets names it (rule H1).
		if strings.Contains(string(committed.Stderr), ledgerfence.RetiredComposerRefusal) {
			return b.stop(code, "an old pre-commit hook refuses every commit here, so nothing was committed", nil,
				ledgerfence.RetiredComposerRemedy, string(committed.Stderr))
		}
		// git's own refusal (a pre-commit hook's, say) already speaks to the
		// person: it is the reason.
		b.stderr.Write(committed.Stderr)
		b.stopped.said("git commit refused the change: "+oneLine(string(committed.Stderr)), nil, "")
		return code
	}
	b.details.Write(committed.Stderr)
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
			return b.stop(1, "the commit was undone: it recorded other files than the ones checked", nil,
				"stage exactly the change, without commit options that pick files, then repeat this command",
				"the commit recorded a tree the landing check never judged (content selected beyond the index)")
		case postTrailer != "":
			return b.stop(1, fmt.Sprintf("the commit was undone: its message ended up with %d %s: lines instead of one", postCount, postTrailer), nil,
				"remove "+postTrailer+": lines from the message (the landing adds them), then repeat this command",
				fmt.Sprintf("expected exactly one %s trailer, found %d", postTrailer, postCount))
		case carriedFailed:
			return b.stop(1, "the commit was undone: the exception's lines in its message came out wrong", nil,
				"remove Carr* lines from the message, then repeat this command", "carried-trailer postcondition: "+carriedDetail)
		default:
			return b.stop(1, "the commit was undone: its message didn't end up with exactly one Goal-Item: line", nil,
				"remove Goal-Item: lines from the message (the landing adds it), then repeat this command")
		}
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
		return b.stop(1, "committed, but not pushed: this checkout isn't on a branch", []string{"git", "switch", "main"},
			"then push the commit", string(symbolic.Stderr))
	}
	branch := strings.TrimSpace(string(symbolic.Stdout))
	if fetched := b.git(root, "fetch", "--quiet", "origin", "+refs/heads/"+branch+":refs/remotes/origin/"+branch); fetched.Code != 0 {
		return b.stop(1, "committed, but not pushed: origin couldn't be reached", []string{"git", "fetch", "origin", branch},
			"once it works, push the commit", string(fetched.Stderr))
	}
	var held bytes.Buffer
	if status := b.owners.Held(root, "refs/remotes/origin/"+branch, "HEAD", "origin", "refs/heads/"+branch, &held, &held); status != 0 {
		return b.stop(1, "committed, but not pushed: the goal is no longer this checkout's to land", nil, "check the goal with metasystem goal show (--verbose shows why)", held.String())
	}
	writeDetails(b.details, held.String())
	pushed := b.git(root, "push", "origin", branch)
	b.details.Write(pushed.Stdout)
	if pushed.Code != 0 {
		return b.stop(1, "committed, but origin refused the push: "+oneLine(string(pushed.Stderr)), []string{"git", "push", "origin", branch},
			"after resolving it; then push transport too", string(pushed.Stderr))
	}
	b.details.Write(pushed.Stderr)
	remotes := b.git(root, "remote")
	for _, remote := range strings.Split(string(remotes.Stdout), "\n") {
		if remote != "transport" {
			continue
		}
		// Transport receives origin's ref, never the local branch.
		var synced bytes.Buffer
		if b.owners.SyncTransport(root, branch, &synced, &synced) != 0 {
			return b.stop(1, "pushed to origin, but the transport copy couldn't be updated",
				[]string{"git", "push", "transport", "refs/remotes/origin/" + branch + ":refs/heads/" + branch}, "after resolving the transport remote",
				synced.String())
		}
		writeDetails(b.details, synced.String())
	}
	return 0
}

// weigh folds the landing into the validation weight, last, after every
// remote accepted it. Weight bookkeeping never refuses a concluded landing.
func (b *boundary) weigh() {
	root := b.request.Root
	numstat := b.git(root, "show", "--no-renames", "--numstat", "-z", "--format=", "HEAD")
	short := strings.TrimSpace(string(b.git(root, "rev-parse", "--short", "HEAD").Stdout))
	if numstat.Code != 0 || b.owners.WeightAdd(root, short, b.prefix, b.request.Goal, numstat.Stdout, b.details, b.details) != 0 {
		writeDetails(b.details, "validation-weight bookkeeping skipped (non-fatal)")
	}
}
