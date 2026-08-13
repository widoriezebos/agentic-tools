package contract

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// A mission contract is a human-authored markdown file with three prose
// sections (Intent, Non-goals, Initial streams) and one fenced `mission`
// key=value block that declares the gate, guards, streams, lifecycle fences,
// and pre-authorized envelope. This file parses and type-checks that grammar,
// seals a contract by measuring a reproducible baseline and binding the frozen
// instruments and priced exposure into a generated `mission-seal` block, and
// runs the preflight that a mission must pass before it may launch: the seal is
// intact, the human approval covers the exact sealed bytes, those bytes are on
// the fetched origin default branch, the gate still measures, the supervisor
// set is armed and fresh, and the mission lease is acquirable.

const (
	contractIDPat        = `[a-z0-9][a-z0-9-]*`
	contractDecimalPat   = `-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?`
	contractNonnegDecPat = `(?:0|[1-9][0-9]*)(?:\.[0-9]+)?`
	contractPosDecPat    = `(?:0\.[0-9]*[1-9][0-9]*|[1-9][0-9]*(?:\.[0-9]+)?)`
)

var (
	contractApprovalRe         = regexp.MustCompile(`^Approval: name=([^;\n]+); date=(\d{4}-\d{2}-\d{2}); contract-sha256=([0-9a-f]{64})$`)
	contractMetricRe           = regexp.MustCompile(`^metric=(` + contractIDPat + `)=(` + contractDecimalPat + `)$`)
	contractThresholdRe        = regexp.MustCompile(`^(>=|<=|>|<)(` + contractDecimalPat + `)$`)
	contractThresholdGrammarRe = regexp.MustCompile(`^(?:>=|<=|>|<)` + contractDecimalPat + `$`)
	contractNonnegDecRe        = regexp.MustCompile(`^` + contractNonnegDecPat + `$`)
	contractDecimalRe          = regexp.MustCompile(`^` + contractDecimalPat + `$`)
	contractPosDecRe           = regexp.MustCompile(`^` + contractPosDecPat + `$`)
	contractGateThresholdRe    = regexp.MustCompile(`^gate\.threshold\.(` + contractIDPat + `)$`)
	contractGateNoiseRe        = regexp.MustCompile(`^gate\.noise-floor\.(` + contractIDPat + `)$`)
	contractGuardRe            = regexp.MustCompile(`^guard\.(` + contractIDPat + `)\.(command|floor|noise)$`)
	contractStreamRe           = regexp.MustCompile(`^stream\.(` + contractIDPat + `)$`)
	contractEnvelopeRe         = regexp.MustCompile(`^envelope\.(` + contractIDPat + `)$`)
	contractLiteralTokenRe     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/@:+<>=-]*$`)
	contractDispatchModelRe    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)
	contractHostRe             = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]*$`)
	contractExposureRe         = regexp.MustCompile(`^[A-Z]{3}:(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
	contractGateRefRe          = regexp.MustCompile(`^[^\s\x00]+$`)
	contractNextHeadingRe      = regexp.MustCompile(`(?m)^# .+$`)
	contractFenceStripRe       = regexp.MustCompile("(?sm)```(?:mission|mission-seal)[ \\t]*\\n.*?^```[ \\t]*$")
	contractProjectRuleRe      = regexp.MustCompile("^\\| `(" + contractIDPat + ")` \\|.*\\| (yes|no) \\|")
	contractShaRangeRe         = regexp.MustCompile(`^[0-9a-f]{40,64}$`)
	contractDigitsRe           = regexp.MustCompile(`^[0-9]+$`)
)

// contractScalars are the single-valued keys every contract must declare.
var contractScalars = []string{
	"gate.command", "gate.ref", "gate.paths", "truth.paths", "truth.certification",
	"gate.direction", "guard.cadence", "ledger.cycle-budget", "ledger.no-gain-budget",
	"fence.wall-clock-hours", "fence.cycles", "fence.jobs", "fence.concurrency",
	"fence.job-cap-min", "host.runtime", "host.model", "host.turn-cap-min", "exposure",
}

var contractScalarSet = func() map[string]bool {
	set := make(map[string]bool, len(contractScalars))
	for _, key := range contractScalars {
		set[key] = true
	}
	return set
}()

// contractIntegerKeys are the scalar keys that must be positive integers.
var contractIntegerKeys = []string{
	"guard.cadence", "ledger.cycle-budget", "ledger.no-gain-budget", "fence.cycles",
	"fence.jobs", "fence.concurrency", "fence.job-cap-min", "host.turn-cap-min",
}

// contractDoc is a parsed mission contract: the raw bytes, the decoded text,
// the authored key=value block, the generated seal block (empty when unsealed),
// and the parsed human approval line (nil when unsigned). The approval slice,
// when present, holds the full match plus the name, date, and recorded digest.
type contractDoc struct {
	path     string
	text     string
	rawBytes []byte
	values   map[string]string
	sealed   map[string]string
	approval []string
}

// contractSealField is one generated seal key and its value, kept in the order
// the seal is assembled so integrity comparisons are deterministic.
type contractSealField struct {
	key   string
	value string
}

// Validate parses and type-checks a mission contract, returning the
// resolved path so a caller can report exactly which file it accepted, plus
// calibration warnings that never refuse the contract.
func Validate(path string) (string, []string, error) {
	doc, _, _, err := contractLoad(path)
	if err != nil {
		return "", nil, err
	}
	return doc.path, doc.calibrationWarnings(), nil
}

// calibrationWarnings flags budget sizings the stop-loss design advises
// against without refusing them: an unattended mission's no-gain budget is a
// last defense sized in the order of the cycle fence, so a budget below half
// the fence warns, naming plans/stop-loss-core.md.
func (d *contractDoc) calibrationWarnings() []string {
	noGain, errNoGain := strconv.Atoi(d.values["ledger.no-gain-budget"])
	cycles, errCycles := strconv.Atoi(d.values["fence.cycles"])
	if errNoGain != nil || errCycles != nil {
		return nil
	}
	if 2*noGain < cycles {
		return []string{fmt.Sprintf(
			"ledger.no-gain-budget=%d is below half of fence.cycles=%d; the stop-loss is a last defense sized in the order of the cycle fence (plans/stop-loss-core.md)",
			noGain, cycles)}
	}
	return nil
}

// Seal measures the reproducible baseline and writes the generated
// mission-seal block, returning the digest a human signs.
func Seal(path string) (string, error) {
	doc, repo, projectRoot, err := contractLoad(path)
	if err != nil {
		return "", err
	}
	return doc.seal(repo, projectRoot)
}

