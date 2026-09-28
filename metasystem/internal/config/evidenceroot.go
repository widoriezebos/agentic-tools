package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EvidenceRootKey is the configuration key of the durable, host-local
// directory outside the checkout where run evidence is mirrored. This file is
// the only one that spells it; every reader resolves it through
// ResolveEvidenceRoot.
const EvidenceRootKey = "evidence.root"

// EvidenceRootParams is one evidence-root resolution.
type EvidenceRootParams struct {
	// ConfPath is the conf file judged; its .local is ConfPath+".local", and
	// the checkout is found from the file's directory.
	ConfPath string
	// LookupEnv resolves an environment variable and answers HOME too; nil
	// means os.LookupEnv.
	LookupEnv func(string) (string, bool)
}

// EvidenceRoot is a resolved evidence root and the source that named it.
type EvidenceRoot struct {
	Path   string // absolute, cleaned
	Origin string // "env", "conf-local", "conf" or "default"
}

// Default reports whether no source named the root and the compiled-in
// default applies.
func (r EvidenceRoot) Default() bool { return r.Origin == "default" }

// Line is the one line setup prints about the root.
func (r EvidenceRoot) Line() string {
	var source string
	switch r.Origin {
	case "env":
		source = EnvName(EvidenceRootKey)
	case "conf-local":
		source = "metasystem.conf.local"
	case "conf":
		source = "metasystem.conf"
	default:
		source = "default; set " + EvidenceRootKey + " in metasystem.conf.local to change"
	}
	return fmt.Sprintf("evidence root: %s (%s)", r.Path, source)
}

// ResolveEvidenceRoot names the evidence root the running system uses: the
// environment, then ConfPath's .local, then ConfPath, then the compiled-in
// default $HOME/metasystem-evidence/<checkout basename>. A source that leaves
// the key absent, empty or a <placeholder> is unspecified and falls through;
// a specified value that is relative or inside the checkout is refused with
// its source named and never replaced. The resolver creates nothing.
func ResolveEvidenceRoot(p EvidenceRootParams) (EvidenceRoot, error) {
	lookup := p.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	installation, err := filepath.Abs(filepath.Dir(p.ConfPath))
	if err != nil {
		return EvidenceRoot{}, fmt.Errorf("resolve %s: %w", EvidenceRootKey, err)
	}
	checkout := checkoutOf(installation)

	envName := EnvName(EvidenceRootKey)
	if value, ok := lookup(envName); ok && specified(value) {
		return judgeEvidenceRoot(value, envName, "env", checkout)
	}
	localPath := p.ConfPath + ".local"
	if isFile(localPath) {
		value, found, err := ConfLookup(localPath, EvidenceRootKey)
		if err != nil {
			return EvidenceRoot{}, err
		}
		if found && specified(value) {
			return judgeEvidenceRoot(value, filepath.Base(localPath), "conf-local", checkout)
		}
	}
	value, found, err := ConfLookup(p.ConfPath, EvidenceRootKey)
	if err != nil {
		return EvidenceRoot{}, err
	}
	if found && specified(value) {
		return judgeEvidenceRoot(value, filepath.Base(p.ConfPath), "conf", checkout)
	}

	home, ok := lookup("HOME")
	if !ok || strings.TrimSpace(home) == "" || !filepath.IsAbs(home) {
		return EvidenceRoot{}, fmt.Errorf("the evidence root has no default because HOME is not set to an absolute path; set HOME, or set %s in metasystem.conf.local", EvidenceRootKey)
	}
	return EvidenceRoot{Path: filepath.Join(home, "metasystem-evidence", filepath.Base(checkout)), Origin: "default"}, nil
}

// specified reports whether a source names a root: not empty after trimming
// and not a <placeholder>.
func specified(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && !(strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">"))
}

func judgeEvidenceRoot(raw, source, origin, checkout string) (EvidenceRoot, error) {
	value := strings.TrimSpace(raw)
	if !filepath.IsAbs(value) {
		return EvidenceRoot{}, fmt.Errorf("%s must be absolute (%s reads %q)", EvidenceRootKey, source, value)
	}
	if withinRepo(ResolvePath(value), ResolvePath(checkout)) {
		return EvidenceRoot{}, fmt.Errorf("%s must be outside the repository (%s reads %q)", EvidenceRootKey, source, value)
	}
	return EvidenceRoot{Path: filepath.Clean(value), Origin: origin}, nil
}

// checkoutOf is the nearest directory from installation upward holding a .git
// entry (file or directory; no Git command runs), else installation itself.
func checkoutOf(installation string) string {
	for dir := installation; ; {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return installation
		}
		dir = parent
	}
}
