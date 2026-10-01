package batch

import (
	"slices"
)

// joinedUnits are the units still joined, in join order.
func joinedUnits(units []Unit) []Unit {
	return slices.DeleteFunc(slices.Clone(units), func(unit Unit) bool { return unit.State != UnitJoined })
}
