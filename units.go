package impose

import (
	"fmt"
	"strings"

	"github.com/unidoc/unipdf/v3/creator"
)

// PointsPerMM is how many points a millimetre has.
var PointsPerMM = creator.PPMM

// Units are the units lengths can be given in.
var Units = []string{"mm", "cm", "in", "pt"}

// UnitPoints gives how many points a length unit has.
func UnitPoints(u string) (float64, error) {
	switch strings.ToLower(u) {
	case "mm":
		return creator.PPMM, nil
	case "cm":
		return 10 * creator.PPMM, nil
	case "in", "inch":
		return 72, nil
	case "pt":
		return 1, nil
	}
	return 0, fmt.Errorf("unknown unit %q, use mm, cm, in or pt", u)
}