// Preflight runs the full launch gate. On success it returns the
// mission id and the sha256 of the approved raw bytes, and — when an output
// path is given — atomically records those exact bytes there.
func Preflight(path, verifiedBytesOutput string) (missionID, rawSHA string, err error) {
	doc, repo, projectRoot, err := contractLoad(path)
	if err != nil {
		return "", "", err
	}
	if err := doc.preflight(repo, projectRoot); err != nil {
		return "", "", err
	}
	rawSHA = sha256Hex(string(doc.rawBytes))
	if verifiedBytesOutput != "" {
		if err := atomicWriteText(verifiedBytesOutput, string(doc.rawBytes)); err != nil {
			return "", "", err
		}
	}
	id, err := contractMissionID(doc.path)
	if err != nil {
		return "", "", err
	}
	return id, rawSHA, nil
}

// contractLoad resolves the path, parses the contract, locates its repository
// and project root, and type-checks it — the shared preamble every verb runs.
func contractLoad(path string) (*contractDoc, string, string, error) {
	resolved := resolvePath(path)
	doc, err := contractRead(resolved)
	if err != nil {
		return nil, "", "", err
	}
	repo, err := contractRepositoryFor(resolved)
	if err != nil {
		return nil, "", "", err
	}
	projectRoot := contractProjectRoot(resolved, repo)
	if err := doc.validate(projectRoot); err != nil {
		return nil, "", "", err
	}
	return doc, repo, projectRoot, nil
}

// contractRead decodes a contract and splits it into its authored block, its
// optional seal block, and its optional approval line, enforcing that each
// appears at most the permitted number of times.
func contractRead(path string) (*contractDoc, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, stateErr("cannot read contract: %v", err)
	}
	if !isValidUTF8(raw) {
		return nil, stateErr("cannot read contract: contract is not UTF-8")
	}
	text := string(raw)
	authored := contractFencedBlocks(text, authoredBlockRe)
	seals := contractFencedBlocks(text, sealBlockRe)
	if len(authored) != 1 {
		return nil, stateErr("contract must contain exactly one fenced mission key=value block")
	}
	if len(seals) > 1 {
		return nil, stateErr("contract contains more than one generated mission-seal block")
	}
	var approvalLines []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "Approval:") {
			approvalLines = append(approvalLines, line)
		}
	}
	if len(approvalLines) > 1 {
		return nil, stateErr("contract contains more than one approval line")
	}
	var approval []string
	if len(approvalLines) == 1 {
		approval = contractApprovalRe.FindStringSubmatch(approvalLines[0])
		if approval == nil {
			return nil, stateErr("approval line has invalid grammar")
		}
	}
	values, err := contractParseKeyValues(authored[0], "mission block")
	if err != nil {
		return nil, err
	}
	sealed := map[string]string{}
	if len(seals) == 1 {
		sealed, err = contractParseKeyValues(seals[0], "mission-seal block")
		if err != nil {
			return nil, err
		}
	}
	return &contractDoc{path: path, text: text, rawBytes: raw, values: values, sealed: sealed, approval: approval}, nil
}

// contractFencedBlocks returns the bodies of every fenced block the regex
// matches.
func contractFencedBlocks(text string, re *regexp.Regexp) []string {
	var out []string
	for _, match := range re.FindAllStringSubmatch(text, -1) {
		out = append(out, match[1])
	}
	return out
}

// contractParseKeyValues reads a fenced block's non-blank lines into a map,
// rejecting non-key=value lines, empty or whitespace-padded keys/values, and
// repeated keys.
func contractParseKeyValues(block, label string) (map[string]string, error) {
	values := map[string]string{}
	for i, raw := range strings.Split(block, "\n") {
		number := i + 1
		if strings.TrimSpace(raw) == "" {
			continue
		}
		if !strings.Contains(raw, "=") {
			return nil, stateErr("%s line %d is not key=value", label, number)
		}
		key, value, _ := strings.Cut(raw, "=")
		if key != strings.TrimSpace(key) || key == "" || value != strings.TrimSpace(value) || value == "" {
			return nil, stateErr("%s line %d has an empty or whitespace-padded key/value", label, number)
		}
		if _, exists := values[key]; exists {
			return nil, stateErr("%s repeats key: %s", label, key)
		}
		values[key] = value
	}
	return values, nil
}

// contractSectionBody returns the prose under a `# heading` section with any
// fenced mission blocks removed, requiring the section to appear exactly once
// and to carry prose.
func contractSectionBody(text, heading string) (string, error) {
	re := regexp.MustCompile(`(?m)^# ` + regexp.QuoteMeta(heading) + `[ \t]*$`)
	locs := re.FindAllStringIndex(text, -1)
	if len(locs) != 1 {
		return "", stateErr("contract must contain exactly one '# %s' section", heading)
	}
	start := locs[0][1]
	rest := text[start:]
	end := len(text)
	if next := contractNextHeadingRe.FindStringIndex(rest); next != nil {
		end = start + next[0]
	}
	body := contractFenceStripRe.ReplaceAllString(text[start:end], "")
	if strings.TrimSpace(body) == "" {
		return "", stateErr("'# %s' must contain prose", heading)
	}
	return body, nil
}

