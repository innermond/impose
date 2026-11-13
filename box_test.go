package impose

import "testing"

func newBoxes(width, height, pw, ph float64, clonex, cloney int, padx, pady float64) *Boxes {
	return &Boxes{
		Big:       &BigBox{Box: &Box{Width: width, Height: height}},
		Small:     &SmallBox{Box: &Box{Width: pw, Height: ph}},
		Col:       1,
		Row:       1,
		CloneX:    clonex,
		CloneY:    cloney,
		ClonePadX: padx,
		ClonePadY: pady,
	}
}

func TestGuessGrid(t *testing.T) {
	cases := []struct {
		name     string
		bb       *Boxes
		margin   float64
		col, row int
	}{
		{"exact fit", newBoxes(300, 200, 100, 100, 1, 1, 0, 0), 0, 3, 2},
		{"remainder is dropped", newBoxes(350, 250, 100, 100, 1, 1, 0, 0), 0, 3, 2},
		{"margins", newBoxes(300, 200, 100, 100, 1, 1, 0, 0), 10, 2, 1},
		{"clones with pads", newBoxes(430, 100, 100, 100, 2, 1, 30, 0), 0, 2, 1},
		{"nothing fits", newBoxes(50, 50, 100, 100, 1, 1, 0, 0), 0, 0, 0},
	}
	for _, c := range cases {
		c.bb.Big.Left, c.bb.Big.Right, c.bb.Big.Top, c.bb.Big.Bottom = c.margin, c.margin, c.margin, c.margin
		col, row := c.bb.GuessGrid()
		if col != c.col || row != c.row {
			t.Errorf("%s: got %dx%d want %dx%d", c.name, col, row, c.col, c.row)
		}
	}
}

func TestEnoughCountsClones(t *testing.T) {
	// 2 clones of 1 page of 100 plus a pad of 10 need 210
	bb := newBoxes(200, 100, 100, 100, 2, 1, 10, 0)
	if bb.EnoughWidth() {
		t.Error("210 should not fit in 200")
	}
	bb.Big.Width = 210
	if !bb.EnoughWidth() {
		t.Error("210 should fit in 210")
	}
	bb = newBoxes(100, 150, 100, 100, 1, 2, 0, 0)
	if bb.EnoughHeight() {
		t.Error("two clones of 100 should not fit in 150")
	}
}

// MergeBleed only affects how a page is drawn, never the grid's size or fit: these
// functions must give the same answer whether it is on or off.
func TestEnoughAndGuessGridIgnoreMergeBleed(t *testing.T) {
	bb := newBoxes(300, 200, 100, 100, 1, 1, 0, 0)
	bb.Col, bb.Row = 3, 2
	wantW, wantH := bb.EnoughWidth(), bb.EnoughHeight()
	wantCol, wantRow := bb.GuessGrid()

	bb.Bleedx, bb.Bleedy, bb.MergeBleed = 5, 5, true
	if w := bb.EnoughWidth(); w != wantW {
		t.Errorf("EnoughWidth changed with MergeBleed: got %v want %v", w, wantW)
	}
	if h := bb.EnoughHeight(); h != wantH {
		t.Errorf("EnoughHeight changed with MergeBleed: got %v want %v", h, wantH)
	}
	if col, row := bb.GuessGrid(); col != wantCol || row != wantRow {
		t.Errorf("GuessGrid changed with MergeBleed: got %dx%d want %dx%d", col, row, wantCol, wantRow)
	}
}
