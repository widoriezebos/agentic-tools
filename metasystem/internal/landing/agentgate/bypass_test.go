package agentgate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGateRefusesEveryBypassClass is the gate's own regression table: one
// row per way around the allowlist the landing agent must not find. Every
// deny row asserts a denial with a plain two-line reason; a "decide" row
// records an option class the gate admits on purpose, with why.
func TestGateRefusesEveryBypassClass(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t, "lane/b1")
	home := filepath.Join(filepath.Dir(bed.checkout), "home")
	edit := func(path string) []byte {
		return payload(t, bed.checkout, "Write", map[string]any{"file_path": path, "content": "x"})
	}
	type row struct {
		class string
		call  []byte
		allow bool
		why   string // required for an allowed ("decide") row
	}
	var rows []row
	deny := func(class string, commands ...string) {
		for _, command := range commands {
			rows = append(rows, row{class: class + ": " + command, call: bash(t, bed.checkout, command)})
		}
	}
	decide := func(class, command, why string) {
		rows = append(rows, row{class: class + ": " + command, call: bash(t, bed.checkout, command), allow: true, why: why})
	}

	// Shell structure the gate cannot read is refused whole.
	deny("redirection", "git log > out.txt", "git log >> out.txt", "cat < /etc/hosts", "git log 2> err.txt", "git log &> all.txt", "git log 2>&1")
	deny("pipe into an unlisted program", "git log | sh", "git log | xargs rm", "cat x | tee y")
	deny("command substitution", "echo $(git push)", "echo `git push`", "git checkout \"$(echo main)\"")
	deny("process substitution", "cat <(git push)", "diff <(git log) >(sh)")
	deny("chaining a forbidden command", "git status; git push", "git status && git push", "git status || git push", "git status\ngit push", "git status & git push")
	deny("subshells and braces", "(git push)", "{ git push; }", "git status && (git push)")
	deny("environment prefixes and env", "GIT_DIR=/tmp git status", "PATH=/tmp git status", "env git push", "env GIT_DIR=/tmp git status", "env -i git status")
	deny("shell wrappers", "bash -c 'git push'", "sh -c 'git status'", "zsh -c 'git status'", "exec git status", "eval git status", "command git push", "xargs git push")
	deny("paths to listed programs", "/usr/bin/git status", "./git status", "bin/git status", "/bin/cat x", "./metasystem landing status", "/tmp/bin/metasystem landing status")
	deny("quoting and escapes around program names", "'gi't push", "\"git\" push", "\\git status", "g\\it status", "git pu\\sh")
	deny("globbing", "git add *", "cat metasystem/*", "ls ?", "git add [a]", "cat ~/x")

	// Git reached around its subcommand.
	deny("git global options", "git -c core.hooksPath=/tmp commit -m x", "git -c alias.st=!sh st", "git --config-env=core.pager=X log",
		"git --git-dir=/tmp/x status", "git --work-tree=/tmp status", "git -C /tmp status", "git --exec-path=/tmp status", "git --no-pager -c x=y log")
	deny("git aliases and unlisted subcommands", "git st", "git lg", "git stash", "git reset --hard", "git merge x", "git update-ref refs/heads/main HEAD", "git worktree add /tmp/w", "git submodule update", "git filter-branch", "git gc")
	deny("git attribute and diff settings", "git config diff.go.binary true", "git config merge.ours.driver true", "git config filter.x.clean sh",
		"git config core.attributesFile /tmp/a", "git config --local attr.tree HEAD", "git -c core.attributesFile=/tmp/a diff",
		"git checkout HEAD -- .gitattributes", "git checkout HEAD -- metasystem/.gitattributes", "git add .gitattributes")
	deny("git config and remote writes", "git config core.hooksPath /tmp", "git config core.sshCommand sh", "git config core.pager sh", "git config --global core.fsmonitor sh", "git remote set-url origin /tmp/x")
	deny("git push in any form", "git push", "git push origin HEAD:main", "git push --force", "git --no-pager push", "git push --no-verify")
	deny("git checkout of protected paths", "git checkout HEAD -- .githooks/pre-push", "git checkout HEAD -- .claude/settings.json",
		"git checkout HEAD -- .codex/config.toml", "git checkout HEAD -- metasystem/metasystem.conf.local", "git checkout HEAD -- metasystem/bin/metasystem",
		"git checkout HEAD -- metasystem/artifacts/agents/context/engine-path", "git checkout HEAD -- ../outside/x")
	deny("git options that run a program or write a file", "git log --output=x", "git log --outp=x", "git diff --ext-diff", "git show --textconv HEAD",
		"git cat-file --filters HEAD:x", "git grep -O x", "git grep -nOless x", "git grep --open-files-in-pager=less x", "git fetch --upload-pack=sh origin",
		"git rebase --exec sh HEAD~1", "git rebase -x sh HEAD~1", "git rebase -i HEAD~1", "git commit -F /etc/hosts", "git commit --template=/etc/hosts", "git commit -n -m x", "git commit --author=x -m y")

	// The listed read programs: none has an option that writes a file or
	// runs a program; the options that read another file are reads, which
	// the gate admits anywhere (the Read tool reads anywhere too).
	decide("grep reads its patterns from a file", "grep -f /etc/hosts metasystem/internal/a.go", "a read of another file, which Read already admits; grep has no option that writes or runs anything")
	decide("grep reads its exclusions from a file", "grep -r --exclude-from=/etc/hosts x metasystem", "a read, as above")
	decide("jq reads a file into a variable", "jq --rawfile x /etc/hosts -n 1", "a read; jq has no option or builtin that writes a file or runs a program")
	decide("jq reads a slurped file", "jq --slurpfile x /etc/hosts -n 1", "a read, as above")
	decide("jq reads its program from a file", "jq -f /etc/hosts -n", "the program is jq's own language, which cannot write or run anything")
	decide("wc reads its file list from a file", "wc --files0-from=/etc/hosts", "a read of each listed file")
	decide("head, tail, cut and cat options", "head -n 5 x && tail -n 5 x && cut -d : -f 1 x && cat -v x && wc -l x", "their options only select and format what they read")
	decide("tail follows a file", "tail -f x", "it only reads, and blocks until the runtime ends the call")

	// Edits into protected files, inside the checkout and outside it.
	for class, path := range map[string]string{
		"the gate's settings file":     filepath.Join(filepath.Dir(bed.checkout), "state", "launch-1", SettingsFileName),
		"a Git hook":                   filepath.Join(bed.checkout, ".git", "hooks", "pre-push"),
		"a tracked hook directory":     filepath.Join(bed.checkout, ".githooks", "pre-commit"),
		"Claude settings":              filepath.Join(bed.checkout, ".claude", "settings.json"),
		"Codex settings":               filepath.Join(bed.checkout, ".codex", "config.toml"),
		"the engine binary":            filepath.Join(bed.module, "bin", "metasystem"),
		"the checkout's engine":        filepath.Join(bed.checkout, "bin", "metasystem"),
		"local configuration":          filepath.Join(bed.module, "metasystem.conf.local"),
		"shipped configuration":        filepath.Join(bed.module, "metasystem.conf"),
		"the pause file":               filepath.Join(home, ".metasystem", "host", "landing-lane-paused.json"),
		"the host lane record":         filepath.Join(home, ".metasystem", "host", "landing-lane.json"),
		"the recorded gate engine":     filepath.Join(bed.module, "artifacts", "agents", "context", "engine-path"),
		"supervision and enrollment":   filepath.Join(bed.module, "artifacts", "agents", "supervision", "state.json"),
		"the kernel's batch records":   filepath.Join(bed.checkout, "artifacts", "agents", "landing-batches", "b1.json"),
		"a goal":                       filepath.Join(bed.module, "plans", "goals", "g.md"),
		"a goal record":                filepath.Join(bed.module, "records", "goals", "g.md"),
		"a path outside the checkout":  filepath.Join(home, "x"),
		"an escape through dots":       filepath.Join(bed.checkout, "..", "home", "x"),
		"a relative path":              "metasystem/internal/a.go",
		"Git's info attributes":        filepath.Join(bed.checkout, ".git", "info", "attributes"),
		"Git's info exclude":           filepath.Join(bed.checkout, ".git", "info", "exclude"),
		"Git's config":                 filepath.Join(bed.checkout, ".git", "config"),
		"upper-case Git config":        filepath.Join(bed.checkout, ".GIT", "Config"),
		"a root .gitattributes":        filepath.Join(bed.checkout, ".gitattributes"),
		"a nested .gitattributes":      filepath.Join(bed.module, "internal", ".gitattributes"),
		"an upper-case .GitAttributes": filepath.Join(bed.module, ".GitAttributes"),
	} {
		rows = append(rows, row{class: "edit " + class, call: edit(path)})
	}

	for _, row := range rows {
		got := Decide(Request{Payload: row.call, Installation: bed.module})
		if row.allow {
			if row.why == "" {
				t.Errorf("%s: a decided row needs its reason", row.class)
			}
			if !got.Allow {
				t.Errorf("%s: decided allow (%s) but denied: %q", row.class, row.why, got.Reason)
			}
			continue
		}
		if got.Allow {
			t.Errorf("%s: allowed", row.class)
			continue
		}
		lines := strings.Split(got.Reason, "\n")
		if len(lines) != 2 || strings.TrimSpace(lines[0]) == "" || !strings.HasPrefix(lines[1], "run: ") || len(lines[1]) <= len("run: ") {
			t.Errorf("%s: reason is not two plain lines: %q", row.class, got.Reason)
		}
	}
}