// validate enforces the whole authored grammar: the prose sections, the
// required scalars and their types, the metric/guard/stream families, and the
// pre-authorized envelope against the project's policy.
func (d *contractDoc) validate(projectRoot string) error {
	for _, heading := range []string{"Intent", "Non-goals", "Initial streams"} {
		if _, err := contractSectionBody(d.text, heading); err != nil {
			return err
		}
	}
	values := d.values
	var missing []string
	for _, key := range contractScalars {
		if _, ok := values[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return stateErr("mission contract is missing required key(s): %s", strings.Join(missing, ", "))
	}

	thresholds := map[string]string{}
	noises := map[string]string{}
	guards := map[string]map[string]string{}
	streams := map[string]string{}
	envelopes := map[string]string{}
	for key, value := range values {
		switch {
		case contractScalarSet[key]:
			continue
		case strings.HasPrefix(key, "cap.min."):
			if err := contractValidatePairCap(key, value); err != nil {
				return err
			}
		case strings.HasPrefix(key, "patience."):
			if err := contractValidatePatience(key, value); err != nil {
				return err
			}
		case contractGateThresholdRe.MatchString(key):
			thresholds[contractGateThresholdRe.FindStringSubmatch(key)[1]] = value
		case contractGateNoiseRe.MatchString(key):
			noises[contractGateNoiseRe.FindStringSubmatch(key)[1]] = value
		case contractGuardRe.MatchString(key):
			m := contractGuardRe.FindStringSubmatch(key)
			if guards[m[1]] == nil {
				guards[m[1]] = map[string]string{}
			}
			guards[m[1]][m[2]] = value
		case contractStreamRe.MatchString(key):
			streams[contractStreamRe.FindStringSubmatch(key)[1]] = value
		case contractEnvelopeRe.MatchString(key):
			envelopes[contractEnvelopeRe.FindStringSubmatch(key)[1]] = value
		default:
			return stateErr("mission contract has an unknown key: %s", key)
		}
	}

	if len(thresholds) == 0 {
		return stateErr("mission contract must declare at least one gate.threshold.<metric>")
	}
	if !contractSameKeySet(thresholds, noises) {
		return stateErr("every gate threshold must have exactly one matching noise floor")
	}
	for metric, threshold := range thresholds {
		if !contractThresholdGrammarRe.MatchString(threshold) {
			return stateErr("gate.threshold.%s must be >=, <=, >, or < followed by a decimal", metric)
		}
		if !contractNonnegDecRe.MatchString(noises[metric]) {
			return stateErr("gate.noise-floor.%s must be a non-negative decimal", metric)
		}
	}

	if len(guards) == 0 {
		return stateErr("mission contract must declare at least one guard")
	}
	for name, fields := range guards {
		if len(fields) != 3 || fields["command"] == "" || fields["floor"] == "" || fields["noise"] == "" {
			return stateErr("guard.%s must declare command, floor, and noise", name)
		}
		if strings.TrimSpace(fields["command"]) == "" {
			return stateErr("guard.%s.command must not be empty", name)
		}
		if !contractDecimalRe.MatchString(fields["floor"]) {
			return stateErr("guard.%s.floor must be a decimal", name)
		}
		if !contractNonnegDecRe.MatchString(fields["noise"]) {
			return stateErr("guard.%s.noise must be a non-negative decimal", name)
		}
	}

	if len(streams) == 0 {
		return stateErr("mission contract must declare at least one non-empty stream.<id>")
	}
	for _, goal := range streams {
		if strings.TrimSpace(goal) == "" {
			return stateErr("mission contract must declare at least one non-empty stream.<id>")
		}
	}

	if err := contractValidateEnvelopes(envelopes, projectRoot); err != nil {
		return err
	}

	if err := d.validateScalars(); err != nil {
		return err
	}
	return d.validateApproval()
}

// contractValidatePairCap checks that a per-pair cap key is canonical for its
// runtime and model and carries a positive integer.
func contractValidatePairCap(key, value string) error {
	parts := strings.SplitN(key, ".", 4)
	if len(parts) != 4 || !idRe.MatchString(parts[2]) {
		return stateErr("mission contract has an invalid pair-cap key: %s", key)
	}
	canonical := config.CanonicalModel(parts[3])
	expected := "cap.min." + parts[2] + "." + canonical
	if canonical == "" || key != expected {
		return stateErr("mission contract pair-cap key %s is not canonical; use %s", key, expected)
	}
	if !positiveIntRe.MatchString(value) {
		return stateErr("%s must be a positive integer", key)
	}
	return nil
}

// contractValidatePatience checks a patience-floor entry
// (plans/patience-satellite-4.md): patience.rounds.<role>.<runtime>.<model>,
// role and runtime in the identifier grammar, the model a canonical key in
// the cap-key encoding, the floor a positive integer counted in value-barren
// rounds. Floors exist only here — no conf, local, environment, or
// per-dispatch surface.
func contractValidatePatience(key, value string) error {
	parts := strings.SplitN(key, ".", 5)
	if len(parts) != 5 || parts[1] != "rounds" || !idRe.MatchString(parts[2]) || !idRe.MatchString(parts[3]) {
		return stateErr("mission contract has an invalid patience key: %s (use patience.rounds.<role>.<runtime>.<model>)", key)
	}
	canonical := config.CanonicalModel(parts[4])
	expected := "patience.rounds." + parts[2] + "." + parts[3] + "." + canonical
	if canonical == "" || key != expected {
		return stateErr("mission contract patience key %s is not canonical; use %s", key, expected)
	}
	if !positiveIntRe.MatchString(value) {
		return stateErr("%s must be a positive integer", key)
	}
	return nil
}

// contractValidateEnvelopes checks that every declared envelope category is
// pre-authorizable per project policy and carries a bounded literal-token list.
func contractValidateEnvelopes(envelopes map[string]string, projectRoot string) error {
	if len(envelopes) == 0 {
		return stateErr("mission contract must declare at least one bounded envelope.<category>")
	}
	if _, ok := envelopes["tier-move"]; ok {
		return stateErr("envelope.tier-move is retired; use envelope.dispatch-allow with exact runtime:model pairs")
	}
	permitted, err := contractPreauthCategories(projectRoot)
	if err != nil {
		return err
	}
	for category, bound := range envelopes {
		if !permitted[category] {
			return stateErr("envelope category is not marked pre-authorizable: %s", category)
		}
		if category == "dispatch-allow" {
			if _, err := contractValidateDispatchAllow(bound); err != nil {
				return err
			}
			continue
		}
		if _, err := contractValidateLiteralTokens(bound, "envelope."+category); err != nil {
			return err
		}
	}
	return nil
}

// validateScalars type-checks the single-valued keys.
func (d *contractDoc) validateScalars() error {
	values := d.values
	if c := values["truth.certification"]; c != "candidate" && c != "certified" {
		return stateErr("truth.certification must be candidate or certified")
	}
	if dir := values["gate.direction"]; dir != "max" && dir != "min" {
		return stateErr("gate.direction must be max or min")
	}
	if strings.TrimSpace(values["gate.command"]) == "" || strings.Contains(values["gate.command"], "\x00") {
		return stateErr("gate.command must be one non-empty command")
	}
	if strings.HasPrefix(values["gate.ref"], "-") || !contractGateRefRe.MatchString(values["gate.ref"]) {
		return stateErr("gate.ref must be one non-empty git commit-ish")
	}
	if _, err := contractValidateGlobs(values["gate.paths"], "gate.paths"); err != nil {
		return err
	}
	if _, err := contractValidateGlobs(values["truth.paths"], "truth.paths"); err != nil {
		return err
	}
	for _, key := range contractIntegerKeys {
		if !positiveIntRe.MatchString(values[key]) {
			return stateErr("%s must be a positive integer", key)
		}
	}
	if !contractPosDecRe.MatchString(values["fence.wall-clock-hours"]) {
		return stateErr("fence.wall-clock-hours must be a positive decimal")
	}
	for _, key := range []string{"host.runtime", "host.model"} {
		if !contractHostRe.MatchString(values[key]) {
			return stateErr("%s must be one literal id", key)
		}
	}
	if !contractExposureRe.MatchString(values["exposure"]) {
		return stateErr("exposure must be a human-priced amount in CURRENCY:amount form")
	}
	return nil
}

// validateApproval checks a present approval's name and date; an absent
// approval is legal at authoring time.
func (d *contractDoc) validateApproval() error {
	if d.approval == nil {
		return nil
	}
	name := d.approval[1]
	if name != strings.TrimSpace(name) {
		return stateErr("approval name has leading/trailing whitespace or control characters")
	}
	for _, r := range name {
		if r < 32 {
			return stateErr("approval name has leading/trailing whitespace or control characters")
		}
	}
	if _, err := time.Parse("2006-01-02", d.approval[2]); err != nil {
		return stateErr("approval date is not a real YYYY-MM-DD date")
	}
	return nil
}

// contractPreauthCategories reads the project policy and returns the envelope
// categories it marks pre-authorizable.
func contractPreauthCategories(projectRoot string) (map[string]bool, error) {
	data, err := os.ReadFile(filepath.Join(projectRoot, "docs", "project-rules.md"))
	if err != nil {
		return nil, stateErr("cannot read envelope policy: %v", err)
	}
	categories := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if m := contractProjectRuleRe.FindStringSubmatch(line); m != nil && m[2] == "yes" {
			categories[m[1]] = true
		}
	}
	if len(categories) == 0 {
		return nil, stateErr("docs/project-rules.md marks no mission envelope category pre-authorizable")
	}
	return categories, nil
}

