package registry

import "sort"

// HostCheckout is one checkout the host registry still names: Armed when an
// open claim or published owner speaks for it, else a stopped checkout whose
// closed records compaction has not dropped yet.
type HostCheckout struct {
	Path  string
	Armed bool
}

// HostCheckouts reads every checkout the host registry at path still names,
// sorted, each once, with whether it is armed. The error is kept (batch-lane
// D14, R24): a registry that cannot be read is not an empty one. A registry
// file that does not exist yet is an empty host. Compaction drops a clean
// closed claim, so a stopped checkout may be absent; only armed checkouts
// are complete.
func HostCheckouts(path string) ([]HostCheckout, error) {
	frames, err := ReadFrames(path)
	if err != nil {
		return nil, err
	}
	reduction, err := Reduce(frames)
	if err != nil {
		return nil, err
	}
	armed := map[string]bool{}
	note := func(path string, open bool) {
		if path == "" {
			return
		}
		armed[path] = armed[path] || open
	}
	for _, tag := range reduction.SortedTags() {
		if claim := reduction.Claims[tag]; claim != nil {
			note(claim.CheckoutPath, claim.Open())
		}
	}
	for _, owner := range reduction.PublishedOwners {
		note(owner.CheckoutPath, owner.Open())
	}
	checkouts := make([]HostCheckout, 0, len(armed))
	for path, open := range armed {
		checkouts = append(checkouts, HostCheckout{Path: path, Armed: open})
	}
	sort.Slice(checkouts, func(i, j int) bool { return checkouts[i].Path < checkouts[j].Path })
	return checkouts, nil
}

// ArmedCheckouts reads the open claims and published owners of the host
// registry at path: the checkouts armed on this host, sorted, each once. The
// error is kept (batch-lane D14, R24): a registry that cannot be read is not
// an empty one, and a caller that decides from the armed seats must tell the
// two apart. A registry file that does not exist yet is an empty host.
func ArmedCheckouts(path string) ([]string, error) {
	host, err := HostCheckouts(path)
	if err != nil {
		return nil, err
	}
	var checkouts []string
	for _, checkout := range host {
		if checkout.Armed {
			checkouts = append(checkouts, checkout.Path)
		}
	}
	return checkouts, nil
}
