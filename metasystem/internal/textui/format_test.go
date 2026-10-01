package textui

import (
	"os"
	"strings"
	"testing"
	"time"
)

var amsterdam = func() *time.Location {
	zone, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		panic(err)
	}
	return zone
}()

// fixedEnv is Wednesday 30 September 2026, 10:58 in Amsterdam.
func fixedEnv() Env {
	return Env{Width: 100, Now: time.Date(2026, 9, 30, 10, 58, 0, 0, amsterdam), Zone: amsterdam,
		Home: "/Users/wido", Repo: "/Users/wido/GitHub/m1e", InRepo: true}
}

func TestTimeIsLocalAndAsShortAsItsDistance(t *testing.T) {
	t.Parallel()
	env := fixedEnv()
	at := func(y int, m time.Month, d, h, min int) time.Time { return time.Date(y, m, d, h, min, 0, 0, amsterdam) }
	for _, c := range []struct {
		at   time.Time
		want string
	}{
		{at(2026, 9, 30, 10, 53), "10:53"},
		{time.Date(2026, 9, 30, 8, 53, 0, 0, time.UTC), "10:53"},
		{at(2026, 9, 29, 5, 43), "Tue 05:43"},
		{at(2026, 9, 28, 12, 39), "Mon 12:39"},
		{at(2026, 9, 24, 12, 39), "Thu 12:39"},
		{at(2026, 9, 23, 12, 39), "23 Sep 12:39"},
		{at(2026, 10, 1, 10, 56), "Thu 10:56"},
		{at(2026, 11, 2, 9, 0), "2 Nov 09:00"},
		{at(2025, 9, 28, 12, 39), "2025-09-28"},
	} {
		if got := env.Time(c.at); got != c.want {
			t.Errorf("Time(%s) = %q, want %q", c.at, got, c.want)
		}
	}
	if got := env.Since(at(2026, 9, 28, 12, 39)); got != "since Mon 12:39" {
		t.Errorf("Since = %q", got)
	}
}

func TestUntilSaysTheEndAndTheTimeLeft(t *testing.T) {
	t.Parallel()
	env := fixedEnv()
	for _, c := range []struct {
		at   time.Time
		want string
	}{
		{time.Date(2026, 10, 1, 10, 56, 0, 0, amsterdam), "until tomorrow 10:56 (23h58m left)"},
		{time.Date(2026, 9, 30, 12, 0, 0, 0, amsterdam), "until 12:00 (1h02m left)"},
		{time.Date(2026, 10, 3, 9, 0, 0, 0, amsterdam), "until Sat 09:00 (70h02m left)"},
		{time.Date(2026, 10, 20, 9, 0, 0, 0, amsterdam), "until 20 Oct 09:00 (19d22h left)"},
		{time.Date(2026, 9, 30, 9, 0, 0, 0, amsterdam), "until 09:00 (ended)"},
	} {
		if got := env.Until(c.at); got != c.want {
			t.Errorf("Until(%s) = %q, want %q", c.at, got, c.want)
		}
	}
}

func TestAgoCountsBackInOneUnit(t *testing.T) {
	t.Parallel()
	env := fixedEnv()
	for _, c := range []struct {
		back time.Duration
		want string
	}{
		{30 * time.Second, "30s ago"},
		{10 * time.Minute, "10m ago"},
		{18 * time.Hour, "18h ago"},
		{47 * time.Hour, "47h ago"},
		{3*24*time.Hour + time.Hour, "3d ago"},
		{-5 * time.Minute, "in 5m"},
	} {
		if got := env.Ago(env.Now.Add(-c.back)); got != c.want {
			t.Errorf("Ago(-%s) = %q, want %q", c.back, got, c.want)
		}
	}
}

