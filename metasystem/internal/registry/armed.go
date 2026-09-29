package registry

import "sort"

// ArmedCheckouts reads the open claims and published owners of the host
// registry at path: the checkouts armed on this host, sorted, each once. The
// error is kept (batch-lane D14, R24): a registry that cannot be read is not
// an empty one, and a caller that decides from the armed seats must tell the
// two apart. A registry file that does not exist yet is an empty host.
func ArmedCheckouts(path string) ([]string, error) {
	frames, err := ReadFrames(path)
	if err != nil {
		return nil, err
	}
	reduction, err := Reduce(frames)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var checkouts []string
	add := func(path string) {
		if path != "" && !seen[path] {
			seen[path] = true
			checkouts = append(checkouts, path)
		}
	}
	for _, tag := range reduction.SortedTags() {
		if claim := reduction.Claims[tag]; claim != nil && claim.Open() {
			add(claim.CheckoutPath)
		}
	}
	for _, owner := range reduction.PublishedOwners {
		if owner.Open() {
			add(owner.CheckoutPath)
		}
	}
	sort.Strings(checkouts)
	return checkouts, nil
}