// commitOn commits one file on a new branch from main and returns to the
// lane branch.
func (b laneBed) commitOn(t *testing.T, branch, path, content string) {
	t.Helper()
	b.git(t, "checkout", "-q", "-b", branch, "main")
	full := filepath.Join(b.checkout, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	b.git(t, "add", "-f", path)
	b.git(t, "-c", "user.name=f", "-c", "user.email=f@invalid", "commit", "-qm", branch)
	b.git(t, "checkout", "-q", "lane/b1")
}

// TestGateRefusesPathAndReplayBypasses (coverage review of the table): case
// variants of protected names on a case-insensitive volume, pathspecs that
// cover a protected path, replays and switches that would write one,
// symbolic links, every edit tool, and the parser's refusals.
func TestGateRefusesPathAndReplayBypasses(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t, "lane/b1")
	bed.commitOn(t, "evil", ".claude/settings.json", "{}\n")
	bed.commitOn(t, "hooks", ".githooks/pre-commit", "#!/bin/sh\n")
	bed.commitOn(t, "good", "metasystem/internal/b.go", "package a\n")
	outside := filepath.Join(filepath.Dir(bed.checkout), "outside")
	for link, target := range map[string]string{
		filepath.Join(bed.module, "internal", "out"):      filepath.Join(outside, "x"),
		filepath.Join(bed.module, "internal", "conf"):     filepath.Join(bed.module, "metasystem.conf.local"),
		filepath.Join(bed.module, "internal", "settings"): filepath.Join(bed.checkout, ".claude"),
	} {
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
	}
	type row struct {
		class string
		call  []byte
		allow bool
	}
	var rows []row
	deny := func(class string, commands ...string) {
		for _, command := range commands {
			rows = append(rows, row{class: class + ": " + command, call: bash(t, bed.checkout, command)})
		}
	}
	allow := func(class string, commands ...string) {
		for _, command := range commands {
			rows = append(rows, row{class: class + ": " + command, call: bash(t, bed.checkout, command), allow: true})
		}
	}
	tool := func(class, name string, input map[string]any, allowed bool) {
		rows = append(rows, row{class: class, call: payload(t, bed.checkout, name, input), allow: allowed})
	}
	edit := func(class, path string) {
		tool("Write "+class, "Write", map[string]any{"file_path": path, "content": "x"}, false)
		tool("MultiEdit "+class, "MultiEdit", map[string]any{"file_path": path, "edits": []any{}}, false)
		tool("NotebookEdit "+class, "NotebookEdit", map[string]any{"notebook_path": path, "new_source": "x"}, false)
	}

	// (1) Case variants: macOS volumes do not tell them apart.
	for class, path := range map[string]string{
		"upper-case Claude settings": filepath.Join(bed.checkout, ".CLAUDE", "settings.json"),
		"capitalised local config":   filepath.Join(bed.module, "Metasystem.conf.local"),
		"upper-case hook directory":  filepath.Join(bed.checkout, ".GITHOOKS", "pre-commit"),
		"upper-case engine":          filepath.Join(bed.module, "BIN", "metasystem"),
		"capitalised artifacts":      filepath.Join(bed.module, "Artifacts", "agents", "x"),
		"upper-case Git directory":   filepath.Join(bed.checkout, ".Git", "hooks", "pre-push"),
		"capitalised goals":          filepath.Join(bed.module, "Plans", "Goals", "g.md"),
	} {
		edit(class, path)
	}
	deny("case variants in pathspecs", "git checkout HEAD -- .CLAUDE/settings.json", "git checkout HEAD -- metasystem/Metasystem.conf.local")

	// (2) Pathspecs that cover a protected path, and pathspec magic.
	deny("directory pathspecs", "git checkout HEAD -- .", "git checkout HEAD -- metasystem", "git checkout HEAD -- metasystem/bin", "git checkout HEAD -- metasystem/internal")
	deny("pathspec magic and patterns", "git checkout HEAD -- ':(top).claude/settings.json'", "git checkout HEAD -- ':/.githooks'", "git checkout HEAD -- '*.json'", "git add ':(top).claude'", "git add 'metasystem/*.local'")
	allow("a file pathspec", "git checkout HEAD -- metasystem/internal/a.go", "git checkout --theirs -- metasystem/internal/a.go")

	// Replays and switches write every path their commits touch.
	deny("replaying a protected path", "git cherry-pick evil", "git cherry-pick main..evil", "git cherry-pick good evil", "git cherry-pick hooks")
	allow("replaying ordinary paths", "git cherry-pick -x good", "git cherry-pick main..good", "git cherry-pick --continue --no-edit", "git cherry-pick --abort")
	deny("switching onto a protected change", "git checkout -b lane/b3 evil", "git checkout -B lane/b3 hooks")
	allow("switching onto ordinary changes", "git checkout -b lane/b3 good", "git checkout -b lane/b4")
	deny("rebasing onto or over a protected change", "git rebase evil", "git rebase --onto evil main", "git rebase --onto=hooks main", "git rebase")
	allow("rebasing over ordinary changes", "git rebase main", "git rebase --onto good main", "git rebase --continue")
	deny("an unresolvable revision", "git cherry-pick no-such-commit", "git checkout -b lane/b5 no-such-commit")

	// (3) Symbolic links resolve before the checks, and the other edit
	// directories and tools.
	edit("through a link to outside", filepath.Join(bed.module, "internal", "out"))
	edit("through a link to local config", filepath.Join(bed.module, "internal", "conf"))
	edit("through a link to the Claude directory", filepath.Join(bed.module, "internal", "settings", "settings.json"))
	edit("into .agents", filepath.Join(bed.checkout, ".agents", "skills", "x", "SKILL.md"))
	edit("into .devin", filepath.Join(bed.checkout, ".devin", "config.json"))
	tool("MultiEdit an ordinary file", "MultiEdit", map[string]any{"file_path": filepath.Join(bed.module, "internal", "a.go"), "edits": []any{}}, true)
	tool("NotebookEdit an ordinary notebook", "NotebookEdit", map[string]any{"notebook_path": filepath.Join(bed.module, "internal", "n.ipynb"), "new_source": "x"}, true)
	deny("checking out through a link", "git checkout HEAD -- metasystem/internal/conf")

	// Parser refusals, and what it admits.
	deny("parser refusals", "git status # push", "! git status", "git log |& cat", "echo \"$HOME\"", "echo \"`git push`\"", "echo \"a\\b\"", "git status ;; git log")
	allow("parser admits", "git log HEAD~1", "ls", "pwd", "echo hi", "git status && git log -1", "git status;")
	// Decided: reads outside the checkout through git are not needed.
	deny("git reads outside the checkout", "git diff --no-index /etc/hosts x", "git blame --contents /etc/hosts metasystem/internal/a.go")

	for _, row := range rows {
		got := Decide(Request{Payload: row.call, Installation: bed.module})
		if row.allow != got.Allow {
			t.Errorf("%s: allow=%t, want %t (%q)", row.class, got.Allow, row.allow, got.Reason)
			continue
		}
		if !got.Allow {
			lines := strings.Split(got.Reason, "\n")
			if len(lines) != 2 || strings.TrimSpace(lines[0]) == "" || !strings.HasPrefix(lines[1], "run: ") || len(lines[1]) <= len("run: ") {
				t.Errorf("%s: reason is not two plain lines: %q", row.class, got.Reason)
			}
		}
	}
}