func TestDurationIsCompact(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{45 * time.Second, "45s"},
		{12*time.Minute + 30*time.Second, "12m"},
		{2*time.Hour + 5*time.Minute, "2h05m"},
		{61*time.Hour + 4*time.Minute, "61h04m"},
		{76 * time.Hour, "3d04h"},
		{-90 * time.Second, "1m"},
	} {
		if got := Duration(c.d); got != c.want {
			t.Errorf("Duration(%s) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestBytesAndCountsAreHumanised(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ got, want string }{
		{Bytes(0), "0 B"},
		{Bytes(1023), "1023 B"},
		{Bytes(20 << 10), "20.0 KiB"},
		{Bytes(26948403), "25.7 MiB"},
		{Bytes(248463720448), "231.4 GiB"},
		{BytesMiB(20 << 10), "20480 B"},
		{BytesMiB(26948403), "25.7 MiB"},
		{BytesMiB(3 << 30), "3.0 GiB"},
		{GiB(0), "0.00 GiB"},
		{GiB(10 << 30), "10.00 GiB"},
		{Count(0, "goal", "goals"), "no goals"},
		{Count(1, "goal", "goals"), "1 goal"},
		{Count(3273, "checkout", "checkouts"), "3,273 checkouts"},
		{Number(0), "0"},
		{Number(999), "999"},
		{Number(1000), "1,000"},
		{Number(-1234567), "-1,234,567"},
		{SHA("bd89898ee4c99d8fa331a90a33ed089dae9ed3b3"), "bd89898ee"},
		{SHA("f9747c8"), "f9747c8"},
		{Command([]string{"metasystem", "grant", "revoke", "YY6-m1e"}), "metasystem grant revoke YY6-m1e"},
		{Command([]string{"metasystem", "work", "build", "--check", "go test ./...", "it's"}), `metasystem work build --check 'go test ./...' 'it'"'"'s'`},
		// A page's home path (Env.Path) stays pasteable: its ~/ expands.
		{Command([]string{"~/GitHub/lane/metasystem/bin/metasystem", "landing"}), "~/GitHub/lane/metasystem/bin/metasystem landing"},
		{Command([]string{"~/my dir/x", "~user/x"}), `'~/my dir/x' '~user/x'`},
	} {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

func TestPathIsRepoRelativeOrUnderHome(t *testing.T) {
	t.Parallel()
	env := fixedEnv()
	for _, c := range []struct{ path, want string }{
		{"/Users/wido/GitHub/m1e/plans/designs/agent-help.md", "plans/designs/agent-help.md"},
		{"/Users/wido/GitHub/m1e", "~/GitHub/m1e"},
		{"/Users/wido/GitHub/m1e-other/x", "~/GitHub/m1e-other/x"},
		{"/Users/wido", "~"},
		{"/Users/widow/x", "/Users/widow/x"},
		{"/tmp/x", "/tmp/x"},
		{"relative/x", "relative/x"},
		{"", ""},
	} {
		if got := env.Path(c.path); got != c.want {
			t.Errorf("Path(%q) = %q, want %q", c.path, got, c.want)
		}
	}
	if got := (Env{}).Path("/Users/wido/x"); got != "/Users/wido/x" {
		t.Errorf("an Env without home or repo shortens %q", got)
	}
}

func TestWrapBreaksAtWordsWithAHangingIndentAndNeverBreaksAToken(t *testing.T) {
	t.Parallel()
	got := Wrap("the quick brown fox jumps over the lazy dog", 16, 2)
	want := []string{"the quick brown", "  fox jumps over", "  the lazy dog"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("Wrap = %q, want %q", got, want)
	}
	got = Wrap("see /a/very/long/path/that/does/not/fit here", 12, 4)
	want = []string{"see", "    /a/very/long/path/that/does/not/fit", "    here"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("Wrap = %q, want %q", got, want)
	}
	if got := Wrap("", 10, 2); len(got) != 1 || got[0] != "" {
		t.Errorf("Wrap of nothing = %q", got)
	}
}

func TestDetectReadsTheStreamAndTheEnvironment(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 30, 10, 58, 0, 0, time.UTC)
	env := func(values map[string]string) func(string) string {
		return func(key string) string { return values[key] }
	}
	for _, c := range []struct {
		name      string
		tty       bool
		termWidth int
		values    map[string]string
		width     int
		color     bool
		ascii     bool
	}{
		{"a pipe", false, 0, nil, 100, false, false},
		{"a terminal", true, 120, map[string]string{"TERM": "xterm-256color", "LANG": "en_US.UTF-8"}, 100, true, false},
		{"a narrow terminal", true, 72, map[string]string{"TERM": "xterm"}, 72, true, false},
		{"NO_COLOR", true, 80, map[string]string{"NO_COLOR": "1"}, 80, false, false},
		{"an empty NO_COLOR", true, 80, map[string]string{"NO_COLOR": ""}, 80, true, false},
		{"a dumb terminal", true, 80, map[string]string{"TERM": "dumb"}, 80, false, false},
		{"COLUMNS wins", true, 120, map[string]string{"COLUMNS": "90"}, 90, true, false},
		{"COLUMNS on a pipe", false, 0, map[string]string{"COLUMNS": "60"}, 60, false, false},
		{"COLUMNS over the cap", false, 0, map[string]string{"COLUMNS": "300"}, 100, false, false},
		{"COLUMNS zero is unset", true, 90, map[string]string{"COLUMNS": "0"}, 90, true, false},
		{"COLUMNS garbage is unset", false, 0, map[string]string{"COLUMNS": "wide"}, 100, false, false},
		{"a C locale", false, 0, map[string]string{"LC_ALL": "C"}, 100, false, true},
		{"LC_ALL wins over LANG", false, 0, map[string]string{"LC_ALL": "en_US.ISO8859-1", "LANG": "en_US.UTF-8"}, 100, false, true},
		{"LC_CTYPE UTF-8", false, 0, map[string]string{"LC_CTYPE": "UTF-8", "LANG": "C"}, 100, false, false},
		{"utf8 spelled small", false, 0, map[string]string{"LANG": "C.utf8"}, 100, false, false},
		{"METASYSTEM_ASCII", false, 0, map[string]string{"METASYSTEM_ASCII": "1", "LANG": "en_US.UTF-8"}, 100, false, true},
	} {
		got := DetectWith(c.tty, c.termWidth, env(c.values), now, time.UTC)
		if got.Width != c.width || got.Color != c.color || got.ASCII != c.ascii || got.TTY != c.tty || !got.Now.Equal(now) || got.Zone != time.UTC {
			t.Errorf("%s: %+v, want width %d colour %v ascii %v", c.name, got, c.width, c.color, c.ascii)
		}
	}
	if got := DetectWith(false, 0, env(nil), now, nil); got.Zone != time.Local {
		t.Errorf("no zone = %v, want Local", got.Zone)
	}
}

// A pipe is not a terminal: no colour, the full width, and no terminal
// width to read.
func TestDetectOnAPipe(t *testing.T) {
	t.Parallel()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	env := Detect(writer.Fd(), func(string) string { return "" }, time.Unix(0, 0), time.UTC)
	if env.TTY || env.Color || env.Width != MaxWidth || TerminalWidth(writer.Fd()) != 0 || IsTerminal(reader.Fd()) {
		t.Errorf("a pipe = %+v", env)
	}
}
