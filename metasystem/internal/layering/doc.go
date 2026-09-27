// Package layering holds the R9 witness of the object-action verb redesign
// (plans/designs/verbs-object-action.md, section 4 rule R9, section 6.3):
// orchestration sits above owners. It has no production code. The witness
// lives in its own package, importing nothing it judges, so a mutation that
// makes an owner import a composition package (an import cycle) fails this
// witness by name instead of only breaking the build of the package it
// guards.
package layering
