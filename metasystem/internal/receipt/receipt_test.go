package receipt

import (
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixedNow() time.Time { return time.Unix(1_755_000_000, 0) }

func noEnv(string) (string, bool) { return "", false }

func baseOptions(t *testing.T) Options {
	t.Helper()
	root := t.TempDir()
	// The registry ships metasystem.conf with every adoption; an empty file
	// stands in for one that simply lacks retro keys.
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return Options{
		Root: root, File: filepath.Join(root, "memory", "receipts.log"),
		Skills: "none", Verify: "skipped", Corrections: "0", StopLoss: "no",
		Now: fixedNow, LookupEnv: noEnv,
	}
}

func TestAddAppendsOneLine(t *testing.T) {
	opts := baseOptions(t)
	opts.Type, opts.Outcome = "implement", "shipped"
	opts.Delegates = []string{"codex:fixture-code:implementer-job", "claude:fixture-review:code-critic-job"}
	result := Add(opts)
	if result.Code != 0 || result.Out[0] != "receipt recorded in "+opts.File {
		t.Fatalf("add failed: %+v", result)
	}
	data, _ := os.ReadFile(opts.File)
	line := strings.TrimSuffix(string(data), "\n")
	want := "1755000000|2025-08-12T12:00:00Z|RECEIPT|type=implement|outcome=shipped|skills=none|verify=skipped|corrections=0|stop_loss=no" +
		"|delegate=codex:fixture-code:implementer-job,claude:fixture-review:code-critic-job|critique_waived=none|waiver_stream=none|note="
	if line != want {
		t.Fatalf("line mismatch:\n got %s\nwant %s", line, want)
	}
	if len(result.Err) != 0 {
		t.Fatalf("unexpected retro note: %v", result.Err)
	}
}

func TestO13ReceiptProvenanceParsesWithOldRows(t *testing.T) {
	opts := baseOptions(t)
	opts.Type, opts.Outcome = "implement", "shipped"
	if result := Add(opts); result.Code != 0 {
		t.Fatalf("old-shape add failed: %+v", result)
	}
	opts.Now = func() time.Time { return fixedNow().Add(time.Second) }
	opts.Goal = "goal-a"
	opts.BuiltBy = "delegate"
	if result := Add(opts); result.Code != 0 {
		t.Fatalf("provenance add failed: %+v", result)
	}
	data, err := os.ReadFile(opts.File)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 || strings.Contains(lines[0], "|goal=") || strings.Contains(lines[0], "|built_by=") {
		t.Fatalf("old row changed shape: %q", lines)
	}
	if !strings.Contains(lines[1], "|goal=goal-a|built_by=delegate|") {
		t.Fatalf("new row lost provenance: %q", lines[1])
	}
	stats := Stats(Options{File: opts.File, All: true})
	if stats.Code != 0 || len(stats.Out) == 0 || stats.Out[0] != "receipts=2" {
		t.Fatalf("mixed old/new rows did not parse: %+v", stats)
	}
}

func TestReceiptMetricsReportTypeAndBuilderValidation(t *testing.T) {
	opts := baseOptions(t)
	opts.Type, opts.Outcome = "metrics-report", "shipped"
	opts.BuiltBy = "coordinator"
	if result := Add(opts); result.Code != 0 {
		t.Fatalf("metrics report receipt failed: %+v", result)
	}
	opts.BuiltBy = "critic"
	if result := Add(opts); result.Code != 2 || result.Err[0] != "invalid --built-by: critic" {
		t.Fatalf("invalid builder accepted: %+v", result)
	}
}

func TestAddValidation(t *testing.T) {
	cases := []struct {
		mutate func(*Options)
		want   string
	}{
		{func(o *Options) { o.Type = "bogus" }, "invalid --type: bogus"},
		{func(o *Options) { o.Outcome = "bogus" }, "invalid --outcome: bogus"},
		{func(o *Options) { o.Verify = "bogus" }, "invalid --verify: bogus"},
		{func(o *Options) { o.StopLoss = "maybe" }, "invalid --stop-loss: maybe"},
		{func(o *Options) { o.Corrections = "x" }, "invalid --corrections: x"},
		{func(o *Options) { o.Skills = "a,code-critique" }, "receipt refused: skills=code-critique requires delegate entries naming a code-critic chain id and the implementer job id in that chain's reviews field"},
	}
	for _, tc := range cases {
		opts := baseOptions(t)
		opts.Type, opts.Outcome = "implement", "shipped"
		tc.mutate(&opts)
		result := Add(opts)
		if result.Code != 2 || result.Err[0] != tc.want {
			t.Fatalf("wanted %q, got %+v", tc.want, result)
		}
	}
}

func TestAddSanitizesAndNotesDueRetro(t *testing.T) {
	opts := baseOptions(t)
	opts.Type, opts.Outcome = "improve", "parked"
	opts.Skills = "a|b,c"
	opts.Note = "line1\nline2|x"
	os.WriteFile(filepath.Join(opts.Root, "metasystem.conf"), []byte("retro.max-receipts=0\n"), 0o644)
	result := Add(opts)
	if result.Code != 0 {
		t.Fatalf("add failed: %+v", result)
	}
	data, _ := os.ReadFile(opts.File)
	if !strings.Contains(string(data), "|skills=a;b,c|") || !strings.Contains(string(data), "|note=line1 line2;x\n") {
		t.Fatalf("sanitization wrong: %s", data)
	}
	if len(result.Err) != 1 || !strings.Contains(result.Err[0], "a metasystem retro is due") {
		t.Fatalf("missing retro note: %+v", result.Err)
	}
	// The note names its repository by ABSOLUTE path: nested repos emit
	// this line into outer logs, and "due in ." identifies nothing.
	wantHome, err := filepath.Abs(opts.Root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Err[0], "due in "+wantHome) {
		t.Fatalf("the due note must name its repository absolutely: %q", result.Err[0])
	}
}

func TestCorrectLifecycle(t *testing.T) {
	opts := baseOptions(t)
	opts.Type, opts.Outcome = "implement", "shipped"
	if result := Add(opts); result.Code != 0 {
		t.Fatalf("seed add failed: %+v", result)
	}
	data, _ := os.ReadFile(opts.File)
	original := strings.TrimSuffix(string(data), "\n")
	opts.RefEpoch = strings.SplitN(original, "|", 2)[0]
	opts.RefSHA1 = fmt.Sprintf("%x", sha1.Sum([]byte(original)))
	opts.Field, opts.Was, opts.NowValue, opts.Reason = "outcome", "shipped", "reworked", "post-hoc review"
	result := Correct(opts)
	if result.Code != 0 || result.Out[0] != "correction recorded in "+opts.File+"; original line unchanged" {
		t.Fatalf("correct failed: %+v", result)
	}
	data, _ = os.ReadFile(opts.File)
	if !strings.HasPrefix(string(data), original+"\n") || !strings.Contains(string(data), "|CORRECTION|") {
		t.Fatalf("ledger wrong after correction: %s", data)
	}
	// The --was value must match the original field.
	opts.Was = "wrong"
	if result := Correct(opts); result.Code != 2 ||
		result.Err[0] != "correction --was value does not match field outcome on the original line" {
		t.Fatalf("bad --was accepted: %+v", result)
	}
}

func TestCorrectRefusals(t *testing.T) {
	opts := baseOptions(t)
	opts.Field, opts.Was, opts.NowValue, opts.Reason = "note", "x", "y", "r"
	sha := strings.Repeat("0", 40)

	opts.RefEpoch, opts.RefSHA1 = "x", sha
	if result := Correct(opts); result.Code != 2 || result.Err[0] != "correct requires a numeric --ref-epoch" {
		t.Fatalf("bad epoch accepted: %+v", result)
	}
	opts.RefEpoch, opts.RefSHA1 = "1000", "short"
	if result := Correct(opts); result.Code != 2 || result.Err[0] != "correct requires a lowercase 40-character --ref-sha1" {
		t.Fatalf("bad sha accepted: %+v", result)
	}
	opts.RefSHA1 = sha
	opts.Field = "Bad Field"
	if result := Correct(opts); result.Code != 2 || result.Err[0] != "correct requires a valid --field" {
		t.Fatalf("bad field accepted: %+v", result)
	}
	opts.Field, opts.Reason = "note", ""
	if result := Correct(opts); result.Code != 2 || result.Err[0] != "correct requires a nonempty --reason" {
		t.Fatalf("empty reason accepted: %+v", result)
	}
	opts.Reason = "r"
	if result := Correct(opts); result.Code != 2 ||
		result.Err[0] != "correction reference file does not exist: "+opts.File {
		t.Fatalf("absent file accepted: %+v", result)
	}

	// A reference that resolves to a non-RECEIPT line refuses.
	retro := "1000|1970-01-01T00:16:40Z|RETRO|note=x"
	os.MkdirAll(filepath.Dir(opts.File), 0o755)
	os.WriteFile(opts.File, []byte(retro+"\n"), 0o644)
	opts.RefEpoch = "1000"
	opts.RefSHA1 = fmt.Sprintf("%x", sha1.Sum([]byte(retro)))
	if result := Correct(opts); result.Code != 2 ||
		result.Err[0] != "correction reference must identify an original RECEIPT line" {
		t.Fatalf("retro reference accepted: %+v", result)
	}

	// Two byte-identical lines are ambiguous.
	receipt := "1000|1970-01-01T00:16:40Z|RECEIPT|type=other|outcome=parked|note=x"
	os.WriteFile(opts.File, []byte(receipt+"\n"+receipt+"\n"), 0o644)
	opts.RefSHA1 = fmt.Sprintf("%x", sha1.Sum([]byte(receipt)))
	if result := Correct(opts); result.Code != 2 ||
		result.Err[0] != "correction reference must identify exactly one original line; matched 2" {
		t.Fatalf("ambiguous reference accepted: %+v", result)
	}
}

func TestRetro(t *testing.T) {
	opts := baseOptions(t)
	if result := Retro(opts); result.Code != 2 ||
		result.Err[0] != "retro requires a summary of the instruction changes made" {
		t.Fatalf("empty summary accepted: %+v", result)
	}
	opts.Summary = "tuned|the\nrules"
	result := Retro(opts)
	if result.Code != 0 || result.Out[0] != "retro recorded; cadence reset" {
		t.Fatalf("retro failed: %+v", result)
	}
	data, _ := os.ReadFile(opts.File)
	if !strings.HasSuffix(string(data), "|RETRO|note=tuned;the rules\n") {
		t.Fatalf("retro line wrong: %s", data)
	}
}

const statsFixture = "100|1970-01-01T00:01:40Z|RECEIPT|type=implement|outcome=shipped|skills=none|verify=caught|corrections=3|stop_loss=yes|delegate=none|critique_waived=none|waiver_stream=none|note=\n" +
	"200|1970-01-01T00:03:20Z|RECEIPT|type=review|outcome=blocked|skills=none|verify=skipped|corrections=2|stop_loss=no|delegate=none|critique_waived=cap|waiver_stream=main|note=\n" +
	"300|1970-01-01T00:05:00Z|RETRO|note=mid\n" +
	"90000|1970-01-02T01:00:00Z|RECEIPT|type=design|outcome=shipped|skills=none|verify=clean|corrections=0|stop_loss=no|delegate=none|critique_waived=|waiver_stream=|note=tail"

func TestStats(t *testing.T) {
	opts := baseOptions(t)
	if result := Stats(opts); result.Code != 0 || result.Out[0] != "receipts=0" {
		t.Fatalf("missing file stats wrong: %+v", result)
	}
	os.MkdirAll(filepath.Dir(opts.File), 0o755)
	os.WriteFile(opts.File, []byte(statsFixture), 0o644)
	result := Stats(opts) // since the retro: the unterminated tail line only
	want := []string{"receipts=1", "outcome_shipped=1", "type_design=1", "corrections=0",
		"caught_by_verify=0", "stop_loss_triggered=0", "critique_waivers=1", "span_days=0.0"}
	if strings.Join(result.Out, "\n") != strings.Join(want, "\n") {
		t.Fatalf("period stats wrong:\n%s", strings.Join(result.Out, "\n"))
	}
	opts.All = true
	result = Stats(opts)
	want = []string{"receipts=3", "outcome_shipped=2", "outcome_blocked=1",
		"type_implement=1", "type_review=1", "type_design=1", "corrections=5",
		"caught_by_verify=1", "stop_loss_triggered=1", "critique_waivers=2", "span_days=1.0"}
	if strings.Join(result.Out, "\n") != strings.Join(want, "\n") {
		t.Fatalf("all stats wrong:\n%s", strings.Join(result.Out, "\n"))
	}
}

func TestCheck(t *testing.T) {
	opts := baseOptions(t)
	if result := Check(opts); result.Code != 0 || result.Out[0] != "no receipts recorded yet" {
		t.Fatalf("missing file check wrong: %+v", result)
	}
	os.MkdirAll(filepath.Dir(opts.File), 0o755)

	os.WriteFile(opts.File, []byte("garbage line\n"), 0o644)
	if result := Check(opts); result.Code != 2 || result.Err[0] != "receipts file is malformed: "+opts.File {
		t.Fatalf("malformed file accepted: %+v", result)
	}

	// Only a retro this period: nothing to mine.
	os.WriteFile(opts.File, []byte(fmt.Sprintf("%d|x|RETRO|note=r\n", fixedNow().Unix())), 0o644)
	if result := Check(opts); result.Code != 0 || result.Out[0] != "retro not due: no receipts this period, nothing to mine" {
		t.Fatalf("empty period wrong: %+v", result)
	}

	fresh := fmt.Sprintf("%d|x|RECEIPT|type=other|outcome=parked|note=\n", fixedNow().Unix())
	os.WriteFile(opts.File, []byte(fresh), 0o644)
	if result := Check(opts); result.Code != 0 || result.Out[0] != "retro not due: 1 receipts, 0 days since last retro" {
		t.Fatalf("fresh receipt wrong: %+v", result)
	}

	// Count backstop: flag beats environment beats configuration.
	os.WriteFile(opts.File, []byte(fresh+fresh), 0o644)
	os.WriteFile(filepath.Join(opts.Root, "metasystem.conf"), []byte("retro.max-receipts=0\n"), 0o644)
	result := Check(opts)
	if result.Code != 1 || result.Err[0] != "metasystem retro due: 2 receipts since the last retro (max 0)" {
		t.Fatalf("conf backstop ignored: %+v", result)
	}
	opts.LookupEnv = func(name string) (string, bool) {
		if name == "METASYSTEM_RETRO_MAX_RECEIPTS" {
			return "5", true
		}
		return "", false
	}
	if result := Check(opts); result.Code != 0 {
		t.Fatalf("environment did not beat configuration: %+v", result)
	}
	opts.MaxReceipts, opts.MaxReceiptsSet = "1", true
	if result := Check(opts); result.Code != 1 {
		t.Fatalf("flag did not beat environment: %+v", result)
	}

	// Age backstop, measured from the last retro line.
	opts.MaxReceipts, opts.MaxReceiptsSet = "", false
	opts.LookupEnv = noEnv
	os.WriteFile(filepath.Join(opts.Root, "metasystem.conf"), nil, 0o644)
	aged := fmt.Sprintf("1000|x|RETRO|note=r\n%s", fresh)
	os.WriteFile(opts.File, []byte(aged), 0o644)
	result = Check(opts)
	if result.Code != 1 || !strings.Contains(result.Err[0], "days since the last retro (max 30)") {
		t.Fatalf("age backstop ignored: %+v", result)
	}
	if result := Check(Options{Root: opts.Root, File: opts.File, Now: fixedNow, LookupEnv: noEnv,
		MaxAgeDays: "1000000", MaxAgeSet: true}); result.Code != 0 {
		t.Fatalf("age flag ignored: %+v", result)
	}

	// A cadence value that is not a number fails loudly.
	os.WriteFile(filepath.Join(opts.Root, "metasystem.conf"), []byte("retro.max-receipts=many\n"), 0o644)
	if result := Check(opts); result.Code != 2 || !strings.Contains(result.Err[0], "invalid retro.max-receipts value: many") {
		t.Fatalf("non-numeric cadence accepted: %+v", result)
	}

	// A root without metasystem.conf cannot resolve the cadence at all.
	os.Remove(filepath.Join(opts.Root, "metasystem.conf"))
	if result := Check(opts); result.Code != 1 || !strings.Contains(result.Err[0], "cannot read metasystem configuration") {
		t.Fatalf("missing configuration tolerated: %+v", result)
	}
}
