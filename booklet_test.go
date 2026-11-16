package impose

import "testing"

func TestSafeCreep(t *testing.T) {
	cases := []struct {
		name             string
		creep, thickness float64
		numPages         int
		bleedx, bleedy   float64
		want             float64
	}{
		// numPages=16 -> sheets=4, amplification factor = 4*(4-1)/4 = 3: the limit on the
		// real displacement is divided by 3 to bound the raw creep parameter.
		{"bounded by bleed", 10, 0, 16, 3, 5, 1},
		{"bounded by thinner bleed axis", 10, 0, 16, 5, 2, 2.0 / 3},
		{"no bleed means no safe creep", 10, 0, 16, 0, 0, 0},
		{"thickness bounds before bleed", 10, 1, 16, 100, 100, 2},
		{"bleed tighter than thickness", 10, 1, 16, 2, 2, 2.0 / 3},
		{"thickness ignored when zero", 1, 0, 16, 100, 100, 1},
		{"under both bounds keeps creep", 1, 1, 16, 100, 100, 1},
		{"single sheet has no creep", 10, 2, 4, 100, 100, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := safeCreep(c.creep, c.thickness, c.numPages, c.bleedx, c.bleedy)
			if got != c.want {
				t.Errorf("safeCreep(%v,%v,%v,%v,%v) = %v, want %v", c.creep, c.thickness, c.numPages, c.bleedx, c.bleedy, got, c.want)
			}
			// SuggestedCreep is safeCreep's limit on its own, with no creep to additionally
			// bound: requesting more than it ever gives back exactly the suggestion.
			suggested := SuggestedCreep(c.thickness, c.numPages, c.bleedx, c.bleedy)
			if got := safeCreep(suggested+1000, c.thickness, c.numPages, c.bleedx, c.bleedy); got != suggested {
				t.Errorf("SuggestedCreep %v disagrees with safeCreep's own limit %v", suggested, got)
			}
		})
	}
}