// contractValidateGlobs splits a comma-separated glob list, rejecting absolute,
// backslashed, or traversing repository-relative globs.
func contractValidateGlobs(value, key string) ([]string, error) {
	var result []string
	for _, raw := range strings.Split(value, ",") {
		item := strings.TrimSpace(raw)
		if item == "" || strings.HasPrefix(item, "/") || strings.Contains(item, "\\") {
			return nil, stateErr("%s must be comma-separated repository-relative globs", key)
		}
		for _, segment := range strings.Split(item, "/") {
			if segment == ".." {
				return nil, stateErr("%s contains an unsafe repository-relative glob: %s", key, item)
			}
		}
		for _, r := range item {
			if r < 32 {
				return nil, stateErr("%s contains an unsafe repository-relative glob: %s", key, item)
			}
		}
		result = append(result, item)
	}
	return result, nil
}

// contractValidateLiteralTokens splits a comma-separated token list and rejects
// any empty, non-literal, or unbounded token.
func contractValidateLiteralTokens(value, key string) ([]string, error) {
	parts := strings.Split(value, ",")
	tokens := make([]string, len(parts))
	for i, part := range parts {
		tokens[i] = strings.TrimSpace(part)
	}
	unbounded := map[string]bool{"all": true, "any": true, "everything": true, "unbounded": true, "unlimited": true}
	for _, token := range tokens {
		if token == "" {
			return nil, stateErr("%s must have a bounded comma-separated literal-token list", key)
		}
	}
	for _, token := range tokens {
		if !contractLiteralTokenRe.MatchString(token) || unbounded[strings.ToLower(token)] {
			return nil, stateErr("%s has an unbounded or non-literal token: %s", key, token)
		}
	}
	return tokens, nil
}

// contractValidateDispatchAllow checks that every token is an exact
// runtime:model pair.
func contractValidateDispatchAllow(value string) ([]string, error) {
	pairs, err := contractValidateLiteralTokens(value, "envelope.dispatch-allow")
	if err != nil {
		return nil, err
	}
	for _, pair := range pairs {
		runtime, model, found := strings.Cut(pair, ":")
		if !found || !idRe.MatchString(runtime) || !contractDispatchModelRe.MatchString(model) {
			return nil, stateErr("envelope.dispatch-allow must be a comma-separated list of exact runtime:model pairs")
		}
	}
	return pairs, nil
}

func contractSameKeySet(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key := range a {
		if _, ok := b[key]; !ok {
			return false
		}
	}
	return true
}

// contractCanonicalSignedBytes is the byte image an approval signs: every line
// stripped of trailing spaces and tabs, approval lines removed, and trailing
// whitespace trimmed off the whole. Signing this rather than the raw file lets
// the approval line be appended without invalidating itself.
func contractCanonicalSignedBytes(text string) []byte {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "Approval:") {
			continue
		}
		lines = append(lines, strings.TrimRight(line, " \t"))
	}
	canonical := strings.TrimRightFunc(strings.Join(lines, "\n"), unicode.IsSpace)
	return []byte(canonical)
}

// hash is the digest a human approval records.
func (d *contractDoc) hash() string {
	return sha256Hex(string(contractCanonicalSignedBytes(d.text)))
}

// --- git-backed instrument freezing ---

// contractGitTrim runs a git command and returns its trimmed stdout.
func contractGitTrim(repo string, args ...string) (string, error) {
	out, err := gitOutput(repo, args...)
	return strings.TrimSpace(out), err
}

