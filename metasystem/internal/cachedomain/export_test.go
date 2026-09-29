package cachedomain

// LiveAncestor exposes the issuer walk over these seams to the external
// tests.
func (s Seams) LiveAncestor(encoded string) (bool, error) {
	return s.withDefaults().liveAncestor(encoded)
}
