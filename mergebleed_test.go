package impose

import "testing"

func TestGridInset(t *testing.T) {
	cases := []struct {
		name       string
		pos, n     int
		group      int
		pad, bleed float64
		merge      bool
		near, far  float64
	}{
		{"single cell, merge on", 0, 1, 0, 0, 4, true, 0, 0},
		{"single cell, merge off", 0, 1, 0, 0, 4, false, 0, 0},
		{"two cells, first", 0, 2, 0, 0, 4, true, 0, 2},
		{"two cells, last", 1, 2, 0, 0, 4, true, 2, 0},
		{"three cells, first", 0, 3, 0, 0, 4, true, 0, 2},
		{"three cells, middle", 1, 3, 0, 0, 4, true, 2, 2},
		{"three cells, last", 2, 3, 0, 0, 4, true, 2, 0},
		{"merge off keeps full bleed", 1, 3, 0, 0, 4, false, 0, 0},
		{"zero bleed", 1, 3, 0, 0, 0, true, 0, 0},
		{"negative bleed ignored", 1, 3, 0, 0, -4, true, 0, 0},
		// clone groups of 2, 2 groups (n=4): 0,1 | 2,3
		{"clone group, zero pad merges across groups: last of group 0", 1, 4, 2, 0, 4, true, 2, 2},
		{"clone group, zero pad merges across groups: first of group 1", 2, 4, 2, 0, 4, true, 2, 2},
		{"clone group, nonzero pad keeps group edges outer: last of group 0", 1, 4, 2, 5, 4, true, 2, 0},
		{"clone group, nonzero pad keeps group edges outer: first of group 1", 2, 4, 2, 5, 4, true, 0, 2},
		// a boundary within a group is unaffected by the clone pad either way
		{"inside a clone group, pad is irrelevant", 0, 4, 2, 5, 4, true, 0, 2},
	}
	for _, c := range cases {
		near, far := gridInset(c.pos, c.n, c.group, c.pad, c.bleed, c.merge)
		if near != c.near || far != c.far {
			t.Errorf("%s: gridInset(%d, %d, %d, %v, %v, %v) = (%v, %v), want (%v, %v)",
				c.name, c.pos, c.n, c.group, c.pad, c.bleed, c.merge, near, far, c.near, c.far)
		}
	}
}

func TestGridSpanAndStartsAgreeWithUnmergedClonePad(t *testing.T) {
	// 3 columns per clone group, 2 clone groups, 5pt pad between them, no merge: the total
	// span and the second group's start must match the plain, pre-merge-bleed formula
	// exactly: group*cellW*clones + (clones-1)*pad.
	n, group, pad, w, bleed := 6, 3, 5.0, 10.0, 4.0
	span := gridSpan(n, group, pad, w, bleed, false)
	want := 6*10.0 + 5.0
	if span != want {
		t.Errorf("span = %v, want %v", span, want)
	}
	starts := gridStarts(n, group, pad, w, bleed, false)
	if starts[3] != 3*w+pad {
		t.Errorf("second group start = %v, want %v", starts[3], 3*w+pad)
	}
}
