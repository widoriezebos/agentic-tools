package main

// These usage strings serve both the complete human page and focused forms.
// Help selectors such as submit are not new execution subcommands.
const (
	reviewGoalUsage         = "metasystem review G [--work NAME] [--dispositions FILE]"
	reviewSubmitUsage       = "metasystem review G --changes|--patch PATCH --brief FILE [--work NAME] [--after COMMIT] [--dispositions FILE]"
	reviewFindingUsage      = "metasystem review G --finding F --test NAME"
	reviewDesignUsage       = "metasystem review design FILE [--goal G] [--dispositions FILE [--after N]] [--retry N]"
	reviewJobUsage          = "metasystem review job J [--dispositions FILE]"
	reviewCommitUsage       = "metasystem review commit SHA --goal G"
	reviewRunUsage          = "metasystem review run RUN [--model MODEL]"
	reviewChangesUsage      = "metasystem review changes --brief FILE [--goal G] [--retry N]"
	reviewDiffUsage         = "metasystem review diff PATCH --brief FILE [--goal G] [--retry N]"
	reviewExplicitGoalUsage = "metasystem review goal G [--work NAME]"
)

func reviewHelpForms() []intentHelpForm {
	return []intentHelpForm{
		{
			name: "goal", purpose: "review a goal's existing built or submitted work for delivery",
			usage: []string{reviewGoalUsage, reviewExplicitGoalUsage},
			inputs: []string{"Goal G with completed work; --work NAME if several items await review.",
				"Use review goal G for any goal id, including design, changes or job."},
			effects:     "May commit and publish built work, start its independent review, and publish a completed clean review. Findings need the author's decisions before delivery.",
			authority:   "Applicable goal claim, approval and review rules still apply. A review never decides its own findings or accepts human-only risk.",
			repetition:  "Repeat to observe or complete the same review. --retry N deliberately retries a stopped examination that failed without findings. Do not combine --retry with --dispositions. Fix accepted material findings with revise G, then review again.",
			flags:       []string{"work", "dispositions", "retry", "model"},
			optionNotes: map[string]intentHelpOptionNote{"dispositions": {description: "the author's decisions for this examination"}, "retry": {description: "one more examination after failed round N is stopped; not with --dispositions"}, "model": {description: "the critic model, subject to roster authorization"}},
			example:     []string{"metasystem", "review", "goal", "G", "--work", "NAME", "--json"},
		},
		{
			name: "submit", purpose: "submit manually written changes as a goal's work for review and delivery",
			usage: []string{reviewSubmitUsage},
			inputs: []string{"Goal G, a nonempty --brief FILE, and exactly one of --changes or --patch PATCH.",
				"--changes captures current tracked and untracked edits; system records and the named brief stay behind.",
				"Use review goal G --changes or --patch for a goal id such as design or changes."},
			effects:     "Claims the goal if needed, commits the supplied work, publishes it, and starts its independent review. This is submission; for feedback without committing use help review changes or help review diff.",
			authority:   "Claim, approval, commit and publication checks apply. The author's findings decisions remain explicit; submission grants no exception or human approval.",
			repetition:  "The same submission rejoins the committed version. A correction names --after COMMIT from status; --work defaults to main. --dispositions supplies decisions. Follow a failed examination's returned review command; --retry is not a submission option.",
			flags:       []string{"changes", "patch", "brief", "work", "after", "dispositions"},
			optionNotes: map[string]intentHelpOptionNote{"brief": {description: "what the submitted work is meant to do; frozen for its review"}, "after": {value: "COMMIT", description: "the commit version this correction replaces, from status"}, "dispositions": {description: "the author's decisions for the submitted work's examination"}},
			example:     []string{"metasystem", "review", "goal", "G", "--changes", "--brief", "brief.md", "--json"},
		},
		{
			name: "design", purpose: "independently critique an existing project design",
			usage: []string{reviewDesignUsage},
			inputs: []string{"FILE is a design in this project's design records; --goal G selects its goal when needed.",
				"Use design G --brief FILE to have a new project design authored."},
			effects:     "Retains and starts an independent critique of the design. Changed designs and their decisions continue the same bounded review; no code is built or landed.",
			authority:   "The author decides findings; critique does not accept the design on the human's behalf. Existing review limits and goal rules apply.",
			repetition:  "Repeat to collect the existing examination. For a changed design supply --dispositions FILE and, if needed, --after N for the answered examination. --retry N retries a stopped failed examination, within the same round limit.",
			flags:       []string{"goal", "dispositions", "after", "retry", "tool-calls"},
			optionNotes: map[string]intentHelpOptionNote{"dispositions": {description: "the author's decisions on the examined design"}, "after": {description: "the examination number the decisions answer"}},
			example:     []string{"metasystem", "review", "design", "plans/designs/example.md", "--goal", "G", "--json"},
		},
		{
			name: "job", purpose: "review completed agent work by its returned job reference",
			usage: []string{reviewJobUsage},
			inputs: []string{"J is a reviewable completed job reference from status work; preserve the full reference.",
				"Its goal and subject come from the recorded work, not a guessed goal or path."},
			effects:     "Starts or collects an independent review. --dispositions FILE applies the author's decisions and completes the review when its requirements are met.",
			authority:   "Only the author decides findings; the reviewer cannot approve itself. Existing job, goal and risk rules still apply.",
			repetition:  "Repeating rejoins the existing review. To fix findings use the returned revise job command with the decisions and correction brief; do not invent a fresh job id.",
			flags:       []string{"dispositions", "tool-calls"},
			optionNotes: map[string]intentHelpOptionNote{"dispositions": {description: "the author's decisions on this job's review"}},
			example:     []string{"metasystem", "review", "job", "J", "--json"},
		},
		{
			name: "commit", purpose: "review one existing committed version of a goal's work",
			usage: []string{reviewCommitUsage},
			inputs: []string{"Commit SHA on the goal's work branch and --goal G.",
				"Supply --brief FILE when the review needs its implementation brief."},
			effects:     "Requests the committed version's independent review, then completes and publishes the review once its findings and checks are resolved. It does not land or conclude the goal.",
			authority:   "The commit must belong to the goal's work. Decisions are the author's; severe or unproven risk still needs the applicable human act.",
			repetition:  "Repeat to observe or collect that review. --dispositions FILE decides its findings. --retry N explicitly retries a stopped examination that failed without findings; preserve the exact commit and goal.",
			flags:       []string{"goal", "brief", "dispositions", "retry", "model"},
			optionNotes: map[string]intentHelpOptionNote{"dispositions": {description: "the author's decisions on this commit's examination"}, "retry": {description: "one more examination after failed round N is stopped"}, "model": {description: "the critic model, subject to roster authorization"}},
			example:     []string{"metasystem", "review", "commit", "SHA", "--goal", "G", "--json"},
		},
		{
			name: "run", purpose: "review a built run's newest round by the run reference it returned",
			usage: []string{reviewRunUsage},
			inputs: []string{"RUN is the run reference build or revise run returned; preserve it exactly.",
				"Its goal, work and round come from the run's own record; review G is the goal-directed route."},
			effects:     "Commits the run's newest round on its goal branch, replacing an earlier round's commit, and requests its independent committed review. It does not land or conclude the goal.",
			authority:   "The run's goal claim and approval rules apply. Decisions are the author's; the reviewer cannot approve itself.",
			repetition:  "Repeat to collect the same review. After revise run the new round is reviewed afresh; the earlier read does not carry over.",
			flags:       []string{"model"},
			optionNotes: map[string]intentHelpOptionNote{"model": {description: "the critic model, subject to roster authorization"}},
			example:     []string{"metasystem", "review", "run", "RUN", "--json"},
		},
		{
			name: "changes", purpose: "get feedback on current changes without submitting them",
			usage: []string{reviewChangesUsage},
			inputs: []string{"--brief FILE states what to examine; tracked and untracked checkout changes are captured.",
				"Optional --goal G associates the feedback; it does not turn feedback into a delivery review."},
			effects:     "Retains a snapshot and starts independent readers. Does not commit, publish work, change the source checkout or grant landing authority. A goal before --changes submits instead; see help review submit.",
			authority:   "Reader launches obey the configured limits. Feedback grants no approval and does not decide findings for the author.",
			repetition:  "The same request rejoins its reads. Use show review REF, wait review REF or stop review REF with the returned REF. --retry N deliberately starts one new attempt after attempt N failed or was stopped.",
			flags:       []string{"brief", "goal", "retry"},
			optionNotes: map[string]intentHelpOptionNote{"brief": {description: "what the independent readers should examine"}, "goal": {description: "optional goal associated with the feedback"}, "retry": {description: "one new feedback attempt after attempt N failed or was stopped"}},
			example:     []string{"metasystem", "review", "changes", "--brief", "brief.md", "--json"},
		},
		{
			name: "diff", purpose: "get feedback on a supplied patch without submitting it",
			usage:       []string{reviewDiffUsage},
			inputs:      []string{"Patch file PATCH and --brief FILE; optional --goal G associates the feedback."},
			effects:     "Retains the patch and starts independent readers. Does not commit, publish work, change the source checkout or grant landing authority. A goal before --patch submits instead; see help review submit.",
			authority:   "Reader launches obey the configured limits. Feedback grants no approval and does not decide findings for the author.",
			repetition:  "The same request rejoins its reads. Use show review REF, wait review REF or stop review REF with the returned REF. --retry N deliberately starts one new attempt after attempt N failed or was stopped.",
			flags:       []string{"brief", "goal", "retry"},
			optionNotes: map[string]intentHelpOptionNote{"brief": {description: "what the independent readers should examine"}, "goal": {description: "optional goal associated with the feedback"}, "retry": {description: "one new feedback attempt after attempt N failed or was stopped"}},
			example:     []string{"metasystem", "review", "diff", "fix.patch", "--brief", "brief.md", "--json"},
		},
		{
			name: "finding", purpose: "record that a review finding's required proof is satisfied",
			usage: []string{reviewFindingUsage},
			inputs: []string{"Goal G, --finding F and --test NAME, or the required fixture proof fields.",
				"Use --review R when several reviews could own the finding; preserve the exact finding and review references."},
			effects:    "Discharges that finding's proof obligation through the existing checks. It does not accept risk, decide other findings, or create successful test evidence.",
			authority:  "The session holding G discharges in its own name. Discharging another session's obligation is a person's act; --by is not proof of permission.",
			repetition: "Use the same finding and actual proof. If refused, inspect the missing evidence or authority; do not invent a test result or a person's identity.",
			flags:      []string{"finding", "test", "review", "implementation-chain", "artifact", "result", "critic", "by"},
			example:    []string{"metasystem", "review", "goal", "G", "--finding", "F", "--test", "TestBehavior", "--json"},
		},
	}
}