// contractTreePaths lists every path recorded at a ref.
func contractTreePaths(repo, ref string) ([]string, error) {
	out, err := gitOutput(repo, "ls-tree", "-r", "--name-only", "-z", ref)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, path := range strings.Split(out, "\x00") {
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

// contractExpandPaths resolves project-relative globs against the tree at ref,
// returning the sorted repository-relative paths they select. A glob that
// matches nothing is an error, so an instrument set can never silently shrink.
func contractExpandPaths(repo, projectRoot, ref string, globs []string, label string) ([]string, error) {
	candidates, err := contractTreePaths(repo, ref)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(resolvePath(repo), resolvePath(projectRoot))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, stateErr("metasystem project root is outside its git repository")
	}
	prefix := ""
	if rel != "." {
		prefix = strings.TrimRight(filepath.ToSlash(rel), "/") + "/"
	}
	type candidate struct{ relative, full string }
	var projectCandidates []candidate
	for _, path := range candidates {
		if prefix == "" || strings.HasPrefix(path, prefix) {
			projectCandidates = append(projectCandidates, candidate{relative: path[len(prefix):], full: path})
		}
	}
	selected := map[string]bool{}
	for _, pattern := range globs {
		re, err := contractGlobRegex(pattern)
		if err != nil {
			return nil, err
		}
		matched := false
		for _, c := range projectCandidates {
			if re.MatchString(c.relative) {
				selected[c.full] = true
				matched = true
			}
		}
		if !matched {
			return nil, stateErr("%s glob matches no path at gate.ref: %s", label, pattern)
		}
	}
	out := make([]string, 0, len(selected))
	for path := range selected {
		out = append(out, path)
	}
	sort.Strings(out)
	return out, nil
}

// contractGlobRegex compiles a shell-style path glob where `*` and `?` span any
// characters, including the path separator, matching the whole candidate.
func contractGlobRegex(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("(?s)^")
	for i := 0; i < len(pattern); {
		c := pattern[i]
		i++
		switch c {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '[':
			j := i
			if j < len(pattern) && pattern[j] == '!' {
				j++
			}
			if j < len(pattern) && pattern[j] == ']' {
				j++
			}
			for j < len(pattern) && pattern[j] != ']' {
				j++
			}
			if j >= len(pattern) {
				b.WriteString(`\[`)
				continue
			}
			stuff := strings.ReplaceAll(pattern[i:j], `\`, `\\`)
			i = j + 1
			switch {
			case stuff[0] == '!':
				stuff = "^" + stuff[1:]
			case stuff[0] == '^' || stuff[0] == '[':
				stuff = `\` + stuff
			}
			b.WriteString("[")
			b.WriteString(stuff)
			b.WriteString("]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

// contractManifestHash folds each path and its content digest into one hash,
// binding a set of files at a ref to a single value.
func contractManifestHash(repo, ref string, paths []string) (string, error) {
	digest := sha256.New()
	for _, path := range paths {
		content, err := gitOutput(repo, "show", ref+":"+path)
		if err != nil {
			return "", err
		}
		inner := sha256.Sum256([]byte(content))
		digest.Write([]byte(path))
		digest.Write([]byte{0})
		digest.Write(inner[:])
		digest.Write([]byte{0})
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// contractThresholdPasses reports whether a measured value satisfies a
// comparison expression such as ">=1".
func contractThresholdPasses(expression, value string) (bool, error) {
	measured, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return false, stateErr("gate metric is not numeric: %s", value)
	}
	return ThresholdPasses(expression, measured)
}

// ThresholdPasses reports whether a measured value satisfies a sealed gate
// threshold expression such as ">=1". The stop-loss replay counts thresholds
// met with exactly this comparison, so the fuse and the gate can never
// disagree about what "met" means.
func ThresholdPasses(expression string, measured float64) (bool, error) {
	m := contractThresholdRe.FindStringSubmatch(expression)
	if m == nil {
		return false, stateErr("gate threshold expression is invalid: %s", expression)
	}
	target, err := strconv.ParseFloat(m[2], 64)
	if err != nil {
		return false, stateErr("gate threshold target is invalid: %s", expression)
	}
	switch m[1] {
	case ">=":
		return measured >= target, nil
	case "<=":
		return measured <= target, nil
	case ">":
		return measured > target, nil
	default:
		return measured < target, nil
	}
}

// thresholdNames returns the sorted metric names the gate must report.
func (d *contractDoc) thresholdNames() []string {
	var names []string
	for key := range d.values {
		if strings.HasPrefix(key, "gate.threshold.") {
			names = append(names, strings.TrimPrefix(key, "gate.threshold."))
		}
	}
	sort.Strings(names)
	return names
}

// runGate measures the pinned candidate against the frozen instruments in
// its own throwaway worktree — materialized and executed through the same
// two recipes every measurement uses (materializeCandidate, measureCommand)
// so the worktree and execution behavior have exactly one home each. It
// returns the reported metrics and how many thresholds they missed.
func (d *contractDoc) runGate(repo, projectRoot, candidateSHA, gateRef string) (map[string]string, int, error) {
	values := d.values
	commandRoot, cleanup, err := d.materializeCandidate(repo, projectRoot, candidateSHA, gateRef)
	if err != nil {
		return nil, 0, err
	}
	defer cleanup()

	capMin, _ := strconv.Atoi(values["fence.job-cap-min"])
	metrics, code, timedOut, err := measureCommand(commandRoot, values["gate.command"], capMin)
	if err != nil {
		return nil, 0, err
	}
	if timedOut {
		return nil, 0, stateErr("gate measurement exceeded named fence.job-cap-min ceiling (%sm)", values["fence.job-cap-min"])
	}
	if code != 0 {
		return nil, 0, stateErr("gate measurement failed with exit %d", code)
	}

	names := d.thresholdNames()
	var missingMetrics []string
	for _, name := range names {
		if _, ok := metrics[name]; !ok {
			missingMetrics = append(missingMetrics, name)
		}
	}
	if len(missingMetrics) > 0 {
		return nil, 0, stateErr("gate output omitted declared metric(s): %s", strings.Join(missingMetrics, ", "))
	}
	failures := 0
	reported := map[string]string{}
	for _, name := range names {
		reported[name] = metrics[name]
		pass, err := contractThresholdPasses(values["gate.threshold."+name], metrics[name])
		if err != nil {
			return nil, 0, err
		}
		if !pass {
			failures++
		}
	}
	return reported, failures, nil
}

// candidateBranch names the branch the candidate lives on — the sealed branch
// when present, otherwise the checkout's current branch.
func (d *contractDoc) candidateBranch(repo string) (string, error) {
	branch := d.sealed["candidate.branch"]
	if branch == "" {
		current, err := contractGitTrim(repo, "branch", "--show-current")
		if err != nil {
			return "", err
		}
		branch = current
	}
	if branch == "" {
		return "", stateErr("candidate must be a named branch")
	}
	return branch, nil
}

// restoredPaths is the sorted union of the gate and truth paths at the gate ref.
func (d *contractDoc) restoredPaths(repo, projectRoot, gateRef string) ([]string, error) {
	gatePaths, err := d.gatePaths(repo, projectRoot, gateRef)
	if err != nil {
		return nil, err
	}
	truthPaths, err := d.truthPaths(repo, projectRoot, gateRef)
	if err != nil {
		return nil, err
	}
	union := map[string]bool{}
	for _, path := range gatePaths {
		union[path] = true
	}
	for _, path := range truthPaths {
		union[path] = true
	}
	out := make([]string, 0, len(union))
	for path := range union {
		out = append(out, path)
	}
	sort.Strings(out)
	return out, nil
}

func (d *contractDoc) gatePaths(repo, projectRoot, gateRef string) ([]string, error) {
	globs, err := contractValidateGlobs(d.values["gate.paths"], "gate.paths")
	if err != nil {
		return nil, err
	}
	return contractExpandPaths(repo, projectRoot, gateRef, globs, "gate.paths")
}

func (d *contractDoc) truthPaths(repo, projectRoot, gateRef string) ([]string, error) {
	globs, err := contractValidateGlobs(d.values["truth.paths"], "truth.paths")
	if err != nil {
		return nil, err
	}
	return contractExpandPaths(repo, projectRoot, gateRef, globs, "truth.paths")
}

// expectedSeal recomputes the deterministic seal fields — frozen instruments,
// priced exposure, and (when measuring) the baseline gate result — in the order
// they are assembled.
func (d *contractDoc) expectedSeal(repo, projectRoot string, runBaseline bool) ([]contractSealField, error) {
	values := d.values
	gateRef, err := contractGitTrim(repo, "rev-parse", values["gate.ref"]+"^{commit}")
	if err != nil {
		return nil, err
	}
	gatePaths, err := d.gatePaths(repo, projectRoot, gateRef)
	if err != nil {
		return nil, err
	}
	truthPaths, err := d.truthPaths(repo, projectRoot, gateRef)
	if err != nil {
		return nil, err
	}
	branch, err := d.candidateBranch(repo)
	if err != nil {
		return nil, err
	}
	gateIntegrity, err := contractManifestHash(repo, gateRef, gatePaths)
	if err != nil {
		return nil, err
	}
	truthIntegrity, err := contractManifestHash(repo, gateRef, truthPaths)
	if err != nil {
		return nil, err
	}
	fields := []contractSealField{
		{"sealed.version", "1"},
		{"candidate.branch", branch},
		{"sealed.gate-ref-sha", gateRef},
		{"sealed.gate-integrity.sha256", gateIntegrity},
		{"sealed.truth-integrity.sha256", truthIntegrity},
		{"sealed.baseline.failure-identifiers", "unavailable"},
	}

	exposureKeys := append([]string(nil), requiredFenceKeys...)
	exposureKeys = append(exposureKeys, contractCapKeys(values)...)
	exposureKeys = append(exposureKeys, contractPatienceKeys(values)...)
	var echo []string
	for _, key := range exposureKeys {
		fields = append(fields, contractSealField{"sealed.exposure." + key, values[key]})
		echo = append(echo, key+"="+values[key])
	}
	fields = append(fields, contractSealField{"sealed.exposure.statement", values["exposure"] + "|" + strings.Join(echo, ",")})

	if runBaseline {
		candidateSHA, gateRef, err := d.resolvePins(repo)
		if err != nil {
			return nil, err
		}
		metrics, failures, err := d.runGate(repo, projectRoot, candidateSHA, gateRef)
		if err != nil {
			return nil, err
		}
		fields = append(fields,
			contractSealField{"sealed.baseline.candidate-sha", candidateSHA},
			contractSealField{"sealed.baseline.failure-count", strconv.Itoa(failures)},
		)
		names := make([]string, 0, len(metrics))
		for name := range metrics {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fields = append(fields, contractSealField{"sealed.baseline." + name, metrics[name]})
		}
	}
	return fields, nil
}

// contractCapKeys returns the sorted per-pair cap keys.
func contractCapKeys(values map[string]string) []string {
	var caps []string
	for key := range values {
		if strings.HasPrefix(key, "cap.min.") {
			caps = append(caps, key)
		}
	}
	sort.Strings(caps)
	return caps
}

// contractPatienceKeys returns the sorted patience-floor keys
// (plans/patience-satellite-4.md). They seal beside the cap entries through
// the same expectedSeal enumeration preflight recomputes, so a signed
// mission's patience behavior is frozen with its signature.
func contractPatienceKeys(values map[string]string) []string {
	var keys []string
	for key := range values {
		if strings.HasPrefix(key, "patience.") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

// seal measures the baseline and appends the generated seal block, returning
// the digest a human signs. It refuses a contract that is already sealed or
// already approved, since the seal must precede the signature it protects.
func (d *contractDoc) seal(repo, projectRoot string) (string, error) {
	if len(d.sealed) > 0 {
		return "", stateErr("contract is already sealed")
	}
	if d.approval != nil {
		return "", stateErr("seal must run before approval is added")
	}
	fields, err := d.expectedSeal(repo, projectRoot, true)
	if err != nil {
		return "", err
	}
	seal := map[string]string{"sealed.at": nowUTC().Format("2006-01-02T15:04:05Z")}
	for _, field := range fields {
		seal[field.key] = field.value
	}

	ordered := []string{
		"sealed.version", "sealed.at", "candidate.branch", "sealed.gate-ref-sha",
		"sealed.gate-integrity.sha256", "sealed.truth-integrity.sha256",
		"sealed.baseline.candidate-sha", "sealed.baseline.failure-count",
		"sealed.baseline.failure-identifiers",
	}
	placed := map[string]bool{}
	for _, key := range ordered {
		placed[key] = true
	}
	var extraBaseline []string
	for key := range seal {
		if strings.HasPrefix(key, "sealed.baseline.") && !placed[key] {
			extraBaseline = append(extraBaseline, key)
		}
	}
	sort.Strings(extraBaseline)
	ordered = append(ordered, extraBaseline...)
	for _, key := range requiredFenceKeys {
		ordered = append(ordered, "sealed.exposure."+key)
	}
	for _, key := range contractCapKeys(d.values) {
		ordered = append(ordered, "sealed.exposure."+key)
	}
	for _, key := range contractPatienceKeys(d.values) {
		ordered = append(ordered, "sealed.exposure."+key)
	}
	ordered = append(ordered, "sealed.exposure.statement")

	lines := make([]string, len(ordered))
	for i, key := range ordered {
		lines[i] = key + "=" + seal[key]
	}
	updated := strings.TrimRightFunc(d.text, unicode.IsSpace) + "\n\n```mission-seal\n" + strings.Join(lines, "\n") + "\n```\n"
	if err := atomicWriteText(d.path, updated); err != nil {
		return "", err
	}
	return sha256Hex(string(contractCanonicalSignedBytes(updated))), nil
}

// --- preflight ---

// preflight is the full launch gate, in the order a mission depends on: an
// intact seal, an approval over the exact sealed bytes, those bytes on origin,
// a still-measuring gate, an armed and fresh supervisor set, and a free lease.
func (d *contractDoc) preflight(repo, projectRoot string) error {
	if err := d.verifySeal(repo, projectRoot); err != nil {
		return err
	}
	if err := d.verifyApproval(); err != nil {
		return err
	}
	if err := d.verifyOrigin(repo); err != nil {
		return err
	}
	preflightSHA, preflightGateRef, err := d.resolvePins(repo)
	if err != nil {
		return err
	}
	if _, _, err := d.runGate(repo, projectRoot, preflightSHA, preflightGateRef); err != nil {
		return err
	}
	if err := contractVerifySupervision(projectRoot); err != nil {
		return err
	}
	return d.verifyLease(projectRoot)
}

// verifySeal checks that the on-disk seal has exactly the generated keys and
// that every deterministic field still matches the recomputed instruments and
// exposure — a mismatched exposure field is a stale price, anything else is a
// broken seal.
func (d *contractDoc) verifySeal(repo, projectRoot string) error {
	if len(d.sealed) == 0 {
		return stateErr("preflight refused: contract is unsealed")
	}
	required, err := d.expectedSeal(repo, projectRoot, false)
	if err != nil {
		return err
	}
	expectedKeys := map[string]bool{
		"sealed.at": true, "sealed.baseline.candidate-sha": true, "sealed.baseline.failure-count": true,
	}
	for _, field := range required {
		expectedKeys[field.key] = true
	}
	metricNames := d.thresholdNames()
	for _, name := range metricNames {
		expectedKeys["sealed.baseline."+name] = true
	}
	if len(d.sealed) != len(expectedKeys) {
		return stateErr("preflight refused: generated seal keys are missing or unexpected")
	}
	for key := range d.sealed {
		if !expectedKeys[key] {
			return stateErr("preflight refused: generated seal keys are missing or unexpected")
		}
	}
	for _, field := range required {
		if d.sealed[field.key] != field.value {
			if strings.HasPrefix(field.key, "sealed.exposure.") {
				return stateErr("preflight refused: exposure is stale at %s", field.key)
			}
			return stateErr("preflight refused: seal integrity mismatch at %s", field.key)
		}
	}
	if _, err := time.Parse("2006-01-02T15:04:05Z", d.sealed["sealed.at"]); err != nil {
		return stateErr("preflight refused: sealed.at is invalid")
	}
	if !contractShaRangeRe.MatchString(d.sealed["sealed.baseline.candidate-sha"]) {
		return stateErr("preflight refused: baseline candidate sha is invalid")
	}
	if !contractDigitsRe.MatchString(d.sealed["sealed.baseline.failure-count"]) {
		return stateErr("preflight refused: baseline failure count is invalid")
	}
	for _, name := range metricNames {
		if !contractDecimalRe.MatchString(d.sealed["sealed.baseline."+name]) {
			return stateErr("preflight refused: baseline metric is invalid: %s", name)
		}
	}
	return nil
}

// verifyApproval requires a signature whose recorded digest covers the current
// sealed bytes.
func (d *contractDoc) verifyApproval() error {
	if d.approval == nil {
		return stateErr("preflight refused: contract is unsigned")
	}
	if d.approval[3] != d.hash() {
		return stateErr("preflight refused: approval hash does not match the sealed bytes")
	}
	return nil
}

// verifyOrigin fetches origin and requires the signed contract bytes to be
// present verbatim on the origin default branch — a launch can only trust what
// the shared remote agrees to.
func (d *contractDoc) verifyOrigin(repo string) error {
	fetch := exec.Command("git", "-C", repo, "fetch", "--quiet", "origin")
	var stderr strings.Builder
	fetch.Stderr = &stderr
	// Network work is bounded (B4): an unreachable remote must fail the
	// preflight, never hang the mission that is waiting on it.
	limit := boundedexec.Timeout(filepath.Join(repo, "metasystem.conf"), boundedexec.Network)
	if err := boundedexec.Run(fetch, limit, "preflight origin fetch"); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return stateErr("preflight refused: origin fetch failed: %s", detail)
	}
	remoteHead, err := contractGitTrim(repo, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err != nil {
		return err
	}
	if !strings.HasPrefix(remoteHead, "refs/remotes/origin/") {
		return stateErr("preflight refused: origin default branch is not declared")
	}
	relative, err := relUnderRepo(d.path, repo)
	if err != nil {
		return err
	}
	published, code := gitTry(repo, "show", remoteHead+":"+relative)
	if code != 0 || published != string(d.rawBytes) {
		return stateErr("preflight refused: signed contract bytes are absent from fetched origin default branch")
	}
	return nil
}

// verifyLease reserves and immediately releases the mission lease directory,
// refusing when another holder already owns it.
func (d *contractDoc) verifyLease(projectRoot string) error {
	mission, err := contractMissionID(d.path)
	if err != nil {
		return err
	}
	dir := filepath.Join(projectRoot, "artifacts", "agents", "missions", mission)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	marker := filepath.Join(dir, "lease.d")
	if err := os.Mkdir(marker, 0o755); err != nil {
		if os.IsExist(err) {
			return stateErr("preflight refused: mission lease is not acquirable")
		}
		return err
	}
	_ = os.Remove(marker)
	return nil
}

// contractMissionID extracts the mission id encoded in a contract filename.
func contractMissionID(path string) (string, error) {
	m := contractNameRe.FindStringSubmatch(filepath.Base(path))
	if m == nil {
		return "", stateErr("mission contract filename must be mission-<mission-id>.contract.md")
	}
	return m[1], nil
}

// --- supervisor verification ---

// contractVerifySupervision requires the supervisor set to be armed and fresh:
// a valid interval, a live and correctly-tagged watcher and reaper with recent
// heartbeats, a recent successful census, and a fingerprint that still matches
// the live supervision code.
func contractVerifySupervision(projectRoot string) error {
	dir := filepath.Join(projectRoot, "artifacts", "agents", "supervision")
	state, err := contractReadJSON(filepath.Join(dir, "state.json"))
	if err != nil {
		return stateErr("preflight refused: supervisor set is unarmed: %v", err)
	}
	lastCensus, err := contractReadJSON(filepath.Join(dir, "last-census.json"))
	if err != nil {
		return stateErr("preflight refused: supervisor set is unarmed: %v", err)
	}
	interval, ok := contractIntField(state["intervalSec"])
	if !ok || interval < 1 {
		return stateErr("preflight refused: supervisor interval is invalid")
	}
	now := nowUTC().Unix()
	components, _ := state["components"].(map[string]any)
	for _, name := range []string{"watcher", "reaper"} {
		component, _ := components[name].(map[string]any)
		pid, pidOK := contractIntField(component["pid"])
		started, startedOK := contractIntField(component["pidStartedAt"])
		tag, _ := component["instanceTag"].(string)
		heartbeatPath, _ := component["heartbeat"].(string)
		heartbeat, err := contractReadJSON(heartbeatPath)
		if err != nil {
			return stateErr("preflight refused: %s is not armed", name)
		}
		if !pidOK || !startedOK || !contractProcessHasTag(projectRoot, pid, started, tag) {
			return stateErr("preflight refused: %s process identity is not live", name)
		}
		function, _ := heartbeat["function"].(string)
		heartbeatPid, _ := contractIntField(heartbeat["pid"])
		heartbeatStarted, _ := contractIntField(heartbeat["pidStartedAt"])
		if function != name || heartbeatPid != pid || heartbeatStarted != started {
			return stateErr("preflight refused: %s heartbeat identity does not match", name)
		}
		observed, observedOK := contractIntField(heartbeat["observedAtEpoch"])
		age := interval * 3
		if observedOK {
			age = now - observed
		}
		if age < -5 || age > interval*2+2 {
			return stateErr("preflight refused: %s heartbeat is stale", name)
		}
	}
	completed, completedOK := contractIntField(lastCensus["completedAtEpoch"])
	censusAge := interval + 1
	if completedOK {
		censusAge = now - completed
	}
	if verdict, _ := lastCensus["verdict"].(string); verdict != "SUCCESS" || censusAge < -5 || censusAge > interval {
		return stateErr("preflight refused: census is absent, failed, or stale")
	}
	stateFingerprint, _ := state["fingerprint"].(string)
	fingerprint, code := contractRunFingerprint(projectRoot)
	if code != 0 || strings.TrimSpace(fingerprint) != stateFingerprint {
		return stateErr("preflight refused: supervisor fingerprint does not match live code")
	}
	if censusFingerprint, _ := lastCensus["fingerprint"].(string); censusFingerprint != stateFingerprint {
		return stateErr("preflight refused: census fingerprint does not match supervisor set")
	}
	return nil
}

// contractProcessHasTag reports whether a pid is live at its recorded start and
// carries the expected instance tag in its command. When the engine is present
// it additionally demands exact-start liveness; the command tag is read from
// the process table, falling back to a fixture identity file when the table is
// unreadable.
func contractProcessHasTag(projectRoot string, pid, started int64, tag string) bool {
	if pid <= 1 || started < 1 || tag == "" {
		return false
	}
	if err := unix.Kill(int(pid), 0); err != nil {
		return false
	}
	if fileExists(filepath.Join(projectRoot, "bin", "metasystem")) {
		if !census.Alive(pid, started) {
			return false
		}
	}
	// The identity owner reads argv natively (go-production-grade P6); an
	// unreadable argv takes the fixture fallback exactly as a failed ps
	// invocation did, never a tag mismatch on absent evidence (B1).
	exact, state, err := (identity.KernelProber{}).Probe(pid)
	if err != nil || state != identity.Alive || !exact.ArgvKnown {
		return contractFixtureIdentityMatches(pid, started, tag)
	}
	return strings.Contains(strings.Join(exact.Argv, " "), tag)
}

// contractFixtureIdentityMatches consults the fixture identity file used when
// the process table cannot be read, matching the recorded start and tag.
func contractFixtureIdentityMatches(pid, started int64, tag string) bool {
	fixture := os.Getenv("METASYSTEM_MISSION_PROCESS_IDENTITY_FILE")
	if fixture == "" {
		return false
	}
	data, err := os.ReadFile(fixture)
	if err != nil {
		return false
	}
	var table map[string]struct {
		PidStartedAt *int64  `json:"pidStartedAt"`
		Command      *string `json:"command"`
	}
	if json.Unmarshal(data, &table) != nil {
		return false
	}
	entry, ok := table[strconv.FormatInt(pid, 10)]
	if !ok || entry.PidStartedAt == nil || entry.Command == nil {
		return false
	}
	return *entry.PidStartedAt == started && strings.Contains(*entry.Command, tag)
}

// contractRunFingerprint asks the live supervision code to print the checkout's
// fingerprint, returning its output and exit code.
func contractRunFingerprint(projectRoot string) (string, int) {
	script := filepath.Join(projectRoot, "scripts", "agents", "arm-supervision.sh")
	command := exec.Command(script, "fingerprint", "--repo", projectRoot)
	var out strings.Builder
	command.Stdout = &out
	// A fingerprint script that hangs must not hang the preflight (B4).
	limit := boundedexec.Timeout(filepath.Join(projectRoot, "metasystem.conf"), boundedexec.Local)
	if err := boundedexec.Run(command, limit, "supervision fingerprint"); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return out.String(), exit.ExitCode()
		}
		return out.String(), 1
	}
	return out.String(), 0
}

// --- path and repository resolution ---

// contractRepositoryFor returns the git repository the contract lives in.
func contractRepositoryFor(path string) (string, error) {
	out, code := gitTry(filepath.Dir(path), "rev-parse", "--show-toplevel")
	if code != 0 {
		return "", stateErr("mission contract is not inside a git repository")
	}
	return resolvePath(strings.TrimSpace(out)), nil
}

// contractProjectRoot returns the metasystem checkout that owns the contract
// when the contract lives inside a metasystem nested under the repository;
// otherwise the repository itself is the project root.
func contractProjectRoot(contractPath, repo string) string {
	root := contractMetasystemRoot()
	if root != "" && contractPathWithin(contractPath, root) && contractPathWithin(root, repo) {
		return resolvePath(root)
	}
	return repo
}

// contractMetasystemRoot locates the metasystem checkout containing this binary,
// confirmed by its shipped supervision assets.
func contractMetasystemRoot() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	// The binary ships at <root>/bin/metasystem — TWO components deep, so
	// the checkout is Dir^2 of the executable. The shell originals derived
	// three levels from scripts/agents/<script>; the port kept three Dir
	// calls on a binary only two deep, landing on the checkout's PARENT and
	// making the confirmation below fail everywhere (review
	// mission-contract-1).
	root := resolvePath(filepath.Dir(filepath.Dir(exe)))
	if fileExists(filepath.Join(root, "metasystem.conf")) || contractDirExists(filepath.Join(root, "scripts", "agents")) {
		return root
	}
	return ""
}

// contractPathWithin reports whether inner is outer or a descendant of it.
func contractPathWithin(inner, outer string) bool {
	rel, err := filepath.Rel(resolvePath(outer), resolvePath(inner))
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return false
	}
	return true
}

func contractDirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// --- typed JSON reads ---

// contractReadJSON decodes a JSON object preserving numbers as json.Number so
// integer and float can be told apart.
func contractReadJSON(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return nil, stateErr("expected a JSON object at %s", path)
	}
	return obj, nil
}

// contractIntField reads an integer field, rejecting booleans and non-integral
// numbers.
func contractIntField(v any) (int64, bool) {
	if _, isBool := v.(bool); isBool {
		return 0, false
	}
	return intValue(v)
}
